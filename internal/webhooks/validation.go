/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package webhooks

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	jiradcv1 "github.com/atlassian/jira-cdc/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// JiraCDCWebhook implements validation and defaulting webhooks for JiraCDC
type JiraCDCWebhook struct {
	client.Client
}

// SetupWithManager sets up the webhook with the Manager
func (w *JiraCDCWebhook) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&jiradcv1.JiraCDC{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-jiracdc-io-v1-jiracdc,mutating=true,failurePolicy=fail,sideEffects=None,groups=jiracdc.io,resources=jiracdc,verbs=create;update,versions=v1,name=mjiracdc.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &JiraCDCWebhook{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (w *JiraCDCWebhook) Default(ctx context.Context, obj runtime.Object) error {
	jiracdc, ok := obj.(*jiradcv1.JiraCDC)
	if !ok {
		return fmt.Errorf("expected JiraCDC object, got %T", obj)
	}

	log := ctrl.LoggerFrom(ctx)
	log.Info("Applying defaults to JiraCDC", "name", jiracdc.Name, "namespace", jiracdc.Namespace)

	// Set default values for JiraInstance
	if jiracdc.Spec.JiraInstance.MaxConcurrentRequests == 0 {
		jiracdc.Spec.JiraInstance.MaxConcurrentRequests = 5
	}
	
	if jiracdc.Spec.JiraInstance.RequestTimeout == "" {
		jiracdc.Spec.JiraInstance.RequestTimeout = "30s"
	}
	
	if jiracdc.Spec.JiraInstance.RateLimitRequests == 0 {
		jiracdc.Spec.JiraInstance.RateLimitRequests = 10
	}

	// Set default values for SyncTarget
	if jiracdc.Spec.SyncTarget.BatchSize == 0 {
		jiracdc.Spec.SyncTarget.BatchSize = 50
	}
	
	if jiracdc.Spec.SyncTarget.SyncInterval == "" {
		jiracdc.Spec.SyncTarget.SyncInterval = "5m"
	}
	
	if jiracdc.Spec.SyncTarget.RetryAttempts == 0 {
		jiracdc.Spec.SyncTarget.RetryAttempts = 3
	}

	// Set default values for GitRepository
	if jiracdc.Spec.GitRepository.Branch == "" {
		jiracdc.Spec.GitRepository.Branch = "main"
	}
	
	if jiracdc.Spec.GitRepository.CommitMessage == "" {
		jiracdc.Spec.GitRepository.CommitMessage = "Update JIRA issues from sync operation"
	}

	// Set default values for Operands
	if jiracdc.Spec.Operands.API.Replicas == 0 {
		jiracdc.Spec.Operands.API.Replicas = 1
	}
	
	if jiracdc.Spec.Operands.UI.Replicas == 0 {
		jiracdc.Spec.Operands.UI.Replicas = 1
	}
	
	if jiracdc.Spec.Operands.Jobs.BackoffLimit == 0 {
		jiracdc.Spec.Operands.Jobs.BackoffLimit = 3
	}

	// Set default resource requests and limits if not specified
	w.setDefaultResources(&jiracdc.Spec.Operands.API.Resources)
	w.setDefaultResources(&jiracdc.Spec.Operands.UI.Resources)

	// Initialize status if empty
	if jiracdc.Status.Phase == "" {
		jiracdc.Status.Phase = jiradcv1.PhaseInitializing
	}

	return nil
}

// setDefaultResources sets default resource requests and limits
func (w *JiraCDCWebhook) setDefaultResources(resources *jiradcv1.ResourceRequirements) {
	if resources.Requests == nil {
		resources.Requests = make(map[string]string)
	}
	if resources.Limits == nil {
		resources.Limits = make(map[string]string)
	}

	// Set default CPU requests/limits
	if _, exists := resources.Requests["cpu"]; !exists {
		resources.Requests["cpu"] = "100m"
	}
	if _, exists := resources.Limits["cpu"]; !exists {
		resources.Limits["cpu"] = "500m"
	}

	// Set default memory requests/limits
	if _, exists := resources.Requests["memory"]; !exists {
		resources.Requests["memory"] = "128Mi"
	}
	if _, exists := resources.Limits["memory"]; !exists {
		resources.Limits["memory"] = "512Mi"
	}
}

//+kubebuilder:webhook:path=/validate-jiracdc-io-v1-jiracdc,mutating=false,failurePolicy=fail,sideEffects=None,groups=jiracdc.io,resources=jiracdc,verbs=create;update,versions=v1,name=vjiracdc.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &JiraCDCWebhook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (w *JiraCDCWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	jiracdc, ok := obj.(*jiradcv1.JiraCDC)
	if !ok {
		return nil, fmt.Errorf("expected JiraCDC object, got %T", obj)
	}

	log := ctrl.LoggerFrom(ctx)
	log.Info("Validating JiraCDC creation", "name", jiracdc.Name, "namespace", jiracdc.Namespace)

	return w.validateJiraCDC(ctx, jiracdc, nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (w *JiraCDCWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newJiraCDC, ok := newObj.(*jiradcv1.JiraCDC)
	if !ok {
		return nil, fmt.Errorf("expected JiraCDC object, got %T", newObj)
	}

	oldJiraCDC, ok := oldObj.(*jiradcv1.JiraCDC)
	if !ok {
		return nil, fmt.Errorf("expected JiraCDC object, got %T", oldObj)
	}

	log := ctrl.LoggerFrom(ctx)
	log.Info("Validating JiraCDC update", "name", newJiraCDC.Name, "namespace", newJiraCDC.Namespace)

	return w.validateJiraCDC(ctx, newJiraCDC, oldJiraCDC)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (w *JiraCDCWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	jiracdc, ok := obj.(*jiradcv1.JiraCDC)
	if !ok {
		return nil, fmt.Errorf("expected JiraCDC object, got %T", obj)
	}

	log := ctrl.LoggerFrom(ctx)
	log.Info("Validating JiraCDC deletion", "name", jiracdc.Name, "namespace", jiracdc.Namespace)

	// No specific validation required for deletion
	return nil, nil
}

// validateJiraCDC performs comprehensive validation of a JiraCDC object
func (w *JiraCDCWebhook) validateJiraCDC(ctx context.Context, jiracdc *jiradcv1.JiraCDC, oldJiraCDC *jiradcv1.JiraCDC) (admission.Warnings, error) {
	var allErrs field.ErrorList
	var warnings admission.Warnings

	// Validate spec
	allErrs = append(allErrs, w.validateJiraInstance(&jiracdc.Spec.JiraInstance, field.NewPath("spec", "jiraInstance"))...)
	allErrs = append(allErrs, w.validateSyncTarget(&jiracdc.Spec.SyncTarget, field.NewPath("spec", "syncTarget"))...)
	allErrs = append(allErrs, w.validateGitRepository(&jiracdc.Spec.GitRepository, field.NewPath("spec", "gitRepository"))...)
	allErrs = append(allErrs, w.validateOperands(&jiracdc.Spec.Operands, field.NewPath("spec", "operands"))...)

	// Validate update-specific constraints
	if oldJiraCDC != nil {
		allErrs = append(allErrs, w.validateUpdate(jiracdc, oldJiraCDC)...)
		warnings = append(warnings, w.getUpdateWarnings(jiracdc, oldJiraCDC)...)
	}

	if len(allErrs) == 0 {
		return warnings, nil
	}

	return warnings, apierrors.NewInvalid(
		schema.GroupKind{Group: "jiracdc.io", Kind: "JiraCDC"},
		jiracdc.Name,
		allErrs,
	)
}

// validateJiraInstance validates JIRA instance configuration
func (w *JiraCDCWebhook) validateJiraInstance(jiraInstance *jiradcv1.JiraInstanceConfig, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate base URL
	if jiraInstance.BaseURL == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("baseURL"), "JIRA base URL is required"))
	} else {
		if _, err := url.Parse(jiraInstance.BaseURL); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("baseURL"), jiraInstance.BaseURL, "invalid URL format"))
		}
	}

	// Validate authentication configuration
	if jiraInstance.Authentication.SecretRef == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("authentication", "secretRef"), "authentication secret reference is required"))
	}

	// Validate rate limiting
	if jiraInstance.RateLimitRequests <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("rateLimitRequests"), jiraInstance.RateLimitRequests, "must be greater than 0"))
	}

	if jiraInstance.MaxConcurrentRequests <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxConcurrentRequests"), jiraInstance.MaxConcurrentRequests, "must be greater than 0"))
	}

	// Validate timeout
	if jiraInstance.RequestTimeout != "" {
		if _, err := time.ParseDuration(jiraInstance.RequestTimeout); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("requestTimeout"), jiraInstance.RequestTimeout, "invalid duration format"))
		}
	}

	return allErrs
}

// validateSyncTarget validates sync target configuration
func (w *JiraCDCWebhook) validateSyncTarget(syncTarget *jiradcv1.SyncTargetConfig, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate project keys
	if len(syncTarget.ProjectKeys) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("projectKeys"), "at least one project key is required"))
	}

	// Validate project key format
	projectKeyRegex := regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	for i, projectKey := range syncTarget.ProjectKeys {
		if !projectKeyRegex.MatchString(projectKey) {
			allErrs = append(allErrs, field.Invalid(
				fldPath.Child("projectKeys").Index(i), 
				projectKey, 
				"project key must start with uppercase letter and contain only uppercase letters, numbers, and underscores"))
		}
	}

	// Validate batch size
	if syncTarget.BatchSize <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("batchSize"), syncTarget.BatchSize, "must be greater than 0"))
	}
	if syncTarget.BatchSize > 1000 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("batchSize"), syncTarget.BatchSize, "must be less than or equal to 1000"))
	}

	// Validate sync interval
	if syncTarget.SyncInterval != "" {
		duration, err := time.ParseDuration(syncTarget.SyncInterval)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("syncInterval"), syncTarget.SyncInterval, "invalid duration format"))
		} else if duration < time.Minute {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("syncInterval"), syncTarget.SyncInterval, "sync interval must be at least 1 minute"))
		}
	}

	// Validate retry attempts
	if syncTarget.RetryAttempts < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("retryAttempts"), syncTarget.RetryAttempts, "must be non-negative"))
	}
	if syncTarget.RetryAttempts > 10 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("retryAttempts"), syncTarget.RetryAttempts, "must be less than or equal to 10"))
	}

	return allErrs
}

// validateGitRepository validates git repository configuration
func (w *JiraCDCWebhook) validateGitRepository(gitRepo *jiradcv1.GitRepositoryConfig, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate repository URL
	if gitRepo.URL == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("url"), "git repository URL is required"))
	} else {
		if _, err := url.Parse(gitRepo.URL); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("url"), gitRepo.URL, "invalid URL format"))
		}
	}

	// Validate branch name
	if gitRepo.Branch == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("branch"), "branch name is required"))
	} else {
		// Basic branch name validation
		branchRegex := regexp.MustCompile(`^[a-zA-Z0-9/_-]+$`)
		if !branchRegex.MatchString(gitRepo.Branch) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("branch"), gitRepo.Branch, "invalid branch name format"))
		}
	}

	// Validate authentication if specified
	if gitRepo.Authentication.SecretRef != "" {
		// Authentication type validation would go here
	}

	return allErrs
}

// validateOperands validates operand configuration
func (w *JiraCDCWebhook) validateOperands(operands *jiradcv1.OperandsConfig, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate API operand
	allErrs = append(allErrs, w.validateOperandConfig(&operands.API.OperandConfig, fldPath.Child("api"))...)
	
	// Validate UI operand
	allErrs = append(allErrs, w.validateOperandConfig(&operands.UI.OperandConfig, fldPath.Child("ui"))...)

	// Validate Jobs configuration
	if operands.Jobs.BackoffLimit < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("jobs", "backoffLimit"), operands.Jobs.BackoffLimit, "must be non-negative"))
	}

	return allErrs
}

// validateOperandConfig validates common operand configuration
func (w *JiraCDCWebhook) validateOperandConfig(operand *jiradcv1.OperandConfig, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate replicas
	if operand.Replicas <= 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), operand.Replicas, "must be greater than 0"))
	}
	if operand.Replicas > 10 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("replicas"), operand.Replicas, "must be less than or equal to 10"))
	}

	// Validate image if specified
	if operand.Image != "" {
		// Basic image format validation
		if !strings.Contains(operand.Image, ":") {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("image"), operand.Image, "image must include a tag"))
		}
	}

	// Validate resources
	allErrs = append(allErrs, w.validateResources(&operand.Resources, fldPath.Child("resources"))...)

	return allErrs
}

// validateResources validates resource requirements
func (w *JiraCDCWebhook) validateResources(resources *jiradcv1.ResourceRequirements, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	// Validate resource formats (this is a simplified validation)
	for resourceName, value := range resources.Requests {
		if value == "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("requests", resourceName), value, "resource value cannot be empty"))
		}
	}

	for resourceName, value := range resources.Limits {
		if value == "" {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("limits", resourceName), value, "resource value cannot be empty"))
		}
	}

	return allErrs
}

// validateUpdate validates update-specific constraints
func (w *JiraCDCWebhook) validateUpdate(new, old *jiradcv1.JiraCDC) field.ErrorList {
	var allErrs field.ErrorList

	// Prevent changes to immutable fields during update
	if new.Spec.JiraInstance.BaseURL != old.Spec.JiraInstance.BaseURL {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec", "jiraInstance", "baseURL"),
			"JIRA base URL cannot be changed after creation"))
	}

	if new.Spec.GitRepository.URL != old.Spec.GitRepository.URL {
		allErrs = append(allErrs, field.Forbidden(
			field.NewPath("spec", "gitRepository", "url"),
			"Git repository URL cannot be changed after creation"))
	}

	// Validate that project keys are not removed (only additions allowed)
	oldProjectKeys := make(map[string]bool)
	for _, key := range old.Spec.SyncTarget.ProjectKeys {
		oldProjectKeys[key] = true
	}

	for _, oldKey := range old.Spec.SyncTarget.ProjectKeys {
		found := false
		for _, newKey := range new.Spec.SyncTarget.ProjectKeys {
			if newKey == oldKey {
				found = true
				break
			}
		}
		if !found {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("spec", "syncTarget", "projectKeys"),
				fmt.Sprintf("project key %s cannot be removed", oldKey)))
		}
	}

	return allErrs
}

// getUpdateWarnings generates warnings for potentially problematic updates
func (w *JiraCDCWebhook) getUpdateWarnings(new, old *jiradcv1.JiraCDC) admission.Warnings {
	var warnings admission.Warnings

	// Warn about significant configuration changes
	if new.Spec.SyncTarget.BatchSize > old.Spec.SyncTarget.BatchSize*2 {
		warnings = append(warnings, "Significant increase in batch size may impact performance")
	}

	if new.Spec.JiraInstance.RateLimitRequests > old.Spec.JiraInstance.RateLimitRequests*2 {
		warnings = append(warnings, "Significant increase in rate limit may overwhelm JIRA instance")
	}

	// Warn about new project keys
	newProjectKeys := make(map[string]bool)
	for _, key := range new.Spec.SyncTarget.ProjectKeys {
		newProjectKeys[key] = true
	}

	for _, newKey := range new.Spec.SyncTarget.ProjectKeys {
		found := false
		for _, oldKey := range old.Spec.SyncTarget.ProjectKeys {
			if oldKey == newKey {
				found = true
				break
			}
		}
		if !found {
			warnings = append(warnings, fmt.Sprintf("Adding new project key %s will trigger full sync", newKey))
		}
	}

	return warnings
}
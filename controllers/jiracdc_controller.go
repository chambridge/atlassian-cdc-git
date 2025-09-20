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

package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
	"github.com/company/jira-cdc-operator/internal/operands"
)

// JiraCDCReconciler reconciles a JiraCDC object
type JiraCDCReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// OperandManager manages the lifecycle of operands
	OperandManager operands.Manager
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *JiraCDCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("jiracdc", req.NamespacedName)

	// Fetch the JiraCDC instance
	var jiracdc jiradcdv1.JiraCDC
	if err := r.Get(ctx, req.NamespacedName, &jiracdc); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			logger.Info("JiraCDC resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get JiraCDC")
		return ctrl.Result{}, err
	}

	// Update last reconcile time
	now := metav1.NewTime(time.Now())
	jiracdc.Status.LastReconcileTime = &now

	// Handle deletion
	if jiracdc.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, &jiracdc)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(&jiracdc, "jiracdc.io/finalizer") {
		controllerutil.AddFinalizer(&jiracdc, "jiracdc.io/finalizer")
		if err := r.Update(ctx, &jiracdc); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		logger.Info("Added finalizer to JiraCDC")
		return ctrl.Result{Requeue: true}, nil
	}

	// Validate the JiraCDC specification
	if err := r.validateSpec(&jiracdc); err != nil {
		logger.Error(err, "JiraCDC specification validation failed")
		r.updateStatusCondition(&jiracdc, "SpecValid", metav1.ConditionFalse, "ValidationFailed", err.Error())
		jiracdc.Status.Phase = "Error"
		if updateErr := r.Status().Update(ctx, &jiracdc); updateErr != nil {
			logger.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{}, err
	}

	r.updateStatusCondition(&jiracdc, "SpecValid", metav1.ConditionTrue, "ValidationPassed", "Specification is valid")

	// Check if credentials exist
	if err := r.validateCredentials(ctx, &jiracdc); err != nil {
		logger.Error(err, "Credential validation failed")
		r.updateStatusCondition(&jiracdc, "CredentialsReady", metav1.ConditionFalse, "CredentialsMissing", err.Error())
		jiracdc.Status.Phase = "Error"
		if updateErr := r.Status().Update(ctx, &jiracdc); updateErr != nil {
			logger.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	r.updateStatusCondition(&jiracdc, "CredentialsReady", metav1.ConditionTrue, "CredentialsFound", "All required credentials are available")

	// Set initial phase if not set
	if jiracdc.Status.Phase == "" {
		jiracdc.Status.Phase = "Pending"
	}

	// Reconcile operands
	if err := r.reconcileOperands(ctx, &jiracdc); err != nil {
		logger.Error(err, "Failed to reconcile operands")
		r.updateStatusCondition(&jiracdc, "OperandsReady", metav1.ConditionFalse, "OperandReconciliationFailed", err.Error())
		jiracdc.Status.Phase = "Error"
		if updateErr := r.Status().Update(ctx, &jiracdc); updateErr != nil {
			logger.Error(updateErr, "Failed to update status")
		}
		return ctrl.Result{}, err
	}

	r.updateStatusCondition(&jiracdc, "OperandsReady", metav1.ConditionTrue, "OperandsReconciled", "All operands are ready")

	// Handle sync configuration changes
	if err := r.handleSyncConfiguration(ctx, &jiracdc); err != nil {
		logger.Error(err, "Failed to handle sync configuration")
		return ctrl.Result{}, err
	}

	// Update final status
	if jiracdc.Status.Phase != "Error" {
		if r.areOperandsReady(&jiracdc) {
			jiracdc.Status.Phase = "Current"
		} else {
			jiracdc.Status.Phase = "Syncing"
		}
	}

	// Update component status
	r.updateComponentStatus(ctx, &jiracdc)

	// Update status
	if err := r.Status().Update(ctx, &jiracdc); err != nil {
		logger.Error(err, "Failed to update JiraCDC status")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled JiraCDC", "phase", jiracdc.Status.Phase)

	// Requeue for periodic reconciliation
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// handleDeletion handles the deletion of JiraCDC resources
func (r *JiraCDCReconciler) handleDeletion(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Cleanup operands
	if err := r.OperandManager.Cleanup(ctx, jiracdc); err != nil {
		logger.Error(err, "Failed to cleanup operands")
		return ctrl.Result{}, err
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(jiracdc, "jiracdc.io/finalizer")
	if err := r.Update(ctx, jiracdc); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully deleted JiraCDC")
	return ctrl.Result{}, nil
}

// validateSpec validates the JiraCDC specification
func (r *JiraCDCReconciler) validateSpec(jiracdc *jiradcdv1.JiraCDC) error {
	// Validate sync target configuration
	switch jiracdc.Spec.SyncTarget.Type {
	case "project":
		if jiracdc.Spec.SyncTarget.ProjectKey == "" {
			return fmt.Errorf("projectKey is required when syncTarget type is 'project'")
		}
	case "issues":
		if len(jiracdc.Spec.SyncTarget.IssueKeys) == 0 {
			return fmt.Errorf("issueKeys are required when syncTarget type is 'issues'")
		}
	case "jql":
		if jiracdc.Spec.SyncTarget.JQLQuery == "" {
			return fmt.Errorf("jqlQuery is required when syncTarget type is 'jql'")
		}
	default:
		return fmt.Errorf("syncTarget type must be one of: project, issues, jql")
	}

	// Validate JIRA instance configuration
	if jiracdc.Spec.JiraInstance.BaseURL == "" {
		return fmt.Errorf("jiraInstance.baseURL is required")
	}
	if jiracdc.Spec.JiraInstance.CredentialsSecret == "" {
		return fmt.Errorf("jiraInstance.credentialsSecret is required")
	}

	// Validate git repository configuration
	if jiracdc.Spec.GitRepository.URL == "" {
		return fmt.Errorf("gitRepository.url is required")
	}
	if jiracdc.Spec.GitRepository.CredentialsSecret == "" {
		return fmt.Errorf("gitRepository.credentialsSecret is required")
	}

	return nil
}

// validateCredentials checks if all required credentials exist
func (r *JiraCDCReconciler) validateCredentials(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Check JIRA credentials
	var jiraSecret corev1.Secret
	jiraSecretKey := types.NamespacedName{
		Name:      jiracdc.Spec.JiraInstance.CredentialsSecret,
		Namespace: jiracdc.Namespace,
	}
	if err := r.Get(ctx, jiraSecretKey, &jiraSecret); err != nil {
		return fmt.Errorf("JIRA credentials secret not found: %w", err)
	}

	// Validate JIRA secret has required fields
	if _, exists := jiraSecret.Data["username"]; !exists {
		return fmt.Errorf("JIRA credentials secret missing 'username' field")
	}
	if _, exists := jiraSecret.Data["token"]; !exists {
		return fmt.Errorf("JIRA credentials secret missing 'token' field")
	}

	// Check Git credentials
	var gitSecret corev1.Secret
	gitSecretKey := types.NamespacedName{
		Name:      jiracdc.Spec.GitRepository.CredentialsSecret,
		Namespace: jiracdc.Namespace,
	}
	if err := r.Get(ctx, gitSecretKey, &gitSecret); err != nil {
		return fmt.Errorf("Git credentials secret not found: %w", err)
	}

	// Validate Git secret has required fields (SSH or HTTPS)
	if _, sshExists := gitSecret.Data["ssh-privatekey"]; sshExists {
		// SSH authentication
		if _, exists := gitSecret.Data["known_hosts"]; !exists {
			return fmt.Errorf("Git SSH credentials secret missing 'known_hosts' field")
		}
	} else if _, userExists := gitSecret.Data["username"]; userExists {
		// HTTPS authentication
		if _, exists := gitSecret.Data["password"]; !exists {
			return fmt.Errorf("Git HTTPS credentials secret missing 'password' field")
		}
	} else {
		return fmt.Errorf("Git credentials secret must contain either SSH key ('ssh-privatekey') or HTTPS credentials ('username')")
	}

	// Check agent credentials if agent is enabled
	if jiracdc.Spec.AgentConfig != nil && jiracdc.Spec.AgentConfig.Enabled {
		var agentSecret corev1.Secret
		agentSecretKey := types.NamespacedName{
			Name:      jiracdc.Spec.AgentConfig.Submodule.CredentialsSecret,
			Namespace: jiracdc.Namespace,
		}
		if err := r.Get(ctx, agentSecretKey, &agentSecret); err != nil {
			return fmt.Errorf("Agent credentials secret not found: %w", err)
		}
	}

	return nil
}

// reconcileOperands manages the lifecycle of operands
func (r *JiraCDCReconciler) reconcileOperands(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	// Use the operand manager to reconcile all operands
	if err := r.OperandManager.Reconcile(ctx, jiracdc); err != nil {
		return fmt.Errorf("failed to reconcile operands: %w", err)
	}

	// Update operand status
	apiStatus, err := r.OperandManager.GetAPIStatus(ctx, jiracdc)
	if err != nil {
		return fmt.Errorf("failed to get API operand status: %w", err)
	}
	jiracdc.Status.OperandStatus.API = *apiStatus

	uiStatus, err := r.OperandManager.GetUIStatus(ctx, jiracdc)
	if err != nil {
		return fmt.Errorf("failed to get UI operand status: %w", err)
	}
	jiracdc.Status.OperandStatus.UI = *uiStatus

	return nil
}

// handleSyncConfiguration processes sync configuration changes
func (r *JiraCDCReconciler) handleSyncConfiguration(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) error {
	logger := log.FromContext(ctx)

	if jiracdc.Spec.SyncConfig == nil {
		return nil
	}

	// Handle bootstrap trigger
	if jiracdc.Spec.SyncConfig.TriggerBootstrap {
		logger.Info("Bootstrap triggered manually")
		// Reset the trigger flag
		jiracdc.Spec.SyncConfig.TriggerBootstrap = false
		if err := r.Update(ctx, jiracdc); err != nil {
			return fmt.Errorf("failed to reset bootstrap trigger: %w", err)
		}

		// Create bootstrap job via operand manager
		if err := r.OperandManager.CreateBootstrapJob(ctx, jiracdc); err != nil {
			return fmt.Errorf("failed to create bootstrap job: %w", err)
		}
	}

	// Handle reconciliation trigger
	if jiracdc.Spec.SyncConfig.TriggerReconciliation {
		logger.Info("Reconciliation triggered manually")
		// Reset the trigger flag
		jiracdc.Spec.SyncConfig.TriggerReconciliation = false
		if err := r.Update(ctx, jiracdc); err != nil {
			return fmt.Errorf("failed to reset reconciliation trigger: %w", err)
		}

		// Create reconciliation job via operand manager
		if err := r.OperandManager.CreateReconciliationJob(ctx, jiracdc); err != nil {
			return fmt.Errorf("failed to create reconciliation job: %w", err)
		}
	}

	// Handle task cancellation
	if jiracdc.Spec.SyncConfig.CancelCurrentTask {
		logger.Info("Task cancellation requested")
		// Reset the trigger flag
		jiracdc.Spec.SyncConfig.CancelCurrentTask = false
		if err := r.Update(ctx, jiracdc); err != nil {
			return fmt.Errorf("failed to reset cancellation trigger: %w", err)
		}

		// Cancel current task via operand manager
		if err := r.OperandManager.CancelCurrentTask(ctx, jiracdc); err != nil {
			return fmt.Errorf("failed to cancel current task: %w", err)
		}
	}

	return nil
}

// areOperandsReady checks if all enabled operands are ready
func (r *JiraCDCReconciler) areOperandsReady(jiracdc *jiradcdv1.JiraCDC) bool {
	// Check if API operand is ready (if enabled)
	if r.isAPIEnabled(jiracdc) && !jiracdc.Status.OperandStatus.API.Ready {
		return false
	}

	// Check if UI operand is ready (if enabled)
	if r.isUIEnabled(jiracdc) && !jiracdc.Status.OperandStatus.UI.Ready {
		return false
	}

	return true
}

// isAPIEnabled checks if the API operand is enabled
func (r *JiraCDCReconciler) isAPIEnabled(jiracdc *jiradcdv1.JiraCDC) bool {
	if jiracdc.Spec.Operands == nil || jiracdc.Spec.Operands.API == nil {
		return true // Default is enabled
	}
	if jiracdc.Spec.Operands.API.Enabled == nil {
		return true // Default is enabled
	}
	return *jiracdc.Spec.Operands.API.Enabled
}

// isUIEnabled checks if the UI operand is enabled
func (r *JiraCDCReconciler) isUIEnabled(jiracdc *jiradcdv1.JiraCDC) bool {
	if jiracdc.Spec.Operands == nil || jiracdc.Spec.Operands.UI == nil {
		return true // Default is enabled
	}
	if jiracdc.Spec.Operands.UI.Enabled == nil {
		return true // Default is enabled
	}
	return *jiracdc.Spec.Operands.UI.Enabled
}

// updateComponentStatus updates the component health status
func (r *JiraCDCReconciler) updateComponentStatus(ctx context.Context, jiracdc *jiradcdv1.JiraCDC) {
	// Initialize with unknown status
	jiracdc.Status.ComponentStatus = jiradcdv1.ComponentStatus{
		JiraConnection: "unhealthy",
		GitRepository:  "unhealthy",
		Kubernetes:     "healthy", // Assume K8s is healthy if we can reconcile
	}

	// Component health will be updated by the operands and sync engines
	// This is a placeholder for the initial implementation
}

// updateStatusCondition updates or adds a status condition
func (r *JiraCDCReconciler) updateStatusCondition(jiracdc *jiradcdv1.JiraCDC, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: jiracdc.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
		Reason:             reason,
		Message:            message,
	}

	// Find existing condition and update, or append new one
	for i, existingCondition := range jiracdc.Status.Conditions {
		if existingCondition.Type == conditionType {
			if existingCondition.Status != status {
				condition.LastTransitionTime = metav1.NewTime(time.Now())
			} else {
				condition.LastTransitionTime = existingCondition.LastTransitionTime
			}
			jiracdc.Status.Conditions[i] = condition
			return
		}
	}

	// Condition not found, append it
	jiracdc.Status.Conditions = append(jiracdc.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *JiraCDCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&jiradcdv1.JiraCDC{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
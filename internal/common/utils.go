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

package common

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SecretGetter provides a common interface for retrieving Kubernetes secrets
type SecretGetter interface {
	GetSecret(ctx context.Context, secretKey types.NamespacedName) (*corev1.Secret, error)
}

// K8sSecretGetter implements SecretGetter using Kubernetes client
type K8sSecretGetter struct {
	Client client.Client
}

// GetSecret retrieves a Kubernetes secret
func (g *K8sSecretGetter) GetSecret(ctx context.Context, secretKey types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := g.Client.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", secretKey, err)
	}
	return secret, nil
}

// ResourceReconciler provides common functionality for reconciling Kubernetes resources
type ResourceReconciler struct {
	Client client.Client
}

// CreateOrUpdateResource provides a common pattern for CreateOrUpdate operations
func (r *ResourceReconciler) CreateOrUpdateResource(
	ctx context.Context,
	obj client.Object,
	mutateFn func() error,
) (controllerutil.OperationResult, error) {
	logger := log.FromContext(ctx)
	
	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, mutateFn)
	if err != nil {
		logger.Error(err, "Failed to create or update resource",
			"kind", obj.GetObjectKind(),
			"name", obj.GetName(),
			"namespace", obj.GetNamespace())
		return result, fmt.Errorf("failed to create or update %s %s/%s: %w",
			obj.GetObjectKind(), obj.GetNamespace(), obj.GetName(), err)
	}
	
	if result != controllerutil.OperationResultNone {
		logger.Info("Resource operation completed",
			"kind", obj.GetObjectKind(),
			"name", obj.GetName(),
			"namespace", obj.GetNamespace(),
			"operation", result)
	}
	
	return result, nil
}

// ValidatorFunc represents a validation function
type ValidatorFunc func() error

// ConfigValidator provides common configuration validation patterns
type ConfigValidator struct {
	validators []ValidatorFunc
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{
		validators: make([]ValidatorFunc, 0),
	}
}

// AddValidator adds a validation function
func (v *ConfigValidator) AddValidator(validator ValidatorFunc) *ConfigValidator {
	v.validators = append(v.validators, validator)
	return v
}

// RequireNonEmpty validates that a string field is not empty
func (v *ConfigValidator) RequireNonEmpty(fieldName, value string) *ConfigValidator {
	return v.AddValidator(func() error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required and cannot be empty", fieldName)
		}
		return nil
	})
}

// RequirePositive validates that a numeric field is positive
func (v *ConfigValidator) RequirePositive(fieldName string, value interface{}) *ConfigValidator {
	return v.AddValidator(func() error {
		val := reflect.ValueOf(value)
		switch val.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if val.Int() <= 0 {
				return fmt.Errorf("%s must be positive, got %d", fieldName, val.Int())
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if val.Uint() == 0 {
				return fmt.Errorf("%s must be positive, got %d", fieldName, val.Uint())
			}
		case reflect.Float32, reflect.Float64:
			if val.Float() <= 0 {
				return fmt.Errorf("%s must be positive, got %f", fieldName, val.Float())
			}
		default:
			return fmt.Errorf("unsupported type for positive validation: %T", value)
		}
		return nil
	})
}

// RequireInRange validates that a numeric value is within a specified range
func (v *ConfigValidator) RequireInRange(fieldName string, value, min, max interface{}) *ConfigValidator {
	return v.AddValidator(func() error {
		val := reflect.ValueOf(value)
		minVal := reflect.ValueOf(min)
		maxVal := reflect.ValueOf(max)
		
		if val.Kind() != minVal.Kind() || val.Kind() != maxVal.Kind() {
			return fmt.Errorf("type mismatch for range validation of %s", fieldName)
		}
		
		switch val.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v := val.Int()
			minV := minVal.Int()
			maxV := maxVal.Int()
			if v < minV || v > maxV {
				return fmt.Errorf("%s must be between %d and %d, got %d", fieldName, minV, maxV, v)
			}
		case reflect.Float32, reflect.Float64:
			v := val.Float()
			minV := minVal.Float()
			maxV := maxVal.Float()
			if v < minV || v > maxV {
				return fmt.Errorf("%s must be between %f and %f, got %f", fieldName, minV, maxV, v)
			}
		default:
			return fmt.Errorf("unsupported type for range validation: %T", value)
		}
		return nil
	})
}

// Validate runs all registered validators
func (v *ConfigValidator) Validate() error {
	var errors []string
	
	for _, validator := range v.validators {
		if err := validator(); err != nil {
			errors = append(errors, err.Error())
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("validation failed: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// ErrorHandler provides common error handling patterns
type ErrorHandler struct {
	logger interface{ Error(error, string, ...interface{}) }
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger interface{ Error(error, string, ...interface{}) }) *ErrorHandler {
	return &ErrorHandler{logger: logger}
}

// HandleError logs and wraps errors with additional context
func (h *ErrorHandler) HandleError(err error, message string, keysAndValues ...interface{}) error {
	if err == nil {
		return nil
	}
	
	h.logger.Error(err, message, keysAndValues...)
	return fmt.Errorf("%s: %w", message, err)
}

// HandleMultipleErrors aggregates multiple errors into a single error
func (h *ErrorHandler) HandleMultipleErrors(errors []error, operation string) error {
	if len(errors) == 0 {
		return nil
	}
	
	if len(errors) == 1 {
		return h.HandleError(errors[0], operation)
	}
	
	var errorMessages []string
	for _, err := range errors {
		errorMessages = append(errorMessages, err.Error())
	}
	
	aggregatedError := fmt.Errorf("%s failed: %s", operation, strings.Join(errorMessages, "; "))
	h.logger.Error(aggregatedError, operation, "errorCount", len(errors))
	
	return aggregatedError
}

// RetryableError indicates whether an error should be retried
type RetryableError interface {
	error
	IsRetryable() bool
}

// retryableError implements RetryableError
type retryableError struct {
	err       error
	retryable bool
}

func (e *retryableError) Error() string {
	return e.err.Error()
}

func (e *retryableError) IsRetryable() bool {
	return e.retryable
}

func (e *retryableError) Unwrap() error {
	return e.err
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error) RetryableError {
	return &retryableError{err: err, retryable: true}
}

// NewNonRetryableError creates a new non-retryable error
func NewNonRetryableError(err error) RetryableError {
	return &retryableError{err: err, retryable: false}
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if retryableErr, ok := err.(RetryableError); ok {
		return retryableErr.IsRetryable()
	}
	return false
}

// ReconcileResult provides common reconciliation result patterns
type ReconcileResult struct {
	Requeue      bool
	RequeueAfter time.Duration
	Error        error
}

// NewSuccessResult creates a successful reconciliation result
func NewSuccessResult() ReconcileResult {
	return ReconcileResult{
		Requeue:      false,
		RequeueAfter: 0,
		Error:        nil,
	}
}

// NewRequeueResult creates a result that triggers immediate requeue
func NewRequeueResult() ReconcileResult {
	return ReconcileResult{
		Requeue:      true,
		RequeueAfter: 0,
		Error:        nil,
	}
}

// NewRequeueAfterResult creates a result that requeues after specified duration
func NewRequeueAfterResult(duration time.Duration) ReconcileResult {
	return ReconcileResult{
		Requeue:      false,
		RequeueAfter: duration,
		Error:        nil,
	}
}

// NewErrorResult creates a result with an error (triggers requeue with backoff)
func NewErrorResult(err error) ReconcileResult {
	return ReconcileResult{
		Requeue:      false,
		RequeueAfter: 0,
		Error:        err,
	}
}

// ToControllerResult converts to controller-runtime Result
func (r ReconcileResult) ToControllerResult() (ctrl.Result, error) {
	return ctrl.Result{
		Requeue:      r.Requeue,
		RequeueAfter: r.RequeueAfter,
	}, r.Error
}

// LabelSetter provides common labeling patterns
type LabelSetter struct {
	labels map[string]string
}

// NewLabelSetter creates a new label setter
func NewLabelSetter() *LabelSetter {
	return &LabelSetter{
		labels: make(map[string]string),
	}
}

// SetStandardLabels sets standard operand labels
func (l *LabelSetter) SetStandardLabels(appName, instanceName, component string) *LabelSetter {
	l.labels["app"] = appName
	l.labels["jiracdc.io/instance"] = instanceName
	l.labels["jiracdc.io/component"] = component
	return l
}

// SetManagedBy sets the managed-by label
func (l *LabelSetter) SetManagedBy(managedBy string) *LabelSetter {
	l.labels["app.kubernetes.io/managed-by"] = managedBy
	return l
}

// SetVersion sets the version label
func (l *LabelSetter) SetVersion(version string) *LabelSetter {
	l.labels["app.kubernetes.io/version"] = version
	return l
}

// SetCustomLabel sets a custom label
func (l *LabelSetter) SetCustomLabel(key, value string) *LabelSetter {
	l.labels[key] = value
	return l
}

// ApplyToObject applies labels to a Kubernetes object
func (l *LabelSetter) ApplyToObject(obj client.Object) {
	if obj.GetLabels() == nil {
		obj.SetLabels(make(map[string]string))
	}
	
	labels := obj.GetLabels()
	for key, value := range l.labels {
		labels[key] = value
	}
	obj.SetLabels(labels)
}

// GetLabels returns the current labels map
func (l *LabelSetter) GetLabels() map[string]string {
	result := make(map[string]string)
	for k, v := range l.labels {
		result[k] = v
	}
	return result
}
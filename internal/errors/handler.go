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

package errors

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"syscall"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ErrorType represents different categories of errors
type ErrorType string

const (
	ErrorTypeTransient    ErrorType = "transient"    // Temporary, can be retried
	ErrorTypePermanent    ErrorType = "permanent"    // Permanent, should not be retried
	ErrorTypeAuthentication ErrorType = "authentication" // Authentication/authorization errors
	ErrorTypeRateLimit    ErrorType = "rate_limit"   // Rate limiting errors
	ErrorTypeValidation   ErrorType = "validation"   // Input validation errors
	ErrorTypeNetwork      ErrorType = "network"      // Network connectivity errors
	ErrorTypeTimeout      ErrorType = "timeout"      // Timeout errors
	ErrorTypeNotFound     ErrorType = "not_found"    // Resource not found errors
	ErrorTypeConflict     ErrorType = "conflict"     // Resource conflict errors
	ErrorTypeInternal     ErrorType = "internal"     // Internal system errors
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// ClassifiedError represents an error with additional classification metadata
type ClassifiedError struct {
	Err         error
	Type        ErrorType
	Severity    ErrorSeverity
	Retryable   bool
	Component   string
	Operation   string
	Context     map[string]interface{}
	Timestamp   time.Time
}

// Error implements the error interface
func (e *ClassifiedError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the underlying error
func (e *ClassifiedError) Unwrap() error {
	return e.Err
}

// ErrorClassifier classifies errors into categories
type ErrorClassifier interface {
	// Classify classifies an error and returns a ClassifiedError
	Classify(ctx context.Context, err error, component, operation string) *ClassifiedError
	
	// IsRetryable determines if an error should be retried
	IsRetryable(err error) bool
	
	// GetRetryDelay calculates the appropriate delay before retrying
	GetRetryDelay(attempt int, err error) time.Duration
}

// RetryConfig contains retry configuration
type RetryConfig struct {
	MaxAttempts     int           `json:"maxAttempts"`
	InitialDelay    time.Duration `json:"initialDelay"`
	MaxDelay        time.Duration `json:"maxDelay"`
	BackoffFactor   float64       `json:"backoffFactor"`
	Jitter          bool          `json:"jitter"`
	RetryableErrors []ErrorType   `json:"retryableErrors"`
}

// errorClassifier implements ErrorClassifier
type errorClassifier struct {
	retryConfig RetryConfig
}

// NewErrorClassifier creates a new error classifier
func NewErrorClassifier(config RetryConfig) ErrorClassifier {
	return &errorClassifier{
		retryConfig: config,
	}
}

// Classify classifies an error and returns a ClassifiedError
func (c *errorClassifier) Classify(ctx context.Context, err error, component, operation string) *ClassifiedError {
	if err == nil {
		return nil
	}
	
	// Check if it's already a classified error
	if classified, ok := err.(*ClassifiedError); ok {
		return classified
	}
	
	errorType, severity := c.classifyError(err)
	
	classified := &ClassifiedError{
		Err:       err,
		Type:      errorType,
		Severity:  severity,
		Retryable: c.isRetryableType(errorType),
		Component: component,
		Operation: operation,
		Context:   extractErrorContext(ctx),
		Timestamp: time.Now(),
	}
	
	return classified
}

// IsRetryable determines if an error should be retried
func (c *errorClassifier) IsRetryable(err error) bool {
	classified := c.Classify(context.Background(), err, "", "")
	return classified.Retryable
}

// GetRetryDelay calculates the appropriate delay before retrying
func (c *errorClassifier) GetRetryDelay(attempt int, err error) time.Duration {
	if attempt <= 0 {
		return c.retryConfig.InitialDelay
	}
	
	// Exponential backoff
	delay := c.retryConfig.InitialDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * c.retryConfig.BackoffFactor)
		if delay > c.retryConfig.MaxDelay {
			delay = c.retryConfig.MaxDelay
			break
		}
	}
	
	// Add jitter if enabled
	if c.retryConfig.Jitter {
		jitter := time.Duration(float64(delay) * (0.1 * (2.0*float64(time.Now().UnixNano()%1000)/1000.0 - 1.0)))
		delay += jitter
	}
	
	return delay
}

// classifyError classifies an error into type and severity
func (c *errorClassifier) classifyError(err error) (ErrorType, ErrorSeverity) {
	errStr := strings.ToLower(err.Error())
	
	// Check for specific error types
	
	// Network errors
	if isNetworkError(err) {
		return ErrorTypeNetwork, SeverityMedium
	}
	
	// Timeout errors
	if isTimeoutError(err) {
		return ErrorTypeTimeout, SeverityMedium
	}
	
	// HTTP errors
	if httpErr, ok := err.(*HTTPError); ok {
		return c.classifyHTTPError(httpErr)
	}
	
	// Authentication errors
	if strings.Contains(errStr, "unauthorized") || 
	   strings.Contains(errStr, "authentication") || 
	   strings.Contains(errStr, "invalid credentials") ||
	   strings.Contains(errStr, "forbidden") {
		return ErrorTypeAuthentication, SeverityHigh
	}
	
	// Rate limit errors
	if strings.Contains(errStr, "rate limit") || 
	   strings.Contains(errStr, "too many requests") ||
	   strings.Contains(errStr, "quota exceeded") {
		return ErrorTypeRateLimit, SeverityMedium
	}
	
	// Validation errors
	if strings.Contains(errStr, "validation") || 
	   strings.Contains(errStr, "invalid") ||
	   strings.Contains(errStr, "bad request") {
		return ErrorTypeValidation, SeverityLow
	}
	
	// Not found errors
	if strings.Contains(errStr, "not found") || 
	   strings.Contains(errStr, "does not exist") {
		return ErrorTypeNotFound, SeverityMedium
	}
	
	// Conflict errors
	if strings.Contains(errStr, "conflict") || 
	   strings.Contains(errStr, "already exists") {
		return ErrorTypeConflict, SeverityMedium
	}
	
	// Connection errors (transient)
	if strings.Contains(errStr, "connection") {
		return ErrorTypeTransient, SeverityMedium
	}
	
	// Default to internal error
	return ErrorTypeInternal, SeverityHigh
}

// classifyHTTPError classifies HTTP errors
func (c *errorClassifier) classifyHTTPError(httpErr *HTTPError) (ErrorType, ErrorSeverity) {
	switch httpErr.StatusCode {
	case http.StatusBadRequest:
		return ErrorTypeValidation, SeverityLow
	case http.StatusUnauthorized:
		return ErrorTypeAuthentication, SeverityHigh
	case http.StatusForbidden:
		return ErrorTypeAuthentication, SeverityHigh
	case http.StatusNotFound:
		return ErrorTypeNotFound, SeverityMedium
	case http.StatusConflict:
		return ErrorTypeConflict, SeverityMedium
	case http.StatusTooManyRequests:
		return ErrorTypeRateLimit, SeverityMedium
	case http.StatusInternalServerError:
		return ErrorTypeTransient, SeverityHigh
	case http.StatusBadGateway:
		return ErrorTypeTransient, SeverityMedium
	case http.StatusServiceUnavailable:
		return ErrorTypeTransient, SeverityMedium
	case http.StatusGatewayTimeout:
		return ErrorTypeTimeout, SeverityMedium
	default:
		if httpErr.StatusCode >= 500 {
			return ErrorTypeTransient, SeverityHigh
		}
		return ErrorTypePermanent, SeverityMedium
	}
}

// isRetryableType checks if an error type is retryable
func (c *errorClassifier) isRetryableType(errorType ErrorType) bool {
	retryableTypes := c.retryConfig.RetryableErrors
	if len(retryableTypes) == 0 {
		// Default retryable types
		retryableTypes = []ErrorType{
			ErrorTypeTransient,
			ErrorTypeNetwork,
			ErrorTypeTimeout,
			ErrorTypeRateLimit,
		}
	}
	
	for _, retryableType := range retryableTypes {
		if errorType == retryableType {
			return true
		}
	}
	
	return false
}

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Message    string
	Response   *http.Response
}

// Error implements the error interface
func (e *HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

// NewHTTPError creates a new HTTP error
func NewHTTPError(statusCode int, message string, resp *http.Response) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Message:    message,
		Response:   resp,
	}
}

// isNetworkError checks if an error is a network error
func isNetworkError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	
	// Check for common network error patterns
	var syscallErr *net.OpError
	if errors.As(err, &syscallErr) {
		return true
	}
	
	// Check for connection refused, host unreachable, etc.
	if errors.Is(err, syscall.ECONNREFUSED) ||
	   errors.Is(err, syscall.EHOSTUNREACH) ||
	   errors.Is(err, syscall.ENETUNREACH) {
		return true
	}
	
	return false
}

// isTimeoutError checks if an error is a timeout error
func isTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		   strings.Contains(errStr, "deadline exceeded")
}

// RetryableFunction represents a function that can be retried
type RetryableFunction func() error

// RetryManager handles retry logic for operations
type RetryManager struct {
	classifier ErrorClassifier
	config     RetryConfig
}

// NewRetryManager creates a new retry manager
func NewRetryManager(config RetryConfig) *RetryManager {
	return &RetryManager{
		classifier: NewErrorClassifier(config),
		config:     config,
	}
}

// Retry executes a function with retry logic
func (r *RetryManager) Retry(ctx context.Context, component, operation string, fn RetryableFunction) error {
	logger := log.FromContext(ctx).WithValues("component", component, "operation", operation)
	
	var lastErr error
	
	for attempt := 0; attempt < r.config.MaxAttempts; attempt++ {
		// Execute function
		err := fn()
		if err == nil {
			// Success
			if attempt > 0 {
				logger.Info("Operation succeeded after retries", "attempt", attempt+1)
			}
			return nil
		}
		
		// Classify error
		classified := r.classifier.Classify(ctx, err, component, operation)
		lastErr = classified
		
		// Log error
		logger.V(1).Info("Operation failed", 
			"attempt", attempt+1,
			"error", err,
			"errorType", classified.Type,
			"severity", classified.Severity,
			"retryable", classified.Retryable)
		
		// Check if we should retry
		if !classified.Retryable {
			logger.Info("Error is not retryable, giving up", "errorType", classified.Type)
			return classified
		}
		
		// Don't delay after last attempt
		if attempt == r.config.MaxAttempts-1 {
			break
		}
		
		// Calculate delay
		delay := r.classifier.GetRetryDelay(attempt, classified)
		
		logger.Info("Retrying operation after delay", 
			"attempt", attempt+1,
			"delay", delay,
			"maxAttempts", r.config.MaxAttempts)
		
		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}
	
	logger.Error(lastErr, "Operation failed after all retries", "attempts", r.config.MaxAttempts)
	return fmt.Errorf("operation failed after %d attempts: %w", r.config.MaxAttempts, lastErr)
}

// RetryWithBackoff is a convenience function for retrying with exponential backoff
func RetryWithBackoff(ctx context.Context, maxAttempts int, initialDelay time.Duration, fn RetryableFunction) error {
	config := RetryConfig{
		MaxAttempts:   maxAttempts,
		InitialDelay:  initialDelay,
		MaxDelay:      5 * time.Minute,
		BackoffFactor: 2.0,
		Jitter:        true,
	}
	
	manager := NewRetryManager(config)
	return manager.Retry(ctx, "unknown", "retry", fn)
}

// extractErrorContext extracts relevant context information for error reporting
func extractErrorContext(ctx context.Context) map[string]interface{} {
	context := make(map[string]interface{})
	
	// Extract common context values
	if requestID := ctx.Value("request_id"); requestID != nil {
		context["request_id"] = requestID
	}
	
	if correlationID := ctx.Value("correlation_id"); correlationID != nil {
		context["correlation_id"] = correlationID
	}
	
	if operation := ctx.Value("operation"); operation != nil {
		context["operation"] = operation
	}
	
	if component := ctx.Value("component"); component != nil {
		context["component"] = component
	}
	
	if projectKey := ctx.Value("project"); projectKey != nil {
		context["project"] = projectKey
	}
	
	return context
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
		RetryableErrors: []ErrorType{
			ErrorTypeTransient,
			ErrorTypeNetwork,
			ErrorTypeTimeout,
			ErrorTypeRateLimit,
		},
	}
}

// WrapHTTPError wraps an HTTP response in an error if the status indicates failure
func WrapHTTPError(resp *http.Response, message string) error {
	if resp.StatusCode >= 400 {
		if message == "" {
			message = resp.Status
		}
		return NewHTTPError(resp.StatusCode, message, resp)
	}
	return nil
}

// IsRetryableError checks if an error is retryable using default classification
func IsRetryableError(err error) bool {
	classifier := NewErrorClassifier(DefaultRetryConfig())
	return classifier.IsRetryable(err)
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"
	StateOpen     CircuitBreakerState = "open" 
	StateHalfOpen CircuitBreakerState = "half_open"
)

// CircuitBreaker implements the circuit breaker pattern for error handling
type CircuitBreaker struct {
	name           string
	maxFailures    int
	resetTimeout   time.Duration
	state          CircuitBreakerState
	failureCount   int
	lastFailureTime time.Time
	mutex          sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
	}
}

// Execute executes a function through the circuit breaker
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mutex.RLock()
	state := cb.state
	failureCount := cb.failureCount
	lastFailureTime := cb.lastFailureTime
	cb.mutex.RUnlock()
	
	// Check if we should transition from open to half-open
	if state == StateOpen && time.Since(lastFailureTime) > cb.resetTimeout {
		cb.mutex.Lock()
		if cb.state == StateOpen {
			cb.state = StateHalfOpen
		}
		cb.mutex.Unlock()
		state = StateHalfOpen
	}
	
	// Reject if circuit is open
	if state == StateOpen {
		return fmt.Errorf("circuit breaker %s is open", cb.name)
	}
	
	// Execute function
	err := fn()
	
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()
		
		// Open circuit if max failures reached
		if cb.failureCount >= cb.maxFailures {
			cb.state = StateOpen
		}
		
		return err
	}
	
	// Success - reset circuit breaker
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
	}
	cb.failureCount = 0
	
	return nil
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// Reset manually resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.state = StateClosed
	cb.failureCount = 0
	cb.lastFailureTime = time.Time{}
}
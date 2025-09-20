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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidator(t *testing.T) {
	tests := []struct {
		name           string
		setupValidator func() *ConfigValidator
		expectError    bool
		errorContains  string
	}{
		{
			name: "empty string validation fails",
			setupValidator: func() *ConfigValidator {
				return NewConfigValidator().RequireNonEmpty("testField", "")
			},
			expectError:   true,
			errorContains: "testField is required and cannot be empty",
		},
		{
			name: "non-empty string validation passes",
			setupValidator: func() *ConfigValidator {
				return NewConfigValidator().RequireNonEmpty("testField", "value")
			},
			expectError: false,
		},
		{
			name: "positive integer validation passes",
			setupValidator: func() *ConfigValidator {
				return NewConfigValidator().RequirePositive("testField", 5)
			},
			expectError: false,
		},
		{
			name: "zero integer validation fails",
			setupValidator: func() *ConfigValidator {
				return NewConfigValidator().RequirePositive("testField", 0)
			},
			expectError:   true,
			errorContains: "testField must be positive, got 0",
		},
		{
			name: "negative integer validation fails",
			setupValidator: func() *ConfigValidator {
				return NewConfigValidator().RequirePositive("testField", -1)
			},
			expectError:   true,
			errorContains: "testField must be positive, got -1",
		},
		{
			name: "range validation passes",
			setupValidator: func() *ConfigValidator {
				return NewConfigValidator().RequireInRange("testField", 5, 1, 10)
			},
			expectError: false,
		},
		{
			name: "range validation fails below minimum",
			setupValidator: func() *ConfigValidator {
				return NewConfigValidator().RequireInRange("testField", 0, 1, 10)
			},
			expectError:   true,
			errorContains: "testField must be between 1 and 10, got 0",
		},
		{
			name: "range validation fails above maximum",
			setupValidator: func() *ConfigValidator {
				return NewConfigValidator().RequireInRange("testField", 15, 1, 10)
			},
			expectError:   true,
			errorContains: "testField must be between 1 and 10, got 15",
		},
		{
			name: "multiple validations pass",
			setupValidator: func() *ConfigValidator {
				return NewConfigValidator().
					RequireNonEmpty("field1", "value").
					RequirePositive("field2", 5).
					RequireInRange("field3", 7, 1, 10)
			},
			expectError: false,
		},
		{
			name: "multiple validations with one failure",
			setupValidator: func() *ConfigValidator {
				return NewConfigValidator().
					RequireNonEmpty("field1", "value").
					RequirePositive("field2", 0).
					RequireInRange("field3", 7, 1, 10)
			},
			expectError:   true,
			errorContains: "field2 must be positive, got 0",
		},
		{
			name: "multiple validations with multiple failures",
			setupValidator: func() *ConfigValidator {
				return NewConfigValidator().
					RequireNonEmpty("field1", "").
					RequirePositive("field2", 0)
			},
			expectError:   true,
			errorContains: "field1 is required and cannot be empty; field2 must be positive, got 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := tt.setupValidator()
			err := validator.Validate()

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRetryableError(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		expectRetry   bool
		expectedError string
	}{
		{
			name:          "retryable error",
			err:           NewRetryableError(assert.AnError),
			expectRetry:   true,
			expectedError: assert.AnError.Error(),
		},
		{
			name:          "non-retryable error",
			err:           NewNonRetryableError(assert.AnError),
			expectRetry:   false,
			expectedError: assert.AnError.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectRetry, IsRetryable(tt.err))
			assert.Equal(t, tt.expectedError, tt.err.Error())
			
			if retryableErr, ok := tt.err.(RetryableError); ok {
				assert.Equal(t, tt.expectRetry, retryableErr.IsRetryable())
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectRetry bool
	}{
		{
			name:        "retryable error returns true",
			err:         NewRetryableError(assert.AnError),
			expectRetry: true,
		},
		{
			name:        "non-retryable error returns false",
			err:         NewNonRetryableError(assert.AnError),
			expectRetry: false,
		},
		{
			name:        "regular error returns false",
			err:         assert.AnError,
			expectRetry: false,
		},
		{
			name:        "nil error returns false",
			err:         nil,
			expectRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectRetry, IsRetryable(tt.err))
		})
	}
}

type mockLogger struct {
	lastError   error
	lastMessage string
	lastArgs    []interface{}
}

func (m *mockLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	m.lastError = err
	m.lastMessage = msg
	m.lastArgs = keysAndValues
}

func TestErrorHandler(t *testing.T) {
	tests := []struct {
		name            string
		inputError      error
		message         string
		expectError     bool
		expectLogCalled bool
	}{
		{
			name:            "nil error returns nil",
			inputError:      nil,
			message:         "test operation",
			expectError:     false,
			expectLogCalled: false,
		},
		{
			name:            "error is logged and wrapped",
			inputError:      assert.AnError,
			message:         "test operation",
			expectError:     true,
			expectLogCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			handler := NewErrorHandler(logger)

			result := handler.HandleError(tt.inputError, tt.message)

			if tt.expectError {
				require.Error(t, result)
				assert.Contains(t, result.Error(), tt.message)
			} else {
				assert.NoError(t, result)
			}

			if tt.expectLogCalled {
				assert.Equal(t, tt.inputError, logger.lastError)
				assert.Equal(t, tt.message, logger.lastMessage)
			}
		})
	}
}

func TestErrorHandler_HandleMultipleErrors(t *testing.T) {
	tests := []struct {
		name        string
		errors      []error
		operation   string
		expectError bool
		errorCount  int
	}{
		{
			name:        "no errors returns nil",
			errors:      []error{},
			operation:   "test operation",
			expectError: false,
		},
		{
			name:        "single error is handled normally",
			errors:      []error{assert.AnError},
			operation:   "test operation",
			expectError: true,
			errorCount:  1,
		},
		{
			name:        "multiple errors are aggregated",
			errors:      []error{assert.AnError, assert.AnError},
			operation:   "test operation",
			expectError: true,
			errorCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &mockLogger{}
			handler := NewErrorHandler(logger)

			result := handler.HandleMultipleErrors(tt.errors, tt.operation)

			if tt.expectError {
				require.Error(t, result)
				assert.Contains(t, result.Error(), tt.operation)
			} else {
				assert.NoError(t, result)
			}
		})
	}
}
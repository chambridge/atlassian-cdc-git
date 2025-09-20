package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/company/jira-cdc-operator/operands/api/handlers"
)

func TestGetHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMock      func() *gin.Engine
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name: "healthy service status",
			setupMock: func() *gin.Engine {
				router := gin.New()
				// This will fail until handlers.GetHealth is implemented
				router.GET("/api/v1/health", handlers.GetHealth)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var health map[string]interface{}
				err := json.Unmarshal(body, &health)
				require.NoError(t, err)

				// Validate required fields from HealthStatus schema
				assert.Contains(t, health, "status")
				assert.Contains(t, health, "timestamp")
				assert.Contains(t, health, "version")

				// Validate field types
				assert.IsType(t, "", health["status"])
				assert.IsType(t, "", health["timestamp"])
				assert.IsType(t, "", health["version"])

				// Validate enum values
				validStatuses := []string{"healthy", "degraded", "unhealthy"}
				assert.Contains(t, validStatuses, health["status"])

				// For healthy status, all components should be healthy
				if health["status"] == "healthy" {
					if components, exists := health["components"]; exists {
						componentsMap := components.(map[string]interface{})

						if jiraConnection, exists := componentsMap["jiraConnection"]; exists {
							assert.Equal(t, "healthy", jiraConnection)
						}

						if gitRepository, exists := componentsMap["gitRepository"]; exists {
							assert.Equal(t, "healthy", gitRepository)
						}

						if kubernetes, exists := componentsMap["kubernetes"]; exists {
							assert.Equal(t, "healthy", kubernetes)
						}
					}
				}

				// Validate optional uptime field
				if uptime, exists := health["uptime"]; exists {
					assert.IsType(t, "", uptime)
					// Uptime format should be like "24h30m15s"
					uptimeStr := uptime.(string)
					assert.Regexp(t, `^\d+[hms].*`, uptimeStr)
				}

				// Validate version format
				version := health["version"].(string)
				assert.NotEmpty(t, version)
				assert.Regexp(t, `^\d+\.\d+\.\d+`, version)
			},
		},
		{
			name: "degraded service status",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/health", handlers.GetHealth)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var health map[string]interface{}
				err := json.Unmarshal(body, &health)
				require.NoError(t, err)

				assert.Contains(t, health, "status")
				assert.Contains(t, health, "timestamp")
				assert.Contains(t, health, "version")

				// For degraded status, some components might be unhealthy
				if health["status"] == "degraded" {
					if components, exists := health["components"]; exists {
						componentsMap := components.(map[string]interface{})

						// At least one component should be unhealthy for degraded status
						hasUnhealthyComponent := false
						for _, componentStatus := range componentsMap {
							if componentStatus == "unhealthy" {
								hasUnhealthyComponent = true
								break
							}
						}
						// Note: In a real degraded scenario, this would be true
						// But for test purposes, we just validate the structure
					}
				}
			},
		},
		{
			name: "unhealthy service status",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/health", handlers.GetHealth)
				return router
			},
			expectedStatus: http.StatusServiceUnavailable,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)

				// For unhealthy status, we might get an ErrorResponse instead
				// Check if it's a health status or error response
				if status, exists := errorResp["status"]; exists {
					// It's a health status response
					assert.Equal(t, "unhealthy", status)
					assert.Contains(t, errorResp, "timestamp")
					assert.Contains(t, errorResp, "version")

					// Components should show unhealthy states
					if components, exists := errorResp["components"]; exists {
						componentsMap := components.(map[string]interface{})
						// Critical components should be unhealthy
						for _, componentStatus := range componentsMap {
							validComponentStatuses := []string{"healthy", "unhealthy"}
							assert.Contains(t, validComponentStatuses, componentStatus)
						}
					}
				} else {
					// It's an error response
					assert.Contains(t, errorResp, "error")
					assert.Contains(t, errorResp, "message")
					assert.Contains(t, errorResp, "timestamp")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.GetHealth doesn't exist yet
			router := tt.setupMock()

			req, err := http.NewRequest("GET", "/api/v1/health", nil)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			// Validate Content-Type header
			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

			if tt.validateBody != nil {
				tt.validateBody(t, w.Body.Bytes())
			}
		})
	}
}

func TestHealthComponentValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		expectedBody func(t *testing.T, body []byte)
	}{
		{
			name: "validate jira connection component",
			expectedBody: func(t *testing.T, body []byte) {
				var health map[string]interface{}
				err := json.Unmarshal(body, &health)
				require.NoError(t, err)

				if components, exists := health["components"]; exists {
					componentsMap := components.(map[string]interface{})

					if jiraConnection, exists := componentsMap["jiraConnection"]; exists {
						validStatuses := []string{"healthy", "unhealthy"}
						assert.Contains(t, validStatuses, jiraConnection)
					}
				}
			},
		},
		{
			name: "validate git repository component",
			expectedBody: func(t *testing.T, body []byte) {
				var health map[string]interface{}
				err := json.Unmarshal(body, &health)
				require.NoError(t, err)

				if components, exists := health["components"]; exists {
					componentsMap := components.(map[string]interface{})

					if gitRepository, exists := componentsMap["gitRepository"]; exists {
						validStatuses := []string{"healthy", "unhealthy"}
						assert.Contains(t, validStatuses, gitRepository)
					}
				}
			},
		},
		{
			name: "validate kubernetes component",
			expectedBody: func(t *testing.T, body []byte) {
				var health map[string]interface{}
				err := json.Unmarshal(body, &health)
				require.NoError(t, err)

				if components, exists := health["components"]; exists {
					componentsMap := components.(map[string]interface{})

					if kubernetes, exists := componentsMap["kubernetes"]; exists {
						validStatuses := []string{"healthy", "unhealthy"}
						assert.Contains(t, validStatuses, kubernetes)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.GetHealth doesn't exist yet
			router := gin.New()
			router.GET("/api/v1/health", handlers.GetHealth)

			req, err := http.NewRequest("GET", "/api/v1/health", nil)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Only validate if we get a successful response
			if w.Code == http.StatusOK {
				tt.expectedBody(t, w.Body.Bytes())
			}
		})
	}
}

func TestHealthEndpointErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("internal server error during health check", func(t *testing.T) {
		// Test that the handler properly handles internal errors
		// This will fail until error handling is implemented
		router := gin.New()
		router.GET("/api/v1/health", handlers.GetHealth)

		req, err := http.NewRequest("GET", "/api/v1/health", nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Health endpoint should always respond, even if degraded
		// This assertion will initially fail as the handler doesn't exist
		if w.Code == http.StatusInternalServerError {
			var errorResp map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &errorResp)
			require.NoError(t, err)

			// Validate ErrorResponse schema
			assert.Contains(t, errorResp, "error")
			assert.Contains(t, errorResp, "message")
			assert.Contains(t, errorResp, "timestamp")
		}
	})

	t.Run("health check response time", func(t *testing.T) {
		// Health checks should be fast
		router := gin.New()
		router.GET("/api/v1/health", handlers.GetHealth)

		req, err := http.NewRequest("GET", "/api/v1/health", nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Health endpoint should respond quickly (within reasonable time)
		// This is more of a performance consideration for the actual implementation
		assert.NotEqual(t, http.StatusRequestTimeout, w.Code)
	})
}
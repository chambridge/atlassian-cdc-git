package contract

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/company/jira-cdc-operator/operands/api/handlers"
)

func TestGetMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMock      func() *gin.Engine
		expectedStatus int
		validateBody   func(t *testing.T, body string)
	}{
		{
			name: "successful metrics retrieval",
			setupMock: func() *gin.Engine {
				router := gin.New()
				// This will fail until handlers.GetMetrics is implemented
				router.GET("/api/v1/metrics", handlers.GetMetrics)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body string) {
				// Validate Prometheus format
				assert.NotEmpty(t, body)

				// Should contain basic JIRA CDC metrics
				assert.Contains(t, body, "jiracdc_sync_operations_total")
				assert.Contains(t, body, "# HELP")
				assert.Contains(t, body, "# TYPE")

				// Validate counter metric format
				lines := strings.Split(body, "\n")
				hasHelpLine := false
				hasTypeLine := false
				hasMetricLine := false

				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}

					if strings.HasPrefix(line, "# HELP jiracdc_sync_operations_total") {
						hasHelpLine = true
						assert.Contains(t, line, "Total number of sync operations")
					}

					if strings.HasPrefix(line, "# TYPE jiracdc_sync_operations_total") {
						hasTypeLine = true
						assert.Contains(t, line, "counter")
					}

					if strings.HasPrefix(line, "jiracdc_sync_operations_total{") {
						hasMetricLine = true
						// Should have labels like status="success"
						assert.Contains(t, line, "status=")
						assert.Contains(t, line, "}")
						// Should end with a number
						parts := strings.Split(line, " ")
						assert.GreaterOrEqual(t, len(parts), 2)
					}
				}

				assert.True(t, hasHelpLine, "Should contain HELP line for sync operations metric")
				assert.True(t, hasTypeLine, "Should contain TYPE line for sync operations metric")
				assert.True(t, hasMetricLine, "Should contain metric data line")
			},
		},
		{
			name: "comprehensive metrics validation",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/metrics", handlers.GetMetrics)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body string) {
				// Check for expected JIRA CDC metrics
				expectedMetrics := []string{
					"jiracdc_sync_operations_total",
					"jiracdc_sync_duration_seconds",
					"jiracdc_active_projects",
					"jiracdc_last_sync_timestamp",
					"jiracdc_issues_synced_total",
					"jiracdc_sync_errors_total",
				}

				for _, metric := range expectedMetrics {
					assert.Contains(t, body, metric, "Should contain metric: %s", metric)
				}

				// Validate that we have both HELP and TYPE for main metrics
				assert.Contains(t, body, "# HELP jiracdc_sync_operations_total")
				assert.Contains(t, body, "# TYPE jiracdc_sync_operations_total counter")

				// Check for project-specific labels
				if strings.Contains(body, "project=") {
					assert.Contains(t, body, "project=\"")
				}

				// Check for status labels
				if strings.Contains(body, "status=") {
					assert.Contains(t, body, "status=\"")
				}
			},
		},
		{
			name: "metrics with different label combinations",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/metrics", handlers.GetMetrics)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body string) {
				// Look for various status combinations
				statusTypes := []string{"success", "error", "pending"}
				operationTypes := []string{"bootstrap", "reconciliation", "maintenance"}

				// Check that metrics can have various label combinations
				lines := strings.Split(body, "\n")
				metricLines := []string{}

				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "jiracdc_") && !strings.HasPrefix(line, "#") {
						metricLines = append(metricLines, line)
					}
				}

				assert.Greater(t, len(metricLines), 0, "Should have at least one metric line")

				// Validate metric line format: metric_name{labels} value
				for _, line := range metricLines {
					if strings.Contains(line, "{") {
						// Has labels
						assert.Contains(t, line, "}")
						parts := strings.Split(line, "}")
						assert.GreaterOrEqual(t, len(parts), 2)

						// Value should be a number
						valuePart := strings.TrimSpace(parts[1])
						assert.NotEmpty(t, valuePart)
					} else {
						// No labels, just metric_name value
						parts := strings.Split(line, " ")
						assert.GreaterOrEqual(t, len(parts), 2)
					}
				}

				// Some metrics might be present with different statuses/operations
				_ = statusTypes    // Available for validation
				_ = operationTypes // Available for validation
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.GetMetrics doesn't exist yet
			router := tt.setupMock()

			req, err := http.NewRequest("GET", "/api/v1/metrics", nil)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			// Validate Content-Type header for Prometheus metrics
			assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

			if tt.validateBody != nil {
				tt.validateBody(t, w.Body.String())
			}
		})
	}
}

func TestMetricsFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("prometheus format validation", func(t *testing.T) {
		// This test will fail because handlers.GetMetrics doesn't exist yet
		router := gin.New()
		router.GET("/api/v1/metrics", handlers.GetMetrics)

		req, err := http.NewRequest("GET", "/api/v1/metrics", nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			body := w.Body.String()

			// Validate basic Prometheus format rules
			lines := strings.Split(body, "\n")

			for i, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				if strings.HasPrefix(line, "# HELP ") {
					// HELP lines should be followed by TYPE lines
					assert.Greater(t, len(lines), i+1, "HELP line should be followed by more content")
					
					if i+1 < len(lines) {
						nextLine := strings.TrimSpace(lines[i+1])
						if nextLine != "" && !strings.HasPrefix(nextLine, "# TYPE ") {
							// This might be acceptable in some cases, but generally TYPE follows HELP
						}
					}
				}

				if strings.HasPrefix(line, "# TYPE ") {
					// TYPE lines should specify metric type
					parts := strings.Split(line, " ")
					assert.GreaterOrEqual(t, len(parts), 4, "TYPE line should have metric name and type")
					
					if len(parts) >= 4 {
						metricType := parts[3]
						validTypes := []string{"counter", "gauge", "histogram", "summary"}
						assert.Contains(t, validTypes, metricType, "Should be valid Prometheus metric type")
					}
				}

				if strings.HasPrefix(line, "jiracdc_") && !strings.HasPrefix(line, "#") {
					// Metric data lines should be valid
					if strings.Contains(line, "{") {
						// Has labels
						assert.Contains(t, line, "}", "Metric with labels should close braces")
						
						// Extract labels part
						start := strings.Index(line, "{")
						end := strings.Index(line, "}")
						assert.Greater(t, end, start, "Closing brace should come after opening brace")
						
						labelsStr := line[start+1 : end]
						if labelsStr != "" {
							// Validate label format: key="value"
							labels := strings.Split(labelsStr, ",")
							for _, label := range labels {
								label = strings.TrimSpace(label)
								assert.Contains(t, label, "=", "Label should have key=value format")
								assert.Contains(t, label, "\"", "Label value should be quoted")
							}
						}
					}

					// Extract value (last part after space)
					parts := strings.Fields(line)
					assert.GreaterOrEqual(t, len(parts), 2, "Metric line should have name and value")
				}
			}
		}
	})
}

func TestMetricsErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("metrics endpoint availability", func(t *testing.T) {
		// Metrics endpoint should always be available for monitoring
		// This test will fail because handlers.GetMetrics doesn't exist yet
		router := gin.New()
		router.GET("/api/v1/metrics", handlers.GetMetrics)

		req, err := http.NewRequest("GET", "/api/v1/metrics", nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Metrics endpoint should not return authentication errors
		// It should be publicly accessible for monitoring systems
		assert.NotEqual(t, http.StatusUnauthorized, w.Code)
		assert.NotEqual(t, http.StatusForbidden, w.Code)
	})

	t.Run("metrics content type", func(t *testing.T) {
		router := gin.New()
		router.GET("/api/v1/metrics", handlers.GetMetrics)

		req, err := http.NewRequest("GET", "/api/v1/metrics", nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return plain text, not JSON
		if w.Code == http.StatusOK {
			contentType := w.Header().Get("Content-Type")
			assert.Contains(t, contentType, "text/plain")
			assert.NotContains(t, contentType, "application/json")
		}
	})
}
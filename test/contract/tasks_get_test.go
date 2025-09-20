package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/company/jira-cdc-operator/operands/api/handlers"
)

func TestGetTasks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    map[string]string
		setupMock      func() *gin.Engine
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name:        "successful tasks list without filters",
			queryParams: map[string]string{},
			setupMock: func() *gin.Engine {
				router := gin.New()
				// This will fail until handlers.GetTasks is implemented
				router.GET("/api/v1/tasks", handlers.GetTasks)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var tasks []map[string]interface{}
				err := json.Unmarshal(body, &tasks)
				require.NoError(t, err)

				// Validate the response structure matches the OpenAPI spec
				assert.IsType(t, []map[string]interface{}{}, tasks)

				if len(tasks) > 0 {
					task := tasks[0]
					// Validate required fields from TaskSummary schema
					assert.Contains(t, task, "id")
					assert.Contains(t, task, "type")
					assert.Contains(t, task, "status")
					assert.Contains(t, task, "projectKey")
					assert.Contains(t, task, "createdAt")

					// Validate field types
					assert.IsType(t, "", task["id"])
					assert.IsType(t, "", task["type"])
					assert.IsType(t, "", task["status"])
					assert.IsType(t, "", task["projectKey"])
					assert.IsType(t, "", task["createdAt"])

					// Validate enum values
					validTypes := []string{"bootstrap", "reconciliation", "maintenance"}
					assert.Contains(t, validTypes, task["type"])

					validStatuses := []string{"pending", "running", "completed", "failed", "cancelled"}
					assert.Contains(t, validStatuses, task["status"])

					// Validate optional fields if present
					if progressData, exists := task["progress"]; exists {
						progress := progressData.(map[string]interface{})
						assert.Contains(t, progress, "totalItems")
						assert.Contains(t, progress, "processedItems")
						assert.Contains(t, progress, "percentComplete")
						assert.IsType(t, float64(0), progress["totalItems"])
						assert.IsType(t, float64(0), progress["processedItems"])
						assert.IsType(t, float64(0), progress["percentComplete"])
					}
				}
			},
		},
		{
			name: "filtered tasks by status",
			queryParams: map[string]string{
				"status": "running",
			},
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/tasks", handlers.GetTasks)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var tasks []map[string]interface{}
				err := json.Unmarshal(body, &tasks)
				require.NoError(t, err)

				// All returned tasks should have status "running"
				for _, task := range tasks {
					assert.Equal(t, "running", task["status"])
				}
			},
		},
		{
			name: "filtered tasks by type",
			queryParams: map[string]string{
				"type": "reconciliation",
			},
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/tasks", handlers.GetTasks)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var tasks []map[string]interface{}
				err := json.Unmarshal(body, &tasks)
				require.NoError(t, err)

				// All returned tasks should have type "reconciliation"
				for _, task := range tasks {
					assert.Equal(t, "reconciliation", task["type"])
				}
			},
		},
		{
			name: "filtered tasks by projectKey",
			queryParams: map[string]string{
				"projectKey": "PROJ",
			},
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/tasks", handlers.GetTasks)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var tasks []map[string]interface{}
				err := json.Unmarshal(body, &tasks)
				require.NoError(t, err)

				// All returned tasks should have projectKey "PROJ"
				for _, task := range tasks {
					assert.Equal(t, "PROJ", task["projectKey"])
				}
			},
		},
		{
			name: "multiple filters combined",
			queryParams: map[string]string{
				"status":     "completed",
				"type":       "bootstrap",
				"projectKey": "PROJ",
			},
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/tasks", handlers.GetTasks)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var tasks []map[string]interface{}
				err := json.Unmarshal(body, &tasks)
				require.NoError(t, err)

				// All returned tasks should match all filters
				for _, task := range tasks {
					assert.Equal(t, "completed", task["status"])
					assert.Equal(t, "bootstrap", task["type"])
					assert.Equal(t, "PROJ", task["projectKey"])
				}
			},
		},
		{
			name: "empty tasks list",
			queryParams: map[string]string{
				"status": "cancelled",
			},
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/tasks", handlers.GetTasks)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var tasks []map[string]interface{}
				err := json.Unmarshal(body, &tasks)
				require.NoError(t, err)
				assert.Equal(t, 0, len(tasks))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.GetTasks doesn't exist yet
			router := tt.setupMock()

			// Build request URL with query parameters
			reqURL := "/api/v1/tasks"
			if len(tt.queryParams) > 0 {
				params := url.Values{}
				for key, value := range tt.queryParams {
					params.Add(key, value)
				}
				reqURL += "?" + params.Encode()
			}

			req, err := http.NewRequest("GET", reqURL, nil)
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

func TestGetTasksValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		queryParams    map[string]string
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name: "invalid status filter",
			queryParams: map[string]string{
				"status": "invalid-status",
			},
			expectedStatus: http.StatusBadRequest,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)

				// Validate ErrorResponse schema
				assert.Contains(t, errorResp, "error")
				assert.Contains(t, errorResp, "message")
				assert.Contains(t, errorResp, "timestamp")
				assert.Contains(t, errorResp["message"], "status must be one of: pending, running, completed, failed, cancelled")
			},
		},
		{
			name: "invalid type filter",
			queryParams: map[string]string{
				"type": "invalid-type",
			},
			expectedStatus: http.StatusBadRequest,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)

				assert.Contains(t, errorResp, "error")
				assert.Contains(t, errorResp, "message")
				assert.Contains(t, errorResp["message"], "type must be one of: bootstrap, reconciliation, maintenance")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.GetTasks doesn't exist yet
			router := gin.New()
			router.GET("/api/v1/tasks", handlers.GetTasks)

			// Build request URL with query parameters
			params := url.Values{}
			for key, value := range tt.queryParams {
				params.Add(key, value)
			}
			reqURL := "/api/v1/tasks?" + params.Encode()

			req, err := http.NewRequest("GET", reqURL, nil)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validateBody != nil {
				tt.validateBody(t, w.Body.Bytes())
			}
		})
	}
}
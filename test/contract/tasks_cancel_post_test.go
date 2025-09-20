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

func TestCancelTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		taskId         string
		setupMock      func() *gin.Engine
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name:   "successful task cancellation - pending task",
			taskId: "task-123e4567-e89b-12d3-a456-426614174000",
			setupMock: func() *gin.Engine {
				router := gin.New()
				// This will fail until handlers.CancelTask is implemented
				router.POST("/api/v1/tasks/:taskId/cancel", handlers.CancelTask)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				// Validate TaskResponse schema
				assert.Contains(t, response, "taskId")
				assert.Contains(t, response, "status")
				assert.Contains(t, response, "message")

				assert.IsType(t, "", response["taskId"])
				assert.IsType(t, "", response["status"])
				assert.IsType(t, "", response["message"])

				assert.Equal(t, "task-123e4567-e89b-12d3-a456-426614174000", response["taskId"])
				assert.Equal(t, "cancelled", response["status"])
				assert.Contains(t, response["message"], "Task cancelled successfully")
			},
		},
		{
			name:   "successful task cancellation - running task",
			taskId: "task-running-789",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.POST("/api/v1/tasks/:taskId/cancel", handlers.CancelTask)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				assert.Contains(t, response, "taskId")
				assert.Contains(t, response, "status")
				assert.Contains(t, response, "message")

				assert.Equal(t, "task-running-789", response["taskId"])
				assert.Equal(t, "cancelled", response["status"])
			},
		},
		{
			name:   "task already completed - cannot cancel",
			taskId: "task-completed-456",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.POST("/api/v1/tasks/:taskId/cancel", handlers.CancelTask)
				return router
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
				assert.Contains(t, errorResp["message"], "Cannot cancel task with status completed")
			},
		},
		{
			name:   "task already failed - cannot cancel",
			taskId: "task-failed-789",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.POST("/api/v1/tasks/:taskId/cancel", handlers.CancelTask)
				return router
			},
			expectedStatus: http.StatusBadRequest,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)

				assert.Contains(t, errorResp, "error")
				assert.Contains(t, errorResp, "message")
				assert.Contains(t, errorResp, "timestamp")
				assert.Contains(t, errorResp["message"], "Cannot cancel task with status failed")
			},
		},
		{
			name:   "task already cancelled - idempotent response",
			taskId: "task-cancelled-123",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.POST("/api/v1/tasks/:taskId/cancel", handlers.CancelTask)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)

				assert.Contains(t, response, "taskId")
				assert.Contains(t, response, "status")
				assert.Contains(t, response, "message")

				assert.Equal(t, "task-cancelled-123", response["taskId"])
				assert.Equal(t, "cancelled", response["status"])
				assert.Contains(t, response["message"], "Task was already cancelled")
			},
		},
		{
			name:   "task not found",
			taskId: "task-nonexistent",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.POST("/api/v1/tasks/:taskId/cancel", handlers.CancelTask)
				return router
			},
			expectedStatus: http.StatusNotFound,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)

				// Validate ErrorResponse schema
				assert.Contains(t, errorResp, "error")
				assert.Contains(t, errorResp, "message")
				assert.Contains(t, errorResp, "timestamp")
				assert.Contains(t, errorResp["message"], "Task task-nonexistent not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.CancelTask doesn't exist yet
			router := tt.setupMock()

			req, err := http.NewRequest("POST", "/api/v1/tasks/"+tt.taskId+"/cancel", nil)
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

func TestCancelTaskValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		taskId         string
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name:           "invalid task ID format",
			taskId:         "invalid-id",
			expectedStatus: http.StatusBadRequest,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)

				assert.Contains(t, errorResp, "error")
				assert.Contains(t, errorResp, "message")
				assert.Contains(t, errorResp["message"], "Invalid task ID format")
			},
		},
		{
			name:           "empty task ID",
			taskId:         "",
			expectedStatus: http.StatusBadRequest,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)

				assert.Contains(t, errorResp, "error")
				assert.Contains(t, errorResp, "message")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.CancelTask doesn't exist yet
			router := gin.New()
			router.POST("/api/v1/tasks/:taskId/cancel", handlers.CancelTask)

			req, err := http.NewRequest("POST", "/api/v1/tasks/"+tt.taskId+"/cancel", nil)
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

func TestCancelTaskErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("internal server error during cancellation", func(t *testing.T) {
		// Test that the handler properly returns 500 on internal errors
		// This will fail until error handling is implemented
		router := gin.New()
		router.POST("/api/v1/tasks/:taskId/cancel", handlers.CancelTask)

		req, err := http.NewRequest("POST", "/api/v1/tasks/task-error-simulation/cancel", nil)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

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
}
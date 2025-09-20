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

func TestGetTaskDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		taskId         string
		setupMock      func() *gin.Engine
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name:   "successful task details retrieval",
			taskId: "task-123e4567-e89b-12d3-a456-426614174000",
			setupMock: func() *gin.Engine {
				router := gin.New()
				// This will fail until handlers.GetTaskDetails is implemented
				router.GET("/api/v1/tasks/:taskId", handlers.GetTaskDetails)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var task map[string]interface{}
				err := json.Unmarshal(body, &task)
				require.NoError(t, err)

				// Validate required fields from TaskDetails schema (extends TaskSummary)
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

				// Validate TaskDetails specific fields
				if config, exists := task["configuration"]; exists {
					configuration := config.(map[string]interface{})
					if issueFilter, exists := configuration["issueFilter"]; exists {
						assert.IsType(t, "", issueFilter)
					}
					if forceRefresh, exists := configuration["forceRefresh"]; exists {
						assert.IsType(t, true, forceRefresh)
					}
				}

				if createdBy, exists := task["createdBy"]; exists {
					assert.IsType(t, "", createdBy)
				}

				if errorMessage, exists := task["errorMessage"]; exists {
					assert.IsType(t, "", errorMessage)
				}

				// Validate progress if present
				if progressData, exists := task["progress"]; exists {
					progress := progressData.(map[string]interface{})
					assert.Contains(t, progress, "totalItems")
					assert.Contains(t, progress, "processedItems")
					assert.Contains(t, progress, "percentComplete")
					assert.IsType(t, float64(0), progress["totalItems"])
					assert.IsType(t, float64(0), progress["processedItems"])
					assert.IsType(t, float64(0), progress["percentComplete"])

					if estimatedTime, exists := progress["estimatedTimeRemaining"]; exists {
						assert.IsType(t, "", estimatedTime)
					}
				}

				// Validate operations array if present
				if ops, exists := task["operations"]; exists {
					operations := ops.([]interface{})
					if len(operations) > 0 {
						operation := operations[0].(map[string]interface{})

						// Validate required SyncOperation fields
						assert.Contains(t, operation, "id")
						assert.Contains(t, operation, "issueKey")
						assert.Contains(t, operation, "operationType")
						assert.Contains(t, operation, "status")

						assert.IsType(t, "", operation["id"])
						assert.IsType(t, "", operation["issueKey"])
						assert.IsType(t, "", operation["operationType"])
						assert.IsType(t, "", operation["status"])

						// Validate enum values
						validOpTypes := []string{"create", "update", "delete", "move"}
						assert.Contains(t, validOpTypes, operation["operationType"])

						validOpStatuses := []string{"pending", "processing", "completed", "failed", "skipped"}
						assert.Contains(t, validOpStatuses, operation["status"])

						// Validate git operation if present
						if gitOp, exists := operation["gitOperation"]; exists {
							gitOperation := gitOp.(map[string]interface{})
							if action, exists := gitOperation["action"]; exists {
								assert.IsType(t, "", action)
							}
							if filePath, exists := gitOperation["filePath"]; exists {
								assert.IsType(t, "", filePath)
							}
							if commitMsg, exists := gitOperation["commitMessage"]; exists {
								assert.IsType(t, "", commitMsg)
							}
							if commitHash, exists := gitOperation["commitHash"]; exists {
								assert.IsType(t, "", commitHash)
							}
						}

						if retryCount, exists := operation["retryCount"]; exists {
							assert.IsType(t, float64(0), retryCount)
						}
					}
				}
			},
		},
		{
			name:   "task with completed status",
			taskId: "task-completed-123",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/tasks/:taskId", handlers.GetTaskDetails)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var task map[string]interface{}
				err := json.Unmarshal(body, &task)
				require.NoError(t, err)

				// For completed tasks, we should have completedAt timestamp
				if task["status"] == "completed" {
					if completedAt, exists := task["completedAt"]; exists {
						assert.IsType(t, "", completedAt)
					}
				}

				// Progress should be 100% for completed tasks
				if progressData, exists := task["progress"]; exists {
					progress := progressData.(map[string]interface{})
					if percentComplete, exists := progress["percentComplete"]; exists {
						assert.Equal(t, float64(100), percentComplete)
					}
				}
			},
		},
		{
			name:   "task with failed status and error details",
			taskId: "task-failed-456",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/tasks/:taskId", handlers.GetTaskDetails)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var task map[string]interface{}
				err := json.Unmarshal(body, &task)
				require.NoError(t, err)

				// Failed tasks should have error message
				if task["status"] == "failed" {
					assert.Contains(t, task, "errorMessage")
					assert.IsType(t, "", task["errorMessage"])
				}
			},
		},
		{
			name:   "task not found",
			taskId: "task-nonexistent",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/tasks/:taskId", handlers.GetTaskDetails)
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
			// This test will fail because handlers.GetTaskDetails doesn't exist yet
			router := tt.setupMock()

			req, err := http.NewRequest("GET", "/api/v1/tasks/"+tt.taskId, nil)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			// Validate Content-Type header for successful responses
			if tt.expectedStatus == http.StatusOK {
				assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
			}

			if tt.validateBody != nil {
				tt.validateBody(t, w.Body.Bytes())
			}
		})
	}
}

func TestGetTaskDetailsValidation(t *testing.T) {
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
			// This test will fail because handlers.GetTaskDetails doesn't exist yet
			router := gin.New()
			router.GET("/api/v1/tasks/:taskId", handlers.GetTaskDetails)

			req, err := http.NewRequest("GET", "/api/v1/tasks/"+tt.taskId, nil)
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
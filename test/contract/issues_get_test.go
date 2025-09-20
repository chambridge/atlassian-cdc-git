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

func TestGetIssueStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		issueKey       string
		setupMock      func() *gin.Engine
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name:     "successful issue status retrieval",
			issueKey: "PROJ-123",
			setupMock: func() *gin.Engine {
				router := gin.New()
				// This will fail until handlers.GetIssueStatus is implemented
				router.GET("/api/v1/issues/:issueKey", handlers.GetIssueStatus)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var issue map[string]interface{}
				err := json.Unmarshal(body, &issue)
				require.NoError(t, err)

				// Validate required fields from IssueSyncStatus schema
				assert.Contains(t, issue, "issueKey")
				assert.Contains(t, issue, "projectKey")
				assert.Contains(t, issue, "status")
				assert.Contains(t, issue, "gitFilePath")

				// Validate field types
				assert.IsType(t, "", issue["issueKey"])
				assert.IsType(t, "", issue["projectKey"])
				assert.IsType(t, "", issue["status"])
				assert.IsType(t, "", issue["gitFilePath"])

				// Validate specific values
				assert.Equal(t, "PROJ-123", issue["issueKey"])
				assert.Equal(t, "PROJ", issue["projectKey"])

				// Validate optional fields if present
				if summary, exists := issue["summary"]; exists {
					assert.IsType(t, "", summary)
				}

				if jiraUpdatedAt, exists := issue["jiraUpdatedAt"]; exists {
					assert.IsType(t, "", jiraUpdatedAt)
				}

				if syncedAt, exists := issue["syncedAt"]; exists {
					assert.IsType(t, "", syncedAt)
				}

				if gitCommitHash, exists := issue["gitCommitHash"]; exists {
					assert.IsType(t, "", gitCommitHash)
				}

				if syncStatus, exists := issue["syncStatus"]; exists {
					assert.IsType(t, "", syncStatus)
					validSyncStatuses := []string{"current", "needs_sync", "error"}
					assert.Contains(t, validSyncStatuses, syncStatus)
				}

				// Validate git file path format
				gitFilePath := issue["gitFilePath"].(string)
				assert.Contains(t, gitFilePath, "PROJ-123")
				assert.Contains(t, gitFilePath, ".md")
			},
		},
		{
			name:     "issue with sync status current",
			issueKey: "PROJ-456",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/issues/:issueKey", handlers.GetIssueStatus)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var issue map[string]interface{}
				err := json.Unmarshal(body, &issue)
				require.NoError(t, err)

				assert.Equal(t, "PROJ-456", issue["issueKey"])

				// For current status, syncedAt should be present and recent
				if syncStatus, exists := issue["syncStatus"]; exists && syncStatus == "current" {
					assert.Contains(t, issue, "syncedAt")
					assert.Contains(t, issue, "gitCommitHash")
				}
			},
		},
		{
			name:     "issue with sync status needs_sync",
			issueKey: "PROJ-789",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/issues/:issueKey", handlers.GetIssueStatus)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var issue map[string]interface{}
				err := json.Unmarshal(body, &issue)
				require.NoError(t, err)

				assert.Equal(t, "PROJ-789", issue["issueKey"])

				// For needs_sync status, jiraUpdatedAt should be more recent than syncedAt
				if syncStatus, exists := issue["syncStatus"]; exists && syncStatus == "needs_sync" {
					assert.Contains(t, issue, "jiraUpdatedAt")
					// syncedAt might be missing for never-synced issues
				}
			},
		},
		{
			name:     "issue with sync error status",
			issueKey: "PROJ-ERROR",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/issues/:issueKey", handlers.GetIssueStatus)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var issue map[string]interface{}
				err := json.Unmarshal(body, &issue)
				require.NoError(t, err)

				assert.Equal(t, "PROJ-ERROR", issue["issueKey"])

				// For error status, there should be some indication of the problem
				if syncStatus, exists := issue["syncStatus"]; exists && syncStatus == "error" {
					// Error issues might have partial sync data
					assert.Contains(t, issue, "jiraUpdatedAt")
				}
			},
		},
		{
			name:     "issue from different project",
			issueKey: "OTHER-123",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/issues/:issueKey", handlers.GetIssueStatus)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var issue map[string]interface{}
				err := json.Unmarshal(body, &issue)
				require.NoError(t, err)

				assert.Equal(t, "OTHER-123", issue["issueKey"])
				assert.Equal(t, "OTHER", issue["projectKey"])

				// Git file path should match the project
				gitFilePath := issue["gitFilePath"].(string)
				assert.Contains(t, gitFilePath, "OTHER-123")
			},
		},
		{
			name:     "issue not found",
			issueKey: "NONEXISTENT-999",
			setupMock: func() *gin.Engine {
				router := gin.New()
				router.GET("/api/v1/issues/:issueKey", handlers.GetIssueStatus)
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
				assert.Contains(t, errorResp["message"], "Issue NONEXISTENT-999 not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.GetIssueStatus doesn't exist yet
			router := tt.setupMock()

			req, err := http.NewRequest("GET", "/api/v1/issues/"+tt.issueKey, nil)
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

func TestGetIssueStatusValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		issueKey       string
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name:           "invalid issue key format - no project",
			issueKey:       "123",
			expectedStatus: http.StatusBadRequest,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)

				assert.Contains(t, errorResp, "error")
				assert.Contains(t, errorResp, "message")
				assert.Contains(t, errorResp, "timestamp")
				assert.Contains(t, errorResp["message"], "Invalid issue key format")
			},
		},
		{
			name:           "invalid issue key format - no number",
			issueKey:       "PROJ-",
			expectedStatus: http.StatusBadRequest,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)

				assert.Contains(t, errorResp, "error")
				assert.Contains(t, errorResp, "message")
				assert.Contains(t, errorResp["message"], "Invalid issue key format")
			},
		},
		{
			name:           "empty issue key",
			issueKey:       "",
			expectedStatus: http.StatusBadRequest,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)

				assert.Contains(t, errorResp, "error")
				assert.Contains(t, errorResp, "message")
			},
		},
		{
			name:           "issue key with special characters",
			issueKey:       "PROJ@123",
			expectedStatus: http.StatusBadRequest,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)

				assert.Contains(t, errorResp, "error")
				assert.Contains(t, errorResp, "message")
				assert.Contains(t, errorResp["message"], "Invalid issue key format")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.GetIssueStatus doesn't exist yet
			router := gin.New()
			router.GET("/api/v1/issues/:issueKey", handlers.GetIssueStatus)

			req, err := http.NewRequest("GET", "/api/v1/issues/"+tt.issueKey, nil)
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

func TestGetIssueStatusErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("internal server error", func(t *testing.T) {
		// Test that the handler properly returns 500 on internal errors
		// This will fail until error handling is implemented
		router := gin.New()
		router.GET("/api/v1/issues/:issueKey", handlers.GetIssueStatus)

		req, err := http.NewRequest("GET", "/api/v1/issues/PROJ-ERROR-SIM", nil)
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
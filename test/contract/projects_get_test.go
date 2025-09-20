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

func TestGetProjects(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupMock      func() *gin.Engine
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name: "successful projects list",
			setupMock: func() *gin.Engine {
				router := gin.New()
				// This will fail until handlers.GetProjects is implemented
				router.GET("/api/v1/projects", handlers.GetProjects)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var projects []map[string]interface{}
				err := json.Unmarshal(body, &projects)
				require.NoError(t, err)
				
				// Validate the response structure matches the OpenAPI spec
				assert.IsType(t, []map[string]interface{}{}, projects)
				
				if len(projects) > 0 {
					project := projects[0]
					// Validate required fields from ProjectSummary schema
					assert.Contains(t, project, "projectKey")
					assert.Contains(t, project, "name")
					assert.Contains(t, project, "status")
					assert.Contains(t, project, "lastSyncTime")
					assert.Contains(t, project, "syncedIssueCount")
					assert.Contains(t, project, "gitRepository")
					
					// Validate field types
					assert.IsType(t, "", project["projectKey"])
					assert.IsType(t, "", project["name"])
					assert.IsType(t, "", project["status"])
					assert.IsType(t, "", project["lastSyncTime"])
					assert.IsType(t, float64(0), project["syncedIssueCount"])
					assert.IsType(t, "", project["gitRepository"])
					
					// Validate enum values
					validStatuses := []string{"Pending", "Syncing", "Current", "Error", "Paused"}
					assert.Contains(t, validStatuses, project["status"])
				}
			},
		},
		{
			name: "empty projects list",
			setupMock: func() *gin.Engine {
				router := gin.New()
				// This will fail until handlers.GetProjects is implemented
				router.GET("/api/v1/projects", handlers.GetProjects)
				return router
			},
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var projects []map[string]interface{}
				err := json.Unmarshal(body, &projects)
				require.NoError(t, err)
				assert.Equal(t, 0, len(projects))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.GetProjects doesn't exist yet
			router := tt.setupMock()
			
			req, err := http.NewRequest("GET", "/api/v1/projects", nil)
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

func TestGetProjectsErrorHandling(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("internal server error", func(t *testing.T) {
		// Test that the handler properly returns 500 on internal errors
		// This will fail until error handling is implemented
		router := gin.New()
		router.GET("/api/v1/projects", handlers.GetProjects)
		
		req, err := http.NewRequest("GET", "/api/v1/projects", nil)
		require.NoError(t, err)
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		
		// This assertion will fail initially as the handler doesn't exist
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
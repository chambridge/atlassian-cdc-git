package contract

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/company/jira-cdc-operator/operands/api/handlers"
)

func TestTriggerSync(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		projectKey     string
		requestBody    map[string]interface{}
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name:       "successful sync trigger - bootstrap",
			projectKey: "PROJ",
			requestBody: map[string]interface{}{
				"type":         "bootstrap",
				"forceRefresh": false,
			},
			expectedStatus: http.StatusAccepted,
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
				
				assert.Equal(t, "started", response["status"])
			},
		},
		{
			name:       "successful sync trigger - reconciliation with filter",
			projectKey: "PROJ",
			requestBody: map[string]interface{}{
				"type":         "reconciliation",
				"forceRefresh": true,
				"issueFilter":  "status != Done AND updated >= -7d",
			},
			expectedStatus: http.StatusAccepted,
			validateBody: func(t *testing.T, body []byte) {
				var response map[string]interface{}
				err := json.Unmarshal(body, &response)
				require.NoError(t, err)
				
				assert.Contains(t, response, "taskId")
				assert.Equal(t, "started", response["status"])
			},
		},
		{
			name:       "invalid sync type",
			projectKey: "PROJ",
			requestBody: map[string]interface{}{
				"type": "invalid-type",
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
				assert.Contains(t, errorResp["message"], "type must be one of: bootstrap, reconciliation, maintenance")
			},
		},
		{
			name:       "project not found",
			projectKey: "NONEXISTENT",
			requestBody: map[string]interface{}{
				"type": "bootstrap",
			},
			expectedStatus: http.StatusNotFound,
			validateBody: func(t *testing.T, body []byte) {
				var errorResp map[string]interface{}
				err := json.Unmarshal(body, &errorResp)
				require.NoError(t, err)
				
				assert.Contains(t, errorResp, "error")
				assert.Contains(t, errorResp, "message")
				assert.Contains(t, errorResp["message"], "Project NONEXISTENT not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.TriggerSync doesn't exist yet
			router := gin.New()
			router.POST("/api/v1/projects/:projectKey/sync", handlers.TriggerSync)
			
			body, err := json.Marshal(tt.requestBody)
			require.NoError(t, err)
			
			req, err := http.NewRequest("POST", "/api/v1/projects/"+tt.projectKey+"/sync", bytes.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.validateBody != nil {
				tt.validateBody(t, w.Body.Bytes())
			}
		})
	}
}

func TestTriggerSyncValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
	}{
		{
			name:           "empty request body",
			requestBody:    map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing required type field",
			requestBody: map[string]interface{}{
				"forceRefresh": true,
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because handlers.TriggerSync doesn't exist yet
			router := gin.New()
			router.POST("/api/v1/projects/:projectKey/sync", handlers.TriggerSync)
			
			var body []byte
			var err error
			
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}
			
			req, err := http.NewRequest("POST", "/api/v1/projects/PROJ/sync", bytes.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTasksHandler_ListTasks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock task manager
	mockTaskManager := new(MockTaskManager)

	// Setup test data
	testTasks := []*Task{
		{
			ID:         "task-1",
			Type:       "bootstrap",
			Status:     "completed",
			ProjectKey: "PROJ1",
			CreatedAt:  time.Now().Add(-2 * time.Hour),
			StartedAt:  &time.Time{},
			CompletedAt: &time.Time{},
			Progress: TaskProgress{
				TotalItems:      100,
				ProcessedItems:  100,
				PercentComplete: 100.0,
			},
		},
		{
			ID:         "task-2",
			Type:       "reconciliation",
			Status:     "running",
			ProjectKey: "PROJ2",
			CreatedAt:  time.Now().Add(-30 * time.Minute),
			StartedAt:  &time.Time{},
			Progress: TaskProgress{
				TotalItems:      50,
				ProcessedItems:  25,
				PercentComplete: 50.0,
			},
		},
		{
			ID:         "task-3",
			Type:       "maintenance",
			Status:     "failed",
			ProjectKey: "PROJ1",
			CreatedAt:  time.Now().Add(-1 * time.Hour),
			ErrorMessage: "Connection timeout",
		},
	}

	// Test no filters
	mockTaskManager.On("ListTasks", map[string]string{}).Return(testTasks, nil)

	// Create handler
	handler := NewTasksHandler(mockTaskManager)

	// Setup router
	router := gin.New()
	router.GET("/api/v1/tasks", handler.ListTasks)

	// Make request without filters
	req, _ := http.NewRequest("GET", "/api/v1/tasks", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response []TaskSummary
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response, 3)
	assert.Equal(t, "task-1", response[0].ID)
	assert.Equal(t, "bootstrap", response[0].Type)
	assert.Equal(t, "completed", response[0].Status)
	assert.Equal(t, "PROJ1", response[0].ProjectKey)
	assert.Equal(t, float64(100), response[0].Progress.PercentComplete)

	mockTaskManager.AssertExpectedCalls(t)
}

func TestTasksHandler_ListTasks_WithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock task manager
	mockTaskManager := new(MockTaskManager)

	// Setup test data - filtered results
	filteredTasks := []*Task{
		{
			ID:         "task-1",
			Type:       "bootstrap",
			Status:     "running",
			ProjectKey: "PROJ1",
			CreatedAt:  time.Now(),
		},
	}

	expectedFilters := map[string]string{
		"status":     "running",
		"type":       "bootstrap",
		"projectKey": "PROJ1",
	}
	mockTaskManager.On("ListTasks", expectedFilters).Return(filteredTasks, nil)

	// Create handler
	handler := NewTasksHandler(mockTaskManager)

	// Setup router
	router := gin.New()
	router.GET("/api/v1/tasks", handler.ListTasks)

	// Make request with filters
	req, _ := http.NewRequest("GET", "/api/v1/tasks?status=running&type=bootstrap&projectKey=PROJ1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response []TaskSummary
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response, 1)
	assert.Equal(t, "task-1", response[0].ID)
	assert.Equal(t, "running", response[0].Status)

	mockTaskManager.AssertExpectedCalls(t)
}

func TestTasksHandler_GetTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock task manager
	mockTaskManager := new(MockTaskManager)

	// Setup test data
	now := time.Now()
	testTask := &Task{
		ID:         "task-123",
		Type:       "reconciliation",
		Status:     "completed",
		ProjectKey: "PROJ",
		CreatedAt:  now.Add(-1 * time.Hour),
		StartedAt:  &now,
		CompletedAt: &now,
		Progress: TaskProgress{
			TotalItems:              50,
			ProcessedItems:          50,
			PercentComplete:         100.0,
			EstimatedTimeRemaining:  "0s",
		},
		Configuration: map[string]interface{}{
			"issueFilter":  "status != Done",
			"forceRefresh": false,
		},
		CreatedBy: "jiracdc-operator",
		Operations: []SyncOperation{
			{
				ID:            "op-1",
				IssueKey:      "PROJ-123",
				OperationType: "update",
				Status:        "completed",
				ProcessedAt:   now,
				GitOperation: GitOperation{
					Action:        "file updated",
					FilePath:      "PROJ-123.md",
					CommitMessage: "feat(PROJ-123): update issue status",
					CommitHash:    "abc123def",
				},
			},
		},
	}

	mockTaskManager.On("GetTask", "task-123").Return(testTask, nil)

	// Create handler
	handler := NewTasksHandler(mockTaskManager)

	// Setup router
	router := gin.New()
	router.GET("/api/v1/tasks/:taskId", handler.GetTask)

	// Make request
	req, _ := http.NewRequest("GET", "/api/v1/tasks/task-123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response TaskDetails
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "task-123", response.ID)
	assert.Equal(t, "reconciliation", response.Type)
	assert.Equal(t, "completed", response.Status)
	assert.Equal(t, "PROJ", response.ProjectKey)
	assert.Equal(t, "jiracdc-operator", response.CreatedBy)
	assert.Len(t, response.Operations, 1)
	assert.Equal(t, "PROJ-123", response.Operations[0].IssueKey)
	assert.Equal(t, "update", response.Operations[0].OperationType)

	mockTaskManager.AssertExpectedCalls(t)
}

func TestTasksHandler_GetTask_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock task manager
	mockTaskManager := new(MockTaskManager)

	// Mock task not found
	mockTaskManager.On("GetTask", "nonexistent").Return(nil, errors.New("task not found"))

	// Create handler
	handler := NewTasksHandler(mockTaskManager)

	// Setup router
	router := gin.New()
	router.GET("/api/v1/tasks/:taskId", handler.GetTask)

	// Make request
	req, _ := http.NewRequest("GET", "/api/v1/tasks/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "NotFound", response.Error)
	assert.Contains(t, response.Message, "nonexistent")

	mockTaskManager.AssertExpectedCalls(t)
}

func TestTasksHandler_CancelTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock task manager
	mockTaskManager := new(MockTaskManager)

	// Setup test data - get task for verification
	testTask := &Task{
		ID:         "task-123",
		Type:       "reconciliation",
		Status:     "running",
		ProjectKey: "PROJ",
		CreatedAt:  time.Now(),
	}

	mockTaskManager.On("GetTask", "task-123").Return(testTask, nil)
	mockTaskManager.On("CancelTask", "task-123").Return(nil)

	// Create handler
	handler := NewTasksHandler(mockTaskManager)

	// Setup router
	router := gin.New()
	router.POST("/api/v1/tasks/:taskId/cancel", handler.CancelTask)

	// Make request
	req, _ := http.NewRequest("POST", "/api/v1/tasks/task-123/cancel", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response TaskResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "task-123", response.TaskID)
	assert.Equal(t, "cancelled", response.Status)
	assert.Contains(t, response.Message, "cancelled")

	mockTaskManager.AssertExpectedCalls(t)
}

func TestTasksHandler_CancelTask_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock task manager
	mockTaskManager := new(MockTaskManager)

	// Mock task not found
	mockTaskManager.On("GetTask", "nonexistent").Return(nil, errors.New("task not found"))

	// Create handler
	handler := NewTasksHandler(mockTaskManager)

	// Setup router
	router := gin.New()
	router.POST("/api/v1/tasks/:taskId/cancel", handler.CancelTask)

	// Make request
	req, _ := http.NewRequest("POST", "/api/v1/tasks/nonexistent/cancel", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "NotFound", response.Error)

	mockTaskManager.AssertExpectedCalls(t)
}

func TestTasksHandler_CancelTask_NotCancellable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock task manager
	mockTaskManager := new(MockTaskManager)

	// Setup test data - completed task (not cancellable)
	testTask := &Task{
		ID:         "task-123",
		Type:       "reconciliation",
		Status:     "completed",
		ProjectKey: "PROJ",
		CreatedAt:  time.Now(),
	}

	mockTaskManager.On("GetTask", "task-123").Return(testTask, nil)

	// Create handler
	handler := NewTasksHandler(mockTaskManager)

	// Setup router
	router := gin.New()
	router.POST("/api/v1/tasks/:taskId/cancel", handler.CancelTask)

	// Make request
	req, _ := http.NewRequest("POST", "/api/v1/tasks/task-123/cancel", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "InvalidRequest", response.Error)
	assert.Contains(t, response.Message, "cannot be cancelled")

	mockTaskManager.AssertExpectedCalls(t)
}

func TestTasksHandler_CancelTask_CancellationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock task manager
	mockTaskManager := new(MockTaskManager)

	// Setup test data
	testTask := &Task{
		ID:         "task-123",
		Type:       "reconciliation",
		Status:     "running",
		ProjectKey: "PROJ",
		CreatedAt:  time.Now(),
	}

	mockTaskManager.On("GetTask", "task-123").Return(testTask, nil)
	mockTaskManager.On("CancelTask", "task-123").Return(errors.New("cancellation failed"))

	// Create handler
	handler := NewTasksHandler(mockTaskManager)

	// Setup router
	router := gin.New()
	router.POST("/api/v1/tasks/:taskId/cancel", handler.CancelTask)

	// Make request
	req, _ := http.NewRequest("POST", "/api/v1/tasks/task-123/cancel", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "InternalError", response.Error)

	mockTaskManager.AssertExpectedCalls(t)
}

func TestTasksHandler_ConvertTaskToSummary(t *testing.T) {
	now := time.Now()
	task := &Task{
		ID:         "task-123",
		Type:       "bootstrap",
		Status:     "running",
		ProjectKey: "PROJ",
		CreatedAt:  now,
		StartedAt:  &now,
		Progress: TaskProgress{
			TotalItems:      100,
			ProcessedItems:  50,
			PercentComplete: 50.0,
		},
	}

	handler := NewTasksHandler(nil)
	summary := handler.convertTaskToSummary(task)

	assert.Equal(t, "task-123", summary.ID)
	assert.Equal(t, "bootstrap", summary.Type)
	assert.Equal(t, "running", summary.Status)
	assert.Equal(t, "PROJ", summary.ProjectKey)
	assert.Equal(t, now, summary.CreatedAt)
	assert.NotNil(t, summary.StartedAt)
	assert.Nil(t, summary.CompletedAt)
	assert.Equal(t, 100, summary.Progress.TotalItems)
	assert.Equal(t, 50, summary.Progress.ProcessedItems)
	assert.Equal(t, 50.0, summary.Progress.PercentComplete)
}

func TestTasksHandler_ConvertTaskToDetails(t *testing.T) {
	now := time.Now()
	task := &Task{
		ID:         "task-123",
		Type:       "reconciliation",
		Status:     "completed",
		ProjectKey: "PROJ",
		CreatedAt:  now,
		StartedAt:  &now,
		CompletedAt: &now,
		Configuration: map[string]interface{}{
			"issueFilter":  "project = PROJ",
			"forceRefresh": true,
		},
		CreatedBy:    "user@example.com",
		ErrorMessage: "",
		Operations: []SyncOperation{
			{
				ID:            "op-1",
				IssueKey:      "PROJ-123",
				OperationType: "create",
				Status:        "completed",
			},
		},
		Progress: TaskProgress{
			TotalItems:      10,
			ProcessedItems:  10,
			PercentComplete: 100.0,
		},
	}

	handler := NewTasksHandler(nil)
	details := handler.convertTaskToDetails(task)

	assert.Equal(t, "task-123", details.ID)
	assert.Equal(t, "reconciliation", details.Type)
	assert.Equal(t, "completed", details.Status)
	assert.Equal(t, "PROJ", details.ProjectKey)
	assert.Equal(t, "user@example.com", details.CreatedBy)
	assert.Equal(t, "", details.ErrorMessage)
	assert.Len(t, details.Operations, 1)
	assert.Equal(t, "PROJ-123", details.Operations[0].IssueKey)

	// Check that configuration is properly converted
	assert.Equal(t, "project = PROJ", details.Configuration["issueFilter"])
	assert.Equal(t, true, details.Configuration["forceRefresh"])
}

func TestTasksHandler_IsCancellable(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{"pending task", "pending", true},
		{"running task", "running", true},
		{"completed task", "completed", false},
		{"failed task", "failed", false},
		{"cancelled task", "cancelled", false},
	}

	handler := NewTasksHandler(nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{Status: tt.status}
			result := handler.isCancellable(task)
			assert.Equal(t, tt.expected, result)
		})
	}
}
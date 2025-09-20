package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSyncEngine implements sync.Engine interface for testing
type MockSyncEngine struct {
	mock.Mock
}

func (m *MockSyncEngine) SynchronizeProject(ctx context.Context, repo *git.Repository, forceRefresh bool, progress chan<- SyncProgress) ([]SyncResult, error) {
	args := m.Called(ctx, repo, forceRefresh, progress)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]SyncResult), args.Error(1)
}

func (m *MockSyncEngine) Bootstrap(ctx context.Context, repo *git.Repository, progress chan<- SyncProgress) ([]SyncResult, error) {
	args := m.Called(ctx, repo, progress)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]SyncResult), args.Error(1)
}

func (m *MockSyncEngine) SynchronizeIssue(ctx context.Context, repo *git.Repository, issueKey string, forceRefresh bool) (*SyncResult, error) {
	args := m.Called(ctx, repo, issueKey, forceRefresh)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SyncResult), args.Error(1)
}

// MockTaskManager implements task management interface for testing
type MockTaskManager struct {
	mock.Mock
}

func (m *MockTaskManager) CreateTask(taskType, projectKey string, config map[string]interface{}) (*Task, error) {
	args := m.Called(taskType, projectKey, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Task), args.Error(1)
}

func (m *MockTaskManager) GetTask(taskID string) (*Task, error) {
	args := m.Called(taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Task), args.Error(1)
}

func (m *MockTaskManager) ListTasks(filters map[string]string) ([]*Task, error) {
	args := m.Called(filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Task), args.Error(1)
}

func (m *MockTaskManager) CancelTask(taskID string) error {
	args := m.Called(taskID)
	return args.Error(0)
}

func TestProjectsHandler_ListProjects(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock dependencies
	mockTaskManager := new(MockTaskManager)

	// Setup test data
	testTasks := []*Task{
		{
			ID:         "task-1",
			Type:       "bootstrap",
			Status:     "completed",
			ProjectKey: "PROJ1",
			CreatedAt:  time.Now(),
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
			CreatedAt:  time.Now(),
			Progress: TaskProgress{
				TotalItems:      50,
				ProcessedItems:  25,
				PercentComplete: 50.0,
			},
		},
	}

	mockTaskManager.On("ListTasks", map[string]string{}).Return(testTasks, nil)

	// Create handler
	handler := NewProjectsHandler(mockTaskManager, nil, nil)

	// Setup router
	router := gin.New()
	router.GET("/api/v1/projects", handler.ListProjects)

	// Make request
	req, _ := http.NewRequest("GET", "/api/v1/projects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response []ProjectSummary
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Len(t, response, 2)
	assert.Equal(t, "PROJ1", response[0].ProjectKey)
	assert.Equal(t, "Current", response[0].Status)
	assert.Equal(t, 100, response[0].SyncedIssueCount)

	assert.Equal(t, "PROJ2", response[1].ProjectKey)
	assert.Equal(t, "Syncing", response[1].Status)
	assert.Equal(t, 25, response[1].SyncedIssueCount)

	mockTaskManager.AssertExpectedCalls(t)
}

func TestProjectsHandler_GetProject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock dependencies
	mockTaskManager := new(MockTaskManager)

	// Setup test data
	testTasks := []*Task{
		{
			ID:         "task-1",
			Type:       "bootstrap",
			Status:     "completed",
			ProjectKey: "PROJ",
			CreatedAt:  time.Now().Add(-1 * time.Hour),
			CompletedAt: &time.Time{},
			Progress: TaskProgress{
				TotalItems:      150,
				ProcessedItems:  150,
				PercentComplete: 100.0,
			},
		},
		{
			ID:         "task-2",
			Type:       "reconciliation",
			Status:     "running",
			ProjectKey: "PROJ",
			CreatedAt:  time.Now().Add(-30 * time.Minute),
			Progress: TaskProgress{
				TotalItems:      10,
				ProcessedItems:  5,
				PercentComplete: 50.0,
			},
		},
	}

	mockTaskManager.On("ListTasks", map[string]string{"projectKey": "PROJ"}).Return(testTasks, nil)

	// Create handler
	handler := NewProjectsHandler(mockTaskManager, nil, nil)

	// Setup router
	router := gin.New()
	router.GET("/api/v1/projects/:projectKey", handler.GetProject)

	// Make request
	req, _ := http.NewRequest("GET", "/api/v1/projects/PROJ", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response ProjectDetails
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "PROJ", response.ProjectKey)
	assert.Equal(t, "Syncing", response.Status) // Has running task
	assert.Equal(t, 150, response.SyncedIssueCount) // From latest completed bootstrap
	assert.NotNil(t, response.LastSyncTime)
	assert.Len(t, response.RecentTasks, 2)

	mockTaskManager.AssertExpectedCalls(t)
}

func TestProjectsHandler_GetProject_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock dependencies
	mockTaskManager := new(MockTaskManager)

	// Setup test data - no tasks for project
	mockTaskManager.On("ListTasks", map[string]string{"projectKey": "NOTFOUND"}).Return([]*Task{}, nil)

	// Create handler
	handler := NewProjectsHandler(mockTaskManager, nil, nil)

	// Setup router
	router := gin.New()
	router.GET("/api/v1/projects/:projectKey", handler.GetProject)

	// Make request
	req, _ := http.NewRequest("GET", "/api/v1/projects/NOTFOUND", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "NotFound", response.Error)
	assert.Contains(t, response.Message, "NOTFOUND")

	mockTaskManager.AssertExpectedCalls(t)
}

func TestProjectsHandler_TriggerSync(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock dependencies
	mockTaskManager := new(MockTaskManager)

	// Setup test data
	testTask := &Task{
		ID:         "task-123",
		Type:       "reconciliation",
		Status:     "pending",
		ProjectKey: "PROJ",
		CreatedAt:  time.Now(),
		Configuration: map[string]interface{}{
			"issueFilter":   "status != Done",
			"forceRefresh":  false,
		},
	}

	mockTaskManager.On("CreateTask", "reconciliation", "PROJ", mock.AnythingOfType("map[string]interface {}")).
		Return(testTask, nil)

	// Create handler
	handler := NewProjectsHandler(mockTaskManager, nil, nil)

	// Setup router
	router := gin.New()
	router.POST("/api/v1/projects/:projectKey/sync", handler.TriggerSync)

	// Create request body
	syncRequest := SyncRequest{
		Type:         "reconciliation",
		ForceRefresh: false,
		IssueFilter:  "status != Done",
	}
	jsonBody, _ := json.Marshal(syncRequest)

	// Make request
	req, _ := http.NewRequest("POST", "/api/v1/projects/PROJ/sync", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response TaskResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "task-123", response.TaskID)
	assert.Equal(t, "started", response.Status)
	assert.Contains(t, response.Message, "reconciliation")

	mockTaskManager.AssertExpectedCalls(t)
}

func TestProjectsHandler_TriggerSync_Bootstrap(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock dependencies
	mockTaskManager := new(MockTaskManager)

	// Setup test data
	testTask := &Task{
		ID:         "task-bootstrap-123",
		Type:       "bootstrap",
		Status:     "pending",
		ProjectKey: "PROJ",
		CreatedAt:  time.Now(),
		Configuration: map[string]interface{}{
			"forceRefresh": true,
		},
	}

	mockTaskManager.On("CreateTask", "bootstrap", "PROJ", mock.AnythingOfType("map[string]interface {}")).
		Return(testTask, nil)

	// Create handler
	handler := NewProjectsHandler(mockTaskManager, nil, nil)

	// Setup router
	router := gin.New()
	router.POST("/api/v1/projects/:projectKey/sync", handler.TriggerSync)

	// Create request body
	syncRequest := SyncRequest{
		Type:         "bootstrap",
		ForceRefresh: true,
	}
	jsonBody, _ := json.Marshal(syncRequest)

	// Make request
	req, _ := http.NewRequest("POST", "/api/v1/projects/PROJ/sync", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response TaskResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "task-bootstrap-123", response.TaskID)
	assert.Contains(t, response.Message, "bootstrap")

	mockTaskManager.AssertExpectedCalls(t)
}

func TestProjectsHandler_TriggerSync_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create handler
	handler := NewProjectsHandler(nil, nil, nil)

	// Setup router
	router := gin.New()
	router.POST("/api/v1/projects/:projectKey/sync", handler.TriggerSync)

	// Test invalid JSON
	req, _ := http.NewRequest("POST", "/api/v1/projects/PROJ/sync", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "InvalidRequest", response.Error)
}

func TestProjectsHandler_TriggerSync_InvalidType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create handler
	handler := NewProjectsHandler(nil, nil, nil)

	// Setup router
	router := gin.New()
	router.POST("/api/v1/projects/:projectKey/sync", handler.TriggerSync)

	// Create request with invalid sync type
	syncRequest := SyncRequest{
		Type: "invalid_type",
	}
	jsonBody, _ := json.Marshal(syncRequest)

	// Make request
	req, _ := http.NewRequest("POST", "/api/v1/projects/PROJ/sync", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "InvalidRequest", response.Error)
	assert.Contains(t, response.Message, "invalid sync type")
}

func TestProjectsHandler_TriggerSync_TaskCreationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock dependencies
	mockTaskManager := new(MockTaskManager)

	// Mock task creation error
	mockTaskManager.On("CreateTask", "reconciliation", "PROJ", mock.AnythingOfType("map[string]interface {}")).
		Return(nil, errors.New("task creation failed"))

	// Create handler
	handler := NewProjectsHandler(mockTaskManager, nil, nil)

	// Setup router
	router := gin.New()
	router.POST("/api/v1/projects/:projectKey/sync", handler.TriggerSync)

	// Create request body
	syncRequest := SyncRequest{
		Type:         "reconciliation",
		ForceRefresh: false,
	}
	jsonBody, _ := json.Marshal(syncRequest)

	// Make request
	req, _ := http.NewRequest("POST", "/api/v1/projects/PROJ/sync", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "InternalError", response.Error)
	assert.Contains(t, response.Message, "task creation failed")

	mockTaskManager.AssertExpectedCalls(t)
}

func TestProjectsHandler_CalculateProjectStatus(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []*Task
		expected string
	}{
		{
			name:     "no tasks",
			tasks:    []*Task{},
			expected: "Pending",
		},
		{
			name: "running task",
			tasks: []*Task{
				{Status: "running", Type: "bootstrap"},
			},
			expected: "Syncing",
		},
		{
			name: "completed tasks only",
			tasks: []*Task{
				{Status: "completed", Type: "bootstrap"},
				{Status: "completed", Type: "reconciliation"},
			},
			expected: "Current",
		},
		{
			name: "failed task",
			tasks: []*Task{
				{Status: "failed", Type: "bootstrap"},
			},
			expected: "Error",
		},
		{
			name: "mixed statuses",
			tasks: []*Task{
				{Status: "completed", Type: "bootstrap"},
				{Status: "running", Type: "reconciliation"},
			},
			expected: "Syncing",
		},
	}

	handler := NewProjectsHandler(nil, nil, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := handler.calculateProjectStatus(tt.tasks)
			assert.Equal(t, tt.expected, status)
		})
	}
}

func TestProjectsHandler_GetSyncedIssueCount(t *testing.T) {
	tasks := []*Task{
		{
			Type:   "bootstrap",
			Status: "completed",
			Progress: TaskProgress{
				TotalItems:     100,
				ProcessedItems: 100,
			},
			CompletedAt: &time.Time{},
		},
		{
			Type:   "reconciliation",
			Status: "running",
			Progress: TaskProgress{
				TotalItems:     20,
				ProcessedItems: 10,
			},
		},
	}

	handler := NewProjectsHandler(nil, nil, nil)
	count := handler.getSyncedIssueCount(tasks)

	// Should return count from latest completed bootstrap/reconciliation
	assert.Equal(t, 100, count)
}

func TestProjectsHandler_GetLastSyncTime(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-1 * time.Hour)

	tasks := []*Task{
		{
			Type:        "bootstrap",
			Status:      "completed",
			CompletedAt: &oneHourAgo,
		},
		{
			Type:   "reconciliation",
			Status: "running",
		},
	}

	handler := NewProjectsHandler(nil, nil, nil)
	lastSync := handler.getLastSyncTime(tasks)

	assert.NotNil(t, lastSync)
	assert.Equal(t, oneHourAgo.Unix(), lastSync.Unix())
}
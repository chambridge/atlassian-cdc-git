package sync

import (
	"context"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockJIRAClient implements jira.Client interface for testing
type MockJIRAClient struct {
	mock.Mock
}

func (m *MockJIRAClient) GetIssue(ctx context.Context, issueKey string) (*Issue, error) {
	args := m.Called(ctx, issueKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Issue), args.Error(1)
}

func (m *MockJIRAClient) SearchIssues(ctx context.Context, jql string, startAt, maxResults int) (*SearchResult, error) {
	args := m.Called(ctx, jql, startAt, maxResults)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SearchResult), args.Error(1)
}

func (m *MockJIRAClient) GetProject(ctx context.Context, projectKey string) (*Project, error) {
	args := m.Called(ctx, projectKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Project), args.Error(1)
}

// MockGitOperations implements git.Operations interface for testing
type MockGitOperations struct {
	mock.Mock
}

func (m *MockGitOperations) CloneRepository(ctx context.Context, url, dir, branch string) (*git.Repository, error) {
	args := m.Called(ctx, url, dir, branch)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*git.Repository), args.Error(1)
}

func (m *MockGitOperations) CreateOrUpdateIssueFile(ctx context.Context, repo *git.Repository, issue IssueData) (string, string, error) {
	args := m.Called(ctx, repo, issue)
	return args.String(0), args.String(1), args.Error(2)
}

func (m *MockGitOperations) DeleteIssueFile(ctx context.Context, repo *git.Repository, issueKey string) (string, error) {
	args := m.Called(ctx, repo, issueKey)
	return args.String(0), args.Error(1)
}

func (m *MockGitOperations) PushChanges(ctx context.Context, repo *git.Repository, branch string) error {
	args := m.Called(ctx, repo, branch)
	return args.Error(0)
}

func TestEngine_SynchronizeIssue(t *testing.T) {
	// Setup mocks
	mockJIRA := new(MockJIRAClient)
	mockGit := new(MockGitOperations)

	// Create test repository
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	// Setup test data
	testIssue := &Issue{
		Key:         "PROJ-123",
		Summary:     "Test Issue",
		Description: "Test Description",
		Status:      "In Progress",
		IssueType:   "Story",
		Priority:    "Medium",
		Assignee:    "john.doe@example.com",
		Reporter:    "jane.smith@example.com",
		Labels:      []string{"feature"},
		Components:  []string{"backend"},
		FixVersions: []string{"1.0.0"},
		UpdatedAt:   time.Now(),
	}

	// Mock JIRA client calls
	mockJIRA.On("GetIssue", mock.Anything, "PROJ-123").Return(testIssue, nil)

	// Mock Git operations
	mockGit.On("CreateOrUpdateIssueFile", mock.Anything, repo, mock.AnythingOfType("IssueData")).
		Return("PROJ-123.md", "abc123", nil)
	mockGit.On("PushChanges", mock.Anything, repo, "main").Return(nil)

	// Create engine
	config := Config{
		ProjectKey:     "PROJ",
		GitRepository: "https://github.com/test/repo.git",
		GitBranch:     "main",
		BatchSize:     10,
		MaxRetries:    3,
	}

	engine := NewEngine(config, mockJIRA, mockGit)

	// Test synchronization
	ctx := context.Background()
	result, err := engine.SynchronizeIssue(ctx, repo, "PROJ-123", false)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "PROJ-123", result.IssueKey)
	assert.Equal(t, OperationTypeUpdate, result.OperationType)
	assert.Equal(t, OperationStatusCompleted, result.Status)
	assert.Equal(t, "PROJ-123.md", result.GitFilePath)
	assert.Equal(t, "abc123", result.GitCommitHash)

	// Verify mock calls
	mockJIRA.AssertExpectedCalls(t)
	mockGit.AssertExpectedCalls(t)
}

func TestEngine_SynchronizeProject(t *testing.T) {
	// Setup mocks
	mockJIRA := new(MockJIRAClient)
	mockGit := new(MockGitOperations)

	// Create test repository
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	// Setup test data
	searchResult := &SearchResult{
		Issues: []Issue{
			{
				Key:       "PROJ-1",
				Summary:   "Issue 1",
				Status:    "Open",
				UpdatedAt: time.Now(),
			},
			{
				Key:       "PROJ-2",
				Summary:   "Issue 2",
				Status:    "In Progress",
				UpdatedAt: time.Now(),
			},
		},
		Total:      2,
		MaxResults: 50,
		StartAt:    0,
	}

	// Mock JIRA client calls
	mockJIRA.On("SearchIssues", mock.Anything, "project = PROJ AND status != Done", 0, 50).
		Return(searchResult, nil)

	// Mock Git operations for each issue
	for _, issue := range searchResult.Issues {
		mockGit.On("CreateOrUpdateIssueFile", mock.Anything, repo, mock.MatchedBy(func(data IssueData) bool {
			return data.Key == issue.Key
		})).Return(issue.Key+".md", "commit123", nil)
	}
	mockGit.On("PushChanges", mock.Anything, repo, "main").Return(nil)

	// Create engine
	config := Config{
		ProjectKey:     "PROJ",
		GitRepository: "https://github.com/test/repo.git",
		GitBranch:     "main",
		BatchSize:     10,
		MaxRetries:    3,
	}

	engine := NewEngine(config, mockJIRA, mockGit)

	// Test project synchronization
	ctx := context.Background()
	progress := make(chan SyncProgress, 10)
	
	results, err := engine.SynchronizeProject(ctx, repo, false, progress)

	require.NoError(t, err)
	assert.Len(t, results, 2)

	// Verify all issues were processed
	for _, result := range results {
		assert.Equal(t, OperationStatusCompleted, result.Status)
		assert.Contains(t, []string{"PROJ-1", "PROJ-2"}, result.IssueKey)
	}

	// Verify progress updates were sent
	close(progress)
	progressCount := 0
	for range progress {
		progressCount++
	}
	assert.Greater(t, progressCount, 0)

	// Verify mock calls
	mockJIRA.AssertExpectedCalls(t)
	mockGit.AssertExpectedCalls(t)
}

func TestEngine_Bootstrap(t *testing.T) {
	// Setup mocks
	mockJIRA := new(MockJIRAClient)
	mockGit := new(MockGitOperations)

	// Create test repository
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	// Setup test data - larger dataset for bootstrap
	issues := make([]Issue, 150) // Test batch processing
	for i := 0; i < 150; i++ {
		issues[i] = Issue{
			Key:       fmt.Sprintf("PROJ-%d", i+1),
			Summary:   fmt.Sprintf("Issue %d", i+1),
			Status:    "Open",
			UpdatedAt: time.Now(),
		}
	}

	// Mock paginated search results
	page1 := &SearchResult{
		Issues:     issues[:50],
		Total:      150,
		MaxResults: 50,
		StartAt:    0,
	}
	page2 := &SearchResult{
		Issues:     issues[50:100],
		Total:      150,
		MaxResults: 50,
		StartAt:    50,
	}
	page3 := &SearchResult{
		Issues:     issues[100:150],
		Total:      150,
		MaxResults: 50,
		StartAt:    100,
	}

	mockJIRA.On("SearchIssues", mock.Anything, "project = PROJ", 0, 50).Return(page1, nil)
	mockJIRA.On("SearchIssues", mock.Anything, "project = PROJ", 50, 50).Return(page2, nil)
	mockJIRA.On("SearchIssues", mock.Anything, "project = PROJ", 100, 50).Return(page3, nil)

	// Mock Git operations for all issues
	for _, issue := range issues {
		mockGit.On("CreateOrUpdateIssueFile", mock.Anything, repo, mock.MatchedBy(func(data IssueData) bool {
			return data.Key == issue.Key
		})).Return(issue.Key+".md", "commit123", nil)
	}
	mockGit.On("PushChanges", mock.Anything, repo, "main").Return(nil).Times(3) // One per batch

	// Create engine with smaller batch size for testing
	config := Config{
		ProjectKey:     "PROJ",
		GitRepository: "https://github.com/test/repo.git",
		GitBranch:     "main",
		BatchSize:     50,
		MaxRetries:    3,
	}

	engine := NewEngine(config, mockJIRA, mockGit)

	// Test bootstrap
	ctx := context.Background()
	progress := make(chan SyncProgress, 200)
	
	results, err := engine.Bootstrap(ctx, repo, progress)

	require.NoError(t, err)
	assert.Len(t, results, 150)

	// Verify all issues were processed
	processedKeys := make(map[string]bool)
	for _, result := range results {
		assert.Equal(t, OperationStatusCompleted, result.Status)
		processedKeys[result.IssueKey] = true
	}
	assert.Len(t, processedKeys, 150)

	// Verify progress updates
	close(progress)
	var finalProgress SyncProgress
	for p := range progress {
		finalProgress = p
	}
	assert.Equal(t, 150, finalProgress.TotalItems)
	assert.Equal(t, 150, finalProgress.ProcessedItems)
	assert.Equal(t, float64(100), finalProgress.PercentComplete)

	// Verify mock calls
	mockJIRA.AssertExpectedCalls(t)
	mockGit.AssertExpectedCalls(t)
}

func TestEngine_HandleErrors(t *testing.T) {
	// Setup mocks
	mockJIRA := new(MockJIRAClient)
	mockGit := new(MockGitOperations)

	// Create test repository
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	// Mock JIRA error
	mockJIRA.On("GetIssue", mock.Anything, "PROJ-404").Return(nil, errors.New("issue not found"))

	// Create engine
	config := Config{
		ProjectKey:     "PROJ",
		GitRepository: "https://github.com/test/repo.git",
		GitBranch:     "main",
		BatchSize:     10,
		MaxRetries:    3,
	}

	engine := NewEngine(config, mockJIRA, mockGit)

	// Test error handling
	ctx := context.Background()
	result, err := engine.SynchronizeIssue(ctx, repo, "PROJ-404", false)

	assert.Error(t, err)
	assert.Nil(t, result)

	// Verify mock calls
	mockJIRA.AssertExpectedCalls(t)
}

func TestEngine_ConvertJIRAIssue(t *testing.T) {
	jiraIssue := &Issue{
		Key:         "PROJ-123",
		Summary:     "Test Issue",
		Description: "Test Description",
		Status:      "In Progress",
		IssueType:   "Story",
		Priority:    "High",
		Assignee:    "john.doe@example.com",
		Reporter:    "jane.smith@example.com",
		Labels:      []string{"feature", "urgent"},
		Components:  []string{"api", "frontend"},
		FixVersions: []string{"2.0.0", "2.1.0"},
		ParentKey:   "PROJ-100",
		UpdatedAt:   time.Date(2025, 9, 19, 14, 30, 0, 0, time.UTC),
	}

	// Create engine
	config := Config{
		ProjectKey: "PROJ",
	}
	engine := NewEngine(config, nil, nil)

	// Convert issue
	issueData := engine.convertJIRAIssue(jiraIssue)

	assert.Equal(t, "PROJ-123", issueData.Key)
	assert.Equal(t, "Test Issue", issueData.Summary)
	assert.Equal(t, "Test Description", issueData.Description)
	assert.Equal(t, "In Progress", issueData.Status)
	assert.Equal(t, "Story", issueData.IssueType)
	assert.Equal(t, "High", issueData.Priority)
	assert.Equal(t, "john.doe@example.com", issueData.Assignee)
	assert.Equal(t, "jane.smith@example.com", issueData.Reporter)
	assert.Equal(t, []string{"feature", "urgent"}, issueData.Labels)
	assert.Equal(t, []string{"api", "frontend"}, issueData.Components)
	assert.Equal(t, []string{"2.0.0", "2.1.0"}, issueData.FixVersions)
	assert.Equal(t, "PROJ-100", issueData.ParentKey)
	assert.Equal(t, jiraIssue.UpdatedAt, issueData.UpdatedAt)
}

func TestEngine_FilterActiveIssues(t *testing.T) {
	issues := []Issue{
		{Key: "PROJ-1", Status: "Open"},
		{Key: "PROJ-2", Status: "In Progress"},
		{Key: "PROJ-3", Status: "Done"},
		{Key: "PROJ-4", Status: "Closed"},
		{Key: "PROJ-5", Status: "Resolved"},
		{Key: "PROJ-6", Status: "To Do"},
	}

	// Create engine
	config := Config{
		ProjectKey: "PROJ",
	}
	engine := NewEngine(config, nil, nil)

	// Test filtering active issues
	activeIssues := engine.filterActiveIssues(issues)

	// Should exclude Done, Closed, Resolved
	expectedKeys := []string{"PROJ-1", "PROJ-2", "PROJ-6"}
	actualKeys := make([]string, len(activeIssues))
	for i, issue := range activeIssues {
		actualKeys[i] = issue.Key
	}

	assert.ElementsMatch(t, expectedKeys, actualKeys)
}
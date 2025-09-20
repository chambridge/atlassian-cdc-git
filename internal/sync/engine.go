/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sync

import (
	"context"
	"fmt"
	"time"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
	"github.com/company/jira-cdc-operator/internal/git"
	"github.com/company/jira-cdc-operator/internal/jira"
)

// Engine orchestrates the synchronization between JIRA and Git
type Engine struct {
	jiraClient  *jira.Client
	gitManager  *git.Manager
	taskManager TaskManager
	progressTracker ProgressTracker
}

// SyncOperation represents a single synchronization operation
type SyncOperation struct {
	ID           string
	IssueKey     string
	OperationType string // create, update, delete, move
	Status       string // pending, processing, completed, failed, skipped
	JiraData     *jira.Issue
	ErrorDetails string
	ProcessedAt  *time.Time
	RetryCount   int
}

// SyncResult represents the result of a synchronization operation
type SyncResult struct {
	Success      bool
	Operation    *SyncOperation
	CommitHash   string
	ErrorMessage string
}

// SyncConfig represents synchronization configuration
type SyncConfig struct {
	ProjectKey      string
	ForceRefresh    bool
	IssueFilter     string
	ActiveIssuesOnly bool
	BatchSize       int
	MaxRetries      int
}

// NewEngine creates a new synchronization engine
func NewEngine(jiraClient *jira.Client, gitManager *git.Manager, taskManager TaskManager, progressTracker ProgressTracker) *Engine {
	return &Engine{
		jiraClient:      jiraClient,
		gitManager:      gitManager,
		taskManager:     taskManager,
		progressTracker: progressTracker,
	}
}

// Bootstrap performs initial synchronization of all issues in a project
func (e *Engine) Bootstrap(ctx context.Context, config SyncConfig) error {
	// Create bootstrap task
	task, err := e.taskManager.CreateTask(ctx, TaskInfo{
		Type:        "bootstrap",
		ProjectKey:  config.ProjectKey,
		Status:      "running",
		StartedAt:   time.Now(),
		Configuration: TaskConfiguration{
			ForceRefresh:     config.ForceRefresh,
			IssueFilter:      config.IssueFilter,
			ActiveIssuesOnly: config.ActiveIssuesOnly,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create bootstrap task: %w", err)
	}

	defer func() {
		// Update task completion status
		task.CompletedAt = &[]time.Time{time.Now()}[0]
		if err != nil {
			task.Status = "failed"
			task.ErrorMessage = err.Error()
		} else {
			task.Status = "completed"
		}
		e.taskManager.UpdateTask(ctx, *task)
	}()

	// Initialize git repository
	if err := e.gitManager.Clone(ctx); err != nil {
		return fmt.Errorf("failed to clone git repository: %w", err)
	}

	// Get all issues from JIRA
	issues, err := e.getAllProjectIssues(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to get project issues: %w", err)
	}

	// Update task progress
	task.Progress = &TaskProgress{
		TotalItems:      int32(len(issues)),
		ProcessedItems:  0,
		PercentComplete: 0,
	}
	e.taskManager.UpdateTask(ctx, *task)

	// Process issues in batches
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 50 // Default batch size
	}

	for i := 0; i < len(issues); i += batchSize {
		end := i + batchSize
		if end > len(issues) {
			end = len(issues)
		}

		batch := issues[i:end]
		if err := e.processBatch(ctx, batch, config, task); err != nil {
			return fmt.Errorf("failed to process batch %d-%d: %w", i, end, err)
		}

		// Update progress
		task.Progress.ProcessedItems = int32(end)
		task.Progress.PercentComplete = float32(end) / float32(len(issues)) * 100
		e.taskManager.UpdateTask(ctx, *task)

		// Check for cancellation
		if task.Status == "cancelled" {
			return fmt.Errorf("bootstrap task was cancelled")
		}
	}

	// Commit all changes
	commitInfo := git.CommitInfo{
		Message: fmt.Sprintf("feat: bootstrap sync of %d issues from project %s", len(issues), config.ProjectKey),
		Author: git.AuthorInfo{
			Name:  "JIRA CDC Operator",
			Email: "jiracdc@example.com",
		},
	}

	commitHash, err := e.gitManager.CommitChanges(ctx, commitInfo)
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	// Push changes
	if err := e.gitManager.Push(ctx); err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}

	// Update task with final commit hash
	task.FinalCommitHash = commitHash
	e.taskManager.UpdateTask(ctx, *task)

	return nil
}

// Reconcile performs incremental synchronization to update changed issues
func (e *Engine) Reconcile(ctx context.Context, config SyncConfig) error {
	// Create reconciliation task
	task, err := e.taskManager.CreateTask(ctx, TaskInfo{
		Type:        "reconciliation",
		ProjectKey:  config.ProjectKey,
		Status:      "running",
		StartedAt:   time.Now(),
		Configuration: TaskConfiguration{
			ForceRefresh:     config.ForceRefresh,
			IssueFilter:      config.IssueFilter,
			ActiveIssuesOnly: config.ActiveIssuesOnly,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create reconciliation task: %w", err)
	}

	defer func() {
		// Update task completion status
		task.CompletedAt = &[]time.Time{time.Now()}[0]
		if err != nil {
			task.Status = "failed"
			task.ErrorMessage = err.Error()
		} else {
			task.Status = "completed"
		}
		e.taskManager.UpdateTask(ctx, *task)
	}()

	// Pull latest changes from git
	if err := e.gitManager.Pull(ctx); err != nil {
		return fmt.Errorf("failed to pull git repository: %w", err)
	}

	// Get changed issues based on last sync time
	issues, err := e.getChangedIssues(ctx, config)
	if err != nil {
		return fmt.Errorf("failed to get changed issues: %w", err)
	}

	if len(issues) == 0 {
		// No changes to sync
		return nil
	}

	// Update task progress
	task.Progress = &TaskProgress{
		TotalItems:      int32(len(issues)),
		ProcessedItems:  0,
		PercentComplete: 0,
	}
	e.taskManager.UpdateTask(ctx, *task)

	// Process changed issues
	hasChanges := false
	for i, issue := range issues {
		if err := e.processIssue(ctx, issue, config); err != nil {
			// Log error but continue processing
			fmt.Printf("Failed to process issue %s: %v\n", issue.Key, err)
		} else {
			hasChanges = true
		}

		// Update progress
		task.Progress.ProcessedItems = int32(i + 1)
		task.Progress.PercentComplete = float32(i+1) / float32(len(issues)) * 100
		e.taskManager.UpdateTask(ctx, *task)

		// Check for cancellation
		if task.Status == "cancelled" {
			return fmt.Errorf("reconciliation task was cancelled")
		}
	}

	// Commit changes if any
	if hasChanges {
		commitInfo := git.CommitInfo{
			Message: fmt.Sprintf("feat: reconcile %d changed issues from project %s", len(issues), config.ProjectKey),
			Author: git.AuthorInfo{
				Name:  "JIRA CDC Operator",
				Email: "jiracdc@example.com",
			},
		}

		commitHash, err := e.gitManager.CommitChanges(ctx, commitInfo)
		if err != nil {
			return fmt.Errorf("failed to commit changes: %w", err)
		}

		// Push changes
		if err := e.gitManager.Push(ctx); err != nil {
			return fmt.Errorf("failed to push changes: %w", err)
		}

		// Update task with final commit hash
		task.FinalCommitHash = commitHash
		e.taskManager.UpdateTask(ctx, *task)
	}

	return nil
}

// SyncSingleIssue synchronizes a single issue
func (e *Engine) SyncSingleIssue(ctx context.Context, issueKey string) (*SyncResult, error) {
	// Get issue from JIRA
	issue, err := e.jiraClient.GetIssue(ctx, issueKey, []string{
		"summary", "description", "status", "issuetype", 
		"assignee", "reporter", "priority", "labels", 
		"components", "fixVersions", "parent", "created", "updated",
	})
	if err != nil {
		return &SyncResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to get issue from JIRA: %v", err),
		}, err
	}

	// Process the issue
	config := SyncConfig{ForceRefresh: false}
	if err := e.processIssue(ctx, *issue, config); err != nil {
		return &SyncResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to process issue: %v", err),
		}, err
	}

	return &SyncResult{
		Success: true,
	}, nil
}

// getAllProjectIssues retrieves all issues for a project
func (e *Engine) getAllProjectIssues(ctx context.Context, config SyncConfig) ([]jira.Issue, error) {
	var allIssues []jira.Issue
	startAt := 0
	maxResults := 50

	for {
		result, err := e.jiraClient.GetProjectIssues(ctx, config.ProjectKey, startAt, maxResults, config.ActiveIssuesOnly)
		if err != nil {
			return nil, fmt.Errorf("failed to get project issues: %w", err)
		}

		allIssues = append(allIssues, result.Issues...)

		if startAt+maxResults >= result.Total {
			break
		}

		startAt += maxResults
	}

	return allIssues, nil
}

// getChangedIssues retrieves issues that have changed since last sync
func (e *Engine) getChangedIssues(ctx context.Context, config SyncConfig) ([]jira.Issue, error) {
	// For now, get all issues and filter based on update time
	// In a real implementation, this would use stored sync timestamps
	
	if config.ForceRefresh {
		return e.getAllProjectIssues(ctx, config)
	}

	// Build JQL query for recently updated issues
	jql := fmt.Sprintf("project = %s AND updated >= -24h", config.ProjectKey)
	if config.ActiveIssuesOnly {
		jql += " AND status != Done AND status != Closed AND status != Resolved"
	}
	if config.IssueFilter != "" {
		jql += " AND " + config.IssueFilter
	}

	searchRequest := jira.SearchRequest{
		JQL:        jql,
		StartAt:    0,
		MaxResults: 1000,
		Fields: []string{
			"summary", "description", "status", "issuetype", 
			"assignee", "reporter", "priority", "labels", 
			"components", "fixVersions", "parent", "created", "updated",
		},
	}

	result, err := e.jiraClient.SearchIssues(ctx, searchRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to search for changed issues: %w", err)
	}

	return result.Issues, nil
}

// processBatch processes a batch of issues
func (e *Engine) processBatch(ctx context.Context, issues []jira.Issue, config SyncConfig, task *TaskInfo) error {
	for _, issue := range issues {
		if err := e.processIssue(ctx, issue, config); err != nil {
			// Log error but continue processing
			fmt.Printf("Failed to process issue %s: %v\n", issue.Key, err)
		}

		// Check for cancellation
		currentTask, err := e.taskManager.GetTask(ctx, task.ID)
		if err == nil && currentTask.Status == "cancelled" {
			task.Status = "cancelled"
			return fmt.Errorf("task was cancelled")
		}
	}

	return nil
}

// processIssue processes a single issue
func (e *Engine) processIssue(ctx context.Context, issue jira.Issue, config SyncConfig) error {
	// Convert JIRA issue to git issue data
	issueData := git.IssueData{
		Key:         issue.Key,
		Summary:     issue.Fields.Summary,
		Description: issue.Fields.Description,
		Status:      issue.Fields.Status.Name,
		Priority:    issue.Fields.Priority.Name,
		Labels:      issue.Fields.Labels,
		Created:     parseJiraTime(issue.Fields.Created),
		Updated:     parseJiraTime(issue.Fields.Updated),
	}

	// Set assignee and reporter
	if issue.Fields.Assignee != nil {
		issueData.Assignee = issue.Fields.Assignee.DisplayName
	}
	if issue.Fields.Reporter != nil {
		issueData.Reporter = issue.Fields.Reporter.DisplayName
	}

	// Set components
	for _, component := range issue.Fields.Components {
		issueData.Components = append(issueData.Components, component.Name)
	}

	// Create or update issue file
	if err := e.gitManager.CreateIssueFile(ctx, issueData); err != nil {
		return fmt.Errorf("failed to create issue file: %w", err)
	}

	return nil
}

// parseJiraTime parses JIRA timestamp format
func parseJiraTime(jiraTime string) time.Time {
	// JIRA uses ISO 8601 format: 2006-01-02T15:04:05.000-0700
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", jiraTime)
	if err != nil {
		// Fallback to basic format
		t, _ = time.Parse(time.RFC3339, jiraTime)
	}
	return t
}
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

	"github.com/google/uuid"
	"github.com/company/jira-cdc-operator/internal/git"
	"github.com/company/jira-cdc-operator/internal/jira"
)

// OperationType represents the type of sync operation
type OperationType string

const (
	OperationTypeCreate OperationType = "create"
	OperationTypeUpdate OperationType = "update"
	OperationTypeDelete OperationType = "delete"
	OperationTypeMove   OperationType = "move"
)

// OperationStatus represents the status of a sync operation
type OperationStatus string

const (
	OperationStatusPending    OperationStatus = "pending"
	OperationStatusProcessing OperationStatus = "processing"
	OperationStatusCompleted  OperationStatus = "completed"
	OperationStatusFailed     OperationStatus = "failed"
	OperationStatusSkipped    OperationStatus = "skipped"
)

// SyncOperation represents a single synchronization operation
type SyncOperation struct {
	ID            string
	TaskID        string
	IssueKey      string
	Type          OperationType
	Status        OperationStatus
	JiraData      *jira.Issue
	GitData       *git.IssueData
	ErrorDetails  string
	ProcessedAt   *time.Time
	RetryCount    int
	MaxRetries    int
	Priority      int
	Dependencies  []string
	Metadata      map[string]interface{}
}

// OperationResult represents the result of processing a sync operation
type OperationResult struct {
	Success        bool
	Operation      *SyncOperation
	CommitRequired bool
	CommitHash     string
	ErrorMessage   string
	FilesModified  []string
	BytesProcessed int64
}

// BatchOperationResult represents the result of processing a batch of operations
type BatchOperationResult struct {
	TotalOperations    int
	SuccessfulOps      int
	FailedOps          int
	SkippedOps         int
	Operations         []*OperationResult
	BatchCommitHash    string
	TotalFilesModified int
	TotalBytesProcessed int64
	ProcessingDuration time.Duration
}

// OperationProcessor processes sync operations
type OperationProcessor interface {
	// ProcessOperation processes a single sync operation
	ProcessOperation(ctx context.Context, operation *SyncOperation) (*OperationResult, error)

	// ProcessBatch processes a batch of sync operations
	ProcessBatch(ctx context.Context, operations []*SyncOperation) (*BatchOperationResult, error)

	// ValidateOperation validates a sync operation before processing
	ValidateOperation(ctx context.Context, operation *SyncOperation) error

	// CreateOperation creates a sync operation from JIRA issue data
	CreateOperation(ctx context.Context, taskID string, issue *jira.Issue, opType OperationType) (*SyncOperation, error)

	// RetryOperation retries a failed operation
	RetryOperation(ctx context.Context, operation *SyncOperation) (*OperationResult, error)

	// GetOperationMetrics returns metrics for operations
	GetOperationMetrics(ctx context.Context) (*OperationMetrics, error)
}

// OperationMetrics represents operation processing metrics
type OperationMetrics struct {
	TotalOperations      int64
	SuccessfulOperations int64
	FailedOperations     int64
	SkippedOperations    int64
	AverageProcessingTime time.Duration
	TotalBytesProcessed  int64
	OperationsPerSecond  float64
}

// OperationProcessorImpl is the concrete implementation of OperationProcessor
type OperationProcessorImpl struct {
	jiraClient      *jira.Client
	gitManager      *git.Manager
	taskManager     TaskManager
	progressTracker ProgressTracker
	config          OperationConfig
	metrics         *OperationMetrics
}

// OperationConfig represents configuration for operation processing
type OperationConfig struct {
	MaxRetries          int
	RetryDelay          time.Duration
	BatchSize           int
	ConcurrentOps       int
	TimeoutPerOperation time.Duration
	CommitBatches       bool
	ValidateBeforeCommit bool
}

// NewOperationProcessor creates a new operation processor
func NewOperationProcessor(
	jiraClient *jira.Client,
	gitManager *git.Manager,
	taskManager TaskManager,
	progressTracker ProgressTracker,
	config OperationConfig,
) OperationProcessor {
	return &OperationProcessorImpl{
		jiraClient:      jiraClient,
		gitManager:      gitManager,
		taskManager:     taskManager,
		progressTracker: progressTracker,
		config:          config,
		metrics: &OperationMetrics{
			TotalOperations:      0,
			SuccessfulOperations: 0,
			FailedOperations:     0,
			SkippedOperations:    0,
		},
	}
}

// ProcessOperation processes a single sync operation
func (op *OperationProcessorImpl) ProcessOperation(ctx context.Context, operation *SyncOperation) (*OperationResult, error) {
	startTime := time.Now()
	operation.Status = OperationStatusProcessing

	// Update metrics
	op.metrics.TotalOperations++

	// Validate operation
	if err := op.ValidateOperation(ctx, operation); err != nil {
		return op.handleOperationError(operation, fmt.Errorf("validation failed: %w", err))
	}

	// Process based on operation type
	result, err := op.processOperationByType(ctx, operation)
	if err != nil {
		return op.handleOperationError(operation, err)
	}

	// Update operation status
	operation.Status = OperationStatusCompleted
	operation.ProcessedAt = &startTime
	result.Success = true

	// Update metrics
	op.metrics.SuccessfulOperations++
	processingTime := time.Since(startTime)
	op.updateAverageProcessingTime(processingTime)

	return result, nil
}

// ProcessBatch processes a batch of sync operations
func (op *OperationProcessorImpl) ProcessBatch(ctx context.Context, operations []*SyncOperation) (*BatchOperationResult, error) {
	startTime := time.Now()
	result := &BatchOperationResult{
		TotalOperations: len(operations),
		Operations:      make([]*OperationResult, 0, len(operations)),
	}

	// Process operations
	for _, operation := range operations {
		opResult, err := op.ProcessOperation(ctx, operation)
		if err != nil {
			result.FailedOps++
			opResult = &OperationResult{
				Success:      false,
				Operation:    operation,
				ErrorMessage: err.Error(),
			}
		} else if operation.Status == OperationStatusSkipped {
			result.SkippedOps++
		} else {
			result.SuccessfulOps++
		}

		result.Operations = append(result.Operations, opResult)
		result.TotalFilesModified += len(opResult.FilesModified)
		result.TotalBytesProcessed += opResult.BytesProcessed

		// Check for cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
	}

	// Commit batch if configured
	if op.config.CommitBatches && result.SuccessfulOps > 0 {
		commitHash, err := op.commitBatch(ctx, operations, result)
		if err != nil {
			return result, fmt.Errorf("failed to commit batch: %w", err)
		}
		result.BatchCommitHash = commitHash
	}

	result.ProcessingDuration = time.Since(startTime)
	return result, nil
}

// ValidateOperation validates a sync operation before processing
func (op *OperationProcessorImpl) ValidateOperation(ctx context.Context, operation *SyncOperation) error {
	if operation == nil {
		return fmt.Errorf("operation is nil")
	}

	if operation.IssueKey == "" {
		return fmt.Errorf("issue key is required")
	}

	if operation.Type == "" {
		return fmt.Errorf("operation type is required")
	}

	if operation.JiraData == nil && operation.Type != OperationTypeDelete {
		return fmt.Errorf("JIRA data is required for %s operations", operation.Type)
	}

	// Validate dependencies
	for _, depID := range operation.Dependencies {
		// Check if dependency is satisfied
		if !op.isDependencySatisfied(ctx, depID) {
			return fmt.Errorf("dependency %s not satisfied", depID)
		}
	}

	return nil
}

// CreateOperation creates a sync operation from JIRA issue data
func (op *OperationProcessorImpl) CreateOperation(ctx context.Context, taskID string, issue *jira.Issue, opType OperationType) (*SyncOperation, error) {
	operation := &SyncOperation{
		ID:         uuid.New().String(),
		TaskID:     taskID,
		IssueKey:   issue.Key,
		Type:       opType,
		Status:     OperationStatusPending,
		JiraData:   issue,
		RetryCount: 0,
		MaxRetries: op.config.MaxRetries,
		Priority:   op.calculateOperationPriority(issue),
		Metadata:   make(map[string]interface{}),
	}

	// Convert JIRA data to git data
	gitData, err := op.convertJiraToGitData(issue)
	if err != nil {
		return nil, fmt.Errorf("failed to convert JIRA data: %w", err)
	}
	operation.GitData = gitData

	return operation, nil
}

// RetryOperation retries a failed operation
func (op *OperationProcessorImpl) RetryOperation(ctx context.Context, operation *SyncOperation) (*OperationResult, error) {
	if operation.RetryCount >= operation.MaxRetries {
		return nil, fmt.Errorf("operation %s exceeded maximum retries (%d)", operation.ID, operation.MaxRetries)
	}

	// Increment retry count
	operation.RetryCount++
	operation.Status = OperationStatusPending
	operation.ErrorDetails = ""

	// Add retry delay
	time.Sleep(op.config.RetryDelay * time.Duration(operation.RetryCount))

	return op.ProcessOperation(ctx, operation)
}

// GetOperationMetrics returns metrics for operations
func (op *OperationProcessorImpl) GetOperationMetrics(ctx context.Context) (*OperationMetrics, error) {
	// Calculate operations per second
	if op.metrics.AverageProcessingTime > 0 {
		op.metrics.OperationsPerSecond = 1.0 / op.metrics.AverageProcessingTime.Seconds()
	}

	// Return a copy to avoid race conditions
	return &OperationMetrics{
		TotalOperations:       op.metrics.TotalOperations,
		SuccessfulOperations:  op.metrics.SuccessfulOperations,
		FailedOperations:      op.metrics.FailedOperations,
		SkippedOperations:     op.metrics.SkippedOperations,
		AverageProcessingTime: op.metrics.AverageProcessingTime,
		TotalBytesProcessed:   op.metrics.TotalBytesProcessed,
		OperationsPerSecond:   op.metrics.OperationsPerSecond,
	}, nil
}

// processOperationByType processes operation based on its type
func (op *OperationProcessorImpl) processOperationByType(ctx context.Context, operation *SyncOperation) (*OperationResult, error) {
	result := &OperationResult{
		Operation: operation,
	}

	switch operation.Type {
	case OperationTypeCreate, OperationTypeUpdate:
		return op.processCreateOrUpdate(ctx, operation)
	case OperationTypeDelete:
		return op.processDelete(ctx, operation)
	case OperationTypeMove:
		return op.processMove(ctx, operation)
	default:
		return nil, fmt.Errorf("unsupported operation type: %s", operation.Type)
	}
}

// processCreateOrUpdate processes create or update operations
func (op *OperationProcessorImpl) processCreateOrUpdate(ctx context.Context, operation *SyncOperation) (*OperationResult, error) {
	// Create or update issue file
	err := op.gitManager.CreateIssueFile(ctx, *operation.GitData)
	if err != nil {
		return nil, fmt.Errorf("failed to create/update issue file: %w", err)
	}

	result := &OperationResult{
		Operation:       operation,
		CommitRequired:  true,
		FilesModified:   []string{fmt.Sprintf("%s.md", operation.IssueKey)},
		BytesProcessed:  int64(len(operation.JiraData.Fields.Description)),
	}

	return result, nil
}

// processDelete processes delete operations
func (op *OperationProcessorImpl) processDelete(ctx context.Context, operation *SyncOperation) (*OperationResult, error) {
	// For now, we don't actually delete files but mark them as deleted
	// This could be enhanced to move files to a deleted directory
	operation.Status = OperationStatusSkipped
	op.metrics.SkippedOperations++

	result := &OperationResult{
		Operation:      operation,
		CommitRequired: false,
	}

	return result, nil
}

// processMove processes move operations
func (op *OperationProcessorImpl) processMove(ctx context.Context, operation *SyncOperation) (*OperationResult, error) {
	// For now, move operations are treated as updates
	return op.processCreateOrUpdate(ctx, operation)
}

// handleOperationError handles operation errors
func (op *OperationProcessorImpl) handleOperationError(operation *SyncOperation, err error) (*OperationResult, error) {
	operation.Status = OperationStatusFailed
	operation.ErrorDetails = err.Error()
	op.metrics.FailedOperations++

	result := &OperationResult{
		Success:      false,
		Operation:    operation,
		ErrorMessage: err.Error(),
	}

	return result, err
}

// commitBatch commits a batch of operations
func (op *OperationProcessorImpl) commitBatch(ctx context.Context, operations []*SyncOperation, result *BatchOperationResult) (string, error) {
	if result.SuccessfulOps == 0 {
		return "", nil
	}

	commitInfo := git.CommitInfo{
		Message: fmt.Sprintf("feat: sync batch of %d operations", result.SuccessfulOps),
		Author: git.AuthorInfo{
			Name:  "JIRA CDC Operator",
			Email: "jiracdc@example.com",
		},
	}

	return op.gitManager.CommitChanges(ctx, commitInfo)
}

// convertJiraToGitData converts JIRA issue data to git issue data
func (op *OperationProcessorImpl) convertJiraToGitData(issue *jira.Issue) (*git.IssueData, error) {
	gitData := &git.IssueData{
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
		gitData.Assignee = issue.Fields.Assignee.DisplayName
	}
	if issue.Fields.Reporter != nil {
		gitData.Reporter = issue.Fields.Reporter.DisplayName
	}

	// Set components
	for _, component := range issue.Fields.Components {
		gitData.Components = append(gitData.Components, component.Name)
	}

	return gitData, nil
}

// calculateOperationPriority calculates operation priority based on issue data
func (op *OperationProcessorImpl) calculateOperationPriority(issue *jira.Issue) int {
	priority := 0

	// Base priority on JIRA priority
	switch issue.Fields.Priority.Name {
	case "Highest":
		priority += 100
	case "High":
		priority += 75
	case "Medium":
		priority += 50
	case "Low":
		priority += 25
	case "Lowest":
		priority += 10
	}

	// Boost priority for recently updated issues
	if issue.Fields.Updated != "" {
		updated := parseJiraTime(issue.Fields.Updated)
		hoursSinceUpdate := time.Since(updated).Hours()
		if hoursSinceUpdate < 24 {
			priority += 20
		} else if hoursSinceUpdate < 168 { // 1 week
			priority += 10
		}
	}

	return priority
}

// isDependencySatisfied checks if a dependency is satisfied
func (op *OperationProcessorImpl) isDependencySatisfied(ctx context.Context, depID string) bool {
	// For now, assume all dependencies are satisfied
	// This could be enhanced to check actual dependency status
	return true
}

// updateAverageProcessingTime updates the average processing time metric
func (op *OperationProcessorImpl) updateAverageProcessingTime(processingTime time.Duration) {
	if op.metrics.AverageProcessingTime == 0 {
		op.metrics.AverageProcessingTime = processingTime
	} else {
		// Simple moving average
		op.metrics.AverageProcessingTime = (op.metrics.AverageProcessingTime + processingTime) / 2
	}
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
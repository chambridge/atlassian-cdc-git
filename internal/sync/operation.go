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

	jiradcv1 "github.com/atlassian/jira-cdc/api/v1"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OperationType defines the type of sync operation
type OperationType string

const (
	OperationBootstrap   OperationType = "bootstrap"
	OperationReconcile   OperationType = "reconcile"
	OperationForcedSync  OperationType = "forced-sync"
	OperationCleanup     OperationType = "cleanup"
)

// SyncOperation represents a single synchronization operation
type SyncOperation struct {
	ID             string                    `json:"id"`
	Type           OperationType             `json:"type"`
	Status         CDCTaskStatus             `json:"status"`
	Progress       ProgressTracker           `json:"progress"`
	Config         *SyncConfig               `json:"config"`
	Tasks          []CDCTask                 `json:"tasks"`
	StartTime      *metav1.Time              `json:"startTime,omitempty"`
	EndTime        *metav1.Time              `json:"endTime,omitempty"`
	ErrorMessage   string                    `json:"errorMessage,omitempty"`
	ResultSummary  *OperationResultSummary   `json:"resultSummary,omitempty"`
}

// OperationResultSummary contains summary information about completed operations
type OperationResultSummary struct {
	TotalTasks       int `json:"totalTasks"`
	CompletedTasks   int `json:"completedTasks"`
	FailedTasks      int `json:"failedTasks"`
	SkippedTasks     int `json:"skippedTasks"`
	ProcessedIssues  int `json:"processedIssues"`
	CreatedFiles     int `json:"createdFiles"`
	UpdatedFiles     int `json:"updatedFiles"`
	DeletedFiles     int `json:"deletedFiles"`
	GitCommits       int `json:"gitCommits"`
	ElapsedTime      time.Duration `json:"elapsedTime"`
}

// SyncOperationProcessor handles the execution of sync operations
type SyncOperationProcessor interface {
	// StartOperation creates and starts a new sync operation
	StartOperation(ctx context.Context, operationType OperationType, config *SyncConfig) (*SyncOperation, error)
	
	// GetOperation retrieves an operation by ID
	GetOperation(ctx context.Context, operationID string) (*SyncOperation, error)
	
	// ListOperations returns all operations, optionally filtered by status
	ListOperations(ctx context.Context, statusFilter *CDCTaskStatus) ([]*SyncOperation, error)
	
	// CancelOperation cancels a running operation
	CancelOperation(ctx context.Context, operationID string) error
	
	// WaitForCompletion waits for an operation to complete
	WaitForCompletion(ctx context.Context, operationID string, timeout time.Duration) (*SyncOperation, error)
	
	// CleanupOldOperations removes old completed operations
	CleanupOldOperations(ctx context.Context, retentionDays int) error
}

// operationProcessor implements SyncOperationProcessor
type operationProcessor struct {
	engine     *Engine
	operations map[string]*SyncOperation
	callbacks  map[string][]ProgressCallback
}

// NewSyncOperationProcessor creates a new operation processor
func NewSyncOperationProcessor(engine *Engine) SyncOperationProcessor {
	return &operationProcessor{
		engine:     engine,
		operations: make(map[string]*SyncOperation),
		callbacks:  make(map[string][]ProgressCallback),
	}
}

// StartOperation creates and starts a new sync operation
func (p *operationProcessor) StartOperation(ctx context.Context, operationType OperationType, config *SyncConfig) (*SyncOperation, error) {
	logger := log.FromContext(ctx)
	
	operationID := uuid.New().String()
	logger.Info("Starting sync operation", "operationID", operationID, "type", operationType)
	
	// Create operation
	operation := &SyncOperation{
		ID:        operationID,
		Type:      operationType,
		Status:    TaskStatusPending,
		Config:    config,
		Tasks:     []CDCTask{},
		StartTime: &metav1.Time{Time: time.Now()},
	}
	
	// Initialize progress tracker
	operation.Progress = NewProgressTracker()
	
	// Store operation
	p.operations[operationID] = operation
	
	// Create tasks based on operation type
	tasks, err := p.createTasksForOperation(ctx, operation)
	if err != nil {
		operation.Status = TaskStatusFailed
		operation.ErrorMessage = fmt.Sprintf("Failed to create tasks: %v", err)
		return operation, err
	}
	
	operation.Tasks = tasks
	operation.Progress.SetTotalSteps(len(tasks))
	
	// Start operation execution in background
	go p.executeOperation(ctx, operation)
	
	return operation, nil
}

// GetOperation retrieves an operation by ID
func (p *operationProcessor) GetOperation(ctx context.Context, operationID string) (*SyncOperation, error) {
	operation, exists := p.operations[operationID]
	if !exists {
		return nil, fmt.Errorf("operation not found: %s", operationID)
	}
	return operation, nil
}

// ListOperations returns all operations, optionally filtered by status
func (p *operationProcessor) ListOperations(ctx context.Context, statusFilter *CDCTaskStatus) ([]*SyncOperation, error) {
	var operations []*SyncOperation
	
	for _, operation := range p.operations {
		if statusFilter == nil || operation.Status == *statusFilter {
			operations = append(operations, operation)
		}
	}
	
	return operations, nil
}

// CancelOperation cancels a running operation
func (p *operationProcessor) CancelOperation(ctx context.Context, operationID string) error {
	operation, exists := p.operations[operationID]
	if !exists {
		return fmt.Errorf("operation not found: %s", operationID)
	}
	
	if operation.Status != TaskStatusRunning {
		return fmt.Errorf("operation %s is not running (status: %s)", operationID, operation.Status)
	}
	
	// Cancel all running tasks
	for i := range operation.Tasks {
		if operation.Tasks[i].Status == TaskStatusRunning {
			if err := operation.Tasks[i].Cancel(); err != nil {
				log.FromContext(ctx).Error(err, "Failed to cancel task", "taskID", operation.Tasks[i].ID)
			}
		}
	}
	
	operation.Status = TaskStatusCancelled
	operation.EndTime = &metav1.Time{Time: time.Now()}
	
	return nil
}

// WaitForCompletion waits for an operation to complete
func (p *operationProcessor) WaitForCompletion(ctx context.Context, operationID string, timeout time.Duration) (*SyncOperation, error) {
	operation, exists := p.operations[operationID]
	if !exists {
		return nil, fmt.Errorf("operation not found: %s", operationID)
	}
	
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return operation, ctx.Err()
		case <-ticker.C:
			if operation.Status == TaskStatusCompleted || 
			   operation.Status == TaskStatusFailed || 
			   operation.Status == TaskStatusCancelled {
				return operation, nil
			}
		}
	}
}

// CleanupOldOperations removes old completed operations
func (p *operationProcessor) CleanupOldOperations(ctx context.Context, retentionDays int) error {
	logger := log.FromContext(ctx)
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	
	var toDelete []string
	for operationID, operation := range p.operations {
		if operation.EndTime != nil && operation.EndTime.Time.Before(cutoff) {
			toDelete = append(toDelete, operationID)
		}
	}
	
	for _, operationID := range toDelete {
		delete(p.operations, operationID)
		delete(p.callbacks, operationID)
		logger.Info("Cleaned up old operation", "operationID", operationID)
	}
	
	return nil
}

// createTasksForOperation creates tasks based on operation type
func (p *operationProcessor) createTasksForOperation(ctx context.Context, operation *SyncOperation) ([]CDCTask, error) {
	var tasks []CDCTask
	
	switch operation.Type {
	case OperationBootstrap:
		tasks = p.createBootstrapTasks(ctx, operation.Config)
	case OperationReconcile:
		tasks = p.createReconcileTasks(ctx, operation.Config)
	case OperationForcedSync:
		tasks = p.createForcedSyncTasks(ctx, operation.Config)
	case OperationCleanup:
		tasks = p.createCleanupTasks(ctx, operation.Config)
	default:
		return nil, fmt.Errorf("unknown operation type: %s", operation.Type)
	}
	
	return tasks, nil
}

// createBootstrapTasks creates tasks for bootstrap operation
func (p *operationProcessor) createBootstrapTasks(ctx context.Context, config *SyncConfig) []CDCTask {
	var tasks []CDCTask
	
	// Task 1: Initialize git repository
	tasks = append(tasks, CDCTask{
		ID:          uuid.New().String(),
		Name:        "Initialize Git Repository",
		Description: "Clone or initialize the target git repository",
		Type:        TaskTypeGitOperation,
		Status:      TaskStatusPending,
		Priority:    TaskPriorityHigh,
		Dependencies: []string{},
		Metadata: map[string]interface{}{
			"repository_url": config.GitRepository.URL,
			"branch":        config.GitRepository.Branch,
		},
	})
	
	// Task 2: Fetch JIRA projects
	tasks = append(tasks, CDCTask{
		ID:          uuid.New().String(),
		Name:        "Fetch JIRA Projects",
		Description: "Retrieve project information from JIRA",
		Type:        TaskTypeJiraOperation,
		Status:      TaskStatusPending,
		Priority:    TaskPriorityHigh,
		Dependencies: []string{},
		Metadata: map[string]interface{}{
			"jira_url":     config.JiraInstance.BaseURL,
			"project_keys": config.SyncTarget.ProjectKeys,
		},
	})
	
	// Task 3: Sync issues (depends on both git and JIRA tasks)
	tasks = append(tasks, CDCTask{
		ID:          uuid.New().String(),
		Name:        "Bootstrap Issue Sync",
		Description: "Perform initial sync of all JIRA issues to git",
		Type:        TaskTypeSyncOperation,
		Status:      TaskStatusPending,
		Priority:    TaskPriorityMedium,
		Dependencies: []string{tasks[0].ID, tasks[1].ID},
		Metadata: map[string]interface{}{
			"sync_type": "bootstrap",
			"batch_size": config.SyncTarget.BatchSize,
		},
	})
	
	return tasks
}

// createReconcileTasks creates tasks for reconcile operation
func (p *operationProcessor) createReconcileTasks(ctx context.Context, config *SyncConfig) []CDCTask {
	var tasks []CDCTask
	
	// Task 1: Check for JIRA updates
	tasks = append(tasks, CDCTask{
		ID:          uuid.New().String(),
		Name:        "Check JIRA Updates",
		Description: "Query JIRA for issues updated since last sync",
		Type:        TaskTypeJiraOperation,
		Status:      TaskStatusPending,
		Priority:    TaskPriorityHigh,
		Dependencies: []string{},
		Metadata: map[string]interface{}{
			"last_sync_time": config.LastSyncTime,
			"project_keys":   config.SyncTarget.ProjectKeys,
		},
	})
	
	// Task 2: Update git repository
	tasks = append(tasks, CDCTask{
		ID:          uuid.New().String(),
		Name:        "Update Git Repository",
		Description: "Pull latest changes from git repository",
		Type:        TaskTypeGitOperation,
		Status:      TaskStatusPending,
		Priority:    TaskPriorityHigh,
		Dependencies: []string{},
		Metadata: map[string]interface{}{
			"repository_url": config.GitRepository.URL,
			"branch":        config.GitRepository.Branch,
		},
	})
	
	// Task 3: Sync updated issues
	tasks = append(tasks, CDCTask{
		ID:          uuid.New().String(),
		Name:        "Sync Updated Issues",
		Description: "Sync updated JIRA issues to git",
		Type:        TaskTypeSyncOperation,
		Status:      TaskStatusPending,
		Priority:    TaskPriorityMedium,
		Dependencies: []string{tasks[0].ID, tasks[1].ID},
		Metadata: map[string]interface{}{
			"sync_type":  "incremental",
			"batch_size": config.SyncTarget.BatchSize,
		},
	})
	
	return tasks
}

// createForcedSyncTasks creates tasks for forced sync operation
func (p *operationProcessor) createForcedSyncTasks(ctx context.Context, config *SyncConfig) []CDCTask {
	// For forced sync, use bootstrap tasks but with different metadata
	tasks := p.createBootstrapTasks(ctx, config)
	
	// Update metadata to indicate forced sync
	for i := range tasks {
		if tasks[i].Type == TaskTypeSyncOperation {
			tasks[i].Name = "Forced Issue Sync"
			tasks[i].Description = "Force sync all JIRA issues to git, overwriting existing files"
			tasks[i].Metadata["sync_type"] = "forced"
			tasks[i].Metadata["overwrite"] = true
		}
	}
	
	return tasks
}

// createCleanupTasks creates tasks for cleanup operation
func (p *operationProcessor) createCleanupTasks(ctx context.Context, config *SyncConfig) []CDCTask {
	var tasks []CDCTask
	
	// Task 1: Identify orphaned files
	tasks = append(tasks, CDCTask{
		ID:          uuid.New().String(),
		Name:        "Identify Orphaned Files",
		Description: "Find git files that no longer have corresponding JIRA issues",
		Type:        TaskTypeSyncOperation,
		Status:      TaskStatusPending,
		Priority:    TaskPriorityMedium,
		Dependencies: []string{},
		Metadata: map[string]interface{}{
			"operation_type": "cleanup_identification",
			"project_keys":   config.SyncTarget.ProjectKeys,
		},
	})
	
	// Task 2: Remove orphaned files
	tasks = append(tasks, CDCTask{
		ID:          uuid.New().String(),
		Name:        "Remove Orphaned Files",
		Description: "Delete git files for issues that no longer exist in JIRA",
		Type:        TaskTypeGitOperation,
		Status:      TaskStatusPending,
		Priority:    TaskPriorityLow,
		Dependencies: []string{tasks[0].ID},
		Metadata: map[string]interface{}{
			"operation_type": "cleanup_removal",
		},
	})
	
	return tasks
}

// executeOperation executes an operation by running its tasks
func (p *operationProcessor) executeOperation(ctx context.Context, operation *SyncOperation) {
	logger := log.FromContext(ctx).WithValues("operationID", operation.ID)
	logger.Info("Executing sync operation")
	
	operation.Status = TaskStatusRunning
	
	// Execute tasks in dependency order
	taskExecutor := NewTaskExecutor()
	
	// Setup progress callback
	progressCallback := func(completedSteps int, message string) {
		operation.Progress.Update(completedSteps, message)
		
		// Notify registered callbacks
		if callbacks, exists := p.callbacks[operation.ID]; exists {
			for _, callback := range callbacks {
				callback(completedSteps, message)
			}
		}
	}
	
	// Execute tasks
	results, err := taskExecutor.ExecuteTasks(ctx, operation.Tasks, progressCallback)
	if err != nil {
		logger.Error(err, "Operation execution failed")
		operation.Status = TaskStatusFailed
		operation.ErrorMessage = err.Error()
	} else {
		operation.Status = TaskStatusCompleted
		logger.Info("Operation completed successfully")
	}
	
	// Set end time and generate summary
	operation.EndTime = &metav1.Time{Time: time.Now()}
	operation.ResultSummary = p.generateResultSummary(operation, results)
	
	logger.Info("Operation finished", "status", operation.Status, "duration", operation.EndTime.Time.Sub(operation.StartTime.Time))
}

// generateResultSummary creates a summary of operation results
func (p *operationProcessor) generateResultSummary(operation *SyncOperation, results []TaskResult) *OperationResultSummary {
	summary := &OperationResultSummary{
		TotalTasks: len(operation.Tasks),
		ElapsedTime: operation.EndTime.Time.Sub(operation.StartTime.Time),
	}
	
	for _, result := range results {
		switch result.Status {
		case TaskStatusCompleted:
			summary.CompletedTasks++
		case TaskStatusFailed:
			summary.FailedTasks++
		case TaskStatusCancelled:
			summary.SkippedTasks++
		}
		
		// Extract metrics from task metadata
		if metrics, ok := result.Metadata["metrics"].(map[string]interface{}); ok {
			if processed, ok := metrics["processed_issues"].(int); ok {
				summary.ProcessedIssues += processed
			}
			if created, ok := metrics["created_files"].(int); ok {
				summary.CreatedFiles += created
			}
			if updated, ok := metrics["updated_files"].(int); ok {
				summary.UpdatedFiles += updated
			}
			if deleted, ok := metrics["deleted_files"].(int); ok {
				summary.DeletedFiles += deleted
			}
			if commits, ok := metrics["git_commits"].(int); ok {
				summary.GitCommits += commits
			}
		}
	}
	
	return summary
}

// RegisterProgressCallback registers a callback for operation progress updates
func (p *operationProcessor) RegisterProgressCallback(operationID string, callback ProgressCallback) {
	if p.callbacks[operationID] == nil {
		p.callbacks[operationID] = []ProgressCallback{}
	}
	p.callbacks[operationID] = append(p.callbacks[operationID], callback)
}

// ToJiraCDCOperation converts internal operation to API type
func (op *SyncOperation) ToJiraCDCOperation() jiradcv1.CDCOperation {
	return jiradcv1.CDCOperation{
		ID:            op.ID,
		Type:          string(op.Type),
		Status:        string(op.Status),
		StartTime:     op.StartTime,
		EndTime:       op.EndTime,
		ErrorMessage:  op.ErrorMessage,
		TotalTasks:    len(op.Tasks),
		CompletedTasks: func() int {
			count := 0
			for _, task := range op.Tasks {
				if task.Status == TaskStatusCompleted {
					count++
				}
			}
			return count
		}(),
		Progress: func() int {
			if len(op.Tasks) == 0 {
				return 0
			}
			completed := 0
			for _, task := range op.Tasks {
				if task.Status == TaskStatusCompleted {
					completed++
				}
			}
			return (completed * 100) / len(op.Tasks)
		}(),
	}
}
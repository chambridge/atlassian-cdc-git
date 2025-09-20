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
	"sync"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
)

// TaskManager manages CDC tasks and their lifecycle
type TaskManager interface {
	// CreateTask creates a new task
	CreateTask(ctx context.Context, info TaskInfo) (*TaskInfo, error)

	// GetTask retrieves a task by ID
	GetTask(ctx context.Context, taskID string) (*TaskInfo, error)

	// UpdateTask updates an existing task
	UpdateTask(ctx context.Context, info TaskInfo) error

	// ListTasks lists tasks with optional filters
	ListTasks(ctx context.Context, filters TaskFilters) ([]*TaskInfo, error)

	// CancelTask cancels a running task
	CancelTask(ctx context.Context, taskID string) error

	// DeleteTask deletes a completed task
	DeleteTask(ctx context.Context, taskID string) error

	// GetCurrentTask returns the currently running task for a project
	GetCurrentTask(ctx context.Context, projectKey string) (*TaskInfo, error)
}

// TaskInfo represents task information
type TaskInfo struct {
	ID                string
	Type              string // bootstrap, reconciliation, maintenance
	Status            string // pending, running, completed, failed, cancelled
	ProjectKey        string
	StartedAt         time.Time
	CompletedAt       *time.Time
	ErrorMessage      string
	Progress          *TaskProgress
	Configuration     TaskConfiguration
	CreatedBy         string
	FinalCommitHash   string
}

// TaskConfiguration represents task-specific configuration
type TaskConfiguration struct {
	IssueFilter      string
	ForceRefresh     bool
	ActiveIssuesOnly bool
	BatchSize        int
	MaxRetries       int
}

// TaskFilters represents filters for listing tasks
type TaskFilters struct {
	ProjectKey string
	Status     string
	Type       string
	Limit      int
	Offset     int
}

// TaskState represents the state of a task in the state machine
type TaskState string

const (
	TaskStatePending   TaskState = "pending"
	TaskStateRunning   TaskState = "running"
	TaskStateCompleted TaskState = "completed"
	TaskStateFailed    TaskState = "failed"
	TaskStateCancelled TaskState = "cancelled"
)

// TaskEvent represents events that can trigger state transitions
type TaskEvent string

const (
	TaskEventStart    TaskEvent = "start"
	TaskEventComplete TaskEvent = "complete"
	TaskEventFail     TaskEvent = "fail"
	TaskEventCancel   TaskEvent = "cancel"
	TaskEventRetry    TaskEvent = "retry"
)

// TaskStateMachine manages task state transitions
type TaskStateMachine struct {
	transitions map[TaskState]map[TaskEvent]TaskState
}

// NewTaskStateMachine creates a new task state machine
func NewTaskStateMachine() *TaskStateMachine {
	sm := &TaskStateMachine{
		transitions: make(map[TaskState]map[TaskEvent]TaskState),
	}

	// Define valid state transitions
	sm.transitions[TaskStatePending] = map[TaskEvent]TaskState{
		TaskEventStart:  TaskStateRunning,
		TaskEventCancel: TaskStateCancelled,
	}

	sm.transitions[TaskStateRunning] = map[TaskEvent]TaskState{
		TaskEventComplete: TaskStateCompleted,
		TaskEventFail:     TaskStateFailed,
		TaskEventCancel:   TaskStateCancelled,
	}

	sm.transitions[TaskStateFailed] = map[TaskEvent]TaskState{
		TaskEventRetry: TaskStatePending,
	}

	// Terminal states have no outgoing transitions
	sm.transitions[TaskStateCompleted] = map[TaskEvent]TaskState{}
	sm.transitions[TaskStateCancelled] = map[TaskEvent]TaskState{}

	return sm
}

// CanTransition checks if a state transition is valid
func (sm *TaskStateMachine) CanTransition(currentState TaskState, event TaskEvent) bool {
	if transitions, exists := sm.transitions[currentState]; exists {
		_, canTransition := transitions[event]
		return canTransition
	}
	return false
}

// Transition performs a state transition if valid
func (sm *TaskStateMachine) Transition(currentState TaskState, event TaskEvent) (TaskState, error) {
	if !sm.CanTransition(currentState, event) {
		return currentState, fmt.Errorf("invalid transition from %s with event %s", currentState, event)
	}

	return sm.transitions[currentState][event], nil
}

// TaskManagerImpl is the concrete implementation of TaskManager
type TaskManagerImpl struct {
	mu        sync.RWMutex
	tasks     map[string]*TaskInfo
	stateMachine *TaskStateMachine
}

// NewTaskManager creates a new task manager
func NewTaskManager() TaskManager {
	return &TaskManagerImpl{
		tasks:        make(map[string]*TaskInfo),
		stateMachine: NewTaskStateMachine(),
	}
}

// CreateTask creates a new task
func (tm *TaskManagerImpl) CreateTask(ctx context.Context, info TaskInfo) (*TaskInfo, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Generate ID if not provided
	if info.ID == "" {
		info.ID = uuid.New().String()
	}

	// Set default values
	if info.Status == "" {
		info.Status = string(TaskStatePending)
	}
	if info.StartedAt.IsZero() {
		info.StartedAt = time.Now()
	}
	if info.CreatedBy == "" {
		info.CreatedBy = "jiracdc-operator"
	}

	// Validate task type
	if !isValidTaskType(info.Type) {
		return nil, fmt.Errorf("invalid task type: %s", info.Type)
	}

	// Check if there's already a running task for this project
	for _, task := range tm.tasks {
		if task.ProjectKey == info.ProjectKey && 
		   (task.Status == string(TaskStateRunning) || task.Status == string(TaskStatePending)) {
			return nil, fmt.Errorf("task %s is already running for project %s", task.ID, info.ProjectKey)
		}
	}

	// Store the task
	tm.tasks[info.ID] = &info

	return &info, nil
}

// GetTask retrieves a task by ID
func (tm *TaskManagerImpl) GetTask(ctx context.Context, taskID string) (*TaskInfo, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	// Return a copy to avoid race conditions
	taskCopy := *task
	return &taskCopy, nil
}

// UpdateTask updates an existing task
func (tm *TaskManagerImpl) UpdateTask(ctx context.Context, info TaskInfo) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	existing, exists := tm.tasks[info.ID]
	if !exists {
		return fmt.Errorf("task %s not found", info.ID)
	}

	// Validate state transition if status changed
	if existing.Status != info.Status {
		currentState := TaskState(existing.Status)
		newState := TaskState(info.Status)
		
		// Determine the event based on the state change
		var event TaskEvent
		switch newState {
		case TaskStateRunning:
			event = TaskEventStart
		case TaskStateCompleted:
			event = TaskEventComplete
		case TaskStateFailed:
			event = TaskEventFail
		case TaskStateCancelled:
			event = TaskEventCancel
		case TaskStatePending:
			event = TaskEventRetry
		default:
			return fmt.Errorf("unknown target state: %s", newState)
		}

		if !tm.stateMachine.CanTransition(currentState, event) {
			return fmt.Errorf("invalid state transition from %s to %s", currentState, newState)
		}
	}

	// Update the task
	tm.tasks[info.ID] = &info

	return nil
}

// ListTasks lists tasks with optional filters
func (tm *TaskManagerImpl) ListTasks(ctx context.Context, filters TaskFilters) ([]*TaskInfo, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []*TaskInfo

	for _, task := range tm.tasks {
		// Apply filters
		if filters.ProjectKey != "" && task.ProjectKey != filters.ProjectKey {
			continue
		}
		if filters.Status != "" && task.Status != filters.Status {
			continue
		}
		if filters.Type != "" && task.Type != filters.Type {
			continue
		}

		// Create a copy to avoid race conditions
		taskCopy := *task
		result = append(result, &taskCopy)
	}

	// Apply limit and offset
	if filters.Offset > 0 {
		if filters.Offset >= len(result) {
			return []*TaskInfo{}, nil
		}
		result = result[filters.Offset:]
	}

	if filters.Limit > 0 && len(result) > filters.Limit {
		result = result[:filters.Limit]
	}

	return result, nil
}

// CancelTask cancels a running task
func (tm *TaskManagerImpl) CancelTask(ctx context.Context, taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	currentState := TaskState(task.Status)
	newState, err := tm.stateMachine.Transition(currentState, TaskEventCancel)
	if err != nil {
		return fmt.Errorf("cannot cancel task: %w", err)
	}

	task.Status = string(newState)
	completedTime := time.Now()
	task.CompletedAt = &completedTime

	return nil
}

// DeleteTask deletes a completed task
func (tm *TaskManagerImpl) DeleteTask(ctx context.Context, taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Only allow deletion of completed or failed tasks
	if task.Status != string(TaskStateCompleted) && 
	   task.Status != string(TaskStateFailed) && 
	   task.Status != string(TaskStateCancelled) {
		return fmt.Errorf("cannot delete task in status %s", task.Status)
	}

	delete(tm.tasks, taskID)
	return nil
}

// GetCurrentTask returns the currently running task for a project
func (tm *TaskManagerImpl) GetCurrentTask(ctx context.Context, projectKey string) (*TaskInfo, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	for _, task := range tm.tasks {
		if task.ProjectKey == projectKey && 
		   (task.Status == string(TaskStateRunning) || task.Status == string(TaskStatePending)) {
			// Return a copy to avoid race conditions
			taskCopy := *task
			return &taskCopy, nil
		}
	}

	return nil, nil // No current task
}

// StartTask transitions a task to running state
func (tm *TaskManagerImpl) StartTask(ctx context.Context, taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	currentState := TaskState(task.Status)
	newState, err := tm.stateMachine.Transition(currentState, TaskEventStart)
	if err != nil {
		return fmt.Errorf("cannot start task: %w", err)
	}

	task.Status = string(newState)
	return nil
}

// CompleteTask transitions a task to completed state
func (tm *TaskManagerImpl) CompleteTask(ctx context.Context, taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	currentState := TaskState(task.Status)
	newState, err := tm.stateMachine.Transition(currentState, TaskEventComplete)
	if err != nil {
		return fmt.Errorf("cannot complete task: %w", err)
	}

	task.Status = string(newState)
	completedTime := time.Now()
	task.CompletedAt = &completedTime

	return nil
}

// FailTask transitions a task to failed state
func (tm *TaskManagerImpl) FailTask(ctx context.Context, taskID string, errorMessage string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	currentState := TaskState(task.Status)
	newState, err := tm.stateMachine.Transition(currentState, TaskEventFail)
	if err != nil {
		return fmt.Errorf("cannot fail task: %w", err)
	}

	task.Status = string(newState)
	task.ErrorMessage = errorMessage
	completedTime := time.Now()
	task.CompletedAt = &completedTime

	return nil
}

// RetryTask transitions a failed task back to pending state
func (tm *TaskManagerImpl) RetryTask(ctx context.Context, taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, exists := tm.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	currentState := TaskState(task.Status)
	newState, err := tm.stateMachine.Transition(currentState, TaskEventRetry)
	if err != nil {
		return fmt.Errorf("cannot retry task: %w", err)
	}

	task.Status = string(newState)
	task.ErrorMessage = ""
	task.CompletedAt = nil

	return nil
}

// isValidTaskType checks if a task type is valid
func isValidTaskType(taskType string) bool {
	validTypes := []string{"bootstrap", "reconciliation", "maintenance"}
	for _, validType := range validTypes {
		if taskType == validType {
			return true
		}
	}
	return false
}

// ConvertToAPITaskInfo converts internal TaskInfo to API TaskInfo
func ConvertToAPITaskInfo(task *TaskInfo) *jiradcdv1.TaskInfo {
	apiTask := &jiradcdv1.TaskInfo{
		ID:     task.ID,
		Type:   task.Type,
		Status: task.Status,
	}

	if !task.StartedAt.IsZero() {
		startTime := metav1.NewTime(task.StartedAt)
		apiTask.StartedAt = &startTime
	}

	if task.CompletedAt != nil {
		completeTime := metav1.NewTime(*task.CompletedAt)
		apiTask.CompletedAt = &completeTime
	}

	if task.ErrorMessage != "" {
		apiTask.ErrorMessage = task.ErrorMessage
	}

	if task.Progress != nil {
		apiTask.Progress = &jiradcdv1.TaskProgress{
			TotalItems:      task.Progress.TotalItems,
			ProcessedItems:  task.Progress.ProcessedItems,
			PercentComplete: task.Progress.PercentComplete,
		}
		if task.Progress.EstimatedRemaining != nil {
			duration := &metav1.Duration{Duration: *task.Progress.EstimatedRemaining}
			apiTask.Progress.EstimatedTimeRemaining = duration
		}
	}

	return apiTask
}
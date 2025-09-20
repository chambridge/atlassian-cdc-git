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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
)

// ProgressTracker tracks progress of synchronization tasks
type ProgressTracker interface {
	// StartTracking starts tracking progress for a task
	StartTracking(ctx context.Context, taskID string, totalItems int32) error

	// UpdateProgress updates the progress for a task
	UpdateProgress(ctx context.Context, taskID string, processedItems int32) error

	// SetError sets an error for a task
	SetError(ctx context.Context, taskID string, err error) error

	// Complete marks a task as completed
	Complete(ctx context.Context, taskID string) error

	// GetProgress returns the current progress for a task
	GetProgress(ctx context.Context, taskID string) (*jiradcdv1.TaskProgress, error)

	// EstimateTimeRemaining estimates remaining time for a task
	EstimateTimeRemaining(ctx context.Context, taskID string) (*time.Duration, error)
}

// TaskProgress represents detailed progress information
type TaskProgress struct {
	TaskID              string
	TotalItems          int32
	ProcessedItems      int32
	PercentComplete     float32
	StartTime           time.Time
	LastUpdateTime      time.Time
	EstimatedRemaining  *time.Duration
	AverageTimePerItem  time.Duration
	ItemsPerSecond      float64
	ErrorCount          int32
	SkippedCount        int32
}

// ProgressTrackerImpl is the concrete implementation of ProgressTracker
type ProgressTrackerImpl struct {
	mu        sync.RWMutex
	progress  map[string]*TaskProgress
	callbacks []ProgressCallback
}

// ProgressCallback is a function that gets called when progress is updated
type ProgressCallback func(taskID string, progress *TaskProgress)

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() ProgressTracker {
	return &ProgressTrackerImpl{
		progress: make(map[string]*TaskProgress),
	}
}

// AddCallback adds a progress callback
func (pt *ProgressTrackerImpl) AddCallback(callback ProgressCallback) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.callbacks = append(pt.callbacks, callback)
}

// StartTracking starts tracking progress for a task
func (pt *ProgressTrackerImpl) StartTracking(ctx context.Context, taskID string, totalItems int32) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.progress[taskID] = &TaskProgress{
		TaskID:         taskID,
		TotalItems:     totalItems,
		ProcessedItems: 0,
		PercentComplete: 0,
		StartTime:      time.Now(),
		LastUpdateTime: time.Now(),
	}

	// Notify callbacks
	pt.notifyCallbacks(taskID, pt.progress[taskID])

	return nil
}

// UpdateProgress updates the progress for a task
func (pt *ProgressTrackerImpl) UpdateProgress(ctx context.Context, taskID string, processedItems int32) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	progress, exists := pt.progress[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	now := time.Now()
	progress.ProcessedItems = processedItems
	progress.LastUpdateTime = now

	// Calculate percentage
	if progress.TotalItems > 0 {
		progress.PercentComplete = float32(processedItems) / float32(progress.TotalItems) * 100
	}

	// Calculate average time per item
	elapsed := now.Sub(progress.StartTime)
	if processedItems > 0 {
		progress.AverageTimePerItem = elapsed / time.Duration(processedItems)
		progress.ItemsPerSecond = float64(processedItems) / elapsed.Seconds()
	}

	// Estimate remaining time
	if processedItems > 0 && processedItems < progress.TotalItems {
		remaining := progress.TotalItems - processedItems
		estimatedRemaining := time.Duration(remaining) * progress.AverageTimePerItem
		progress.EstimatedRemaining = &estimatedRemaining
	}

	// Notify callbacks
	pt.notifyCallbacks(taskID, progress)

	return nil
}

// SetError sets an error for a task
func (pt *ProgressTrackerImpl) SetError(ctx context.Context, taskID string, err error) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	progress, exists := pt.progress[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	progress.ErrorCount++
	progress.LastUpdateTime = time.Now()

	// Notify callbacks
	pt.notifyCallbacks(taskID, progress)

	return nil
}

// Complete marks a task as completed
func (pt *ProgressTrackerImpl) Complete(ctx context.Context, taskID string) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	progress, exists := pt.progress[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	progress.ProcessedItems = progress.TotalItems
	progress.PercentComplete = 100
	progress.LastUpdateTime = time.Now()
	progress.EstimatedRemaining = &[]time.Duration{0}[0]

	// Notify callbacks
	pt.notifyCallbacks(taskID, progress)

	return nil
}

// GetProgress returns the current progress for a task
func (pt *ProgressTrackerImpl) GetProgress(ctx context.Context, taskID string) (*jiradcdv1.TaskProgress, error) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	progress, exists := pt.progress[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	// Convert to API type
	apiProgress := &jiradcdv1.TaskProgress{
		TotalItems:      progress.TotalItems,
		ProcessedItems:  progress.ProcessedItems,
		PercentComplete: progress.PercentComplete,
	}

	if progress.EstimatedRemaining != nil {
		duration := &metav1.Duration{Duration: *progress.EstimatedRemaining}
		apiProgress.EstimatedTimeRemaining = duration
	}

	return apiProgress, nil
}

// EstimateTimeRemaining estimates remaining time for a task
func (pt *ProgressTrackerImpl) EstimateTimeRemaining(ctx context.Context, taskID string) (*time.Duration, error) {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	progress, exists := pt.progress[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return progress.EstimatedRemaining, nil
}

// CleanupTask removes a task from tracking
func (pt *ProgressTrackerImpl) CleanupTask(taskID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	delete(pt.progress, taskID)
}

// GetAllProgress returns progress for all tracked tasks
func (pt *ProgressTrackerImpl) GetAllProgress() map[string]*TaskProgress {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	result := make(map[string]*TaskProgress)
	for k, v := range pt.progress {
		// Create a copy to avoid race conditions
		result[k] = &TaskProgress{
			TaskID:              v.TaskID,
			TotalItems:          v.TotalItems,
			ProcessedItems:      v.ProcessedItems,
			PercentComplete:     v.PercentComplete,
			StartTime:           v.StartTime,
			LastUpdateTime:      v.LastUpdateTime,
			AverageTimePerItem:  v.AverageTimePerItem,
			ItemsPerSecond:      v.ItemsPerSecond,
			ErrorCount:          v.ErrorCount,
			SkippedCount:        v.SkippedCount,
		}
		if v.EstimatedRemaining != nil {
			estimated := *v.EstimatedRemaining
			result[k].EstimatedRemaining = &estimated
		}
	}
	return result
}

// notifyCallbacks notifies all registered callbacks
func (pt *ProgressTrackerImpl) notifyCallbacks(taskID string, progress *TaskProgress) {
	for _, callback := range pt.callbacks {
		go callback(taskID, progress)
	}
}

// ProgressReporter provides formatted progress reporting
type ProgressReporter struct {
	tracker ProgressTracker
}

// NewProgressReporter creates a new progress reporter
func NewProgressReporter(tracker ProgressTracker) *ProgressReporter {
	return &ProgressReporter{tracker: tracker}
}

// GetFormattedProgress returns formatted progress string
func (pr *ProgressReporter) GetFormattedProgress(ctx context.Context, taskID string) (string, error) {
	progress, err := pr.tracker.GetProgress(ctx, taskID)
	if err != nil {
		return "", err
	}

	if progress.EstimatedTimeRemaining != nil {
		return fmt.Sprintf("%.1f%% (%d/%d items, ~%s remaining)",
			progress.PercentComplete,
			progress.ProcessedItems,
			progress.TotalItems,
			progress.EstimatedTimeRemaining.Duration.String(),
		), nil
	}

	return fmt.Sprintf("%.1f%% (%d/%d items)",
		progress.PercentComplete,
		progress.ProcessedItems,
		progress.TotalItems,
	), nil
}

// GetProgressBar returns a visual progress bar
func (pr *ProgressReporter) GetProgressBar(ctx context.Context, taskID string, width int) (string, error) {
	progress, err := pr.tracker.GetProgress(ctx, taskID)
	if err != nil {
		return "", err
	}

	filled := int(progress.PercentComplete / 100 * float32(width))
	if filled > width {
		filled = width
	}

	bar := ""
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < width; i++ {
		bar += "░"
	}

	return fmt.Sprintf("[%s] %.1f%%", bar, progress.PercentComplete), nil
}

// InMemoryProgressStore stores progress in memory (for development)
type InMemoryProgressStore struct {
	mu   sync.RWMutex
	data map[string]*jiradcdv1.TaskProgress
}

// NewInMemoryProgressStore creates a new in-memory progress store
func NewInMemoryProgressStore() *InMemoryProgressStore {
	return &InMemoryProgressStore{
		data: make(map[string]*jiradcdv1.TaskProgress),
	}
}

// Store stores progress data
func (s *InMemoryProgressStore) Store(taskID string, progress *jiradcdv1.TaskProgress) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[taskID] = progress
	return nil
}

// Load loads progress data
func (s *InMemoryProgressStore) Load(taskID string) (*jiradcdv1.TaskProgress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	progress, exists := s.data[taskID]
	if !exists {
		return nil, fmt.Errorf("progress not found for task %s", taskID)
	}
	return progress, nil
}

// Delete removes progress data
func (s *InMemoryProgressStore) Delete(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, taskID)
	return nil
}

// List returns all stored progress data
func (s *InMemoryProgressStore) List() (map[string]*jiradcdv1.TaskProgress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	result := make(map[string]*jiradcdv1.TaskProgress)
	for k, v := range s.data {
		result[k] = v
	}
	return result, nil
}
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

package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/company/jira-cdc-operator/internal/sync"
)

// TaskHandler handles task-related API endpoints
type TaskHandler struct {
	taskManager     sync.TaskManager
	progressTracker sync.ProgressTracker
}

// NewTaskHandler creates a new task handler
func NewTaskHandler(taskManager sync.TaskManager, progressTracker sync.ProgressTracker) *TaskHandler {
	return &TaskHandler{
		taskManager:     taskManager,
		progressTracker: progressTracker,
	}
}

// TaskResponse represents a task for API responses
type TaskResponse struct {
	ID              string             `json:"id"`
	Type            string             `json:"type"`
	Status          string             `json:"status"`
	ProjectKey      string             `json:"projectKey"`
	StartedAt       string             `json:"startedAt"`
	CompletedAt     *string            `json:"completedAt,omitempty"`
	ErrorMessage    string             `json:"errorMessage,omitempty"`
	Progress        *TaskProgress      `json:"progress,omitempty"`
	Configuration   TaskConfiguration  `json:"configuration"`
	CreatedBy       string             `json:"createdBy"`
	FinalCommitHash string             `json:"finalCommitHash,omitempty"`
}

// TaskProgress represents task progress for API responses
type TaskProgress struct {
	TotalItems      int32   `json:"totalItems"`
	ProcessedItems  int32   `json:"processedItems"`
	PercentComplete float32 `json:"percentComplete"`
	EstimatedTimeRemaining *string `json:"estimatedTimeRemaining,omitempty"`
}

// TaskConfiguration represents task configuration for API responses
type TaskConfiguration struct {
	IssueFilter      string `json:"issueFilter,omitempty"`
	ForceRefresh     bool   `json:"forceRefresh"`
	ActiveIssuesOnly bool   `json:"activeIssuesOnly"`
	BatchSize        int    `json:"batchSize"`
	MaxRetries       int    `json:"maxRetries"`
}

// TaskListResponse represents the response for task list endpoints
type TaskListResponse struct {
	Tasks  []TaskResponse `json:"tasks"`
	Total  int            `json:"total"`
	Offset int            `json:"offset"`
	Limit  int            `json:"limit"`
}

// CancelTaskRequest represents a request to cancel a task
type CancelTaskRequest struct {
	Reason string `json:"reason,omitempty"`
}

// GetTasks handles GET /tasks
func (h *TaskHandler) GetTasks(c *gin.Context) {
	// Parse query parameters
	projectKey := c.Query("projectKey")
	status := c.Query("status")
	taskType := c.Query("type")
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000 // Cap at 1000
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Build filters
	filters := sync.TaskFilters{
		ProjectKey: projectKey,
		Status:     status,
		Type:       taskType,
		Limit:      limit,
		Offset:     offset,
	}

	// Get tasks from task manager
	tasks, err := h.taskManager.ListTasks(context.TODO(), filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve tasks",
			"details": err.Error(),
		})
		return
	}

	// Convert to API response format
	taskResponses := make([]TaskResponse, 0, len(tasks))
	for _, task := range tasks {
		taskResponse := h.convertTaskToResponse(task)
		taskResponses = append(taskResponses, taskResponse)
	}

	response := TaskListResponse{
		Tasks:  taskResponses,
		Total:  len(taskResponses),
		Offset: offset,
		Limit:  limit,
	}

	c.JSON(http.StatusOK, response)
}

// GetTask handles GET /tasks/:id
func (h *TaskHandler) GetTask(c *gin.Context) {
	taskID := c.Param("id")

	// Get task from task manager
	task, err := h.taskManager.GetTask(context.TODO(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Task not found",
			"taskId": taskID,
		})
		return
	}

	// Convert to API response format
	taskResponse := h.convertTaskToResponse(task)

	c.JSON(http.StatusOK, taskResponse)
}

// PostTaskCancel handles POST /tasks/:id/cancel
func (h *TaskHandler) PostTaskCancel(c *gin.Context) {
	taskID := c.Param("id")

	var request CancelTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		// Allow empty body for cancel requests
		request = CancelTaskRequest{}
	}

	// Get current task to check if it can be cancelled
	task, err := h.taskManager.GetTask(context.TODO(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Task not found",
			"taskId": taskID,
		})
		return
	}

	// Check if task can be cancelled
	if task.Status == "completed" || task.Status == "failed" || task.Status == "cancelled" {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Task cannot be cancelled",
			"reason": "Task is already in terminal state",
			"currentStatus": task.Status,
		})
		return
	}

	// Cancel the task
	err = h.taskManager.CancelTask(context.TODO(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to cancel task",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task cancelled successfully",
		"taskId":  taskID,
		"reason":  request.Reason,
	})
}

// PostTaskRetry handles POST /tasks/:id/retry
func (h *TaskHandler) PostTaskRetry(c *gin.Context) {
	taskID := c.Param("id")

	// Get current task to check if it can be retried
	task, err := h.taskManager.GetTask(context.TODO(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Task not found",
			"taskId": taskID,
		})
		return
	}

	// Check if task can be retried
	if task.Status != "failed" {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Task cannot be retried",
			"reason": "Only failed tasks can be retried",
			"currentStatus": task.Status,
		})
		return
	}

	// Retry the task
	err = h.taskManager.RetryTask(context.TODO(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retry task",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task retry initiated",
		"taskId":  taskID,
	})
}

// DeleteTask handles DELETE /tasks/:id
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	taskID := c.Param("id")

	// Get current task to check if it can be deleted
	task, err := h.taskManager.GetTask(context.TODO(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Task not found",
			"taskId": taskID,
		})
		return
	}

	// Check if task can be deleted
	if task.Status == "running" || task.Status == "pending" {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Task cannot be deleted",
			"reason": "Running or pending tasks must be cancelled first",
			"currentStatus": task.Status,
		})
		return
	}

	// Delete the task
	err = h.taskManager.DeleteTask(context.TODO(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete task",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task deleted successfully",
		"taskId":  taskID,
	})
}

// GetTaskProgress handles GET /tasks/:id/progress
func (h *TaskHandler) GetTaskProgress(c *gin.Context) {
	taskID := c.Param("id")

	// Get task to ensure it exists
	task, err := h.taskManager.GetTask(context.TODO(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Task not found",
			"taskId": taskID,
		})
		return
	}

	// Get progress from progress tracker
	progress, err := h.progressTracker.GetProgress(context.TODO(), taskID)
	if err != nil {
		// If no progress found, return basic info from task
		progressResponse := gin.H{
			"taskId": taskID,
			"status": task.Status,
		}

		if task.Progress != nil {
			progressResponse["totalItems"] = task.Progress.TotalItems
			progressResponse["processedItems"] = task.Progress.ProcessedItems
			progressResponse["percentComplete"] = task.Progress.PercentComplete
		}

		c.JSON(http.StatusOK, progressResponse)
		return
	}

	// Convert to API response format
	progressResponse := gin.H{
		"taskId":          taskID,
		"status":          task.Status,
		"totalItems":      progress.TotalItems,
		"processedItems":  progress.ProcessedItems,
		"percentComplete": progress.PercentComplete,
	}

	if progress.EstimatedTimeRemaining != nil {
		progressResponse["estimatedTimeRemaining"] = progress.EstimatedTimeRemaining.Duration.String()
	}

	c.JSON(http.StatusOK, progressResponse)
}

// GetTaskLogs handles GET /tasks/:id/logs
func (h *TaskHandler) GetTaskLogs(c *gin.Context) {
	taskID := c.Param("id")
	
	// Get task to ensure it exists
	_, err := h.taskManager.GetTask(context.TODO(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Task not found",
			"taskId": taskID,
		})
		return
	}

	// For now, return placeholder logs
	// In a full implementation, this would retrieve actual logs from pods
	logs := []gin.H{
		{
			"timestamp": "2025-01-01T10:00:00Z",
			"level":     "INFO",
			"message":   "Task started",
		},
		{
			"timestamp": "2025-01-01T10:00:01Z",
			"level":     "INFO",
			"message":   "Processing batch 1/10",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"taskId": taskID,
		"logs":   logs,
	})
}

// convertTaskToResponse converts an internal TaskInfo to API TaskResponse
func (h *TaskHandler) convertTaskToResponse(task *sync.TaskInfo) TaskResponse {
	response := TaskResponse{
		ID:         task.ID,
		Type:       task.Type,
		Status:     task.Status,
		ProjectKey: task.ProjectKey,
		StartedAt:  task.StartedAt.Format("2006-01-02T15:04:05Z"),
		Configuration: TaskConfiguration{
			IssueFilter:      task.Configuration.IssueFilter,
			ForceRefresh:     task.Configuration.ForceRefresh,
			ActiveIssuesOnly: task.Configuration.ActiveIssuesOnly,
			BatchSize:        task.Configuration.BatchSize,
			MaxRetries:       task.Configuration.MaxRetries,
		},
		CreatedBy:       task.CreatedBy,
		FinalCommitHash: task.FinalCommitHash,
		ErrorMessage:    task.ErrorMessage,
	}

	// Add completion time if available
	if task.CompletedAt != nil {
		completedAtStr := task.CompletedAt.Format("2006-01-02T15:04:05Z")
		response.CompletedAt = &completedAtStr
	}

	// Add progress if available
	if task.Progress != nil {
		progress := &TaskProgress{
			TotalItems:      task.Progress.TotalItems,
			ProcessedItems:  task.Progress.ProcessedItems,
			PercentComplete: task.Progress.PercentComplete,
		}
		
		if task.Progress.EstimatedRemaining != nil {
			estimatedStr := task.Progress.EstimatedRemaining.String()
			progress.EstimatedTimeRemaining = &estimatedStr
		}
		
		response.Progress = progress
	}

	return response
}
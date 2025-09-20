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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
	"github.com/company/jira-cdc-operator/internal/sync"
)

// ProjectHandler handles project-related API endpoints
type ProjectHandler struct {
	k8sClient   client.Client
	taskManager sync.TaskManager
}

// NewProjectHandler creates a new project handler
func NewProjectHandler(k8sClient client.Client, taskManager sync.TaskManager) *ProjectHandler {
	return &ProjectHandler{
		k8sClient:   k8sClient,
		taskManager: taskManager,
	}
}

// ProjectStatus represents the status of a project
type ProjectStatus struct {
	ProjectKey    string                 `json:"projectKey"`
	InstanceName  string                 `json:"instanceName"`
	Status        string                 `json:"status"`
	JiraURL       string                 `json:"jiraUrl"`
	GitRepository string                 `json:"gitRepository"`
	LastSync      *string                `json:"lastSync,omitempty"`
	NextSync      *string                `json:"nextSync,omitempty"`
	CurrentTask   *TaskInfo              `json:"currentTask,omitempty"`
	Operands      []OperandStatus        `json:"operands"`
	SyncStats     ProjectSyncStats       `json:"syncStats"`
	Configuration ProjectConfiguration   `json:"configuration"`
}

// TaskInfo represents basic task information for API responses
type TaskInfo struct {
	ID       string  `json:"id"`
	Type     string  `json:"type"`
	Status   string  `json:"status"`
	Progress *string `json:"progress,omitempty"`
}

// OperandStatus represents the status of an operand
type OperandStatus struct {
	Type      string `json:"type"`
	Ready     bool   `json:"ready"`
	Available bool   `json:"available"`
	Message   string `json:"message"`
	Replicas  int32  `json:"replicas"`
}

// ProjectSyncStats represents synchronization statistics
type ProjectSyncStats struct {
	TotalIssues     int32  `json:"totalIssues"`
	SyncedIssues    int32  `json:"syncedIssues"`
	FailedIssues    int32  `json:"failedIssues"`
	LastSyncTime    string `json:"lastSyncTime"`
	SyncDuration    string `json:"syncDuration"`
	AverageIssueTime string `json:"averageIssueTime"`
}

// ProjectConfiguration represents project configuration
type ProjectConfiguration struct {
	ActiveIssuesOnly bool   `json:"activeIssuesOnly"`
	IssueFilter      string `json:"issueFilter"`
	Schedule         string `json:"schedule"`
	BatchSize        int    `json:"batchSize"`
	MaxRetries       int    `json:"maxRetries"`
}

// SyncRequest represents a sync operation request
type SyncRequest struct {
	Type         string `json:"type" binding:"required"`         // bootstrap, reconciliation
	ForceRefresh bool   `json:"forceRefresh"`
	IssueFilter  string `json:"issueFilter,omitempty"`
	BatchSize    int    `json:"batchSize,omitempty"`
}

// GetProjects handles GET /projects
func (h *ProjectHandler) GetProjects(c *gin.Context) {
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "default"
	}

	// List all JiraCDC instances in the namespace
	jiradcdList := &jiradcdv1.JiraCDCList{}
	if err := h.k8sClient.List(context.TODO(), jiradcdList, client.InNamespace(namespace)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list JiraCDC instances",
			"details": err.Error(),
		})
		return
	}

	// Convert to project status list
	projects := make([]ProjectStatus, 0, len(jiradcdList.Items))
	for _, jiracdc := range jiradcdList.Items {
		status, err := h.getProjectStatus(&jiracdc)
		if err != nil {
			// Log error but continue with other projects
			continue
		}
		projects = append(projects, *status)
	}

	c.JSON(http.StatusOK, gin.H{
		"projects": projects,
		"total":    len(projects),
	})
}

// GetProject handles GET /projects/:key
func (h *ProjectHandler) GetProject(c *gin.Context) {
	projectKey := c.Param("key")
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "default"
	}

	// Find JiraCDC instance for this project
	jiracdc, err := h.findJiraCDCByProject(projectKey, namespace)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Project not found",
			"projectKey": projectKey,
		})
		return
	}

	// Get project status
	status, err := h.getProjectStatus(jiracdc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get project status",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, status)
}

// PostProjectSync handles POST /projects/:key/sync
func (h *ProjectHandler) PostProjectSync(c *gin.Context) {
	projectKey := c.Param("key")
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "default"
	}

	var request SyncRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Find JiraCDC instance for this project
	jiracdc, err := h.findJiraCDCByProject(projectKey, namespace)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Project not found",
			"projectKey": projectKey,
		})
		return
	}

	// Check if there's already a running task
	currentTask, err := h.taskManager.GetCurrentTask(context.TODO(), projectKey)
	if err == nil && currentTask != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Sync already in progress",
			"currentTask": TaskInfo{
				ID:     currentTask.ID,
				Type:   currentTask.Type,
				Status: currentTask.Status,
			},
		})
		return
	}

	// Create sync task
	taskInfo := sync.TaskInfo{
		Type:       request.Type,
		ProjectKey: projectKey,
		Status:     "pending",
		Configuration: sync.TaskConfiguration{
			ForceRefresh:     request.ForceRefresh,
			IssueFilter:      request.IssueFilter,
			ActiveIssuesOnly: jiracdc.Spec.SyncTarget.ActiveIssuesOnly,
			BatchSize:        request.BatchSize,
			MaxRetries:       3,
		},
		CreatedBy: "api",
	}

	// Set default batch size if not specified
	if taskInfo.Configuration.BatchSize <= 0 {
		taskInfo.Configuration.BatchSize = 50
	}

	// Create the task
	createdTask, err := h.taskManager.CreateTask(context.TODO(), taskInfo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create sync task",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"message": "Sync task created",
		"task": TaskInfo{
			ID:     createdTask.ID,
			Type:   createdTask.Type,
			Status: createdTask.Status,
		},
	})
}

// GetProjectHealth handles GET /projects/:key/health
func (h *ProjectHandler) GetProjectHealth(c *gin.Context) {
	projectKey := c.Param("key")
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "default"
	}

	// Find JiraCDC instance for this project
	jiracdc, err := h.findJiraCDCByProject(projectKey, namespace)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Project not found",
			"projectKey": projectKey,
		})
		return
	}

	// Check operand health
	health := gin.H{
		"projectKey": projectKey,
		"healthy":    true,
		"operands":   []gin.H{},
	}

	// Check if JiraCDC instance is ready
	if jiracdc.Status.Phase != "Ready" {
		health["healthy"] = false
		health["message"] = "JiraCDC instance not ready"
	}

	c.JSON(http.StatusOK, health)
}

// getProjectStatus converts a JiraCDC instance to ProjectStatus
func (h *ProjectHandler) getProjectStatus(jiracdc *jiradcdv1.JiraCDC) (*ProjectStatus, error) {
	status := &ProjectStatus{
		ProjectKey:   jiracdc.Spec.JiraInstance.ProjectKey,
		InstanceName: jiracdc.Name,
		Status:       string(jiracdc.Status.Phase),
		JiraURL:      jiracdc.Spec.JiraInstance.URL,
		GitRepository: jiracdc.Spec.GitRepository.URL,
		Operands:     []OperandStatus{},
		SyncStats: ProjectSyncStats{
			TotalIssues:  0,
			SyncedIssues: 0,
			FailedIssues: 0,
		},
		Configuration: ProjectConfiguration{
			ActiveIssuesOnly: jiracdc.Spec.SyncTarget.ActiveIssuesOnly,
			IssueFilter:      jiracdc.Spec.SyncTarget.IssueFilter,
			Schedule:         jiracdc.Spec.SyncTarget.Schedule,
			BatchSize:        50, // Default batch size
			MaxRetries:       3,  // Default max retries
		},
	}

	// Get current task if any
	currentTask, err := h.taskManager.GetCurrentTask(context.TODO(), jiracdc.Spec.JiraInstance.ProjectKey)
	if err == nil && currentTask != nil {
		status.CurrentTask = &TaskInfo{
			ID:     currentTask.ID,
			Type:   currentTask.Type,
			Status: currentTask.Status,
		}
		if currentTask.Progress != nil {
			progressStr := strconv.FormatFloat(float64(currentTask.Progress.PercentComplete), 'f', 1, 32) + "%"
			status.CurrentTask.Progress = &progressStr
		}
	}

	// Add operand status from JiraCDC status
	if jiracdc.Status.Operands != nil {
		for _, operand := range jiracdc.Status.Operands {
			status.Operands = append(status.Operands, OperandStatus{
				Type:      operand.Type,
				Ready:     operand.Ready,
				Available: operand.Available,
				Message:   operand.Message,
				Replicas:  operand.Replicas,
			})
		}
	}

	// Add sync statistics from status
	if jiracdc.Status.SyncStatus != nil {
		status.SyncStats.TotalIssues = jiracdc.Status.SyncStatus.TotalIssues
		status.SyncStats.SyncedIssues = jiracdc.Status.SyncStatus.SyncedIssues
		if jiracdc.Status.SyncStatus.LastSyncTime != nil {
			status.SyncStats.LastSyncTime = jiracdc.Status.SyncStatus.LastSyncTime.String()
			status.LastSync = &status.SyncStats.LastSyncTime
		}
	}

	return status, nil
}

// findJiraCDCByProject finds a JiraCDC instance by project key
func (h *ProjectHandler) findJiraCDCByProject(projectKey, namespace string) (*jiradcdv1.JiraCDC, error) {
	// List all JiraCDC instances in the namespace
	jiradcdList := &jiradcdv1.JiraCDCList{}
	if err := h.k8sClient.List(context.TODO(), jiradcdList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	// Find instance matching the project key
	for _, jiracdc := range jiradcdList.Items {
		if jiracdc.Spec.JiraInstance.ProjectKey == projectKey {
			return &jiracdc, nil
		}
	}

	return nil, client.ErrNotFound
}

// GetProjectMetrics handles GET /projects/:key/metrics
func (h *ProjectHandler) GetProjectMetrics(c *gin.Context) {
	projectKey := c.Param("key")
	namespace := c.Query("namespace")
	if namespace == "" {
		namespace = "default"
	}

	// Find JiraCDC instance for this project
	jiracdc, err := h.findJiraCDCByProject(projectKey, namespace)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Project not found",
			"projectKey": projectKey,
		})
		return
	}

	// Get metrics from task manager and other sources
	metrics := gin.H{
		"projectKey": projectKey,
		"instance":   jiracdc.Name,
		"metrics": gin.H{
			"totalTasks":       0,
			"completedTasks":   0,
			"failedTasks":      0,
			"averageTaskTime":  "0s",
			"lastTaskTime":     nil,
		},
	}

	// Get task statistics
	filters := sync.TaskFilters{
		ProjectKey: projectKey,
		Limit:      100,
	}
	
	tasks, err := h.taskManager.ListTasks(context.TODO(), filters)
	if err == nil {
		totalTasks := len(tasks)
		completedTasks := 0
		failedTasks := 0

		for _, task := range tasks {
			switch task.Status {
			case "completed":
				completedTasks++
			case "failed":
				failedTasks++
			}
		}

		metrics["metrics"] = gin.H{
			"totalTasks":     totalTasks,
			"completedTasks": completedTasks,
			"failedTasks":    failedTasks,
		}
	}

	c.JSON(http.StatusOK, metrics)
}
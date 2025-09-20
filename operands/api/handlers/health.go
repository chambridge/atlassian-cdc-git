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
	"time"

	"github.com/gin-gonic/gin"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/company/jira-cdc-operator/internal/jira"
	"github.com/company/jira-cdc-operator/internal/git"
	"github.com/company/jira-cdc-operator/internal/sync"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	k8sClient   client.Client
	jiraClient  *jira.Client
	gitManager  *git.Manager
	taskManager sync.TaskManager
	startTime   time.Time
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(
	k8sClient client.Client,
	jiraClient *jira.Client,
	gitManager *git.Manager,
	taskManager sync.TaskManager,
) *HealthHandler {
	return &HealthHandler{
		k8sClient:   k8sClient,
		jiraClient:  jiraClient,
		gitManager:  gitManager,
		taskManager: taskManager,
		startTime:   time.Now(),
	}
}

// HealthResponse represents the overall health status
type HealthResponse struct {
	Status      string            `json:"status"`     // healthy, degraded, unhealthy
	Timestamp   string            `json:"timestamp"`
	Uptime      string            `json:"uptime"`
	Version     string            `json:"version"`
	Components  []ComponentHealth `json:"components"`
	Summary     HealthSummary     `json:"summary"`
}

// ComponentHealth represents the health of an individual component
type ComponentHealth struct {
	Name        string            `json:"name"`
	Status      string            `json:"status"`     // healthy, degraded, unhealthy
	Message     string            `json:"message,omitempty"`
	LastChecked string            `json:"lastChecked"`
	Details     map[string]string `json:"details,omitempty"`
	ResponseTime string           `json:"responseTime,omitempty"`
}

// HealthSummary provides a summary of health status
type HealthSummary struct {
	HealthyComponents   int `json:"healthyComponents"`
	DegradedComponents  int `json:"degradedComponents"`
	UnhealthyComponents int `json:"unhealthyComponents"`
	TotalComponents     int `json:"totalComponents"`
}

// ReadinessResponse represents readiness status
type ReadinessResponse struct {
	Ready     bool              `json:"ready"`
	Timestamp string            `json:"timestamp"`
	Checks    []ReadinessCheck  `json:"checks"`
}

// ReadinessCheck represents an individual readiness check
type ReadinessCheck struct {
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Message string `json:"message,omitempty"`
}

// GetHealth handles GET /health
func (h *HealthHandler) GetHealth(c *gin.Context) {
	ctx := context.Background()
	checkTime := time.Now()

	// Perform health checks for all components
	components := []ComponentHealth{
		h.checkKubernetesHealth(ctx, checkTime),
		h.checkJiraHealth(ctx, checkTime),
		h.checkGitHealth(ctx, checkTime),
		h.checkTaskManagerHealth(ctx, checkTime),
		h.checkDatabaseHealth(ctx, checkTime),
	}

	// Calculate overall status
	overallStatus := h.calculateOverallStatus(components)

	// Create summary
	summary := h.createHealthSummary(components)

	response := HealthResponse{
		Status:     overallStatus,
		Timestamp:  checkTime.Format(time.RFC3339),
		Uptime:     time.Since(h.startTime).String(),
		Version:    "1.0.0", // This could come from build info
		Components: components,
		Summary:    summary,
	}

	// Set HTTP status based on health
	httpStatus := http.StatusOK
	if overallStatus == "unhealthy" {
		httpStatus = http.StatusServiceUnavailable
	} else if overallStatus == "degraded" {
		httpStatus = http.StatusOK // Still serving requests
	}

	c.JSON(httpStatus, response)
}

// GetReady handles GET /ready
func (h *HealthHandler) GetReady(c *gin.Context) {
	ctx := context.Background()
	checkTime := time.Now()

	// Perform readiness checks
	checks := []ReadinessCheck{
		h.checkKubernetesReadiness(ctx),
		h.checkJiraReadiness(ctx),
		h.checkGitReadiness(ctx),
		h.checkTaskManagerReadiness(ctx),
	}

	// Determine overall readiness
	ready := true
	for _, check := range checks {
		if !check.Ready {
			ready = false
			break
		}
	}

	response := ReadinessResponse{
		Ready:     ready,
		Timestamp: checkTime.Format(time.RFC3339),
		Checks:    checks,
	}

	// Set HTTP status based on readiness
	httpStatus := http.StatusOK
	if !ready {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, response)
}

// GetLive handles GET /live
func (h *HealthHandler) GetLive(c *gin.Context) {
	// Simple liveness check - if we can respond, we're alive
	c.JSON(http.StatusOK, gin.H{
		"status":    "alive",
		"timestamp": time.Now().Format(time.RFC3339),
		"uptime":    time.Since(h.startTime).String(),
	})
}

// checkKubernetesHealth checks Kubernetes API connectivity
func (h *HealthHandler) checkKubernetesHealth(ctx context.Context, checkTime time.Time) ComponentHealth {
	start := time.Now()
	
	// Try to list namespaces as a simple connectivity test
	namespaceList := &client.ObjectList{}
	err := h.k8sClient.List(ctx, namespaceList)
	
	responseTime := time.Since(start)
	
	if err != nil {
		return ComponentHealth{
			Name:         "kubernetes",
			Status:       "unhealthy",
			Message:      "Cannot connect to Kubernetes API",
			LastChecked:  checkTime.Format(time.RFC3339),
			ResponseTime: responseTime.String(),
			Details: map[string]string{
				"error": err.Error(),
			},
		}
	}

	return ComponentHealth{
		Name:         "kubernetes",
		Status:       "healthy",
		Message:      "Kubernetes API accessible",
		LastChecked:  checkTime.Format(time.RFC3339),
		ResponseTime: responseTime.String(),
	}
}

// checkJiraHealth checks JIRA connectivity
func (h *HealthHandler) checkJiraHealth(ctx context.Context, checkTime time.Time) ComponentHealth {
	if h.jiraClient == nil {
		return ComponentHealth{
			Name:        "jira",
			Status:      "unhealthy",
			Message:     "JIRA client not initialized",
			LastChecked: checkTime.Format(time.RFC3339),
		}
	}

	start := time.Now()
	
	// Try to get server info as a simple connectivity test
	_, err := h.jiraClient.GetServerInfo(ctx)
	
	responseTime := time.Since(start)
	
	if err != nil {
		return ComponentHealth{
			Name:         "jira",
			Status:       "unhealthy",
			Message:      "Cannot connect to JIRA",
			LastChecked:  checkTime.Format(time.RFC3339),
			ResponseTime: responseTime.String(),
			Details: map[string]string{
				"error": err.Error(),
			},
		}
	}

	status := "healthy"
	message := "JIRA accessible"
	
	// Check response time and adjust status if needed
	if responseTime > 5*time.Second {
		status = "degraded"
		message = "JIRA responding slowly"
	}

	return ComponentHealth{
		Name:         "jira",
		Status:       status,
		Message:      message,
		LastChecked:  checkTime.Format(time.RFC3339),
		ResponseTime: responseTime.String(),
	}
}

// checkGitHealth checks Git repository connectivity
func (h *HealthHandler) checkGitHealth(ctx context.Context, checkTime time.Time) ComponentHealth {
	if h.gitManager == nil {
		return ComponentHealth{
			Name:        "git",
			Status:      "unhealthy",
			Message:     "Git manager not initialized",
			LastChecked: checkTime.Format(time.RFC3339),
		}
	}

	start := time.Now()
	
	// Try to get current branch as a simple connectivity test
	_, err := h.gitManager.GetCurrentBranch(ctx)
	
	responseTime := time.Since(start)
	
	if err != nil {
		return ComponentHealth{
			Name:         "git",
			Status:       "degraded", // Git issues are usually not critical for API health
			Message:      "Git repository not accessible",
			LastChecked:  checkTime.Format(time.RFC3339),
			ResponseTime: responseTime.String(),
			Details: map[string]string{
				"error": err.Error(),
			},
		}
	}

	return ComponentHealth{
		Name:         "git",
		Status:       "healthy",
		Message:      "Git repository accessible",
		LastChecked:  checkTime.Format(time.RFC3339),
		ResponseTime: responseTime.String(),
	}
}

// checkTaskManagerHealth checks task manager functionality
func (h *HealthHandler) checkTaskManagerHealth(ctx context.Context, checkTime time.Time) ComponentHealth {
	if h.taskManager == nil {
		return ComponentHealth{
			Name:        "taskManager",
			Status:      "unhealthy",
			Message:     "Task manager not initialized",
			LastChecked: checkTime.Format(time.RFC3339),
		}
	}

	start := time.Now()
	
	// Try to list tasks as a simple functionality test
	_, err := h.taskManager.ListTasks(ctx, sync.TaskFilters{Limit: 1})
	
	responseTime := time.Since(start)
	
	if err != nil {
		return ComponentHealth{
			Name:         "taskManager",
			Status:       "unhealthy",
			Message:      "Task manager not functioning",
			LastChecked:  checkTime.Format(time.RFC3339),
			ResponseTime: responseTime.String(),
			Details: map[string]string{
				"error": err.Error(),
			},
		}
	}

	return ComponentHealth{
		Name:         "taskManager",
		Status:       "healthy",
		Message:      "Task manager functioning",
		LastChecked:  checkTime.Format(time.RFC3339),
		ResponseTime: responseTime.String(),
	}
}

// checkDatabaseHealth checks database connectivity (placeholder)
func (h *HealthHandler) checkDatabaseHealth(ctx context.Context, checkTime time.Time) ComponentHealth {
	// For now, we don't have a separate database, so this is a placeholder
	return ComponentHealth{
		Name:        "database",
		Status:      "healthy",
		Message:     "No external database configured",
		LastChecked: checkTime.Format(time.RFC3339),
		Details: map[string]string{
			"type": "embedded",
		},
	}
}

// Readiness check methods
func (h *HealthHandler) checkKubernetesReadiness(ctx context.Context) ReadinessCheck {
	err := h.k8sClient.List(ctx, &client.ObjectList{})
	if err != nil {
		return ReadinessCheck{
			Name:    "kubernetes",
			Ready:   false,
			Message: "Kubernetes API not accessible",
		}
	}
	return ReadinessCheck{
		Name:  "kubernetes",
		Ready: true,
	}
}

func (h *HealthHandler) checkJiraReadiness(ctx context.Context) ReadinessCheck {
	if h.jiraClient == nil {
		return ReadinessCheck{
			Name:    "jira",
			Ready:   false,
			Message: "JIRA client not initialized",
		}
	}

	_, err := h.jiraClient.GetServerInfo(ctx)
	if err != nil {
		return ReadinessCheck{
			Name:    "jira",
			Ready:   false,
			Message: "JIRA not accessible",
		}
	}
	return ReadinessCheck{
		Name:  "jira",
		Ready: true,
	}
}

func (h *HealthHandler) checkGitReadiness(ctx context.Context) ReadinessCheck {
	if h.gitManager == nil {
		return ReadinessCheck{
			Name:    "git",
			Ready:   false,
			Message: "Git manager not initialized",
		}
	}

	// Git readiness is less critical for serving API requests
	return ReadinessCheck{
		Name:  "git",
		Ready: true,
	}
}

func (h *HealthHandler) checkTaskManagerReadiness(ctx context.Context) ReadinessCheck {
	if h.taskManager == nil {
		return ReadinessCheck{
			Name:    "taskManager",
			Ready:   false,
			Message: "Task manager not initialized",
		}
	}

	_, err := h.taskManager.ListTasks(ctx, sync.TaskFilters{Limit: 1})
	if err != nil {
		return ReadinessCheck{
			Name:    "taskManager",
			Ready:   false,
			Message: "Task manager not functioning",
		}
	}
	return ReadinessCheck{
		Name:  "taskManager",
		Ready: true,
	}
}

// calculateOverallStatus determines overall health status from components
func (h *HealthHandler) calculateOverallStatus(components []ComponentHealth) string {
	unhealthyCount := 0
	degradedCount := 0

	for _, component := range components {
		switch component.Status {
		case "unhealthy":
			unhealthyCount++
		case "degraded":
			degradedCount++
		}
	}

	// If any critical component is unhealthy, overall status is unhealthy
	if unhealthyCount > 0 {
		return "unhealthy"
	}

	// If any component is degraded, overall status is degraded
	if degradedCount > 0 {
		return "degraded"
	}

	return "healthy"
}

// createHealthSummary creates a summary of component health
func (h *HealthHandler) createHealthSummary(components []ComponentHealth) HealthSummary {
	summary := HealthSummary{
		TotalComponents: len(components),
	}

	for _, component := range components {
		switch component.Status {
		case "healthy":
			summary.HealthyComponents++
		case "degraded":
			summary.DegradedComponents++
		case "unhealthy":
			summary.UnhealthyComponents++
		}
	}

	return summary
}
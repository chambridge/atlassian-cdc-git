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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
	"github.com/company/jira-cdc-operator/internal/sync"
)

// MetricsHandler handles metrics-related endpoints
type MetricsHandler struct {
	k8sClient   client.Client
	taskManager sync.TaskManager
	registry    *prometheus.Registry
	metrics     *MetricsCollector
}

// MetricsCollector holds Prometheus metrics
type MetricsCollector struct {
	// Task metrics
	tasksTotal          *prometheus.CounterVec
	tasksCompleted      *prometheus.CounterVec
	tasksFailed         *prometheus.CounterVec
	taskDuration        *prometheus.HistogramVec
	tasksActive         *prometheus.GaugeVec
	
	// Issue metrics
	issuesProcessed     *prometheus.CounterVec
	issuesSynced        *prometheus.GaugeVec
	issueErrors         *prometheus.CounterVec
	
	// API metrics
	apiRequests         *prometheus.CounterVec
	apiDuration         *prometheus.HistogramVec
	
	// System metrics
	systemInfo          *prometheus.GaugeVec
	lastSyncTime        *prometheus.GaugeVec
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(k8sClient client.Client, taskManager sync.TaskManager) *MetricsHandler {
	registry := prometheus.NewRegistry()
	metrics := newMetricsCollector()
	
	// Register metrics with registry
	registry.MustRegister(
		metrics.tasksTotal,
		metrics.tasksCompleted,
		metrics.tasksFailed,
		metrics.taskDuration,
		metrics.tasksActive,
		metrics.issuesProcessed,
		metrics.issuesSynced,
		metrics.issueErrors,
		metrics.apiRequests,
		metrics.apiDuration,
		metrics.systemInfo,
		metrics.lastSyncTime,
	)

	return &MetricsHandler{
		k8sClient:   k8sClient,
		taskManager: taskManager,
		registry:    registry,
		metrics:     metrics,
	}
}

// newMetricsCollector creates a new metrics collector with all Prometheus metrics
func newMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		tasksTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "jiracdc_tasks_total",
				Help: "Total number of sync tasks created",
			},
			[]string{"project_key", "task_type", "instance"},
		),
		tasksCompleted: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "jiracdc_tasks_completed_total",
				Help: "Total number of completed sync tasks",
			},
			[]string{"project_key", "task_type", "instance"},
		),
		tasksFailed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "jiracdc_tasks_failed_total",
				Help: "Total number of failed sync tasks",
			},
			[]string{"project_key", "task_type", "instance"},
		),
		taskDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "jiracdc_task_duration_seconds",
				Help:    "Duration of sync tasks in seconds",
				Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~17 minutes
			},
			[]string{"project_key", "task_type", "instance"},
		),
		tasksActive: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "jiracdc_tasks_active",
				Help: "Number of currently active sync tasks",
			},
			[]string{"project_key", "task_type", "instance"},
		),
		issuesProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "jiracdc_issues_processed_total",
				Help: "Total number of issues processed",
			},
			[]string{"project_key", "instance", "status"},
		),
		issuesSynced: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "jiracdc_issues_synced",
				Help: "Number of issues currently synced",
			},
			[]string{"project_key", "instance"},
		),
		issueErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "jiracdc_issue_errors_total",
				Help: "Total number of issue processing errors",
			},
			[]string{"project_key", "instance", "error_type"},
		),
		apiRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "jiracdc_api_requests_total",
				Help: "Total number of API requests",
			},
			[]string{"method", "endpoint", "status_code"},
		),
		apiDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "jiracdc_api_request_duration_seconds",
				Help:    "Duration of API requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		systemInfo: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "jiracdc_system_info",
				Help: "System information",
			},
			[]string{"version", "instance", "namespace"},
		),
		lastSyncTime: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "jiracdc_last_sync_timestamp",
				Help: "Timestamp of last successful sync",
			},
			[]string{"project_key", "instance"},
		),
	}
}

// GetPrometheusMetrics handles GET /metrics (Prometheus format)
func (h *MetricsHandler) GetPrometheusMetrics() gin.HandlerFunc {
	handler := promhttp.HandlerFor(h.registry, promhttp.HandlerOpts{})
	return gin.WrapH(handler)
}

// GetMetrics handles GET /api/metrics (JSON format)
func (h *MetricsHandler) GetMetrics(c *gin.Context) {
	ctx := context.Background()
	
	// Collect current metrics
	metrics, err := h.collectCurrentMetrics(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to collect metrics",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// GetProjectMetrics handles GET /api/projects/:key/metrics
func (h *MetricsHandler) GetProjectMetrics(c *gin.Context) {
	projectKey := c.Param("key")
	ctx := context.Background()
	
	// Collect project-specific metrics
	metrics, err := h.collectProjectMetrics(ctx, projectKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to collect project metrics",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

// collectCurrentMetrics collects current system-wide metrics
func (h *MetricsHandler) collectCurrentMetrics(ctx context.Context) (gin.H, error) {
	// Get task statistics
	allTasks, err := h.taskManager.ListTasks(ctx, sync.TaskFilters{Limit: 1000})
	if err != nil {
		return nil, err
	}

	// Process task statistics
	taskStats := h.processTaskStatistics(allTasks)

	// Get JiraCDC instances
	jiradcdList := &jiradcdv1.JiraCDCList{}
	if err := h.k8sClient.List(ctx, jiradcdList); err != nil {
		return nil, err
	}

	// Process instance statistics
	instanceStats := h.processInstanceStatistics(jiradcdList.Items)

	metrics := gin.H{
		"timestamp":  time.Now().Format(time.RFC3339),
		"tasks":      taskStats,
		"instances":  instanceStats,
		"summary": gin.H{
			"total_instances": len(jiradcdList.Items),
			"total_tasks":     len(allTasks),
		},
	}

	return metrics, nil
}

// collectProjectMetrics collects metrics for a specific project
func (h *MetricsHandler) collectProjectMetrics(ctx context.Context, projectKey string) (gin.H, error) {
	// Get tasks for this project
	tasks, err := h.taskManager.ListTasks(ctx, sync.TaskFilters{
		ProjectKey: projectKey,
		Limit:      1000,
	})
	if err != nil {
		return nil, err
	}

	// Process task statistics
	taskStats := h.processTaskStatistics(tasks)

	// Find JiraCDC instance for this project
	jiradcdList := &jiradcdv1.JiraCDCList{}
	if err := h.k8sClient.List(ctx, jiradcdList); err != nil {
		return nil, err
	}

	var projectInstance *jiradcdv1.JiraCDC
	for _, instance := range jiradcdList.Items {
		if instance.Spec.JiraInstance.ProjectKey == projectKey {
			projectInstance = &instance
			break
		}
	}

	metrics := gin.H{
		"projectKey": projectKey,
		"timestamp":  time.Now().Format(time.RFC3339),
		"tasks":      taskStats,
	}

	if projectInstance != nil {
		metrics["instance"] = gin.H{
			"name":      projectInstance.Name,
			"namespace": projectInstance.Namespace,
			"status":    projectInstance.Status.Phase,
		}

		if projectInstance.Status.SyncStatus != nil {
			metrics["syncStatus"] = gin.H{
				"totalIssues":  projectInstance.Status.SyncStatus.TotalIssues,
				"syncedIssues": projectInstance.Status.SyncStatus.SyncedIssues,
				"lastSyncTime": projectInstance.Status.SyncStatus.LastSyncTime,
			}
		}
	}

	return metrics, nil
}

// processTaskStatistics processes task statistics
func (h *MetricsHandler) processTaskStatistics(tasks []*sync.TaskInfo) gin.H {
	stats := gin.H{
		"total":     len(tasks),
		"pending":   0,
		"running":   0,
		"completed": 0,
		"failed":    0,
		"cancelled": 0,
		"by_type": gin.H{
			"bootstrap":      0,
			"reconciliation": 0,
			"maintenance":    0,
		},
	}

	totalDuration := time.Duration(0)
	completedTasks := 0

	for _, task := range tasks {
		// Count by status
		switch task.Status {
		case "pending":
			stats["pending"] = stats["pending"].(int) + 1
		case "running":
			stats["running"] = stats["running"].(int) + 1
		case "completed":
			stats["completed"] = stats["completed"].(int) + 1
			completedTasks++
			if task.CompletedAt != nil {
				duration := task.CompletedAt.Sub(task.StartedAt)
				totalDuration += duration
			}
		case "failed":
			stats["failed"] = stats["failed"].(int) + 1
		case "cancelled":
			stats["cancelled"] = stats["cancelled"].(int) + 1
		}

		// Count by type
		byType := stats["by_type"].(gin.H)
		switch task.Type {
		case "bootstrap":
			byType["bootstrap"] = byType["bootstrap"].(int) + 1
		case "reconciliation":
			byType["reconciliation"] = byType["reconciliation"].(int) + 1
		case "maintenance":
			byType["maintenance"] = byType["maintenance"].(int) + 1
		}
	}

	// Calculate average duration
	if completedTasks > 0 {
		avgDuration := totalDuration / time.Duration(completedTasks)
		stats["average_duration"] = avgDuration.String()
	} else {
		stats["average_duration"] = "0s"
	}

	return stats
}

// processInstanceStatistics processes JiraCDC instance statistics
func (h *MetricsHandler) processInstanceStatistics(instances []jiradcdv1.JiraCDC) gin.H {
	stats := gin.H{
		"total":   len(instances),
		"ready":   0,
		"pending": 0,
		"failed":  0,
		"by_project": gin.H{},
	}

	byProject := stats["by_project"].(gin.H)

	for _, instance := range instances {
		// Count by status
		switch instance.Status.Phase {
		case "Ready":
			stats["ready"] = stats["ready"].(int) + 1
		case "Pending":
			stats["pending"] = stats["pending"].(int) + 1
		case "Failed":
			stats["failed"] = stats["failed"].(int) + 1
		}

		// Track by project
		projectKey := instance.Spec.JiraInstance.ProjectKey
		byProject[projectKey] = gin.H{
			"instance": instance.Name,
			"status":   instance.Status.Phase,
		}
	}

	return stats
}

// RecordTaskMetric records a task-related metric
func (h *MetricsHandler) RecordTaskMetric(projectKey, taskType, instance, status string) {
	switch status {
	case "created":
		h.metrics.tasksTotal.WithLabelValues(projectKey, taskType, instance).Inc()
		h.metrics.tasksActive.WithLabelValues(projectKey, taskType, instance).Inc()
	case "completed":
		h.metrics.tasksCompleted.WithLabelValues(projectKey, taskType, instance).Inc()
		h.metrics.tasksActive.WithLabelValues(projectKey, taskType, instance).Dec()
	case "failed":
		h.metrics.tasksFailed.WithLabelValues(projectKey, taskType, instance).Inc()
		h.metrics.tasksActive.WithLabelValues(projectKey, taskType, instance).Dec()
	}
}

// RecordTaskDuration records task duration
func (h *MetricsHandler) RecordTaskDuration(projectKey, taskType, instance string, duration time.Duration) {
	h.metrics.taskDuration.WithLabelValues(projectKey, taskType, instance).Observe(duration.Seconds())
}

// RecordIssueMetric records an issue-related metric
func (h *MetricsHandler) RecordIssueMetric(projectKey, instance, status string, count int) {
	h.metrics.issuesProcessed.WithLabelValues(projectKey, instance, status).Add(float64(count))
}

// UpdateSyncedIssues updates the count of synced issues
func (h *MetricsHandler) UpdateSyncedIssues(projectKey, instance string, count int) {
	h.metrics.issuesSynced.WithLabelValues(projectKey, instance).Set(float64(count))
}

// RecordAPIRequest records an API request metric
func (h *MetricsHandler) RecordAPIRequest(method, endpoint, statusCode string, duration time.Duration) {
	h.metrics.apiRequests.WithLabelValues(method, endpoint, statusCode).Inc()
	h.metrics.apiDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// UpdateLastSyncTime updates the last sync timestamp
func (h *MetricsHandler) UpdateLastSyncTime(projectKey, instance string, timestamp time.Time) {
	h.metrics.lastSyncTime.WithLabelValues(projectKey, instance).Set(float64(timestamp.Unix()))
}

// SetSystemInfo sets system information metrics
func (h *MetricsHandler) SetSystemInfo(version, instance, namespace string) {
	h.metrics.systemInfo.WithLabelValues(version, instance, namespace).Set(1)
}
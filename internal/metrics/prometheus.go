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

package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Metric names and labels
const (
	MetricPrefix = "jiracdc_"
	
	// Label names
	LabelInstance    = "instance"
	LabelProject     = "project"
	LabelOperation   = "operation"
	LabelStatus      = "status"
	LabelError       = "error"
	LabelAuthType    = "auth_type"
	LabelRepoType    = "repo_type"
)

// JiraCDCMetrics contains all Prometheus metrics for JIRA CDC
type JiraCDCMetrics struct {
	// Sync operation metrics
	SyncOperationsTotal    *prometheus.CounterVec
	SyncOperationDuration  *prometheus.HistogramVec
	SyncOperationsActive   *prometheus.GaugeVec
	
	// Issue synchronization metrics
	IssuesProcessedTotal   *prometheus.CounterVec
	IssuesFailedTotal      *prometheus.CounterVec
	IssuesSyncDuration     *prometheus.HistogramVec
	
	// Git operation metrics
	GitOperationsTotal     *prometheus.CounterVec
	GitOperationDuration   *prometheus.HistogramVec
	GitCommitsTotal        *prometheus.CounterVec
	GitFilesModifiedTotal  *prometheus.CounterVec
	
	// JIRA API metrics
	JiraAPIRequestsTotal   *prometheus.CounterVec
	JiraAPIRequestDuration *prometheus.HistogramVec
	JiraAPIRateLimitHits   *prometheus.CounterVec
	JiraAPIErrors          *prometheus.CounterVec
	
	// Authentication metrics
	AuthRefreshTotal       *prometheus.CounterVec
	AuthFailuresTotal      *prometheus.CounterVec
	
	// Resource metrics
	WatchedSecretsTotal    prometheus.Gauge
	ActiveConnectionsTotal *prometheus.GaugeVec
	
	// Performance metrics
	MemoryUsageBytes       prometheus.Gauge
	CPUUsagePercent        prometheus.Gauge
	
	// Health metrics
	HealthChecksTotal      *prometheus.CounterVec
	ComponentHealthStatus  *prometheus.GaugeVec
}

var (
	// Global metrics instance
	Metrics *JiraCDCMetrics
)

// InitMetrics initializes all Prometheus metrics
func InitMetrics() *JiraCDCMetrics {
	m := &JiraCDCMetrics{
		// Sync operation metrics
		SyncOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "sync_operations_total",
				Help: "Total number of sync operations by type and status",
			},
			[]string{LabelInstance, LabelProject, LabelOperation, LabelStatus},
		),
		
		SyncOperationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    MetricPrefix + "sync_operation_duration_seconds",
				Help:    "Duration of sync operations in seconds",
				Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~17min
			},
			[]string{LabelInstance, LabelProject, LabelOperation},
		),
		
		SyncOperationsActive: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: MetricPrefix + "sync_operations_active",
				Help: "Number of currently active sync operations",
			},
			[]string{LabelInstance, LabelProject, LabelOperation},
		),
		
		// Issue synchronization metrics
		IssuesProcessedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "issues_processed_total",
				Help: "Total number of issues processed by project",
			},
			[]string{LabelInstance, LabelProject, LabelStatus},
		),
		
		IssuesFailedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "issues_failed_total",
				Help: "Total number of issues that failed processing",
			},
			[]string{LabelInstance, LabelProject, LabelError},
		),
		
		IssuesSyncDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    MetricPrefix + "issues_sync_duration_seconds",
				Help:    "Duration of individual issue sync operations",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 8), // 100ms to ~25s
			},
			[]string{LabelInstance, LabelProject},
		),
		
		// Git operation metrics
		GitOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "git_operations_total",
				Help: "Total number of git operations by type and status",
			},
			[]string{LabelInstance, LabelProject, LabelOperation, LabelStatus},
		),
		
		GitOperationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    MetricPrefix + "git_operation_duration_seconds",
				Help:    "Duration of git operations in seconds",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 100ms to ~1.7min
			},
			[]string{LabelInstance, LabelProject, LabelOperation},
		),
		
		GitCommitsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "git_commits_total",
				Help: "Total number of git commits created",
			},
			[]string{LabelInstance, LabelProject},
		),
		
		GitFilesModifiedTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "git_files_modified_total",
				Help: "Total number of git files modified",
			},
			[]string{LabelInstance, LabelProject, LabelOperation},
		),
		
		// JIRA API metrics
		JiraAPIRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "jira_api_requests_total",
				Help: "Total number of JIRA API requests by endpoint and status",
			},
			[]string{LabelInstance, LabelProject, "endpoint", LabelStatus},
		),
		
		JiraAPIRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    MetricPrefix + "jira_api_request_duration_seconds",
				Help:    "Duration of JIRA API requests in seconds",
				Buckets: prometheus.ExponentialBuckets(0.01, 2, 10), // 10ms to ~10s
			},
			[]string{LabelInstance, LabelProject, "endpoint"},
		),
		
		JiraAPIRateLimitHits: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "jira_api_rate_limit_hits_total",
				Help: "Total number of JIRA API rate limit hits",
			},
			[]string{LabelInstance, LabelProject},
		),
		
		JiraAPIErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "jira_api_errors_total",
				Help: "Total number of JIRA API errors by type",
			},
			[]string{LabelInstance, LabelProject, LabelError},
		),
		
		// Authentication metrics
		AuthRefreshTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "auth_refresh_total",
				Help: "Total number of authentication refreshes",
			},
			[]string{LabelInstance, LabelAuthType, LabelStatus},
		),
		
		AuthFailuresTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "auth_failures_total",
				Help: "Total number of authentication failures",
			},
			[]string{LabelInstance, LabelAuthType, LabelError},
		),
		
		// Resource metrics
		WatchedSecretsTotal: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: MetricPrefix + "watched_secrets_total",
				Help: "Total number of secrets being watched",
			},
		),
		
		ActiveConnectionsTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: MetricPrefix + "active_connections_total",
				Help: "Number of active connections by type",
			},
			[]string{LabelInstance, "connection_type"},
		),
		
		// Performance metrics
		MemoryUsageBytes: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: MetricPrefix + "memory_usage_bytes",
				Help: "Current memory usage in bytes",
			},
		),
		
		CPUUsagePercent: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: MetricPrefix + "cpu_usage_percent",
				Help: "Current CPU usage percentage",
			},
		),
		
		// Health metrics
		HealthChecksTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: MetricPrefix + "health_checks_total",
				Help: "Total number of health checks by component and status",
			},
			[]string{LabelInstance, "component", LabelStatus},
		),
		
		ComponentHealthStatus: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: MetricPrefix + "component_health_status",
				Help: "Health status of components (1=healthy, 0=unhealthy)",
			},
			[]string{LabelInstance, "component"},
		),
	}
	
	// Register with controller-runtime metrics
	metrics.Registry.MustRegister(
		m.SyncOperationsTotal,
		m.SyncOperationDuration,
		m.SyncOperationsActive,
		m.IssuesProcessedTotal,
		m.IssuesFailedTotal,
		m.IssuesSyncDuration,
		m.GitOperationsTotal,
		m.GitOperationDuration,
		m.GitCommitsTotal,
		m.GitFilesModifiedTotal,
		m.JiraAPIRequestsTotal,
		m.JiraAPIRequestDuration,
		m.JiraAPIRateLimitHits,
		m.JiraAPIErrors,
		m.AuthRefreshTotal,
		m.AuthFailuresTotal,
		m.WatchedSecretsTotal,
		m.ActiveConnectionsTotal,
		m.MemoryUsageBytes,
		m.CPUUsagePercent,
		m.HealthChecksTotal,
		m.ComponentHealthStatus,
	)
	
	// Set global instance
	Metrics = m
	
	return m
}

// TimerWrapper wraps a function call with duration measurement
type TimerWrapper struct {
	histogram *prometheus.HistogramVec
	labels    prometheus.Labels
	startTime time.Time
}

// NewTimer creates a new timer for measuring operation duration
func (m *JiraCDCMetrics) NewTimer(histogram *prometheus.HistogramVec, labels prometheus.Labels) *TimerWrapper {
	return &TimerWrapper{
		histogram: histogram,
		labels:    labels,
		startTime: time.Now(),
	}
}

// Finish records the duration and completes the timer
func (t *TimerWrapper) Finish() {
	duration := time.Since(t.startTime).Seconds()
	t.histogram.With(t.labels).Observe(duration)
}

// RecordSyncOperation records metrics for a sync operation
func (m *JiraCDCMetrics) RecordSyncOperation(instance, project, operation, status string, duration time.Duration) {
	labels := prometheus.Labels{
		LabelInstance:  instance,
		LabelProject:   project,
		LabelOperation: operation,
		LabelStatus:    status,
	}
	
	m.SyncOperationsTotal.With(labels).Inc()
	
	if duration > 0 {
		durationLabels := prometheus.Labels{
			LabelInstance:  instance,
			LabelProject:   project,
			LabelOperation: operation,
		}
		m.SyncOperationDuration.With(durationLabels).Observe(duration.Seconds())
	}
}

// RecordIssueProcessed records metrics for processed issues
func (m *JiraCDCMetrics) RecordIssueProcessed(instance, project, status string, duration time.Duration) {
	labels := prometheus.Labels{
		LabelInstance: instance,
		LabelProject:  project,
		LabelStatus:   status,
	}
	
	m.IssuesProcessedTotal.With(labels).Inc()
	
	if duration > 0 {
		durationLabels := prometheus.Labels{
			LabelInstance: instance,
			LabelProject:  project,
		}
		m.IssuesSyncDuration.With(durationLabels).Observe(duration.Seconds())
	}
}

// RecordIssueFailed records metrics for failed issue processing
func (m *JiraCDCMetrics) RecordIssueFailed(instance, project, errorType string) {
	labels := prometheus.Labels{
		LabelInstance: instance,
		LabelProject:  project,
		LabelError:    errorType,
	}
	
	m.IssuesFailedTotal.With(labels).Inc()
}

// RecordGitOperation records metrics for git operations
func (m *JiraCDCMetrics) RecordGitOperation(instance, project, operation, status string, duration time.Duration) {
	labels := prometheus.Labels{
		LabelInstance:  instance,
		LabelProject:   project,
		LabelOperation: operation,
		LabelStatus:    status,
	}
	
	m.GitOperationsTotal.With(labels).Inc()
	
	if duration > 0 {
		durationLabels := prometheus.Labels{
			LabelInstance:  instance,
			LabelProject:   project,
			LabelOperation: operation,
		}
		m.GitOperationDuration.With(durationLabels).Observe(duration.Seconds())
	}
}

// RecordGitCommit records a git commit
func (m *JiraCDCMetrics) RecordGitCommit(instance, project string) {
	labels := prometheus.Labels{
		LabelInstance: instance,
		LabelProject:  project,
	}
	
	m.GitCommitsTotal.With(labels).Inc()
}

// RecordGitFilesModified records git file modifications
func (m *JiraCDCMetrics) RecordGitFilesModified(instance, project, operation string, count int) {
	labels := prometheus.Labels{
		LabelInstance:  instance,
		LabelProject:   project,
		LabelOperation: operation,
	}
	
	m.GitFilesModifiedTotal.With(labels).Add(float64(count))
}

// RecordJiraAPIRequest records JIRA API request metrics
func (m *JiraCDCMetrics) RecordJiraAPIRequest(instance, project, endpoint, status string, duration time.Duration) {
	labels := prometheus.Labels{
		LabelInstance: instance,
		LabelProject:  project,
		"endpoint":    endpoint,
		LabelStatus:   status,
	}
	
	m.JiraAPIRequestsTotal.With(labels).Inc()
	
	if duration > 0 {
		durationLabels := prometheus.Labels{
			LabelInstance: instance,
			LabelProject:  project,
			"endpoint":    endpoint,
		}
		m.JiraAPIRequestDuration.With(durationLabels).Observe(duration.Seconds())
	}
}

// RecordJiraAPIRateLimit records JIRA API rate limit hits
func (m *JiraCDCMetrics) RecordJiraAPIRateLimit(instance, project string) {
	labels := prometheus.Labels{
		LabelInstance: instance,
		LabelProject:  project,
	}
	
	m.JiraAPIRateLimitHits.With(labels).Inc()
}

// RecordJiraAPIError records JIRA API errors
func (m *JiraCDCMetrics) RecordJiraAPIError(instance, project, errorType string) {
	labels := prometheus.Labels{
		LabelInstance: instance,
		LabelProject:  project,
		LabelError:    errorType,
	}
	
	m.JiraAPIErrors.With(labels).Inc()
}

// RecordAuthRefresh records authentication refresh events
func (m *JiraCDCMetrics) RecordAuthRefresh(instance, authType, status string) {
	labels := prometheus.Labels{
		LabelInstance: instance,
		LabelAuthType: authType,
		LabelStatus:   status,
	}
	
	m.AuthRefreshTotal.With(labels).Inc()
}

// RecordAuthFailure records authentication failures
func (m *JiraCDCMetrics) RecordAuthFailure(instance, authType, errorType string) {
	labels := prometheus.Labels{
		LabelInstance: instance,
		LabelAuthType: authType,
		LabelError:    errorType,
	}
	
	m.AuthFailuresTotal.With(labels).Inc()
}

// SetWatchedSecrets sets the number of watched secrets
func (m *JiraCDCMetrics) SetWatchedSecrets(count int) {
	m.WatchedSecretsTotal.Set(float64(count))
}

// SetActiveConnections sets the number of active connections
func (m *JiraCDCMetrics) SetActiveConnections(instance, connectionType string, count int) {
	labels := prometheus.Labels{
		LabelInstance:    instance,
		"connection_type": connectionType,
	}
	
	m.ActiveConnectionsTotal.With(labels).Set(float64(count))
}

// SetMemoryUsage sets current memory usage
func (m *JiraCDCMetrics) SetMemoryUsage(bytes int64) {
	m.MemoryUsageBytes.Set(float64(bytes))
}

// SetCPUUsage sets current CPU usage percentage
func (m *JiraCDCMetrics) SetCPUUsage(percent float64) {
	m.CPUUsagePercent.Set(percent)
}

// RecordHealthCheck records health check results
func (m *JiraCDCMetrics) RecordHealthCheck(instance, component, status string) {
	labels := prometheus.Labels{
		LabelInstance: instance,
		"component":   component,
		LabelStatus:   status,
	}
	
	m.HealthChecksTotal.With(labels).Inc()
}

// SetComponentHealth sets component health status
func (m *JiraCDCMetrics) SetComponentHealth(instance, component string, healthy bool) {
	labels := prometheus.Labels{
		LabelInstance: instance,
		"component":   component,
	}
	
	value := 0.0
	if healthy {
		value = 1.0
	}
	
	m.ComponentHealthStatus.With(labels).Set(value)
}

// SetSyncOperationActive sets the number of active sync operations
func (m *JiraCDCMetrics) SetSyncOperationActive(instance, project, operation string, count int) {
	labels := prometheus.Labels{
		LabelInstance:  instance,
		LabelProject:   project,
		LabelOperation: operation,
	}
	
	m.SyncOperationsActive.With(labels).Set(float64(count))
}
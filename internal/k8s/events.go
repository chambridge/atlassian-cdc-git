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

package k8s

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// EventType represents the type of Kubernetes event
type EventType string

const (
	EventTypeNormal  EventType = "Normal"
	EventTypeWarning EventType = "Warning"
)

// EventReason represents common event reasons for JIRA CDC operations
type EventReason string

const (
	// Sync operation events
	ReasonSyncStarted   EventReason = "SyncStarted"
	ReasonSyncCompleted EventReason = "SyncCompleted"
	ReasonSyncFailed    EventReason = "SyncFailed"
	ReasonSyncCancelled EventReason = "SyncCancelled"
	
	// Authentication events
	ReasonAuthSuccess EventReason = "AuthSuccess"
	ReasonAuthFailed  EventReason = "AuthFailed"
	ReasonAuthRefresh EventReason = "AuthRefresh"
	
	// Git operation events
	ReasonGitClone    EventReason = "GitClone"
	ReasonGitCommit   EventReason = "GitCommit"
	ReasonGitPush     EventReason = "GitPush"
	ReasonGitFailed   EventReason = "GitFailed"
	
	// JIRA operation events
	ReasonJiraConnected    EventReason = "JiraConnected"
	ReasonJiraDisconnected EventReason = "JiraDisconnected"
	ReasonJiraRateLimit    EventReason = "JiraRateLimit"
	ReasonJiraAPIError     EventReason = "JiraAPIError"
	
	// Issue processing events
	ReasonIssueProcessed EventReason = "IssueProcessed"
	ReasonIssueFailed    EventReason = "IssueFailed"
	ReasonIssueSkipped   EventReason = "IssueSkipped"
	
	// Configuration events
	ReasonConfigUpdated  EventReason = "ConfigUpdated"
	ReasonConfigInvalid  EventReason = "ConfigInvalid"
	ReasonSecretUpdated  EventReason = "SecretUpdated"
	ReasonSecretMissing  EventReason = "SecretMissing"
	
	// Health and status events
	ReasonHealthy    EventReason = "Healthy"
	ReasonUnhealthy  EventReason = "Unhealthy"
	ReasonDegraded   EventReason = "Degraded"
	ReasonRecovering EventReason = "Recovering"
	
	// Operator lifecycle events
	ReasonStarted   EventReason = "Started"
	ReasonStopping  EventReason = "Stopping"
	ReasonStopped   EventReason = "Stopped"
	ReasonRestarted EventReason = "Restarted"
)

// EventPublisher publishes Kubernetes events for operator status and operations
type EventPublisher interface {
	// PublishEvent publishes a Kubernetes event
	PublishEvent(ctx context.Context, obj client.Object, eventType EventType, reason EventReason, message string) error
	
	// PublishSyncEvent publishes a sync operation event
	PublishSyncEvent(ctx context.Context, obj client.Object, operation string, status string, message string) error
	
	// PublishAuthEvent publishes an authentication event
	PublishAuthEvent(ctx context.Context, obj client.Object, authType string, success bool, message string) error
	
	// PublishGitEvent publishes a git operation event
	PublishGitEvent(ctx context.Context, obj client.Object, operation string, success bool, message string) error
	
	// PublishJiraEvent publishes a JIRA operation event
	PublishJiraEvent(ctx context.Context, obj client.Object, operation string, success bool, message string) error
	
	// PublishIssueEvent publishes an issue processing event
	PublishIssueEvent(ctx context.Context, obj client.Object, issueKey string, status string, message string) error
	
	// PublishHealthEvent publishes a health status event
	PublishHealthEvent(ctx context.Context, obj client.Object, component string, healthy bool, message string) error
	
	// PublishConfigEvent publishes a configuration event
	PublishConfigEvent(ctx context.Context, obj client.Object, configType string, valid bool, message string) error
}

// eventPublisher implements EventPublisher
type eventPublisher struct {
	recorder record.EventRecorder
	client   client.Client
	scheme   *runtime.Scheme
}

// NewEventPublisher creates a new event publisher
func NewEventPublisher(recorder record.EventRecorder, client client.Client, scheme *runtime.Scheme) EventPublisher {
	return &eventPublisher{
		recorder: recorder,
		client:   client,
		scheme:   scheme,
	}
}

// PublishEvent publishes a Kubernetes event
func (p *eventPublisher) PublishEvent(ctx context.Context, obj client.Object, eventType EventType, reason EventReason, message string) error {
	logger := log.FromContext(ctx)
	
	if obj == nil {
		return fmt.Errorf("object cannot be nil")
	}
	
	// Publish event using the recorder
	p.recorder.Event(obj, string(eventType), string(reason), message)
	
	logger.V(1).Info("Published Kubernetes event", 
		"object", fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName()),
		"type", eventType,
		"reason", reason,
		"message", message)
	
	return nil
}

// PublishSyncEvent publishes a sync operation event
func (p *eventPublisher) PublishSyncEvent(ctx context.Context, obj client.Object, operation string, status string, message string) error {
	var eventType EventType
	var reason EventReason
	
	switch status {
	case "started":
		eventType = EventTypeNormal
		reason = ReasonSyncStarted
		if message == "" {
			message = fmt.Sprintf("Started %s operation", operation)
		}
	case "completed":
		eventType = EventTypeNormal
		reason = ReasonSyncCompleted
		if message == "" {
			message = fmt.Sprintf("Completed %s operation", operation)
		}
	case "failed":
		eventType = EventTypeWarning
		reason = ReasonSyncFailed
		if message == "" {
			message = fmt.Sprintf("Failed %s operation", operation)
		}
	case "cancelled":
		eventType = EventTypeWarning
		reason = ReasonSyncCancelled
		if message == "" {
			message = fmt.Sprintf("Cancelled %s operation", operation)
		}
	default:
		eventType = EventTypeNormal
		reason = EventReason(fmt.Sprintf("Sync%s", operation))
	}
	
	return p.PublishEvent(ctx, obj, eventType, reason, message)
}

// PublishAuthEvent publishes an authentication event
func (p *eventPublisher) PublishAuthEvent(ctx context.Context, obj client.Object, authType string, success bool, message string) error {
	var eventType EventType
	var reason EventReason
	
	if success {
		eventType = EventTypeNormal
		reason = ReasonAuthSuccess
		if message == "" {
			message = fmt.Sprintf("Successfully authenticated using %s", authType)
		}
	} else {
		eventType = EventTypeWarning
		reason = ReasonAuthFailed
		if message == "" {
			message = fmt.Sprintf("Authentication failed using %s", authType)
		}
	}
	
	return p.PublishEvent(ctx, obj, eventType, reason, message)
}

// PublishGitEvent publishes a git operation event
func (p *eventPublisher) PublishGitEvent(ctx context.Context, obj client.Object, operation string, success bool, message string) error {
	var eventType EventType
	var reason EventReason
	
	if success {
		eventType = EventTypeNormal
		switch operation {
		case "clone":
			reason = ReasonGitClone
		case "commit":
			reason = ReasonGitCommit
		case "push":
			reason = ReasonGitPush
		default:
			reason = EventReason(fmt.Sprintf("Git%s", operation))
		}
		
		if message == "" {
			message = fmt.Sprintf("Git %s operation successful", operation)
		}
	} else {
		eventType = EventTypeWarning
		reason = ReasonGitFailed
		if message == "" {
			message = fmt.Sprintf("Git %s operation failed", operation)
		}
	}
	
	return p.PublishEvent(ctx, obj, eventType, reason, message)
}

// PublishJiraEvent publishes a JIRA operation event
func (p *eventPublisher) PublishJiraEvent(ctx context.Context, obj client.Object, operation string, success bool, message string) error {
	var eventType EventType
	var reason EventReason
	
	if success {
		eventType = EventTypeNormal
		switch operation {
		case "connect":
			reason = ReasonJiraConnected
		default:
			reason = EventReason(fmt.Sprintf("Jira%s", operation))
		}
		
		if message == "" {
			message = fmt.Sprintf("JIRA %s operation successful", operation)
		}
	} else {
		eventType = EventTypeWarning
		switch operation {
		case "connect":
			reason = ReasonJiraDisconnected
		case "rate_limit":
			reason = ReasonJiraRateLimit
		default:
			reason = ReasonJiraAPIError
		}
		
		if message == "" {
			message = fmt.Sprintf("JIRA %s operation failed", operation)
		}
	}
	
	return p.PublishEvent(ctx, obj, eventType, reason, message)
}

// PublishIssueEvent publishes an issue processing event
func (p *eventPublisher) PublishIssueEvent(ctx context.Context, obj client.Object, issueKey string, status string, message string) error {
	var eventType EventType
	var reason EventReason
	
	switch status {
	case "processed":
		eventType = EventTypeNormal
		reason = ReasonIssueProcessed
		if message == "" {
			message = fmt.Sprintf("Successfully processed issue %s", issueKey)
		}
	case "failed":
		eventType = EventTypeWarning
		reason = ReasonIssueFailed
		if message == "" {
			message = fmt.Sprintf("Failed to process issue %s", issueKey)
		}
	case "skipped":
		eventType = EventTypeNormal
		reason = ReasonIssueSkipped
		if message == "" {
			message = fmt.Sprintf("Skipped processing issue %s", issueKey)
		}
	default:
		eventType = EventTypeNormal
		reason = EventReason(fmt.Sprintf("Issue%s", status))
	}
	
	return p.PublishEvent(ctx, obj, eventType, reason, message)
}

// PublishHealthEvent publishes a health status event
func (p *eventPublisher) PublishHealthEvent(ctx context.Context, obj client.Object, component string, healthy bool, message string) error {
	var eventType EventType
	var reason EventReason
	
	if healthy {
		eventType = EventTypeNormal
		reason = ReasonHealthy
		if message == "" {
			message = fmt.Sprintf("Component %s is healthy", component)
		}
	} else {
		eventType = EventTypeWarning
		reason = ReasonUnhealthy
		if message == "" {
			message = fmt.Sprintf("Component %s is unhealthy", component)
		}
	}
	
	return p.PublishEvent(ctx, obj, eventType, reason, message)
}

// PublishConfigEvent publishes a configuration event
func (p *eventPublisher) PublishConfigEvent(ctx context.Context, obj client.Object, configType string, valid bool, message string) error {
	var eventType EventType
	var reason EventReason
	
	if valid {
		eventType = EventTypeNormal
		reason = ReasonConfigUpdated
		if message == "" {
			message = fmt.Sprintf("Configuration %s updated successfully", configType)
		}
	} else {
		eventType = EventTypeWarning
		reason = ReasonConfigInvalid
		if message == "" {
			message = fmt.Sprintf("Configuration %s is invalid", configType)
		}
	}
	
	return p.PublishEvent(ctx, obj, eventType, reason, message)
}

// EventAggregator aggregates and deduplicates events to prevent spam
type EventAggregator struct {
	publisher    EventPublisher
	events       map[string]*aggregatedEvent
	mutex        sync.RWMutex
	maxAge       time.Duration
	cleanupTimer *time.Timer
}

// aggregatedEvent represents an aggregated event
type aggregatedEvent struct {
	Object      client.Object
	EventType   EventType
	Reason      EventReason
	Message     string
	Count       int
	FirstTime   time.Time
	LastTime    time.Time
}

// NewEventAggregator creates a new event aggregator
func NewEventAggregator(publisher EventPublisher, maxAge time.Duration) *EventAggregator {
	aggregator := &EventAggregator{
		publisher: publisher,
		events:    make(map[string]*aggregatedEvent),
		maxAge:    maxAge,
	}
	
	// Start cleanup timer
	aggregator.startCleanup()
	
	return aggregator
}

// PublishEvent publishes an event, aggregating duplicates
func (a *EventAggregator) PublishEvent(ctx context.Context, obj client.Object, eventType EventType, reason EventReason, message string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	// Create event key for deduplication
	key := fmt.Sprintf("%s/%s/%s/%s/%s", 
		obj.GetNamespace(), 
		obj.GetName(), 
		eventType, 
		reason, 
		message)
	
	now := time.Now()
	
	if existing, exists := a.events[key]; exists {
		// Update existing event
		existing.Count++
		existing.LastTime = now
		
		// Update message with count if multiple occurrences
		if existing.Count > 1 {
			aggregatedMessage := fmt.Sprintf("%s (occurred %d times)", message, existing.Count)
			existing.Message = aggregatedMessage
		}
		
		// Publish aggregated event
		return a.publisher.PublishEvent(ctx, obj, eventType, reason, existing.Message)
	} else {
		// Create new aggregated event
		a.events[key] = &aggregatedEvent{
			Object:    obj,
			EventType: eventType,
			Reason:    reason,
			Message:   message,
			Count:     1,
			FirstTime: now,
			LastTime:  now,
		}
		
		// Publish event
		return a.publisher.PublishEvent(ctx, obj, eventType, reason, message)
	}
}

// startCleanup starts the cleanup timer for old events
func (a *EventAggregator) startCleanup() {
	a.cleanupTimer = time.AfterFunc(a.maxAge/2, func() {
		a.cleanup()
		a.startCleanup() // Reschedule
	})
}

// cleanup removes old events from the aggregator
func (a *EventAggregator) cleanup() {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	
	cutoff := time.Now().Add(-a.maxAge)
	
	for key, event := range a.events {
		if event.LastTime.Before(cutoff) {
			delete(a.events, key)
		}
	}
}

// Stop stops the event aggregator
func (a *EventAggregator) Stop() {
	if a.cleanupTimer != nil {
		a.cleanupTimer.Stop()
	}
}

// EventMetrics tracks event publishing metrics
type EventMetrics struct {
	EventsPublished   map[EventReason]int64
	EventsByType      map[EventType]int64
	LastEventTime     time.Time
	mutex             sync.RWMutex
}

// NewEventMetrics creates new event metrics
func NewEventMetrics() *EventMetrics {
	return &EventMetrics{
		EventsPublished: make(map[EventReason]int64),
		EventsByType:    make(map[EventType]int64),
	}
}

// RecordEvent records an event in metrics
func (m *EventMetrics) RecordEvent(eventType EventType, reason EventReason) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.EventsPublished[reason]++
	m.EventsByType[eventType]++
	m.LastEventTime = time.Now()
}

// GetStats returns event metrics statistics
func (m *EventMetrics) GetStats() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	stats := make(map[string]interface{})
	stats["events_by_reason"] = m.EventsPublished
	stats["events_by_type"] = m.EventsByType
	stats["last_event_time"] = m.LastEventTime
	
	return stats
}

// metricsEventPublisher wraps an EventPublisher with metrics collection
type metricsEventPublisher struct {
	EventPublisher
	metrics *EventMetrics
}

// NewMetricsEventPublisher creates an event publisher with metrics
func NewMetricsEventPublisher(publisher EventPublisher) EventPublisher {
	return &metricsEventPublisher{
		EventPublisher: publisher,
		metrics:        NewEventMetrics(),
	}
}

// PublishEvent publishes an event and records metrics
func (p *metricsEventPublisher) PublishEvent(ctx context.Context, obj client.Object, eventType EventType, reason EventReason, message string) error {
	err := p.EventPublisher.PublishEvent(ctx, obj, eventType, reason, message)
	if err == nil {
		p.metrics.RecordEvent(eventType, reason)
	}
	return err
}

// GetMetrics returns the event metrics
func (p *metricsEventPublisher) GetMetrics() *EventMetrics {
	return p.metrics
}
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

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// LogLevel represents logging levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogFormat represents log output formats
type LogFormat string

const (
	LogFormatJSON    LogFormat = "json"
	LogFormatConsole LogFormat = "console"
)

// LogConfig contains logger configuration
type LogConfig struct {
	Level      LogLevel  `json:"level"`
	Format     LogFormat `json:"format"`
	TimeFormat string    `json:"timeFormat"`
	CallerInfo bool      `json:"callerInfo"`
}

// ContextKey represents context keys for logging
type ContextKey string

const (
	// Standard context keys
	ContextKeyRequestID     ContextKey = "request_id"
	ContextKeyCorrelationID ContextKey = "correlation_id"
	ContextKeyUserID        ContextKey = "user_id"
	ContextKeyOperation     ContextKey = "operation"
	ContextKeyComponent     ContextKey = "component"
	ContextKeyInstance      ContextKey = "instance"
	ContextKeyProject       ContextKey = "project"
	ContextKeyIssueKey      ContextKey = "issue_key"
	ContextKeyTaskID        ContextKey = "task_id"
	ContextKeyOperationID   ContextKey = "operation_id"
)

// LoggerManager manages structured logging for JIRA CDC
type LoggerManager struct {
	config     LogConfig
	baseLogger logr.Logger
	zapLogger  *zap.Logger
}

// NewLoggerManager creates a new logger manager
func NewLoggerManager(config LogConfig) (*LoggerManager, error) {
	zapLogger, err := createZapLogger(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create zap logger: %w", err)
	}
	
	// Create logr logger from zap
	logrLogger := zapr.NewLogger(zapLogger)
	
	// Set as global controller-runtime logger
	log.SetLogger(logrLogger)
	
	return &LoggerManager{
		config:     config,
		baseLogger: logrLogger,
		zapLogger:  zapLogger,
	}, nil
}

// GetLogger returns a logger for the given context
func (m *LoggerManager) GetLogger(ctx context.Context) logr.Logger {
	logger := m.baseLogger
	
	// Add context values as key-value pairs
	for _, key := range []ContextKey{
		ContextKeyRequestID,
		ContextKeyCorrelationID,
		ContextKeyUserID,
		ContextKeyOperation,
		ContextKeyComponent,
		ContextKeyInstance,
		ContextKeyProject,
		ContextKeyIssueKey,
		ContextKeyTaskID,
		ContextKeyOperationID,
	} {
		if value := ctx.Value(key); value != nil {
			logger = logger.WithValues(string(key), value)
		}
	}
	
	return logger
}

// GetZapLogger returns the underlying zap logger
func (m *LoggerManager) GetZapLogger() *zap.Logger {
	return m.zapLogger
}

// Sync flushes any buffered log entries
func (m *LoggerManager) Sync() error {
	return m.zapLogger.Sync()
}

// createZapLogger creates a configured zap logger
func createZapLogger(config LogConfig) (*zap.Logger, error) {
	// Configure log level
	var level zapcore.Level
	switch config.Level {
	case LogLevelDebug:
		level = zapcore.DebugLevel
	case LogLevelInfo:
		level = zapcore.InfoLevel
	case LogLevelWarn:
		level = zapcore.WarnLevel
	case LogLevelError:
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}
	
	// Configure encoder
	var encoder zapcore.Encoder
	encoderConfig := createEncoderConfig(config)
	
	switch config.Format {
	case LogFormatJSON:
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	case LogFormatConsole:
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	default:
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}
	
	// Configure output
	writeSyncer := zapcore.AddSync(os.Stdout)
	
	// Create core
	core := zapcore.NewCore(encoder, writeSyncer, level)
	
	// Create logger with caller info if enabled
	var options []zap.Option
	if config.CallerInfo {
		options = append(options, zap.AddCaller(), zap.AddCallerSkip(1))
	}
	
	return zap.New(core, options...), nil
}

// createEncoderConfig creates encoder configuration
func createEncoderConfig(config LogConfig) zapcore.EncoderConfig {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
	
	// Configure time format
	timeFormat := config.TimeFormat
	if timeFormat == "" {
		timeFormat = time.RFC3339
	}
	
	switch timeFormat {
	case "iso8601":
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	case "epoch":
		encoderConfig.EncodeTime = zapcore.EpochTimeEncoder
	case "epochmillis":
		encoderConfig.EncodeTime = zapcore.EpochMillisTimeEncoder
	case "epochnanos":
		encoderConfig.EncodeTime = zapcore.EpochNanosTimeEncoder
	default:
		encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	}
	
	return encoderConfig
}

// DefaultLogConfig returns a default logging configuration
func DefaultLogConfig() LogConfig {
	return LogConfig{
		Level:      LogLevelInfo,
		Format:     LogFormatJSON,
		TimeFormat: "iso8601",
		CallerInfo: true,
	}
}

// Context enhancement functions

// WithRequestID adds a request ID to the context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ContextKeyRequestID, requestID)
}

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, ContextKeyCorrelationID, correlationID)
}

// WithUserID adds a user ID to the context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ContextKeyUserID, userID)
}

// WithOperation adds an operation name to the context
func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, ContextKeyOperation, operation)
}

// WithComponent adds a component name to the context
func WithComponent(ctx context.Context, component string) context.Context {
	return context.WithValue(ctx, ContextKeyComponent, component)
}

// WithInstance adds an instance ID to the context
func WithInstance(ctx context.Context, instance string) context.Context {
	return context.WithValue(ctx, ContextKeyInstance, instance)
}

// WithProject adds a project key to the context
func WithProject(ctx context.Context, project string) context.Context {
	return context.WithValue(ctx, ContextKeyProject, project)
}

// WithIssueKey adds an issue key to the context
func WithIssueKey(ctx context.Context, issueKey string) context.Context {
	return context.WithValue(ctx, ContextKeyIssueKey, issueKey)
}

// WithTaskID adds a task ID to the context
func WithTaskID(ctx context.Context, taskID string) context.Context {
	return context.WithValue(ctx, ContextKeyTaskID, taskID)
}

// WithOperationID adds an operation ID to the context
func WithOperationID(ctx context.Context, operationID string) context.Context {
	return context.WithValue(ctx, ContextKeyOperationID, operationID)
}

// StructuredEvent represents a structured log event
type StructuredEvent struct {
	EventType   string                 `json:"event_type"`
	Timestamp   time.Time             `json:"timestamp"`
	Message     string                `json:"message"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Error       *EventError           `json:"error,omitempty"`
	Performance *PerformanceData      `json:"performance,omitempty"`
}

// EventError represents error information in structured events
type EventError struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	Code       string `json:"code,omitempty"`
	Stacktrace string `json:"stacktrace,omitempty"`
}

// PerformanceData represents performance information
type PerformanceData struct {
	Duration     time.Duration          `json:"duration"`
	MemoryUsage  int64                 `json:"memory_usage,omitempty"`
	CPUUsage     float64               `json:"cpu_usage,omitempty"`
	Metrics      map[string]interface{} `json:"metrics,omitempty"`
}

// EventLogger provides structured event logging
type EventLogger struct {
	logger logr.Logger
}

// NewEventLogger creates a new event logger
func NewEventLogger(logger logr.Logger) *EventLogger {
	return &EventLogger{
		logger: logger,
	}
}

// LogEvent logs a structured event
func (e *EventLogger) LogEvent(ctx context.Context, event StructuredEvent) {
	// Serialize event to JSON for structured logging
	eventJSON, err := json.Marshal(event)
	if err != nil {
		e.logger.Error(err, "Failed to marshal structured event")
		return
	}
	
	// Log at appropriate level based on event content
	if event.Error != nil {
		e.logger.Error(fmt.Errorf("%s", event.Error.Message), event.Message, "event", string(eventJSON))
	} else {
		e.logger.Info(event.Message, "event", string(eventJSON))
	}
}

// LogSyncEvent logs a sync operation event
func (e *EventLogger) LogSyncEvent(ctx context.Context, eventType, message string, data map[string]interface{}) {
	event := StructuredEvent{
		EventType: eventType,
		Timestamp: time.Now(),
		Message:   message,
		Context:   extractContextFromCtx(ctx),
		Data:      data,
	}
	
	e.LogEvent(ctx, event)
}

// LogErrorEvent logs an error event
func (e *EventLogger) LogErrorEvent(ctx context.Context, err error, message string, data map[string]interface{}) {
	// Get stack trace
	var stacktrace string
	if pc, file, line, ok := runtime.Caller(1); ok {
		fn := runtime.FuncForPC(pc)
		stacktrace = fmt.Sprintf("%s:%d %s", file, line, fn.Name())
	}
	
	event := StructuredEvent{
		EventType: "error",
		Timestamp: time.Now(),
		Message:   message,
		Context:   extractContextFromCtx(ctx),
		Data:      data,
		Error: &EventError{
			Type:       fmt.Sprintf("%T", err),
			Message:    err.Error(),
			Stacktrace: stacktrace,
		},
	}
	
	e.LogEvent(ctx, event)
}

// LogPerformanceEvent logs a performance event
func (e *EventLogger) LogPerformanceEvent(ctx context.Context, operation string, duration time.Duration, metrics map[string]interface{}) {
	// Get memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	event := StructuredEvent{
		EventType: "performance",
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Performance metrics for %s", operation),
		Context:   extractContextFromCtx(ctx),
		Performance: &PerformanceData{
			Duration:    duration,
			MemoryUsage: int64(m.Alloc),
			Metrics:     metrics,
		},
	}
	
	e.LogEvent(ctx, event)
}

// extractContextFromCtx extracts logging context from Go context
func extractContextFromCtx(ctx context.Context) map[string]interface{} {
	context := make(map[string]interface{})
	
	contextKeys := []ContextKey{
		ContextKeyRequestID,
		ContextKeyCorrelationID,
		ContextKeyUserID,
		ContextKeyOperation,
		ContextKeyComponent,
		ContextKeyInstance,
		ContextKeyProject,
		ContextKeyIssueKey,
		ContextKeyTaskID,
		ContextKeyOperationID,
	}
	
	for _, key := range contextKeys {
		if value := ctx.Value(key); value != nil {
			context[string(key)] = value
		}
	}
	
	return context
}

// LogLevelFromString converts string to LogLevel
func LogLevelFromString(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

// LogFormatFromString converts string to LogFormat
func LogFormatFromString(format string) LogFormat {
	switch strings.ToLower(format) {
	case "json":
		return LogFormatJSON
	case "console":
		return LogFormatConsole
	default:
		return LogFormatJSON
	}
}

// GetLoggerFromContext is a convenience function to get logger from context
func GetLoggerFromContext(ctx context.Context) logr.Logger {
	return log.FromContext(ctx)
}
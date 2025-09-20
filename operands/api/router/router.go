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

package router

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/company/jira-cdc-operator/internal/common"
	"github.com/company/jira-cdc-operator/operands/api/handlers"
	"github.com/company/jira-cdc-operator/internal/jira"
	"github.com/company/jira-cdc-operator/internal/git"
	"github.com/company/jira-cdc-operator/internal/sync"
)

// RouterConfig holds configuration for the API router
type RouterConfig struct {
	Debug                bool
	TrustedProxies       []string
	RequestTimeout       time.Duration
	MaxRequestBodySize   int64
	EnableCORS           bool
	EnableMetrics        bool
	EnableRequestLogging bool
}

// Dependencies holds all the dependencies needed by the router
type Dependencies struct {
	K8sClient       client.Client
	JiraClient      *jira.Client
	GitManager      *git.Manager
	TaskManager     sync.TaskManager
	ProgressTracker sync.ProgressTracker
}

// SetupRouter creates and configures the Gin router with all endpoints and middleware
func SetupRouter(config RouterConfig, deps Dependencies) *gin.Engine {
	// Set Gin mode
	if config.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin engine
	r := gin.New()

	// Set trusted proxies
	if len(config.TrustedProxies) > 0 {
		r.SetTrustedProxies(config.TrustedProxies)
	}

	// Add global middleware
	r.Use(gin.Recovery())
	
	if config.EnableRequestLogging {
		r.Use(LoggingMiddleware())
	}
	
	r.Use(SecurityMiddleware())
	r.Use(TimeoutMiddleware(config.RequestTimeout))
	
	if config.EnableCORS {
		r.Use(CORSMiddleware())
	}

	// Create handlers
	projectHandler := handlers.NewProjectHandler(deps.K8sClient, deps.TaskManager)
	taskHandler := handlers.NewTaskHandler(deps.TaskManager, deps.ProgressTracker)
	issueHandler := handlers.NewIssueHandler(deps.K8sClient, deps.JiraClient, deps.GitManager)
	healthHandler := handlers.NewHealthHandler(deps.K8sClient, deps.JiraClient, deps.GitManager, deps.TaskManager)
	metricsHandler := handlers.NewMetricsHandler(deps.K8sClient, deps.TaskManager)

	// Add request size limit middleware
	if config.MaxRequestBodySize > 0 {
		r.Use(RequestSizeLimitMiddleware(config.MaxRequestBodySize))
	}

	// Health check endpoints (no auth required)
	r.GET("/health", healthHandler.GetHealth)
	r.GET("/ready", healthHandler.GetReady)
	r.GET("/live", healthHandler.GetLive)

	// Prometheus metrics endpoint (no auth required)
	if config.EnableMetrics {
		r.GET("/metrics", metricsHandler.GetPrometheusMetrics())
	}

	// API routes with auth middleware
	api := r.Group("/api/v1")
	api.Use(AuthMiddleware()) // Add authentication middleware for API routes
	{
		// Project endpoints
		projects := api.Group("/projects")
		{
			projects.GET("", projectHandler.GetProjects)
			projects.GET("/:key", projectHandler.GetProject)
			projects.POST("/:key/sync", projectHandler.PostProjectSync)
			projects.GET("/:key/health", projectHandler.GetProjectHealth)
			projects.GET("/:key/metrics", projectHandler.GetProjectMetrics)
		}

		// Task endpoints
		tasks := api.Group("/tasks")
		{
			tasks.GET("", taskHandler.GetTasks)
			tasks.GET("/:id", taskHandler.GetTask)
			tasks.POST("/:id/cancel", taskHandler.PostTaskCancel)
			tasks.POST("/:id/retry", taskHandler.PostTaskRetry)
			tasks.DELETE("/:id", taskHandler.DeleteTask)
			tasks.GET("/:id/progress", taskHandler.GetTaskProgress)
			tasks.GET("/:id/logs", taskHandler.GetTaskLogs)
		}

		// Issue endpoints
		issues := api.Group("/issues")
		{
			issues.GET("", issueHandler.GetIssues)
			issues.GET("/:key", issueHandler.GetIssue)
			issues.POST("/:key/sync", issueHandler.PostIssueSync)
			issues.GET("/:key/history", issueHandler.GetIssueHistory)
			issues.GET("/:key/comments", issueHandler.GetIssueComments)
		}

		// System metrics endpoints
		api.GET("/metrics", metricsHandler.GetMetrics)
	}

	// Add rate limiting middleware to API routes
	api.Use(RateLimitMiddleware())

	// Add metrics collection middleware if enabled
	if config.EnableMetrics {
		api.Use(MetricsMiddleware(metricsHandler))
	}

	// Catch-all for undefined routes
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Not Found",
			"message": fmt.Sprintf("The requested endpoint %s was not found", c.Request.URL.Path),
		})
	})

	// Handle method not allowed
	r.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error":   "Method Not Allowed",
			"message": fmt.Sprintf("Method %s is not allowed for endpoint %s", c.Request.Method, c.Request.URL.Path),
		})
	})

	return r
}

// LoggingMiddleware provides request logging
func LoggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s %s\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
			param.ClientIP,
		)
	})
}

// SecurityMiddleware adds security headers
func SecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Security headers
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		
		// Remove server information
		c.Header("Server", "")
		
		c.Next()
	}
}

// CORSMiddleware handles Cross-Origin Resource Sharing
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Allow specific origins or configure as needed
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400") // 24 hours

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// TimeoutMiddleware adds request timeout
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if timeout > 0 {
			// Set timeout for the request context
			ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
			defer cancel()
			
			c.Request = c.Request.WithContext(ctx)
		}
		
		c.Next()
	}
}

// RequestSizeLimitMiddleware limits request body size
func RequestSizeLimitMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "Request Too Large",
				"message": fmt.Sprintf("Request body size exceeds limit of %d bytes", maxSize),
			})
			c.Abort()
			return
		}
		
		// Limit the reader
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		
		c.Next()
	}
}

// AuthMiddleware handles authentication (placeholder)
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// For now, this is a placeholder
		// In a production environment, this would validate JWT tokens,
		// API keys, or other authentication mechanisms
		
		// Check for Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// For MVP, we'll allow requests without auth
			// but set a default user context
			c.Set("user", "anonymous")
		} else {
			// Parse and validate token (placeholder)
			c.Set("user", "authenticated")
		}
		
		c.Next()
	}
}

// RateLimitMiddleware implements rate limiting (placeholder)
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// For now, this is a placeholder
		// In a production environment, this would implement actual rate limiting
		// based on IP, user, or API key
		
		c.Next()
	}
}

// MetricsMiddleware collects request metrics
func MetricsMiddleware(metricsHandler *handlers.MetricsHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		// Process request
		c.Next()
		
		// Record metrics
		duration := time.Since(start)
		statusCode := strconv.Itoa(c.Writer.Status())
		
		metricsHandler.RecordAPIRequest(
			c.Request.Method,
			c.FullPath(),
			statusCode,
			duration,
		)
	}
}

// ErrorHandlerMiddleware handles errors in a consistent way
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		
		// Handle any errors that occurred during request processing
		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			
			// Log the error
			fmt.Printf("Request error: %v\n", err.Error())
			
			// Return appropriate error response
			switch err.Type {
			case gin.ErrorTypeBind:
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid Request",
					"message": "Request body could not be parsed",
				})
			case gin.ErrorTypePublic:
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Internal Server Error",
					"message": "An internal error occurred",
				})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Internal Server Error",
					"message": "An unexpected error occurred",
				})
			}
		}
	}
}

// HealthCheckRoutes adds health check routes to an existing router
func HealthCheckRoutes(r *gin.Engine, healthHandler *handlers.HealthHandler) {
	r.GET("/health", healthHandler.GetHealth)
	r.GET("/ready", healthHandler.GetReady)
	r.GET("/live", healthHandler.GetLive)
}

// MetricsRoutes adds metrics routes to an existing router
func MetricsRoutes(r *gin.Engine, metricsHandler *handlers.MetricsHandler) {
	r.GET("/metrics", metricsHandler.GetPrometheusMetrics())
}

// ValidateRouterConfig validates router configuration
func ValidateRouterConfig(config RouterConfig) error {
	validator := common.NewConfigValidator()
	
	// Request timeout validation (allowing 0 for no timeout)
	if config.RequestTimeout < 0 {
		validator.AddValidator(func() error {
			return fmt.Errorf("request timeout cannot be negative")
		})
	}
	
	// Max request body size validation (allowing 0 for no limit)
	if config.MaxRequestBodySize < 0 {
		validator.AddValidator(func() error {
			return fmt.Errorf("max request body size cannot be negative")
		})
	}
	
	return validator.Validate()
}
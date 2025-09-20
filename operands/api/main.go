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

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
	"github.com/company/jira-cdc-operator/operands/api/router"
	"github.com/company/jira-cdc-operator/internal/jira"
	"github.com/company/jira-cdc-operator/internal/git"
	"github.com/company/jira-cdc-operator/internal/sync"
)

const (
	defaultPort           = "8080"
	defaultMetricsPort    = "9090"
	defaultRequestTimeout = 30 * time.Second
	defaultMaxBodySize    = 10 * 1024 * 1024 // 10MB
	defaultShutdownTimeout = 15 * time.Second
)

// Config holds the configuration for the API server
type Config struct {
	Port              string
	MetricsPort       string
	Debug             bool
	RequestTimeout    time.Duration
	MaxRequestBodySize int64
	EnableCORS        bool
	EnableMetrics     bool
	TrustedProxies    []string
	
	// Instance configuration
	InstanceName      string
	InstanceNamespace string
	
	// External service configuration
	JiraURL               string
	JiraCredentialsPath   string
	GitURL                string
	GitCredentialsPath    string
	GitWorkspaceDir       string
}

func main() {
	// Initialize logging
	log.SetLogger(zap.New(zap.UseDevMode(true)))
	logger := log.Log.WithName("api-server")

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		logger.Error(err, "Failed to load configuration")
		os.Exit(1)
	}

	logger.Info("Starting JIRA CDC API Server",
		"port", config.Port,
		"metricsPort", config.MetricsPort,
		"instance", config.InstanceName,
		"namespace", config.InstanceNamespace,
	)

	// Set up Kubernetes client
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(jiradcdv1.AddToScheme(scheme))

	k8sConfig, err := config.GetConfig()
	if err != nil {
		logger.Error(err, "Failed to get Kubernetes config")
		os.Exit(1)
	}

	k8sClient, err := client.New(k8sConfig, client.Options{Scheme: scheme})
	if err != nil {
		logger.Error(err, "Failed to create Kubernetes client")
		os.Exit(1)
	}

	// Initialize external services
	ctx := context.Background()
	
	// Initialize JIRA client
	jiraClient, err := initializeJiraClient(ctx, config, k8sClient)
	if err != nil {
		logger.Error(err, "Failed to initialize JIRA client")
		os.Exit(1)
	}

	// Initialize Git manager
	gitManager, err := initializeGitManager(ctx, config, k8sClient)
	if err != nil {
		logger.Error(err, "Failed to initialize Git manager")
		os.Exit(1)
	}

	// Initialize sync components
	taskManager := sync.NewTaskManager()
	progressTracker := sync.NewProgressTracker()

	// Set up router dependencies
	deps := router.Dependencies{
		K8sClient:       k8sClient,
		JiraClient:      jiraClient,
		GitManager:      gitManager,
		TaskManager:     taskManager,
		ProgressTracker: progressTracker,
	}

	// Create router configuration
	routerConfig := router.RouterConfig{
		Debug:                config.Debug,
		TrustedProxies:       config.TrustedProxies,
		RequestTimeout:       config.RequestTimeout,
		MaxRequestBodySize:   config.MaxRequestBodySize,
		EnableCORS:           config.EnableCORS,
		EnableMetrics:        config.EnableMetrics,
		EnableRequestLogging: true,
	}

	// Validate router configuration
	if err := router.ValidateRouterConfig(routerConfig); err != nil {
		logger.Error(err, "Invalid router configuration")
		os.Exit(1)
	}

	// Set up router
	r := router.SetupRouter(routerConfig, deps)

	// Create HTTP server
	srv := &http.Server{
		Addr:           ":" + config.Port,
		Handler:        r,
		ReadTimeout:    config.RequestTimeout,
		WriteTimeout:   config.RequestTimeout + 10*time.Second,
		IdleTimeout:    2 * time.Minute,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(err, "Failed to start HTTP server")
			os.Exit(1)
		}
	}()

	// Start metrics server if enabled
	var metricsSrv *http.Server
	if config.EnableMetrics && config.MetricsPort != config.Port {
		metricsSrv = &http.Server{
			Addr:    ":" + config.MetricsPort,
			Handler: createMetricsHandler(deps),
		}

		go func() {
			logger.Info("Starting metrics server", "addr", metricsSrv.Addr)
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error(err, "Failed to start metrics server")
			}
		}()
	}

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	// Create shutdown context
	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	// Shutdown main server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error(err, "Failed to shutdown HTTP server gracefully")
	}

	// Shutdown metrics server if running
	if metricsSrv != nil {
		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error(err, "Failed to shutdown metrics server gracefully")
		}
	}

	// Cleanup resources
	if gitManager != nil {
		if err := gitManager.Cleanup(shutdownCtx); err != nil {
			logger.Error(err, "Failed to cleanup git manager")
		}
	}

	logger.Info("Server shutdown complete")
}

// loadConfig loads configuration from environment variables
func loadConfig() (*Config, error) {
	config := &Config{
		Port:              getEnvOrDefault("JIRACDC_API_PORT", defaultPort),
		MetricsPort:       getEnvOrDefault("JIRACDC_METRICS_PORT", defaultMetricsPort),
		Debug:             getEnvBool("JIRACDC_DEBUG", false),
		RequestTimeout:    getEnvDuration("JIRACDC_REQUEST_TIMEOUT", defaultRequestTimeout),
		MaxRequestBodySize: getEnvInt64("JIRACDC_MAX_BODY_SIZE", defaultMaxBodySize),
		EnableCORS:        getEnvBool("JIRACDC_ENABLE_CORS", true),
		EnableMetrics:     getEnvBool("JIRACDC_ENABLE_METRICS", true),
		
		InstanceName:      getEnvOrDefault("JIRACDC_INSTANCE_NAME", ""),
		InstanceNamespace: getEnvOrDefault("JIRACDC_INSTANCE_NAMESPACE", "default"),
		
		JiraURL:               getEnvOrDefault("JIRA_URL", ""),
		JiraCredentialsPath:   getEnvOrDefault("JIRA_CREDENTIALS_PATH", "/etc/jira-credentials"),
		GitURL:                getEnvOrDefault("GIT_REPOSITORY_URL", ""),
		GitCredentialsPath:    getEnvOrDefault("GIT_CREDENTIALS_PATH", "/etc/git-credentials"),
		GitWorkspaceDir:       getEnvOrDefault("WORKSPACE_DIR", "/workspace"),
	}

	// Validate required configuration
	if config.InstanceName == "" {
		return nil, fmt.Errorf("JIRACDC_INSTANCE_NAME is required")
	}

	if config.JiraURL == "" {
		return nil, fmt.Errorf("JIRA_URL is required")
	}

	if config.GitURL == "" {
		return nil, fmt.Errorf("GIT_REPOSITORY_URL is required")
	}

	return config, nil
}

// initializeJiraClient creates and configures the JIRA client
func initializeJiraClient(ctx context.Context, config *Config, k8sClient client.Client) (*jira.Client, error) {
	jiraConfig := jira.Config{
		BaseURL:           config.JiraURL,
		CredentialsSecret: "jira-credentials", // This would come from JiraCDC spec
		Namespace:         config.InstanceNamespace,
		MaxRetries:        3,
		RequestTimeout:    30 * time.Second,
	}

	return jira.NewClient(ctx, k8sClient, jiraConfig)
}

// initializeGitManager creates and configures the Git manager
func initializeGitManager(ctx context.Context, config *Config, k8sClient client.Client) (*git.Manager, error) {
	gitConfig := git.Config{
		RepositoryURL:     config.GitURL,
		Branch:            "main", // This would come from JiraCDC spec
		CredentialsSecret: "git-credentials", // This would come from JiraCDC spec
		Namespace:         config.InstanceNamespace,
		WorkingDirectory:  config.GitWorkspaceDir,
	}

	return git.NewManager(ctx, k8sClient, gitConfig)
}

// createMetricsHandler creates a simple metrics-only HTTP handler
func createMetricsHandler(deps router.Dependencies) http.Handler {
	// Create a simple metrics-only router
	r := http.NewServeMux()
	
	// Add health check endpoint
	r.HandleFunc("/health", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	return r
}

// Environment variable helper functions
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
package performance

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"jiracdc-operator/internal/jira"
	"jiracdc-operator/internal/git"
	"jiracdc-operator/internal/sync"
)

const (
	// Performance targets
	maxBootstrapTime1000Issues = 30 * time.Minute
	maxSyncTime100Issues       = 5 * time.Minute
	maxMemoryUsage             = 512 * 1024 * 1024 // 512MB
	maxCPUUsage                = 2.0               // 2 CPU cores
)

// TestBootstrapPerformance tests bootstrap performance with 1000 issues
func TestBootstrapPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Skip if performance test environment variables are not set
	if os.Getenv("PERF_TEST_JIRA_URL") == "" {
		t.Skip("Skipping performance test: PERF_TEST_JIRA_URL not set")
	}

	ctx := context.Background()
	
	// Setup test environment
	jiraClient, gitOps, engine := setupPerformanceTest(t)
	
	// Create test repository
	repo := setupTestRepository(t)
	defer cleanupTestRepository(repo)

	// Measure bootstrap performance
	startTime := time.Now()
	startMemory := getCurrentMemoryUsage()
	
	// Run bootstrap with progress monitoring
	progress := make(chan sync.SyncProgress, 100)
	go monitorProgress(t, progress)
	
	results, err := engine.Bootstrap(ctx, repo, progress)
	close(progress)
	
	endTime := time.Now()
	endMemory := getCurrentMemoryUsage()
	
	// Verify bootstrap succeeded
	require.NoError(t, err)
	require.NotEmpty(t, results)
	
	// Performance assertions
	bootstrapDuration := endTime.Sub(startTime)
	memoryDelta := endMemory - startMemory
	
	t.Logf("Bootstrap Performance Results:")
	t.Logf("  Issues processed: %d", len(results))
	t.Logf("  Time taken: %v", bootstrapDuration)
	t.Logf("  Memory used: %d bytes (%.2f MB)", memoryDelta, float64(memoryDelta)/(1024*1024))
	t.Logf("  Throughput: %.2f issues/minute", float64(len(results))/(bootstrapDuration.Minutes()))
	
	// Verify performance targets
	if len(results) >= 1000 {
		assert.LessOrEqual(t, bootstrapDuration, maxBootstrapTime1000Issues,
			"Bootstrap of 1000+ issues should complete within %v", maxBootstrapTime1000Issues)
	}
	
	assert.LessOrEqual(t, memoryDelta, int64(maxMemoryUsage),
		"Memory usage should not exceed %d bytes", maxMemoryUsage)
	
	// Verify all operations succeeded
	for _, result := range results {
		assert.Equal(t, sync.OperationStatusCompleted, result.Status,
			"All operations should complete successfully")
	}
}

// TestConcurrentSyncPerformance tests performance with multiple concurrent syncs
func TestConcurrentSyncPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	if os.Getenv("PERF_TEST_JIRA_URL") == "" {
		t.Skip("Skipping performance test: PERF_TEST_JIRA_URL not set")
	}

	ctx := context.Background()
	
	// Setup multiple test projects
	numProjects := 3
	projects := make([]string, numProjects)
	for i := 0; i < numProjects; i++ {
		projects[i] = fmt.Sprintf("PERF%d", i+1)
	}
	
	// Setup test environment for each project
	engines := make([]*sync.Engine, numProjects)
	repos := make([]*git.Repository, numProjects)
	
	for i := 0; i < numProjects; i++ {
		jiraClient, gitOps, engine := setupPerformanceTest(t)
		engines[i] = engine
		repos[i] = setupTestRepository(t)
	}
	
	defer func() {
		for _, repo := range repos {
			cleanupTestRepository(repo)
		}
	}()

	// Measure concurrent sync performance
	startTime := time.Now()
	
	// Start concurrent syncs
	type syncResult struct {
		projectIndex int
		results      []sync.SyncResult
		err          error
		duration     time.Duration
	}
	
	resultsChan := make(chan syncResult, numProjects)
	
	for i := 0; i < numProjects; i++ {
		go func(projectIndex int) {
			projectStartTime := time.Now()
			progress := make(chan sync.SyncProgress, 100)
			
			results, err := engines[projectIndex].SynchronizeProject(ctx, repos[projectIndex], false, progress)
			close(progress)
			
			resultsChan <- syncResult{
				projectIndex: projectIndex,
				results:      results,
				err:          err,
				duration:     time.Since(projectStartTime),
			}
		}(i)
	}
	
	// Collect results
	allResults := make([]syncResult, 0, numProjects)
	for i := 0; i < numProjects; i++ {
		result := <-resultsChan
		allResults = append(allResults, result)
		require.NoError(t, result.err, "Project %d sync should succeed", result.projectIndex)
	}
	
	totalDuration := time.Since(startTime)
	
	// Calculate performance metrics
	totalIssues := 0
	maxProjectDuration := time.Duration(0)
	
	for _, result := range allResults {
		totalIssues += len(result.results)
		if result.duration > maxProjectDuration {
			maxProjectDuration = result.duration
		}
	}
	
	t.Logf("Concurrent Sync Performance Results:")
	t.Logf("  Projects: %d", numProjects)
	t.Logf("  Total issues: %d", totalIssues)
	t.Logf("  Total time: %v", totalDuration)
	t.Logf("  Max project time: %v", maxProjectDuration)
	t.Logf("  Concurrent throughput: %.2f issues/minute", float64(totalIssues)/(totalDuration.Minutes()))
	
	// Performance assertions
	assert.LessOrEqual(t, maxProjectDuration, maxSyncTime100Issues,
		"Individual project sync should complete within %v", maxSyncTime100Issues)
	
	// Concurrent execution should be more efficient than sequential
	expectedSequentialTime := time.Duration(numProjects) * maxProjectDuration
	efficiency := float64(expectedSequentialTime) / float64(totalDuration)
	assert.Greater(t, efficiency, 1.5, "Concurrent execution should be at least 50%% more efficient")
}

// TestMemoryUsagePattern tests memory usage patterns during sync
func TestMemoryUsagePattern(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	if os.Getenv("PERF_TEST_JIRA_URL") == "" {
		t.Skip("Skipping performance test: PERF_TEST_JIRA_URL not set")
	}

	ctx := context.Background()
	
	// Setup test environment
	jiraClient, gitOps, engine := setupPerformanceTest(t)
	repo := setupTestRepository(t)
	defer cleanupTestRepository(repo)

	// Monitor memory usage during sync
	memoryReadings := make([]int64, 0)
	stopMonitoring := make(chan bool)
	
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				memoryReadings = append(memoryReadings, getCurrentMemoryUsage())
			case <-stopMonitoring:
				return
			}
		}
	}()

	// Run sync operation
	progress := make(chan sync.SyncProgress, 100)
	results, err := engine.SynchronizeProject(ctx, repo, false, progress)
	close(progress)
	
	// Stop memory monitoring
	stopMonitoring <- true
	
	require.NoError(t, err)
	require.NotEmpty(t, results)
	
	// Analyze memory usage pattern
	if len(memoryReadings) > 0 {
		minMemory := memoryReadings[0]
		maxMemory := memoryReadings[0]
		
		for _, reading := range memoryReadings {
			if reading < minMemory {
				minMemory = reading
			}
			if reading > maxMemory {
				maxMemory = reading
			}
		}
		
		memoryGrowth := maxMemory - minMemory
		
		t.Logf("Memory Usage Pattern:")
		t.Logf("  Min memory: %d bytes (%.2f MB)", minMemory, float64(minMemory)/(1024*1024))
		t.Logf("  Max memory: %d bytes (%.2f MB)", maxMemory, float64(maxMemory)/(1024*1024))
		t.Logf("  Memory growth: %d bytes (%.2f MB)", memoryGrowth, float64(memoryGrowth)/(1024*1024))
		t.Logf("  Readings taken: %d", len(memoryReadings))
		
		// Memory growth should be reasonable
		assert.LessOrEqual(t, memoryGrowth, int64(maxMemoryUsage),
			"Memory growth should not exceed %d bytes", maxMemoryUsage)
		
		// Memory should not grow linearly with number of issues (indicating a leak)
		growthPerIssue := float64(memoryGrowth) / float64(len(results))
		assert.LessOrEqual(t, growthPerIssue, 1024*10, // 10KB per issue max
			"Memory growth per issue should be reasonable")
	}
}

// TestRateLimitCompliance tests JIRA API rate limit compliance
func TestRateLimitCompliance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	if os.Getenv("PERF_TEST_JIRA_URL") == "" {
		t.Skip("Skipping performance test: PERF_TEST_JIRA_URL not set")
	}

	ctx := context.Background()
	
	// Setup JIRA client with rate limiting
	jiraConfig := jira.Config{
		BaseURL:           os.Getenv("PERF_TEST_JIRA_URL"),
		Username:          os.Getenv("PERF_TEST_JIRA_USERNAME"),
		APIToken:          os.Getenv("PERF_TEST_JIRA_TOKEN"),
		RequestsPerMinute: 300, // Conservative rate limit
		MaxRetries:        3,
	}
	
	jiraClient, err := jira.NewClient(jiraConfig)
	require.NoError(t, err)

	// Track API call timing
	startTime := time.Now()
	requestTimes := make([]time.Time, 0)
	
	// Make a series of API calls
	projectKey := os.Getenv("PERF_TEST_PROJECT_KEY")
	numCalls := 50
	
	for i := 0; i < numCalls; i++ {
		requestStart := time.Now()
		_, err := jiraClient.SearchIssues(ctx, fmt.Sprintf("project = %s", projectKey), i*50, 50)
		if err != nil {
			t.Logf("API call %d failed: %v", i, err)
			continue
		}
		requestTimes = append(requestTimes, requestStart)
	}
	
	totalDuration := time.Since(startTime)
	
	// Analyze rate limiting compliance
	if len(requestTimes) > 1 {
		// Calculate requests per minute
		actualRPM := float64(len(requestTimes)) / totalDuration.Minutes()
		
		t.Logf("Rate Limit Compliance:")
		t.Logf("  Successful requests: %d", len(requestTimes))
		t.Logf("  Total duration: %v", totalDuration)
		t.Logf("  Actual RPM: %.2f", actualRPM)
		t.Logf("  Configured RPM: %d", jiraConfig.RequestsPerMinute)
		
		// Should not exceed configured rate limit
		assert.LessOrEqual(t, actualRPM, float64(jiraConfig.RequestsPerMinute)*1.1,
			"Actual RPM should not exceed configured limit by more than 10%%")
		
		// Should be reasonably close to the limit (efficient)
		efficiency := actualRPM / float64(jiraConfig.RequestsPerMinute)
		assert.Greater(t, efficiency, 0.8,
			"Should utilize at least 80%% of available rate limit")
	}
}

// TestScalabilityLimits tests system behavior at scale limits
func TestScalabilityLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Test with large result sets
	testSizes := []int{100, 500, 1000, 2000}
	
	for _, size := range testSizes {
		t.Run(fmt.Sprintf("Size_%d", size), func(t *testing.T) {
			if os.Getenv("PERF_TEST_JIRA_URL") == "" {
				t.Skip("Skipping performance test: PERF_TEST_JIRA_URL not set")
			}

			ctx := context.Background()
			jiraClient, gitOps, engine := setupPerformanceTest(t)
			repo := setupTestRepository(t)
			defer cleanupTestRepository(repo)

			// Measure performance for this size
			startTime := time.Now()
			startMemory := getCurrentMemoryUsage()
			
			// Use JQL to limit results to test size
			jql := fmt.Sprintf("project = %s ORDER BY created DESC", os.Getenv("PERF_TEST_PROJECT_KEY"))
			
			results, err := jiraClient.SearchIssues(ctx, jql, 0, size)
			
			endTime := time.Now()
			endMemory := getCurrentMemoryUsage()
			
			duration := endTime.Sub(startTime)
			memoryUsed := endMemory - startMemory
			
			if err != nil {
				t.Logf("Size %d failed: %v", size, err)
				return
			}
			
			t.Logf("Size %d Results:", size)
			t.Logf("  Issues found: %d", len(results.Issues))
			t.Logf("  Time: %v", duration)
			t.Logf("  Memory: %.2f MB", float64(memoryUsed)/(1024*1024))
			t.Logf("  Throughput: %.2f issues/second", float64(len(results.Issues))/duration.Seconds())
			
			// Performance should scale reasonably
			timePerIssue := duration.Seconds() / float64(len(results.Issues))
			memoryPerIssue := float64(memoryUsed) / float64(len(results.Issues))
			
			assert.LessOrEqual(t, timePerIssue, 0.1, "Should process more than 10 issues per second")
			assert.LessOrEqual(t, memoryPerIssue, 1024*100, "Should use less than 100KB per issue")
		})
	}
}

// Helper functions

func setupPerformanceTest(t *testing.T) (*jira.Client, *git.Operations, *sync.Engine) {
	// Setup JIRA client
	jiraConfig := jira.Config{
		BaseURL:           os.Getenv("PERF_TEST_JIRA_URL"),
		Username:          os.Getenv("PERF_TEST_JIRA_USERNAME"),
		APIToken:          os.Getenv("PERF_TEST_JIRA_TOKEN"),
		RequestsPerMinute: 300,
		MaxRetries:        3,
	}
	
	jiraClient, err := jira.NewClient(jiraConfig)
	require.NoError(t, err)

	// Setup Git operations
	gitConfig := git.Config{
		AuthMethod: "none", // Use none for testing
		CommitAuthor: git.CommitAuthor{
			Name:  "Performance Test",
			Email: "perf-test@example.com",
		},
	}
	
	gitOps, err := git.NewOperations(gitConfig)
	require.NoError(t, err)

	// Setup sync engine
	syncConfig := sync.Config{
		ProjectKey:     os.Getenv("PERF_TEST_PROJECT_KEY"),
		GitRepository: "memory://test-repo", // Use memory for testing
		GitBranch:     "main",
		BatchSize:     50,
		MaxRetries:    3,
	}
	
	engine := sync.NewEngine(syncConfig, jiraClient, gitOps)
	
	return jiraClient, gitOps, engine
}

func setupTestRepository(t *testing.T) *git.Repository {
	// Create in-memory repository for testing
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)
	return repo
}

func cleanupTestRepository(repo *git.Repository) {
	// Memory storage is automatically cleaned up
}

func getCurrentMemoryUsage() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Alloc)
}

func monitorProgress(t *testing.T, progress <-chan sync.SyncProgress) {
	for p := range progress {
		if p.TotalItems > 0 {
			t.Logf("Progress: %d/%d (%.1f%%) - %s",
				p.ProcessedItems, p.TotalItems, p.PercentComplete,
				p.EstimatedTimeRemaining)
		}
	}
}
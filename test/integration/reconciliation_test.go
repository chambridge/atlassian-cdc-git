package integration

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
)

type ReconciliationTestSuite struct {
	suite.Suite
	ctx       context.Context
	k8sClient client.Client
	namespace string
	jiracdc   *jiradcdv1.JiraCDC
}

func (suite *ReconciliationTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.namespace = "jiracdc-reconciliation-test"
	
	// This will fail until proper k8s client setup for integration tests
	suite.k8sClient = nil // Will be set up when integration test infrastructure is ready
	
	// Create test JiraCDC resource that has already completed bootstrap
	suite.jiracdc = &jiradcdv1.JiraCDC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reconciliation-test-jiracdc",
			Namespace: suite.namespace,
		},
		Spec: jiradcdv1.JiraCDCSpec{
			JiraInstance: jiradcdv1.JiraInstanceConfig{
				BaseURL:           "https://reconcile-test.atlassian.net",
				CredentialsSecret: "reconcile-jira-creds",
			},
			SyncTarget: jiradcdv1.SyncTargetConfig{
				Type:       "project",
				ProjectKey: "RECON",
			},
			GitRepository: jiradcdv1.GitRepositoryConfig{
				URL:               "git@github.com:test/reconcile-repo.git",
				CredentialsSecret: "reconcile-git-creds",
				Branch:            "main",
			},
			SyncConfig: jiradcdv1.SyncConfig{
				Bootstrap:        false, // Already bootstrapped
				Interval:         "5m",  // Frequent reconciliation for testing
				ActiveIssuesOnly: true,
			},
		},
		Status: jiradcdv1.JiraCDCStatus{
			Phase:             "Current",
			SyncedIssueCount:  50,
			LastSyncTime:      &metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
			LastCommitHash:    "abc123def456",
			ComponentStatus: jiradcdv1.ComponentStatus{
				JiraConnection: "healthy",
				GitRepository:  "healthy",
			},
		},
	}
}

func (suite *ReconciliationTestSuite) TestManualReconciliationTrigger() {
	suite.T().Skip("This test will fail until reconciliation triggering is implemented")
	
	// Test manual trigger of reconciliation task
	err := suite.k8sClient.Create(suite.ctx, suite.jiracdc)
	require.NoError(suite.T(), err, "Should create JiraCDC resource")
	
	// Trigger manual reconciliation
	updated := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(suite.jiracdc), updated)
	require.NoError(suite.T(), err)
	
	updated.Spec.SyncConfig.TriggerReconciliation = true
	err = suite.k8sClient.Update(suite.ctx, updated)
	require.NoError(suite.T(), err)
	
	// Wait for reconciliation task to be created
	suite.waitForTaskType("reconciliation", 30*time.Second)
	
	// Verify task creation
	final := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(suite.jiracdc), final)
	require.NoError(suite.T(), err)
	
	assert.Equal(suite.T(), "reconciliation", final.Status.CurrentTask.Type)
	assert.Equal(suite.T(), "pending", final.Status.CurrentTask.Status)
	assert.Equal(suite.T(), "Syncing", final.Status.Phase)
}

func (suite *ReconciliationTestSuite) TestScheduledReconciliation() {
	suite.T().Skip("This test will fail until scheduled reconciliation is implemented")
	
	// Test that reconciliation runs automatically based on interval
	shortIntervalJiraCDC := suite.jiracdc.DeepCopy()
	shortIntervalJiraCDC.Name = "scheduled-reconcile-test"
	shortIntervalJiraCDC.Spec.SyncConfig.Interval = "1m" // Very frequent for testing
	
	err := suite.k8sClient.Create(suite.ctx, shortIntervalJiraCDC)
	require.NoError(suite.T(), err)
	
	// Wait for automatic reconciliation to trigger
	suite.waitForTaskTypeWithResource(shortIntervalJiraCDC, "reconciliation", 90*time.Second)
	
	// Verify scheduled task
	updated := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(shortIntervalJiraCDC), updated)
	require.NoError(suite.T(), err)
	
	assert.Equal(suite.T(), "reconciliation", updated.Status.CurrentTask.Type)
	assert.Contains(suite.T(), []string{"pending", "running"}, updated.Status.CurrentTask.Status)
}

func (suite *ReconciliationTestSuite) TestReconciliationWithFilter() {
	suite.T().Skip("This test will fail until filtered reconciliation is implemented")
	
	// Test reconciliation with issue filter (only active issues)
	filteredJiraCDC := suite.jiracdc.DeepCopy()
	filteredJiraCDC.Name = "filtered-reconcile-test"
	filteredJiraCDC.Spec.SyncConfig.ActiveIssuesOnly = true
	filteredJiraCDC.Spec.SyncConfig.TriggerReconciliation = true
	
	err := suite.k8sClient.Create(suite.ctx, filteredJiraCDC)
	require.NoError(suite.T(), err)
	
	// Wait for reconciliation to complete
	suite.waitForTaskStatusWithResource(filteredJiraCDC, "completed", 120*time.Second)
	
	// Verify filtered reconciliation
	updated := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(filteredJiraCDC), updated)
	require.NoError(suite.T(), err)
	
	// Should have processed fewer items than total project issues
	if updated.Status.CurrentTask.Progress != nil {
		assert.Less(suite.T(), updated.Status.CurrentTask.Progress.ProcessedItems, updated.Status.SyncedIssueCount,
			"Filtered reconciliation should process fewer items than total synced issues")
	}
	
	// Should still be current after reconciliation
	assert.Equal(suite.T(), "Current", updated.Status.Phase)
}

func (suite *ReconciliationTestSuite) TestReconciliationDetectsChanges() {
	suite.T().Skip("This test will fail until change detection is implemented")
	
	// Test that reconciliation detects and syncs JIRA changes
	err := suite.k8sClient.Create(suite.ctx, suite.jiracdc)
	require.NoError(suite.T(), err)
	
	// Simulate JIRA changes (this would require mock JIRA server or real changes)
	// For integration test, we assume some issues have been updated in JIRA
	
	// Trigger reconciliation
	updated := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(suite.jiracdc), updated)
	require.NoError(suite.T(), err)
	
	originalCommitHash := updated.Status.LastCommitHash
	updated.Spec.SyncConfig.TriggerReconciliation = true
	err = suite.k8sClient.Update(suite.ctx, updated)
	require.NoError(suite.T(), err)
	
	// Wait for reconciliation to complete
	suite.waitForTaskStatus("completed", 120*time.Second)
	
	// Verify changes were detected and synced
	final := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(suite.jiracdc), final)
	require.NoError(suite.T(), err)
	
	// If changes were detected, commit hash should be different
	if final.Status.CurrentTask.Progress.ProcessedItems > 0 {
		assert.NotEqual(suite.T(), originalCommitHash, final.Status.LastCommitHash,
			"Commit hash should change when issues are updated")
	}
	
	// Last sync time should be updated
	assert.True(suite.T(), final.Status.LastSyncTime.After(updated.Status.LastSyncTime.Time),
		"Last sync time should be updated")
}

func (suite *ReconciliationTestSuite) TestReconciliationPerformance() {
	suite.T().Skip("This test will fail until performance optimization is implemented")
	
	// Test reconciliation performance with large project
	largeProjectJiraCDC := suite.jiracdc.DeepCopy()
	largeProjectJiraCDC.Name = "large-reconcile-test"
	largeProjectJiraCDC.Spec.SyncTarget.ProjectKey = "LARGE" // Assumed large project
	largeProjectJiraCDC.Status.SyncedIssueCount = 1000       // Already has 1000 synced issues
	
	err := suite.k8sClient.Create(suite.ctx, largeProjectJiraCDC)
	require.NoError(suite.T(), err)
	
	// Trigger reconciliation and measure time
	startTime := time.Now()
	
	updated := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(largeProjectJiraCDC), updated)
	require.NoError(suite.T(), err)
	
	updated.Spec.SyncConfig.TriggerReconciliation = true
	err = suite.k8sClient.Update(suite.ctx, updated)
	require.NoError(suite.T(), err)
	
	// Wait for completion (should be much faster than bootstrap)
	suite.waitForTaskStatusWithResource(largeProjectJiraCDC, "completed", 600*time.Second) // 10 minutes max
	
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	
	// Reconciliation should be faster than bootstrap
	assert.Less(suite.T(), duration, 10*time.Minute, "Reconciliation should complete within 10 minutes")
	
	// Verify completion
	final := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(largeProjectJiraCDC), final)
	require.NoError(suite.T(), err)
	
	assert.Equal(suite.T(), "completed", final.Status.CurrentTask.Status)
	assert.Equal(suite.T(), "Current", final.Status.Phase)
}

func (suite *ReconciliationTestSuite) TestReconciliationErrorRecovery() {
	suite.T().Skip("This test will fail until error recovery is implemented")
	
	// Test reconciliation recovery from errors
	errorJiraCDC := suite.jiracdc.DeepCopy()
	errorJiraCDC.Name = "error-recovery-test"
	errorJiraCDC.Spec.JiraInstance.CredentialsSecret = "invalid-creds" // Will cause error
	
	err := suite.k8sClient.Create(suite.ctx, errorJiraCDC)
	require.NoError(suite.T(), err)
	
	// Trigger reconciliation that will fail
	updated := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(errorJiraCDC), updated)
	require.NoError(suite.T(), err)
	
	updated.Spec.SyncConfig.TriggerReconciliation = true
	err = suite.k8sClient.Update(suite.ctx, updated)
	require.NoError(suite.T(), err)
	
	// Wait for error state
	suite.waitForTaskStatusWithResource(errorJiraCDC, "failed", 60*time.Second)
	
	// Fix the credentials
	fixed := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(errorJiraCDC), fixed)
	require.NoError(suite.T(), err)
	
	fixed.Spec.JiraInstance.CredentialsSecret = "reconcile-jira-creds" // Fix credentials
	fixed.Spec.SyncConfig.TriggerReconciliation = true                 // Retry
	err = suite.k8sClient.Update(suite.ctx, fixed)
	require.NoError(suite.T(), err)
	
	// Wait for successful reconciliation
	suite.waitForTaskStatusWithResource(errorJiraCDC, "completed", 120*time.Second)
	
	// Verify recovery
	final := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(errorJiraCDC), final)
	require.NoError(suite.T(), err)
	
	assert.Equal(suite.T(), "completed", final.Status.CurrentTask.Status)
	assert.Equal(suite.T(), "Current", final.Status.Phase)
	assert.Equal(suite.T(), "healthy", final.Status.ComponentStatus.JiraConnection)
}

func (suite *ReconciliationTestSuite) TestReconciliationCancellation() {
	suite.T().Skip("This test will fail until task cancellation is implemented")
	
	// Test cancellation of running reconciliation
	err := suite.k8sClient.Create(suite.ctx, suite.jiracdc)
	require.NoError(suite.T(), err)
	
	// Start reconciliation
	updated := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(suite.jiracdc), updated)
	require.NoError(suite.T(), err)
	
	updated.Spec.SyncConfig.TriggerReconciliation = true
	err = suite.k8sClient.Update(suite.ctx, updated)
	require.NoError(suite.T(), err)
	
	// Wait for reconciliation to start
	suite.waitForTaskStatus("running", 30*time.Second)
	
	// Cancel the task
	cancelling := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(suite.jiracdc), cancelling)
	require.NoError(suite.T(), err)
	
	cancelling.Spec.SyncConfig.CancelCurrentTask = true
	err = suite.k8sClient.Update(suite.ctx, cancelling)
	require.NoError(suite.T(), err)
	
	// Wait for cancellation
	suite.waitForTaskStatus("cancelled", 30*time.Second)
	
	// Verify cancellation
	final := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(suite.jiracdc), final)
	require.NoError(suite.T(), err)
	
	assert.Equal(suite.T(), "cancelled", final.Status.CurrentTask.Status)
	assert.Equal(suite.T(), "Paused", final.Status.Phase)
}

func (suite *ReconciliationTestSuite) TestReconciliationWithForceRefresh() {
	suite.T().Skip("This test will fail until force refresh is implemented")
	
	// Test reconciliation with force refresh (re-sync all issues)
	forceRefreshJiraCDC := suite.jiracdc.DeepCopy()
	forceRefreshJiraCDC.Name = "force-refresh-test"
	forceRefreshJiraCDC.Spec.SyncConfig.ForceRefresh = true
	forceRefreshJiraCDC.Spec.SyncConfig.TriggerReconciliation = true
	
	err := suite.k8sClient.Create(suite.ctx, forceRefreshJiraCDC)
	require.NoError(suite.T(), err)
	
	// Wait for reconciliation to complete
	suite.waitForTaskStatusWithResource(forceRefreshJiraCDC, "completed", 180*time.Second)
	
	// Verify force refresh behavior
	updated := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(forceRefreshJiraCDC), updated)
	require.NoError(suite.T(), err)
	
	// With force refresh, should process all synced issues
	if updated.Status.CurrentTask.Progress != nil {
		assert.Equal(suite.T(), updated.Status.SyncedIssueCount, updated.Status.CurrentTask.Progress.ProcessedItems,
			"Force refresh should process all synced issues")
	}
	
	// Commit hash should change (all files re-written)
	assert.NotEqual(suite.T(), suite.jiracdc.Status.LastCommitHash, updated.Status.LastCommitHash,
		"Force refresh should create new commits")
}

func (suite *ReconciliationTestSuite) TestConcurrentReconciliations() {
	suite.T().Skip("This test will fail until concurrency control is implemented")
	
	// Test that only one reconciliation can run at a time
	err := suite.k8sClient.Create(suite.ctx, suite.jiracdc)
	require.NoError(suite.T(), err)
	
	// Start first reconciliation
	updated := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(suite.jiracdc), updated)
	require.NoError(suite.T(), err)
	
	updated.Spec.SyncConfig.TriggerReconciliation = true
	err = suite.k8sClient.Update(suite.ctx, updated)
	require.NoError(suite.T(), err)
	
	// Wait for first reconciliation to start
	suite.waitForTaskStatus("running", 30*time.Second)
	
	// Try to start second reconciliation
	concurrent := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(suite.jiracdc), concurrent)
	require.NoError(suite.T(), err)
	
	concurrent.Spec.SyncConfig.TriggerReconciliation = true
	err = suite.k8sClient.Update(suite.ctx, concurrent)
	require.NoError(suite.T(), err)
	
	// Second reconciliation should be queued or rejected
	time.Sleep(5 * time.Second) // Give time for processing
	
	final := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(suite.jiracdc), final)
	require.NoError(suite.T(), err)
	
	// Should still be running the original task
	assert.Equal(suite.T(), "running", final.Status.CurrentTask.Status)
	assert.Equal(suite.T(), "reconciliation", final.Status.CurrentTask.Type)
}

// Helper functions (similar to bootstrap_test.go)

func (suite *ReconciliationTestSuite) waitForTaskType(expectedType string, timeout time.Duration) {
	suite.waitForTaskTypeWithResource(suite.jiracdc, expectedType, timeout)
}

func (suite *ReconciliationTestSuite) waitForTaskTypeWithResource(resource *jiradcdv1.JiraCDC, expectedType string, timeout time.Duration) {
	suite.T().Helper()
	
	ctx, cancel := context.WithTimeout(suite.ctx, timeout)
	defer cancel()
	
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			suite.T().Fatalf("Timeout waiting for task type %s", expectedType)
		case <-ticker.C:
			updated := &jiradcdv1.JiraCDC{}
			err := suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(resource), updated)
			if err != nil {
				continue
			}
			
			if updated.Status.CurrentTask != nil && updated.Status.CurrentTask.Type == expectedType {
				return
			}
		}
	}
}

func (suite *ReconciliationTestSuite) waitForTaskStatus(expectedStatus string, timeout time.Duration) {
	suite.waitForTaskStatusWithResource(suite.jiracdc, expectedStatus, timeout)
}

func (suite *ReconciliationTestSuite) waitForTaskStatusWithResource(resource *jiradcdv1.JiraCDC, expectedStatus string, timeout time.Duration) {
	suite.T().Helper()
	
	ctx, cancel := context.WithTimeout(suite.ctx, timeout)
	defer cancel()
	
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			suite.T().Fatalf("Timeout waiting for task status %s", expectedStatus)
		case <-ticker.C:
			updated := &jiradcdv1.JiraCDC{}
			err := suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(resource), updated)
			if err != nil {
				continue
			}
			
			if updated.Status.CurrentTask != nil && updated.Status.CurrentTask.Status == expectedStatus {
				return
			}
		}
	}
}

func (suite *ReconciliationTestSuite) TearDownTest() {
	// Clean up after each test
	if suite.jiracdc != nil {
		err := suite.k8sClient.Delete(suite.ctx, suite.jiracdc)
		if err != nil {
			suite.T().Logf("Error cleaning up JiraCDC: %v", err)
		}
		
		// Wait for deletion
		for i := 0; i < 10; i++ {
			updated := &jiradcdv1.JiraCDC{}
			err := suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(suite.jiracdc), updated)
			if client.IgnoreNotFound(err) == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}
}

func (suite *ReconciliationTestSuite) TearDownSuite() {
	suite.T().Log("Tearing down reconciliation integration test suite")
}

// Test runner function
func TestReconciliationIntegration(t *testing.T) {
	// This will fail until the integration test environment is set up
	t.Skip("Skipping reconciliation integration tests until test environment is ready")
	
	suite.Run(t, new(ReconciliationTestSuite))
}
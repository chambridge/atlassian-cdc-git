package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	jiracdc "jiracdc-operator/api/v1"
)

const (
	testTimeout = 10 * time.Minute
	pollInterval = 5 * time.Second
)

// TestCompleteWorkflow tests the complete end-to-end workflow
func TestCompleteWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end test in short mode")
	}

	// Skip if required environment variables are not set
	if os.Getenv("E2E_JIRA_URL") == "" || os.Getenv("E2E_GIT_URL") == "" {
		t.Skip("Skipping e2e test: E2E_JIRA_URL or E2E_GIT_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Setup test environment
	testEnv, k8sClient := setupTestEnvironment(t)
	defer testEnv.Stop()

	// Create test secrets
	createTestSecrets(t, ctx, k8sClient)

	// Create JiraCDC resource
	jiracdc := createTestJiraCDC(t, ctx, k8sClient)

	// Test operator deployment workflow
	t.Run("OperatorDeployment", func(t *testing.T) {
		testOperatorDeployment(t, ctx, k8sClient, jiracdc)
	})

	// Test bootstrap operation
	t.Run("BootstrapOperation", func(t *testing.T) {
		testBootstrapOperation(t, ctx, k8sClient, jiracdc)
	})

	// Test real-time synchronization
	t.Run("RealtimeSync", func(t *testing.T) {
		testRealtimeSync(t, ctx, k8sClient, jiracdc)
	})

	// Test API operand functionality
	t.Run("APIOperand", func(t *testing.T) {
		testAPIOperand(t, ctx, k8sClient, jiracdc)
	})

	// Test UI operand functionality
	t.Run("UIOperand", func(t *testing.T) {
		testUIOperand(t, ctx, k8sClient, jiracdc)
	})

	// Test agent integration
	t.Run("AgentIntegration", func(t *testing.T) {
		testAgentIntegration(t, ctx, jiracdc)
	})

	// Test error handling and recovery
	t.Run("ErrorHandling", func(t *testing.T) {
		testErrorHandling(t, ctx, k8sClient, jiracdc)
	})

	// Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		testCleanup(t, ctx, k8sClient, jiracdc)
	})
}

func setupTestEnvironment(t *testing.T) (*envtest.Environment, client.Client) {
	// Setup test environment
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)

	// Setup scheme
	err = jiracdc.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	// Create client
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)

	return testEnv, k8sClient
}

func createTestSecrets(t *testing.T, ctx context.Context, k8sClient client.Client) {
	// Create JIRA credentials secret
	jiraSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jira-credentials",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"username":  []byte(os.Getenv("E2E_JIRA_USERNAME")),
			"apiToken":  []byte(os.Getenv("E2E_JIRA_TOKEN")),
		},
	}
	err := k8sClient.Create(ctx, jiraSecret)
	require.NoError(t, err)

	// Create Git credentials secret
	gitSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "git-credentials",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"ssh-privatekey": []byte(os.Getenv("E2E_GIT_SSH_KEY")),
		},
	}
	err = k8sClient.Create(ctx, gitSecret)
	require.NoError(t, err)
}

func createTestJiraCDC(t *testing.T, ctx context.Context, k8sClient client.Client) *jiracdc.JiraCDC {
	jiracdc := &jiracdc.JiraCDC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "e2e-test-jiracdc",
			Namespace: "default",
		},
		Spec: jiracdc.JiraCDCSpec{
			JiraInstance: jiracdc.JiraInstanceConfig{
				BaseURL:           os.Getenv("E2E_JIRA_URL"),
				CredentialsSecret: "jira-credentials",
			},
			SyncTarget: jiracdc.SyncTargetConfig{
				Type:       "project",
				ProjectKey: os.Getenv("E2E_JIRA_PROJECT"),
			},
			GitRepository: jiracdc.GitRepositoryConfig{
				URL:               os.Getenv("E2E_GIT_URL"),
				CredentialsSecret: "git-credentials",
				Branch:            "main",
			},
			Operands: jiracdc.OperandsConfig{
				API: jiracdc.APIOperandConfig{
					Enabled:  true,
					Replicas: 1,
				},
				UI: jiracdc.UIOperandConfig{
					Enabled:  true,
					Replicas: 1,
				},
			},
			SyncConfig: jiracdc.SyncConfig{
				Interval:          "2m",
				Bootstrap:         true,
				ActiveIssuesOnly:  true,
			},
		},
	}

	err := k8sClient.Create(ctx, jiracdc)
	require.NoError(t, err)

	return jiracdc
}

func testOperatorDeployment(t *testing.T, ctx context.Context, k8sClient client.Client, jiracdc *jiracdc.JiraCDC) {
	// Wait for operator to create operands
	require.Eventually(t, func() bool {
		// Check API deployment
		apiDeployment := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      "jiracdc-api-" + jiracdc.Name,
			Namespace: jiracdc.Namespace,
		}, apiDeployment)
		if err != nil {
			return false
		}

		// Check UI deployment
		uiDeployment := &appsv1.Deployment{}
		err = k8sClient.Get(ctx, types.NamespacedName{
			Name:      "jiracdc-ui-" + jiracdc.Name,
			Namespace: jiracdc.Namespace,
		}, uiDeployment)
		return err == nil
	}, 2*time.Minute, pollInterval, "Operands should be deployed")

	// Wait for deployments to be ready
	require.Eventually(t, func() bool {
		// Check API deployment status
		apiDeployment := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      "jiracdc-api-" + jiracdc.Name,
			Namespace: jiracdc.Namespace,
		}, apiDeployment)
		if err != nil || apiDeployment.Status.ReadyReplicas != *apiDeployment.Spec.Replicas {
			return false
		}

		// Check UI deployment status
		uiDeployment := &appsv1.Deployment{}
		err = k8sClient.Get(ctx, types.NamespacedName{
			Name:      "jiracdc-ui-" + jiracdc.Name,
			Namespace: jiracdc.Namespace,
		}, uiDeployment)
		return err == nil && uiDeployment.Status.ReadyReplicas == *uiDeployment.Spec.Replicas
	}, 3*time.Minute, pollInterval, "Operands should be ready")

	// Verify services are created
	apiService := &corev1.Service{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "jiracdc-api-" + jiracdc.Name,
		Namespace: jiracdc.Namespace,
	}, apiService)
	require.NoError(t, err)

	uiService := &corev1.Service{}
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      "jiracdc-ui-" + jiracdc.Name,
		Namespace: jiracdc.Namespace,
	}, uiService)
	require.NoError(t, err)
}

func testBootstrapOperation(t *testing.T, ctx context.Context, k8sClient client.Client, jiracdc *jiracdc.JiraCDC) {
	// Wait for JiraCDC to enter syncing phase
	require.Eventually(t, func() bool {
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      jiracdc.Name,
			Namespace: jiracdc.Namespace,
		}, jiracdc)
		if err != nil {
			return false
		}
		return jiracdc.Status.Phase == "Syncing"
	}, 5*time.Minute, pollInterval, "JiraCDC should enter syncing phase")

	// Wait for bootstrap to complete
	require.Eventually(t, func() bool {
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      jiracdc.Name,
			Namespace: jiracdc.Namespace,
		}, jiracdc)
		if err != nil {
			return false
		}
		return jiracdc.Status.Phase == "Current" && jiracdc.Status.SyncedIssueCount > 0
	}, 8*time.Minute, pollInterval, "Bootstrap should complete successfully")

	// Verify status fields are populated
	assert.NotNil(t, jiracdc.Status.LastSyncTime)
	assert.Greater(t, jiracdc.Status.SyncedIssueCount, 0)
	assert.True(t, jiracdc.Status.OperandStatus.API.Ready)
	assert.True(t, jiracdc.Status.OperandStatus.UI.Ready)
}

func testRealtimeSync(t *testing.T, ctx context.Context, k8sClient client.Client, jiracdc *jiracdc.JiraCDC) {
	// Record initial sync state
	initialSyncTime := jiracdc.Status.LastSyncTime
	initialIssueCount := jiracdc.Status.SyncedIssueCount

	// Wait for next sync cycle (interval is 2m)
	time.Sleep(3 * time.Minute)

	// Verify sync has occurred
	require.Eventually(t, func() bool {
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      jiracdc.Name,
			Namespace: jiracdc.Namespace,
		}, jiracdc)
		if err != nil {
			return false
		}
		
		// Check if sync time has been updated
		return jiracdc.Status.LastSyncTime != nil && 
			   jiracdc.Status.LastSyncTime.After(initialSyncTime.Time)
	}, 1*time.Minute, pollInterval, "Real-time sync should occur")

	// Issue count should remain stable or increase (never decrease)
	assert.GreaterOrEqual(t, jiracdc.Status.SyncedIssueCount, initialIssueCount)
}

func testAPIOperand(t *testing.T, ctx context.Context, k8sClient client.Client, jiracdc *jiracdc.JiraCDC) {
	// Port-forward to API service (in real e2e, this would be done by test infrastructure)
	apiEndpoint := fmt.Sprintf("http://jiracdc-api-%s.%s.svc.cluster.local:8080", jiracdc.Name, jiracdc.Namespace)
	
	// Test health endpoint
	resp, err := http.Get(apiEndpoint + "/api/v1/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test projects endpoint
	resp, err = http.Get(apiEndpoint + "/api/v1/projects")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test specific project endpoint
	projectKey := os.Getenv("E2E_JIRA_PROJECT")
	resp, err = http.Get(apiEndpoint + "/api/v1/projects/" + projectKey)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test tasks endpoint
	resp, err = http.Get(apiEndpoint + "/api/v1/tasks")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test metrics endpoint
	resp, err = http.Get(apiEndpoint + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func testUIOperand(t *testing.T, ctx context.Context, k8sClient client.Client, jiracdc *jiracdc.JiraCDC) {
	// Port-forward to UI service
	uiEndpoint := fmt.Sprintf("http://jiracdc-ui-%s.%s.svc.cluster.local:3000", jiracdc.Name, jiracdc.Namespace)
	
	// Test UI is serving content
	resp, err := http.Get(uiEndpoint)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify content type
	contentType := resp.Header.Get("Content-Type")
	assert.Contains(t, contentType, "text/html")
}

func testAgentIntegration(t *testing.T, ctx context.Context, jiracdc *jiracdc.JiraCDC) {
	// Clone the git repository to verify agent can access it
	gitURL := os.Getenv("E2E_GIT_URL")
	tempDir := filepath.Join(os.TempDir(), "e2e-agent-test")
	defer os.RemoveAll(tempDir)

	// Simulate agent submodule access
	cmd := exec.Command("git", "clone", gitURL, tempDir)
	err := cmd.Run()
	require.NoError(t, err)

	// Verify issue files exist
	files, err := filepath.Glob(filepath.Join(tempDir, "*.md"))
	require.NoError(t, err)
	assert.Greater(t, len(files), 0, "Should have issue files")

	// Verify file format
	if len(files) > 0 {
		content, err := os.ReadFile(files[0])
		require.NoError(t, err)
		
		// Should have YAML frontmatter
		assert.Contains(t, string(content), "---")
		assert.Contains(t, string(content), "key:")
		assert.Contains(t, string(content), "syncedAt:")
	}
}

func testErrorHandling(t *testing.T, ctx context.Context, k8sClient client.Client, jiracdc *jiracdc.JiraCDC) {
	// Test invalid credential handling by temporarily corrupting secret
	jiraSecret := &corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      "jira-credentials",
		Namespace: "default",
	}, jiraSecret)
	require.NoError(t, err)

	// Backup original data
	originalData := jiraSecret.Data["apiToken"]

	// Corrupt the API token
	jiraSecret.Data["apiToken"] = []byte("invalid-token")
	err = k8sClient.Update(ctx, jiraSecret)
	require.NoError(t, err)

	// Wait for error condition
	require.Eventually(t, func() bool {
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      jiracdc.Name,
			Namespace: jiracdc.Namespace,
		}, jiracdc)
		if err != nil {
			return false
		}
		
		// Check for error condition
		for _, condition := range jiracdc.Status.Conditions {
			if condition.Type == "Error" && condition.Status == "True" {
				return true
			}
		}
		return false
	}, 3*time.Minute, pollInterval, "Should detect authentication error")

	// Restore credentials
	jiraSecret.Data["apiToken"] = originalData
	err = k8sClient.Update(ctx, jiraSecret)
	require.NoError(t, err)

	// Wait for recovery
	require.Eventually(t, func() bool {
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      jiracdc.Name,
			Namespace: jiracdc.Namespace,
		}, jiracdc)
		if err != nil {
			return false
		}
		return jiracdc.Status.Phase == "Current"
	}, 3*time.Minute, pollInterval, "Should recover from error")
}

func testCleanup(t *testing.T, ctx context.Context, k8sClient client.Client, jiracdc *jiracdc.JiraCDC) {
	// Delete JiraCDC resource
	err := k8sClient.Delete(ctx, jiracdc)
	require.NoError(t, err)

	// Verify operands are cleaned up
	require.Eventually(t, func() bool {
		apiDeployment := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      "jiracdc-api-" + jiracdc.Name,
			Namespace: jiracdc.Namespace,
		}, apiDeployment)
		return errors.IsNotFound(err)
	}, 2*time.Minute, pollInterval, "API deployment should be deleted")

	require.Eventually(t, func() bool {
		uiDeployment := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      "jiracdc-ui-" + jiracdc.Name,
			Namespace: jiracdc.Namespace,
		}, uiDeployment)
		return errors.IsNotFound(err)
	}, 2*time.Minute, pollInterval, "UI deployment should be deleted")

	// Clean up secrets
	jiraSecret := &corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      "jira-credentials",
		Namespace: "default",
	}, jiraSecret)
	if err == nil {
		k8sClient.Delete(ctx, jiraSecret)
	}

	gitSecret := &corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      "git-credentials",
		Namespace: "default",
	}, gitSecret)
	if err == nil {
		k8sClient.Delete(ctx, gitSecret)
	}
}
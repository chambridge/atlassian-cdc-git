package integration

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
	"github.com/company/jira-cdc-operator/internal/agent"
	gitops "github.com/company/jira-cdc-operator/internal/git"
)

type AgentIntegrationTestSuite struct {
	suite.Suite
	ctx              context.Context
	k8sClient        client.Client
	namespace        string
	tempDir          string
	mainRepoDir      string
	agentRepoDir     string
	agentSubmodule   *agent.Submodule
}

func (suite *AgentIntegrationTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.namespace = "jiracdc-agent-test"
	
	// This will fail until proper k8s client setup for integration tests
	suite.k8sClient = nil // Will be set up when integration test infrastructure is ready
	
	// Create temporary directory for test repositories
	var err error
	suite.tempDir, err = ioutil.TempDir("", "jiracdc-agent-test-")
	require.NoError(suite.T(), err, "Should create temp directory")
	
	suite.mainRepoDir = filepath.Join(suite.tempDir, "main-repo")
	suite.agentRepoDir = filepath.Join(suite.tempDir, "agent-repo")
	
	// Set up test repositories
	suite.setupTestRepositories()
}

func (suite *AgentIntegrationTestSuite) setupTestRepositories() {
	// Create main repository
	err := os.MkdirAll(suite.mainRepoDir, 0755)
	require.NoError(suite.T(), err, "Should create main repo directory")
	
	mainRepo, err := git.PlainInit(suite.mainRepoDir, false)
	require.NoError(suite.T(), err, "Should initialize main repository")
	
	// Create initial files in main repo
	mainWorktree, err := mainRepo.Worktree()
	require.NoError(suite.T(), err, "Should get main worktree")
	
	readmePath := filepath.Join(suite.mainRepoDir, "README.md")
	err = ioutil.WriteFile(readmePath, []byte("# Main Repository\n\nThis repository contains JIRA issues and agent submodule.\n"), 0644)
	require.NoError(suite.T(), err, "Should create main README")
	
	_, err = mainWorktree.Add("README.md")
	require.NoError(suite.T(), err, "Should add main README")
	
	// Create agent repository
	err = os.MkdirAll(suite.agentRepoDir, 0755)
	require.NoError(suite.T(), err, "Should create agent repo directory")
	
	agentRepo, err := git.PlainInit(suite.agentRepoDir, false)
	require.NoError(suite.T(), err, "Should initialize agent repository")
	
	agentWorktree, err := agentRepo.Worktree()
	require.NoError(suite.T(), err, "Should get agent worktree")
	
	// Create agent files
	agentReadmePath := filepath.Join(suite.agentRepoDir, "README.md")
	err = ioutil.WriteFile(agentReadmePath, []byte("# Agent Submodule\n\nThis contains agent code and configurations.\n"), 0644)
	require.NoError(suite.T(), err, "Should create agent README")
	
	agentConfigPath := filepath.Join(suite.agentRepoDir, "agent-config.yaml")
	agentConfig := `apiVersion: v1
kind: AgentConfig
metadata:
  name: jira-cdc-agent
spec:
  capabilities:
    - jira-issue-processing
    - git-operations
    - task-automation
  tools:
    - jira-cli
    - git
    - kubectl
  permissions:
    - read-issues
    - write-git
    - update-status
  configuration:
    max-concurrent-tasks: 5
    timeout: 30m
    retry-attempts: 3
`
	err = ioutil.WriteFile(agentConfigPath, []byte(agentConfig), 0644)
	require.NoError(suite.T(), err, "Should create agent config")
	
	_, err = agentWorktree.Add(".")
	require.NoError(suite.T(), err, "Should add agent files")
	
	// Create commits
	suite.createInitialCommits(mainWorktree, agentWorktree)
}

func (suite *AgentIntegrationTestSuite) createInitialCommits(mainWorktree, agentWorktree *git.Worktree) {
	// Commit to agent repo first
	_, err := agentWorktree.Commit("Initial agent commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(suite.T(), err, "Should create agent commit")
	
	// Commit to main repo
	_, err = mainWorktree.Commit("Initial main commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(suite.T(), err, "Should create main commit")
}

func (suite *AgentIntegrationTestSuite) TestAgentSubmoduleCreation() {
	suite.T().Skip("This test will fail until agent submodule implementation is ready")
	
	// Test creating agent submodule manager
	config := agent.SubmoduleConfig{
		MainRepositoryPath:  suite.mainRepoDir,
		AgentRepositoryURL:  fmt.Sprintf("file://%s", suite.agentRepoDir),
		SubmodulePath:       "agents/jira-cdc",
		Branch:              "main",
		CredentialsSecret:   "agent-git-creds",
		Namespace:           suite.namespace,
	}
	
	var err error
	suite.agentSubmodule, err = agent.NewSubmodule(suite.ctx, suite.k8sClient, config)
	require.NoError(suite.T(), err, "Should create agent submodule")
	assert.NotNil(suite.T(), suite.agentSubmodule, "Agent submodule should not be nil")
}

func (suite *AgentIntegrationTestSuite) TestAgentSubmoduleInitialization() {
	suite.T().Skip("This test will fail until agent submodule initialization is implemented")
	
	// Test initializing agent submodule
	err := suite.agentSubmodule.Initialize(suite.ctx)
	require.NoError(suite.T(), err, "Should initialize agent submodule")
	
	// Verify submodule was added to main repository
	submodulePath := filepath.Join(suite.mainRepoDir, "agents", "jira-cdc")
	assert.DirExists(suite.T(), submodulePath, "Agent submodule directory should exist")
	
	// Verify .gitmodules file was created
	gitmodulesPath := filepath.Join(suite.mainRepoDir, ".gitmodules")
	assert.FileExists(suite.T(), gitmodulesPath, ".gitmodules file should exist")
	
	// Verify .gitmodules content
	content, err := ioutil.ReadFile(gitmodulesPath)
	require.NoError(suite.T(), err, "Should read .gitmodules")
	
	contentStr := string(content)
	assert.Contains(suite.T(), contentStr, "[submodule \"agents/jira-cdc\"]", "Should contain submodule definition")
	assert.Contains(suite.T(), contentStr, fmt.Sprintf("url = file://%s", suite.agentRepoDir), "Should contain correct URL")
}

func (suite *AgentIntegrationTestSuite) TestAgentSubmoduleUpdate() {
	suite.T().Skip("This test will fail until agent submodule updates are implemented")
	
	// First initialize the submodule
	suite.TestAgentSubmoduleInitialization()
	
	// Make changes to agent repository
	agentRepo, err := git.PlainOpen(suite.agentRepoDir)
	require.NoError(suite.T(), err, "Should open agent repository")
	
	agentWorktree, err := agentRepo.Worktree()
	require.NoError(suite.T(), err, "Should get agent worktree")
	
	// Add new agent capability
	newCapabilityPath := filepath.Join(suite.agentRepoDir, "capabilities", "new-feature.yaml")
	err = os.MkdirAll(filepath.Dir(newCapabilityPath), 0755)
	require.NoError(suite.T(), err, "Should create capabilities directory")
	
	newCapability := `apiVersion: v1
kind: AgentCapability
metadata:
  name: new-feature
spec:
  description: "New feature for testing submodule updates"
  commands:
    - name: "process-feature"
      description: "Process the new feature"
      script: "echo 'Processing new feature'"
`
	err = ioutil.WriteFile(newCapabilityPath, []byte(newCapability), 0644)
	require.NoError(suite.T(), err, "Should create new capability file")
	
	_, err = agentWorktree.Add("capabilities/new-feature.yaml")
	require.NoError(suite.T(), err, "Should add new capability")
	
	_, err = agentWorktree.Commit("Add new feature capability", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(suite.T(), err, "Should commit new capability")
	
	// Update submodule to latest
	err = suite.agentSubmodule.UpdateToLatest(suite.ctx)
	require.NoError(suite.T(), err, "Should update submodule")
	
	// Verify submodule has new content
	submoduleCapabilityPath := filepath.Join(suite.mainRepoDir, "agents", "jira-cdc", "capabilities", "new-feature.yaml")
	assert.FileExists(suite.T(), submoduleCapabilityPath, "New capability should exist in submodule")
}

func (suite *AgentIntegrationTestSuite) TestAgentConfigurationAccess() {
	suite.T().Skip("This test will fail until agent configuration access is implemented")
	
	// Initialize submodule
	suite.TestAgentSubmoduleInitialization()
	
	// Test accessing agent configuration
	config, err := suite.agentSubmodule.GetConfiguration(suite.ctx)
	require.NoError(suite.T(), err, "Should get agent configuration")
	
	assert.Equal(suite.T(), "jira-cdc-agent", config.Name, "Should have correct agent name")
	assert.Contains(suite.T(), config.Capabilities, "jira-issue-processing", "Should have JIRA processing capability")
	assert.Contains(suite.T(), config.Capabilities, "git-operations", "Should have git operations capability")
	assert.Contains(suite.T(), config.Tools, "jira-cli", "Should have JIRA CLI tool")
	assert.Contains(suite.T(), config.Tools, "git", "Should have git tool")
	
	// Test configuration validation
	err = suite.agentSubmodule.ValidateConfiguration(config)
	assert.NoError(suite.T(), err, "Configuration should be valid")
}

func (suite *AgentIntegrationTestSuite) TestAgentCapabilityDiscovery() {
	suite.T().Skip("This test will fail until agent capability discovery is implemented")
	
	// Initialize submodule
	suite.TestAgentSubmoduleUpdate() // This includes adding a new capability
	
	// Test discovering agent capabilities
	capabilities, err := suite.agentSubmodule.DiscoverCapabilities(suite.ctx)
	require.NoError(suite.T(), err, "Should discover capabilities")
	
	assert.Greater(suite.T(), len(capabilities), 0, "Should find capabilities")
	
	// Look for the new capability we added
	var foundNewFeature bool
	for _, capability := range capabilities {
		if capability.Name == "new-feature" {
			foundNewFeature = true
			assert.Equal(suite.T(), "New feature for testing submodule updates", capability.Description)
			assert.Contains(suite.T(), capability.Commands, "process-feature")
		}
	}
	assert.True(suite.T(), foundNewFeature, "Should find new-feature capability")
}

func (suite *AgentIntegrationTestSuite) TestAgentExecutionEnvironment() {
	suite.T().Skip("This test will fail until agent execution environment is implemented")
	
	// Test setting up agent execution environment
	execEnv, err := suite.agentSubmodule.CreateExecutionEnvironment(suite.ctx)
	require.NoError(suite.T(), err, "Should create execution environment")
	
	// Verify environment has access to agent tools
	tools, err := execEnv.GetAvailableTools()
	require.NoError(suite.T(), err, "Should get available tools")
	
	assert.Contains(suite.T(), tools, "jira-cli", "Should have JIRA CLI available")
	assert.Contains(suite.T(), tools, "git", "Should have git available")
	assert.Contains(suite.T(), tools, "kubectl", "Should have kubectl available")
	
	// Test tool execution
	result, err := execEnv.ExecuteTool(suite.ctx, "git", []string{"--version"})
	assert.NoError(suite.T(), err, "Should execute git command")
	assert.Contains(suite.T(), result.Output, "git version", "Should return git version")
	assert.Equal(suite.T(), 0, result.ExitCode, "Should exit successfully")
}

func (suite *AgentIntegrationTestSuite) TestAgentTaskExecution() {
	suite.T().Skip("This test will fail until agent task execution is implemented")
	
	// Test executing agent tasks
	suite.TestAgentCapabilityDiscovery()
	
	// Create a test task
	task := agent.Task{
		ID:         "test-task-001",
		Type:       "jira-issue-processing",
		Capability: "process-feature",
		Parameters: map[string]interface{}{
			"issueKey":    "TEST-123",
			"action":      "update-status",
			"newStatus":   "In Progress",
		},
		Context: agent.TaskContext{
			JiraCDCName:      "test-jiracdc",
			JiraCDCNamespace: suite.namespace,
			IssueKey:        "TEST-123",
			ProjectKey:      "TEST",
		},
	}
	
	// Execute the task
	result, err := suite.agentSubmodule.ExecuteTask(suite.ctx, task)
	require.NoError(suite.T(), err, "Should execute task")
	
	assert.Equal(suite.T(), "completed", result.Status, "Task should complete successfully")
	assert.NotEmpty(suite.T(), result.Output, "Should have task output")
	assert.Greater(suite.T(), result.Duration, time.Duration(0), "Should have execution duration")
}

func (suite *AgentIntegrationTestSuite) TestAgentIntegrationWithJiraCDC() {
	suite.T().Skip("This test will fail until full JiraCDC agent integration is implemented")
	
	// Test agent integration within JiraCDC context
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-git-creds",
			Namespace: suite.namespace,
		},
		Data: map[string][]byte{
			"ssh-privatekey": []byte("-----BEGIN OPENSSH PRIVATE KEY-----\ntest-key\n-----END OPENSSH PRIVATE KEY-----"),
			"known_hosts":    []byte("github.com ssh-rsa AAAAB..."),
		},
	}
	
	err := suite.k8sClient.Create(suite.ctx, secret)
	require.NoError(suite.T(), err, "Should create agent credentials")
	
	// Create JiraCDC with agent configuration
	jiracdc := &jiradcdv1.JiraCDC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "agent-integration-test",
			Namespace: suite.namespace,
		},
		Spec: jiradcdv1.JiraCDCSpec{
			JiraInstance: jiradcdv1.JiraInstanceConfig{
				BaseURL:           "https://test.atlassian.net",
				CredentialsSecret: "jira-creds",
			},
			SyncTarget: jiradcdv1.SyncTargetConfig{
				Type:       "project",
				ProjectKey: "TEST",
			},
			GitRepository: jiradcdv1.GitRepositoryConfig{
				URL:               fmt.Sprintf("file://%s", suite.mainRepoDir),
				CredentialsSecret: "git-creds",
				Branch:            "main",
			},
			AgentConfig: &jiradcdv1.AgentConfig{
				Enabled: true,
				Submodule: jiradcdv1.AgentSubmoduleConfig{
					URL:               fmt.Sprintf("file://%s", suite.agentRepoDir),
					Path:              "agents/jira-cdc",
					CredentialsSecret: "agent-git-creds",
					AutoUpdate:        true,
				},
				Capabilities: []string{
					"jira-issue-processing",
					"git-operations",
					"task-automation",
				},
			},
		},
	}
	
	err = suite.k8sClient.Create(suite.ctx, jiracdc)
	require.NoError(suite.T(), err, "Should create JiraCDC with agent config")
	
	// Wait for controller to initialize agent submodule
	time.Sleep(10 * time.Second)
	
	// Check that agent is properly initialized
	updated := &jiradcdv1.JiraCDC{}
	err = suite.k8sClient.Get(suite.ctx, client.ObjectKeyFromObject(jiracdc), updated)
	require.NoError(suite.T(), err, "Should get updated JiraCDC")
	
	assert.True(suite.T(), updated.Status.AgentStatus.Enabled, "Agent should be enabled")
	assert.Equal(suite.T(), "healthy", updated.Status.AgentStatus.Status, "Agent should be healthy")
	assert.NotEmpty(suite.T(), updated.Status.AgentStatus.Version, "Should have agent version")
	assert.Greater(suite.T(), len(updated.Status.AgentStatus.AvailableCapabilities), 0, "Should have capabilities")
}

func (suite *AgentIntegrationTestSuite) TestAgentSubmoduleVersioning() {
	suite.T().Skip("This test will fail until agent versioning is implemented")
	
	// Test agent submodule versioning and pinning
	suite.TestAgentSubmoduleInitialization()
	
	// Get current version
	currentVersion, err := suite.agentSubmodule.GetCurrentVersion(suite.ctx)
	require.NoError(suite.T(), err, "Should get current version")
	assert.NotEmpty(suite.T(), currentVersion, "Should have current version")
	
	// Pin to specific version
	err = suite.agentSubmodule.PinToVersion(suite.ctx, currentVersion)
	require.NoError(suite.T(), err, "Should pin to version")
	
	// Make new changes to agent repo
	suite.TestAgentSubmoduleUpdate()
	
	// Verify submodule is still pinned to old version
	pinnedVersion, err := suite.agentSubmodule.GetCurrentVersion(suite.ctx)
	require.NoError(suite.T(), err, "Should get pinned version")
	assert.Equal(suite.T(), currentVersion, pinnedVersion, "Should still be on pinned version")
	
	// Update to latest
	err = suite.agentSubmodule.UpdateToLatest(suite.ctx)
	require.NoError(suite.T(), err, "Should update to latest")
	
	// Verify version changed
	latestVersion, err := suite.agentSubmodule.GetCurrentVersion(suite.ctx)
	require.NoError(suite.T(), err, "Should get latest version")
	assert.NotEqual(suite.T(), currentVersion, latestVersion, "Should be on new version")
}

func (suite *AgentIntegrationTestSuite) TestAgentSubmoduleSecurity() {
	suite.T().Skip("This test will fail until agent security features are implemented")
	
	// Test agent security and sandboxing
	suite.TestAgentSubmoduleInitialization()
	
	// Test capability restrictions
	securityConfig := agent.SecurityConfig{
		AllowedCapabilities: []string{"jira-issue-processing"},
		DeniedCapabilities:  []string{"git-operations"}, // Deny git operations for this test
		ResourceLimits: agent.ResourceLimits{
			MaxMemory:    "128Mi",
			MaxCPU:       "100m",
			MaxDuration:  time.Minute * 5,
		},
		NetworkPolicy: agent.NetworkPolicy{
			AllowOutbound: false,
			AllowedHosts:  []string{"jira.example.com"},
		},
	}
	
	err := suite.agentSubmodule.ApplySecurityConfig(suite.ctx, securityConfig)
	require.NoError(suite.T(), err, "Should apply security config")
	
	// Test that allowed capability works
	allowedTask := agent.Task{
		Type:       "jira-issue-processing",
		Capability: "process-feature",
		Parameters: map[string]interface{}{"issueKey": "TEST-123"},
	}
	
	result, err := suite.agentSubmodule.ExecuteTask(suite.ctx, allowedTask)
	assert.NoError(suite.T(), err, "Allowed capability should work")
	assert.Equal(suite.T(), "completed", result.Status, "Allowed task should complete")
	
	// Test that denied capability is blocked
	deniedTask := agent.Task{
		Type:       "git-operations",
		Capability: "commit-changes",
		Parameters: map[string]interface{}{"message": "test commit"},
	}
	
	_, err = suite.agentSubmodule.ExecuteTask(suite.ctx, deniedTask)
	assert.Error(suite.T(), err, "Denied capability should be blocked")
	assert.Contains(suite.T(), err.Error(), "capability not allowed", "Should indicate capability restriction")
}

func (suite *AgentIntegrationTestSuite) TearDownTest() {
	// Clean up after each test
	if suite.agentSubmodule != nil {
		suite.agentSubmodule.Cleanup(suite.ctx)
	}
}

func (suite *AgentIntegrationTestSuite) TearDownSuite() {
	// Clean up temporary directory
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
	suite.T().Log("Tearing down agent integration test suite")
}

// Test runner function
func TestAgentIntegration(t *testing.T) {
	// This will fail until the integration test environment is set up
	t.Skip("Skipping agent integration tests until test environment and agent submodule are ready")
	
	suite.Run(t, new(AgentIntegrationTestSuite))
}
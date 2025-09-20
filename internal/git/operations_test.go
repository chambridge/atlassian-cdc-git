package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperations_CloneRepository(t *testing.T) {
	// Create a temporary directory for test repository
	tempDir, err := os.MkdirTemp("", "git-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize a bare repository for testing
	bareRepo, err := git.PlainInit(tempDir, true)
	require.NoError(t, err)

	// Create operations instance
	ops, err := NewOperations(Config{
		AuthMethod: "none",
		CommitAuthor: CommitAuthor{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err)

	// Test cloning
	cloneDir, err := os.MkdirTemp("", "git-clone-*")
	require.NoError(t, err)
	defer os.RemoveAll(cloneDir)

	ctx := context.Background()
	repo, err := ops.CloneRepository(ctx, tempDir, cloneDir, "main")
	require.NoError(t, err)
	assert.NotNil(t, repo)

	// Verify repository exists
	_, err = os.Stat(filepath.Join(cloneDir, ".git"))
	assert.NoError(t, err)
}

func TestOperations_CreateOrUpdateIssueFile(t *testing.T) {
	// Create a test repository in memory
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	// Create operations instance
	ops, err := NewOperations(Config{
		AuthMethod: "none",
		CommitAuthor: CommitAuthor{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err)

	// Create test issue data
	issueData := IssueData{
		Key:         "PROJ-123",
		Summary:     "Test Issue",
		Description: "This is a test issue",
		Status:      "Open",
		IssueType:   "Bug",
		Priority:    "High",
		Assignee:    "john.doe@example.com",
		Reporter:    "jane.smith@example.com",
		Labels:      []string{"bug", "urgent"},
		Components:  []string{"backend"},
		FixVersions: []string{"1.0.0"},
		UpdatedAt:   time.Now(),
	}

	ctx := context.Background()
	
	// Test creating new file
	filePath, commitHash, err := ops.CreateOrUpdateIssueFile(ctx, repo, issueData)
	require.NoError(t, err)
	assert.Equal(t, "PROJ-123.md", filePath)
	assert.NotEmpty(t, commitHash)

	// Test updating existing file
	issueData.Summary = "Updated Test Issue"
	filePath2, commitHash2, err := ops.CreateOrUpdateIssueFile(ctx, repo, issueData)
	require.NoError(t, err)
	assert.Equal(t, filePath, filePath2)
	assert.NotEqual(t, commitHash, commitHash2)
}

func TestOperations_DeleteIssueFile(t *testing.T) {
	// Create a test repository in memory
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	// Create operations instance
	ops, err := NewOperations(Config{
		AuthMethod: "none",
		CommitAuthor: CommitAuthor{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err)

	// First create a file
	issueData := IssueData{
		Key:       "PROJ-123",
		Summary:   "Test Issue",
		Status:    "Done",
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	filePath, _, err := ops.CreateOrUpdateIssueFile(ctx, repo, issueData)
	require.NoError(t, err)

	// Then delete it
	commitHash, err := ops.DeleteIssueFile(ctx, repo, "PROJ-123")
	require.NoError(t, err)
	assert.NotEmpty(t, commitHash)
}

func TestOperations_GenerateCommitMessage(t *testing.T) {
	ops, err := NewOperations(Config{
		AuthMethod: "none",
		CommitAuthor: CommitAuthor{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		issueKey  string
		operation string
		summary   string
		expected  string
	}{
		{
			name:      "create operation",
			issueKey:  "PROJ-123",
			operation: "create",
			summary:   "Test Issue",
			expected:  "feat(PROJ-123): add Test Issue",
		},
		{
			name:      "update operation",
			issueKey:  "PROJ-456",
			operation: "update",
			summary:   "Updated Issue",
			expected:  "feat(PROJ-456): update Updated Issue",
		},
		{
			name:      "delete operation",
			issueKey:  "PROJ-789",
			operation: "delete",
			summary:   "Deleted Issue",
			expected:  "feat(PROJ-789): remove Deleted Issue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := ops.GenerateCommitMessage(tt.issueKey, tt.operation, tt.summary)
			assert.Equal(t, tt.expected, message)
		})
	}
}

func TestOperations_FormatIssueContent(t *testing.T) {
	ops, err := NewOperations(Config{
		AuthMethod: "none",
		CommitAuthor: CommitAuthor{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err)

	issueData := IssueData{
		Key:         "PROJ-123",
		Summary:     "Test Issue",
		Description: "This is a test issue\nwith multiple lines",
		Status:      "In Progress",
		IssueType:   "Story",
		Priority:    "Medium",
		Assignee:    "john.doe@example.com",
		Reporter:    "jane.smith@example.com",
		Labels:      []string{"feature", "backend"},
		Components:  []string{"api", "database"},
		FixVersions: []string{"2.0.0"},
		ParentKey:   "PROJ-100",
		UpdatedAt:   time.Date(2025, 9, 19, 14, 30, 0, 0, time.UTC),
	}

	content := ops.FormatIssueContent(issueData)

	// Verify YAML frontmatter exists
	assert.Contains(t, content, "---")
	assert.Contains(t, content, "key: PROJ-123")
	assert.Contains(t, content, "summary: Test Issue")
	assert.Contains(t, content, "status: In Progress")
	assert.Contains(t, content, "issueType: Story")
	assert.Contains(t, content, "priority: Medium")
	assert.Contains(t, content, "assignee: john.doe@example.com")
	assert.Contains(t, content, "reporter: jane.smith@example.com")
	assert.Contains(t, content, "parentKey: PROJ-100")
	assert.Contains(t, content, "syncedAt:")

	// Verify arrays are formatted correctly
	assert.Contains(t, content, "labels:\n  - feature\n  - backend")
	assert.Contains(t, content, "components:\n  - api\n  - database")
	assert.Contains(t, content, "fixVersions:\n  - 2.0.0")

	// Verify description is in markdown content
	assert.Contains(t, content, "This is a test issue")
	assert.Contains(t, content, "with multiple lines")
}

func TestOperations_PushChanges(t *testing.T) {
	// Create operations instance
	ops, err := NewOperations(Config{
		AuthMethod: "none",
		CommitAuthor: CommitAuthor{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err)

	// Create a test repository in memory
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	// Add a remote (this would normally fail without real remote, but we test the logic)
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{"https://github.com/test/repo.git"},
	})
	require.NoError(t, err)

	ctx := context.Background()
	
	// This will fail due to no real remote, but we test that the method handles it gracefully
	err = ops.PushChanges(ctx, repo, "main")
	assert.Error(t, err) // Expected to fail with memory storage and fake remote
}

func TestOperations_SSHKeyAuth(t *testing.T) {
	// Create a temporary SSH key file
	tempDir, err := os.MkdirTemp("", "ssh-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	keyFile := filepath.Join(tempDir, "test_key")
	err = os.WriteFile(keyFile, []byte("-----BEGIN OPENSSH PRIVATE KEY-----\nfake-key-content\n-----END OPENSSH PRIVATE KEY-----"), 0600)
	require.NoError(t, err)

	config := Config{
		AuthMethod: "ssh",
		SSHKeyPath: keyFile,
		CommitAuthor: CommitAuthor{
			Name:  "Test User",
			Email: "test@example.com",
		},
	}

	ops, err := NewOperations(config)
	require.NoError(t, err)
	assert.NotNil(t, ops)
}

func TestOperations_HTTPSAuth(t *testing.T) {
	config := Config{
		AuthMethod: "https",
		Username:   "testuser",
		Token:      "testtoken",
		CommitAuthor: CommitAuthor{
			Name:  "Test User",
			Email: "test@example.com",
		},
	}

	ops, err := NewOperations(config)
	require.NoError(t, err)
	assert.NotNil(t, ops)
}

func TestOperations_InvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "invalid auth method",
			config: Config{
				AuthMethod: "invalid",
			},
		},
		{
			name: "ssh without key path",
			config: Config{
				AuthMethod: "ssh",
				SSHKeyPath: "",
			},
		},
		{
			name: "https without credentials",
			config: Config{
				AuthMethod: "https",
				Username:   "",
				Token:      "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops, err := NewOperations(tt.config)
			assert.Error(t, err)
			assert.Nil(t, ops)
		})
	}
}

func TestOperations_GetRepositoryStatus(t *testing.T) {
	// Create a test repository in memory
	repo, err := git.Init(memory.NewStorage(), nil)
	require.NoError(t, err)

	ops, err := NewOperations(Config{
		AuthMethod: "none",
		CommitAuthor: CommitAuthor{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err)

	// Create some test content
	issueData := IssueData{
		Key:       "PROJ-123",
		Summary:   "Test Issue",
		Status:    "Open",
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	_, _, err = ops.CreateOrUpdateIssueFile(ctx, repo, issueData)
	require.NoError(t, err)

	// Get repository status
	head, err := repo.Head()
	require.NoError(t, err)

	commit, err := repo.CommitObject(head.Hash())
	require.NoError(t, err)

	assert.Equal(t, "Test User", commit.Author.Name)
	assert.Equal(t, "test@example.com", commit.Author.Email)
	assert.Contains(t, commit.Message, "PROJ-123")
}
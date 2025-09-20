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

package git

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Config represents the configuration for git operations
type Config struct {
	RepositoryURL     string
	Branch            string
	CredentialsSecret string
	Namespace         string
	WorkingDirectory  string
}

// Manager handles git repository operations
type Manager struct {
	config    Config
	k8sClient client.Client
	repo      *git.Repository
	worktree  *git.Worktree
	auth      transport.AuthMethod
}

// IssueData represents issue data for git file creation
type IssueData struct {
	Key         string
	Summary     string
	Description string
	Status      string
	Assignee    string
	Reporter    string
	Created     time.Time
	Updated     time.Time
	Labels      []string
	Components  []string
	Priority    string
}

// CommitInfo represents commit information
type CommitInfo struct {
	Message string
	Author  AuthorInfo
}

// AuthorInfo represents commit author information
type AuthorInfo struct {
	Name  string
	Email string
}

// ConflictResolutionStrategy represents conflict resolution strategy
type ConflictResolutionStrategy struct {
	Strategy string // "prefer-jira", "prefer-git", "manual"
}

// ConflictInfo represents information about a conflict
type ConflictInfo struct {
	FilePath    string
	IssueKey    string
	ConflictType string
	Description string
}

// NewManager creates a new git operations manager
func NewManager(ctx context.Context, k8sClient client.Client, config Config) (*Manager, error) {
	manager := &Manager{
		config:    config,
		k8sClient: k8sClient,
	}

	// Load authentication
	if err := manager.loadAuth(ctx); err != nil {
		return nil, fmt.Errorf("failed to load authentication: %w", err)
	}

	return manager, nil
}

// loadAuth loads git authentication from Kubernetes secret
func (m *Manager) loadAuth(ctx context.Context) error {
	var secret corev1.Secret
	secretKey := types.NamespacedName{
		Name:      m.config.CredentialsSecret,
		Namespace: m.config.Namespace,
	}

	if err := m.k8sClient.Get(ctx, secretKey, &secret); err != nil {
		return fmt.Errorf("failed to get credentials secret: %w", err)
	}

	// Check for SSH authentication
	if sshKey, exists := secret.Data["ssh-privatekey"]; exists {
		publicKey, err := ssh.NewPublicKeys("git", sshKey, "")
		if err != nil {
			return fmt.Errorf("failed to create SSH public key auth: %w", err)
		}
		
		// Set known hosts if provided
		if knownHosts, exists := secret.Data["known_hosts"]; exists {
			publicKey.HostKeyCallback = ssh.HostKeyCallbackHelper{KnownHosts: string(knownHosts)}
		}
		
		m.auth = publicKey
		return nil
	}

	// Check for HTTPS authentication
	if username, userExists := secret.Data["username"]; userExists {
		if password, passExists := secret.Data["password"]; passExists {
			m.auth = &http.BasicAuth{
				Username: string(username),
				Password: string(password),
			}
			return nil
		}
	}

	return fmt.Errorf("no valid authentication method found in secret")
}

// Clone clones the repository to the working directory
func (m *Manager) Clone(ctx context.Context) error {
	// Ensure working directory exists
	if err := os.MkdirAll(m.config.WorkingDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create working directory: %w", err)
	}

	// Clone repository
	repo, err := git.PlainCloneContext(ctx, m.config.WorkingDirectory, false, &git.CloneOptions{
		URL:           m.config.RepositoryURL,
		Auth:          m.auth,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", m.config.Branch)),
		SingleBranch:  true,
		Progress:      os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	m.repo = repo

	// Get worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	m.worktree = worktree
	return nil
}

// Pull pulls the latest changes from the remote repository
func (m *Manager) Pull(ctx context.Context) error {
	if m.worktree == nil {
		return fmt.Errorf("repository not initialized, call Clone first")
	}

	err := m.worktree.PullContext(ctx, &git.PullOptions{
		Auth:          m.auth,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", m.config.Branch)),
		SingleBranch:  true,
		Progress:      os.Stdout,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to pull repository: %w", err)
	}

	return nil
}

// Push pushes changes to the remote repository
func (m *Manager) Push(ctx context.Context) error {
	if m.repo == nil {
		return fmt.Errorf("repository not initialized, call Clone first")
	}

	err := m.repo.PushContext(ctx, &git.PushOptions{
		Auth:     m.auth,
		Progress: os.Stdout,
	})

	if err != nil {
		return fmt.Errorf("failed to push repository: %w", err)
	}

	return nil
}

// CreateIssueFile creates a markdown file for a JIRA issue
func (m *Manager) CreateIssueFile(ctx context.Context, issue IssueData) error {
	if m.worktree == nil {
		return fmt.Errorf("repository not initialized, call Clone first")
	}

	// Generate file content
	content := m.generateIssueMarkdown(issue)

	// Write file
	filePath := fmt.Sprintf("%s.md", issue.Key)
	fullPath := filepath.Join(m.config.WorkingDirectory, filePath)

	if err := ioutil.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write issue file: %w", err)
	}

	// Add file to git
	_, err := m.worktree.Add(filePath)
	if err != nil {
		return fmt.Errorf("failed to add file to git: %w", err)
	}

	return nil
}

// UpdateIssueFile updates an existing issue file
func (m *Manager) UpdateIssueFile(ctx context.Context, issue IssueData) error {
	// Same as CreateIssueFile for now, as it overwrites the file
	return m.CreateIssueFile(ctx, issue)
}

// CommitChanges commits all staged changes
func (m *Manager) CommitChanges(ctx context.Context, commitInfo CommitInfo) (string, error) {
	if m.worktree == nil {
		return "", fmt.Errorf("repository not initialized, call Clone first")
	}

	// Create commit
	hash, err := m.worktree.Commit(commitInfo.Message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  commitInfo.Author.Name,
			Email: commitInfo.Author.Email,
			When:  time.Now(),
		},
	})

	if err != nil {
		return "", fmt.Errorf("failed to commit changes: %w", err)
	}

	return hash.String(), nil
}

// CreateBranch creates a new branch
func (m *Manager) CreateBranch(ctx context.Context, branchName string) error {
	if m.repo == nil {
		return fmt.Errorf("repository not initialized, call Clone first")
	}

	// Get HEAD reference
	headRef, err := m.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	// Create new branch reference
	refName := plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branchName))
	ref := plumbing.NewHashReference(refName, headRef.Hash())

	err = m.repo.Storer.SetReference(ref)
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	return nil
}

// CheckoutBranch checks out a specific branch
func (m *Manager) CheckoutBranch(ctx context.Context, branchName string) error {
	if m.worktree == nil {
		return fmt.Errorf("repository not initialized, call Clone first")
	}

	err := m.worktree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branchName)),
	})

	if err != nil {
		return fmt.Errorf("failed to checkout branch: %w", err)
	}

	return nil
}

// GetCurrentBranch returns the current branch name
func (m *Manager) GetCurrentBranch(ctx context.Context) (string, error) {
	if m.repo == nil {
		return "", fmt.Errorf("repository not initialized, call Clone first")
	}

	headRef, err := m.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	branchName := headRef.Name().Short()
	return branchName, nil
}

// GetWorkingDirectory returns the working directory path
func (m *Manager) GetWorkingDirectory() string {
	return m.config.WorkingDirectory
}

// GetSSHConfig returns SSH configuration (placeholder)
func (m *Manager) GetSSHConfig(ctx context.Context) (interface{}, error) {
	// This is a placeholder for SSH configuration
	return map[string]string{"type": "ssh"}, nil
}

// RefreshCredentials refreshes the authentication credentials
func (m *Manager) RefreshCredentials(ctx context.Context) error {
	return m.loadAuth(ctx)
}

// DetectConflicts detects merge conflicts (placeholder)
func (m *Manager) DetectConflicts(ctx context.Context) ([]ConflictInfo, error) {
	// This is a placeholder for conflict detection
	return []ConflictInfo{}, nil
}

// ResolveConflicts resolves merge conflicts (placeholder)
func (m *Manager) ResolveConflicts(ctx context.Context, conflicts []ConflictInfo) error {
	// This is a placeholder for conflict resolution
	return nil
}

// SetConflictResolutionStrategy sets the conflict resolution strategy
func (m *Manager) SetConflictResolutionStrategy(strategy ConflictResolutionStrategy) error {
	// This is a placeholder for setting conflict resolution strategy
	return nil
}

// Cleanup cleans up resources
func (m *Manager) Cleanup(ctx context.Context) error {
	// Close repository if needed
	if m.repo != nil {
		// go-git doesn't have explicit cleanup, but we can clear references
		m.repo = nil
		m.worktree = nil
	}
	return nil
}

// generateIssueMarkdown generates markdown content for a JIRA issue
func (m *Manager) generateIssueMarkdown(issue IssueData) string {
	var content strings.Builder

	// YAML frontmatter
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("issueKey: %s\n", issue.Key))
	content.WriteString(fmt.Sprintf("summary: \"%s\"\n", strings.ReplaceAll(issue.Summary, "\"", "\\\"")))
	content.WriteString(fmt.Sprintf("status: %s\n", issue.Status))
	content.WriteString(fmt.Sprintf("assignee: %s\n", issue.Assignee))
	content.WriteString(fmt.Sprintf("reporter: %s\n", issue.Reporter))
	content.WriteString(fmt.Sprintf("priority: %s\n", issue.Priority))
	content.WriteString(fmt.Sprintf("created: %s\n", issue.Created.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("updated: %s\n", issue.Updated.Format(time.RFC3339)))
	
	if len(issue.Labels) > 0 {
		content.WriteString("labels:\n")
		for _, label := range issue.Labels {
			content.WriteString(fmt.Sprintf("  - %s\n", label))
		}
	}
	
	if len(issue.Components) > 0 {
		content.WriteString("components:\n")
		for _, component := range issue.Components {
			content.WriteString(fmt.Sprintf("  - %s\n", component))
		}
	}
	
	content.WriteString("---\n\n")

	// Markdown content
	content.WriteString(fmt.Sprintf("# %s: %s\n\n", issue.Key, issue.Summary))
	content.WriteString(fmt.Sprintf("**Status:** %s\n", issue.Status))
	content.WriteString(fmt.Sprintf("**Assignee:** %s\n", issue.Assignee))
	content.WriteString(fmt.Sprintf("**Reporter:** %s\n", issue.Reporter))
	content.WriteString(fmt.Sprintf("**Priority:** %s\n", issue.Priority))
	
	if len(issue.Labels) > 0 {
		content.WriteString(fmt.Sprintf("**Labels:** %s\n", strings.Join(issue.Labels, ", ")))
	}
	
	if len(issue.Components) > 0 {
		content.WriteString(fmt.Sprintf("**Components:** %s\n", strings.Join(issue.Components, ", ")))
	}
	
	content.WriteString("\n## Description\n\n")
	content.WriteString(issue.Description)
	content.WriteString("\n\n")
	
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("*Created: %s*\n", issue.Created.Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("*Last Updated: %s*\n", issue.Updated.Format("2006-01-02 15:04:05")))

	return content.String()
}
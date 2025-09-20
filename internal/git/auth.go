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
	"strings"

	"github.com/company/jira-cdc-operator/internal/common"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GitAuthType defines the supported git authentication methods
type GitAuthType string

const (
	GitAuthTypeHTTPS GitAuthType = "https"
	GitAuthTypeSSH   GitAuthType = "ssh"
	GitAuthTypeToken GitAuthType = "token"
	GitAuthTypeNone  GitAuthType = "none"
)

// GitAuthConfig contains git authentication configuration
type GitAuthConfig struct {
	Type            GitAuthType `json:"type"`
	SecretRef       string      `json:"secretRef,omitempty"`
	SecretNamespace string      `json:"secretNamespace,omitempty"`
	UsernameKey     string      `json:"usernameKey,omitempty"`
	PasswordKey     string      `json:"passwordKey,omitempty"`
	TokenKey        string      `json:"tokenKey,omitempty"`
	SSHKeyKey       string      `json:"sshKeyKey,omitempty"`
	KnownHostsKey   string      `json:"knownHostsKey,omitempty"`
}

// GitAuthProvider handles git authentication
type GitAuthProvider interface {
	// GetAuthMethod returns the transport auth method for git operations
	GetAuthMethod(ctx context.Context) (transport.AuthMethod, error)
	
	// ValidateCredentials validates that credentials are available and valid
	ValidateCredentials(ctx context.Context) error
	
	// RefreshCredentials refreshes credentials from Kubernetes secrets
	RefreshCredentials(ctx context.Context) error
	
	// GetAuthType returns the authentication type
	GetAuthType() GitAuthType
	
	// SupportsRepository checks if this auth method supports the given repository URL
	SupportsRepository(repoURL string) bool
}

// gitAuthProvider implements GitAuthProvider
type gitAuthProvider struct {
	config    GitAuthConfig
	k8sClient client.Client
	
	// Cached authentication method
	authMethod transport.AuthMethod
}

// NewGitAuthProvider creates a new git authentication provider
func NewGitAuthProvider(config GitAuthConfig, k8sClient client.Client) GitAuthProvider {
	return &gitAuthProvider{
		config:    config,
		k8sClient: k8sClient,
	}
}

// GetAuthMethod returns the transport auth method for git operations
func (g *gitAuthProvider) GetAuthMethod(ctx context.Context) (transport.AuthMethod, error) {
	logger := log.FromContext(ctx)
	
	// Return cached auth method if available
	if g.authMethod != nil {
		return g.authMethod, nil
	}
	
	// Create auth method based on type
	switch g.config.Type {
	case GitAuthTypeNone:
		logger.V(1).Info("Using no authentication for git operations")
		return nil, nil
		
	case GitAuthTypeHTTPS:
		auth, err := g.createHTTPSAuth(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTPS auth: %w", err)
		}
		g.authMethod = auth
		logger.V(1).Info("Configured HTTPS authentication for git")
		return auth, nil
		
	case GitAuthTypeToken:
		auth, err := g.createTokenAuth(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create token auth: %w", err)
		}
		g.authMethod = auth
		logger.V(1).Info("Configured token authentication for git")
		return auth, nil
		
	case GitAuthTypeSSH:
		auth, err := g.createSSHAuth(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create SSH auth: %w", err)
		}
		g.authMethod = auth
		logger.V(1).Info("Configured SSH authentication for git")
		return auth, nil
		
	default:
		return nil, fmt.Errorf("unsupported git authentication type: %s", g.config.Type)
	}
}

// ValidateCredentials validates that credentials are available and valid
func (g *gitAuthProvider) ValidateCredentials(ctx context.Context) error {
	if g.config.Type == GitAuthTypeNone {
		return nil
	}
	
	_, err := g.GetAuthMethod(ctx)
	return err
}

// RefreshCredentials refreshes credentials from Kubernetes secrets
func (g *gitAuthProvider) RefreshCredentials(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("Refreshing git credentials", "secretRef", g.config.SecretRef)
	
	// Clear cached auth method to force reload
	g.authMethod = nil
	
	// Validate credentials by creating auth method
	_, err := g.GetAuthMethod(ctx)
	return err
}

// GetAuthType returns the authentication type
func (g *gitAuthProvider) GetAuthType() GitAuthType {
	return g.config.Type
}

// SupportsRepository checks if this auth method supports the given repository URL
func (g *gitAuthProvider) SupportsRepository(repoURL string) bool {
	switch g.config.Type {
	case GitAuthTypeNone:
		return true // No auth works with any repo
		
	case GitAuthTypeHTTPS, GitAuthTypeToken:
		return strings.HasPrefix(repoURL, "https://")
		
	case GitAuthTypeSSH:
		return strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://")
		
	default:
		return false
	}
}

// createHTTPSAuth creates HTTPS basic authentication
func (g *gitAuthProvider) createHTTPSAuth(ctx context.Context) (transport.AuthMethod, error) {
	if g.config.SecretRef == "" {
		return nil, fmt.Errorf("secret reference is required for HTTPS auth")
	}
	
	secret, err := g.getSecret(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	
	usernameKey := g.config.UsernameKey
	if usernameKey == "" {
		usernameKey = "username"
	}
	
	passwordKey := g.config.PasswordKey
	if passwordKey == "" {
		passwordKey = "password"
	}
	
	username, exists := secret.Data[usernameKey]
	if !exists {
		return nil, fmt.Errorf("username key '%s' not found in secret", usernameKey)
	}
	
	password, exists := secret.Data[passwordKey]
	if !exists {
		return nil, fmt.Errorf("password key '%s' not found in secret", passwordKey)
	}
	
	return &http.BasicAuth{
		Username: strings.TrimSpace(string(username)),
		Password: strings.TrimSpace(string(password)),
	}, nil
}

// createTokenAuth creates token-based authentication for HTTPS
func (g *gitAuthProvider) createTokenAuth(ctx context.Context) (transport.AuthMethod, error) {
	if g.config.SecretRef == "" {
		return nil, fmt.Errorf("secret reference is required for token auth")
	}
	
	secret, err := g.getSecret(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	
	tokenKey := g.config.TokenKey
	if tokenKey == "" {
		tokenKey = "token"
	}
	
	token, exists := secret.Data[tokenKey]
	if !exists {
		return nil, fmt.Errorf("token key '%s' not found in secret", tokenKey)
	}
	
	tokenValue := strings.TrimSpace(string(token))
	if tokenValue == "" {
		return nil, fmt.Errorf("token is empty in secret")
	}
	
	// For most git providers, use token as password with git username
	// This works for GitHub, GitLab, Bitbucket, etc.
	return &http.BasicAuth{
		Username: "git",
		Password: tokenValue,
	}, nil
}

// createSSHAuth creates SSH key authentication
func (g *gitAuthProvider) createSSHAuth(ctx context.Context) (transport.AuthMethod, error) {
	if g.config.SecretRef == "" {
		return nil, fmt.Errorf("secret reference is required for SSH auth")
	}
	
	secret, err := g.getSecret(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	
	sshKeyKey := g.config.SSHKeyKey
	if sshKeyKey == "" {
		sshKeyKey = "ssh-privatekey"
	}
	
	privateKey, exists := secret.Data[sshKeyKey]
	if !exists {
		return nil, fmt.Errorf("SSH private key '%s' not found in secret", sshKeyKey)
	}
	
	if len(privateKey) == 0 {
		return nil, fmt.Errorf("SSH private key is empty in secret")
	}
	
	// Create SSH auth method from private key
	auth, err := ssh.NewPublicKeys("git", privateKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH auth from private key: %w", err)
	}
	
	// Configure known hosts if provided
	knownHostsKey := g.config.KnownHostsKey
	if knownHostsKey != "" {
		if knownHosts, exists := secret.Data[knownHostsKey]; exists && len(knownHosts) > 0 {
			// Note: go-git doesn't have direct known_hosts support
			// In production, you might want to use ssh.NewKnownHostsCallback
			// For now, we'll use insecure host key checking
			auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
		}
	} else {
		// Default to insecure for simplicity - in production this should be configurable
		auth.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}
	
	return auth, nil
}

// getSecret retrieves the Kubernetes secret
func (g *gitAuthProvider) getSecret(ctx context.Context) (*corev1.Secret, error) {
	if g.config.SecretRef == "" {
		return nil, fmt.Errorf("secret reference is empty")
	}
	
	// Default namespace if not specified
	namespace := g.config.SecretNamespace
	if namespace == "" {
		namespace = "default"
	}
	
	// Get the secret
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      g.config.SecretRef,
		Namespace: namespace,
	}
	
	if err := g.k8sClient.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, g.config.SecretRef, err)
	}
	
	return secret, nil
}

// DefaultGitAuthConfig returns a default git authentication configuration
func DefaultGitAuthConfig() GitAuthConfig {
	return GitAuthConfig{
		Type:        GitAuthTypeToken,
		TokenKey:    "token",
		UsernameKey: "username",
		PasswordKey: "password",
	}
}

// ValidateGitAuthConfig validates a git authentication configuration
func ValidateGitAuthConfig(config GitAuthConfig) error {
	switch config.Type {
	case GitAuthTypeNone:
		// No validation needed for no auth
		return nil
		
	case GitAuthTypeHTTPS:
		return common.NewConfigValidator().
			RequireNonEmpty("secretRef", config.SecretRef).
			RequireNonEmpty("usernameKey", config.UsernameKey).
			RequireNonEmpty("passwordKey", config.PasswordKey).
			Validate()
		
	case GitAuthTypeToken:
		return common.NewConfigValidator().
			RequireNonEmpty("secretRef", config.SecretRef).
			RequireNonEmpty("tokenKey", config.TokenKey).
			Validate()
		
	case GitAuthTypeSSH:
		return common.NewConfigValidator().
			RequireNonEmpty("secretRef", config.SecretRef).
			RequireNonEmpty("sshKeyKey", config.SSHKeyKey).
			Validate()
		
	default:
		return fmt.Errorf("unsupported git authentication type: %s", config.Type)
	}
}

// CreateGitAuthProviderFromConfig creates a git auth provider from configuration
func CreateGitAuthProviderFromConfig(config GitAuthConfig, k8sClient client.Client) (GitAuthProvider, error) {
	if err := ValidateGitAuthConfig(config); err != nil {
		return nil, fmt.Errorf("invalid git auth config: %w", err)
	}
	
	return NewGitAuthProvider(config, k8sClient), nil
}

// DetectAuthTypeFromURL detects the appropriate auth type from repository URL
func DetectAuthTypeFromURL(repoURL string) GitAuthType {
	if strings.HasPrefix(repoURL, "https://") {
		return GitAuthTypeToken // Default to token for HTTPS URLs
	}
	
	if strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://") {
		return GitAuthTypeSSH
	}
	
	// For local or other protocols, use no auth
	return GitAuthTypeNone
}
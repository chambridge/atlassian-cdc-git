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

package jira

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AuthType defines the supported authentication methods
type AuthType string

const (
	AuthTypeBasic  AuthType = "basic"
	AuthTypeBearer AuthType = "bearer"
	AuthTypeToken  AuthType = "token"
)

// AuthConfig contains authentication configuration
type AuthConfig struct {
	Type           AuthType `json:"type"`
	SecretRef      string   `json:"secretRef"`
	SecretNamespace string  `json:"secretNamespace"`
	UsernameKey    string   `json:"usernameKey,omitempty"`
	PasswordKey    string   `json:"passwordKey,omitempty"`
	TokenKey       string   `json:"tokenKey,omitempty"`
}

// AuthProvider handles JIRA authentication
type AuthProvider interface {
	// ConfigureAuth configures the HTTP client with authentication
	ConfigureAuth(ctx context.Context, req *http.Request) error
	
	// ValidateCredentials validates that credentials are available and valid
	ValidateCredentials(ctx context.Context) error
	
	// RefreshCredentials refreshes credentials from Kubernetes secrets
	RefreshCredentials(ctx context.Context) error
	
	// GetAuthType returns the authentication type
	GetAuthType() AuthType
}

// authProvider implements AuthProvider
type authProvider struct {
	config    AuthConfig
	k8sClient client.Client
	
	// Cached credentials
	username string
	password string
	token    string
}

// NewAuthProvider creates a new authentication provider
func NewAuthProvider(config AuthConfig, k8sClient client.Client) AuthProvider {
	return &authProvider{
		config:    config,
		k8sClient: k8sClient,
	}
}

// ConfigureAuth configures the HTTP request with authentication
func (a *authProvider) ConfigureAuth(ctx context.Context, req *http.Request) error {
	logger := log.FromContext(ctx)
	
	// Ensure credentials are available
	if err := a.ensureCredentials(ctx); err != nil {
		return fmt.Errorf("failed to ensure credentials: %w", err)
	}
	
	switch a.config.Type {
	case AuthTypeBasic:
		if a.username == "" || a.password == "" {
			return fmt.Errorf("username or password is empty for basic auth")
		}
		
		auth := base64.StdEncoding.EncodeToString([]byte(a.username + ":" + a.password))
		req.Header.Set("Authorization", "Basic "+auth)
		logger.V(1).Info("Configured basic authentication", "username", a.username)
		
	case AuthTypeBearer:
		if a.token == "" {
			return fmt.Errorf("token is empty for bearer auth")
		}
		
		req.Header.Set("Authorization", "Bearer "+a.token)
		logger.V(1).Info("Configured bearer token authentication")
		
	case AuthTypeToken:
		if a.token == "" {
			return fmt.Errorf("token is empty for token auth")
		}
		
		// For JIRA API tokens, use basic auth with email and token
		if a.username == "" {
			return fmt.Errorf("username (email) is required for JIRA token auth")
		}
		
		auth := base64.StdEncoding.EncodeToString([]byte(a.username + ":" + a.token))
		req.Header.Set("Authorization", "Basic "+auth)
		logger.V(1).Info("Configured JIRA token authentication", "username", a.username)
		
	default:
		return fmt.Errorf("unsupported authentication type: %s", a.config.Type)
	}
	
	return nil
}

// ValidateCredentials validates that credentials are available and valid
func (a *authProvider) ValidateCredentials(ctx context.Context) error {
	if err := a.ensureCredentials(ctx); err != nil {
		return fmt.Errorf("failed to load credentials: %w", err)
	}
	
	switch a.config.Type {
	case AuthTypeBasic, AuthTypeToken:
		if a.username == "" {
			return fmt.Errorf("username is required for %s authentication", a.config.Type)
		}
		if a.config.Type == AuthTypeBasic && a.password == "" {
			return fmt.Errorf("password is required for basic authentication")
		}
		if a.config.Type == AuthTypeToken && a.token == "" {
			return fmt.Errorf("token is required for token authentication")
		}
		
	case AuthTypeBearer:
		if a.token == "" {
			return fmt.Errorf("token is required for bearer authentication")
		}
		
	default:
		return fmt.Errorf("unsupported authentication type: %s", a.config.Type)
	}
	
	return nil
}

// RefreshCredentials refreshes credentials from Kubernetes secrets
func (a *authProvider) RefreshCredentials(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Info("Refreshing JIRA credentials", "secretRef", a.config.SecretRef)
	
	return a.loadCredentialsFromSecret(ctx)
}

// GetAuthType returns the authentication type
func (a *authProvider) GetAuthType() AuthType {
	return a.config.Type
}

// ensureCredentials ensures credentials are loaded
func (a *authProvider) ensureCredentials(ctx context.Context) error {
	// Check if credentials are already loaded
	switch a.config.Type {
	case AuthTypeBasic:
		if a.username != "" && a.password != "" {
			return nil
		}
	case AuthTypeBearer:
		if a.token != "" {
			return nil
		}
	case AuthTypeToken:
		if a.username != "" && a.token != "" {
			return nil
		}
	}
	
	// Load credentials from secret
	return a.loadCredentialsFromSecret(ctx)
}

// loadCredentialsFromSecret loads credentials from Kubernetes secret
func (a *authProvider) loadCredentialsFromSecret(ctx context.Context) error {
	logger := log.FromContext(ctx)
	
	if a.config.SecretRef == "" {
		return fmt.Errorf("secret reference is empty")
	}
	
	// Default namespace if not specified
	namespace := a.config.SecretNamespace
	if namespace == "" {
		namespace = "default"
	}
	
	// Get the secret
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      a.config.SecretRef,
		Namespace: namespace,
	}
	
	if err := a.k8sClient.Get(ctx, secretKey, secret); err != nil {
		return fmt.Errorf("failed to get secret %s/%s: %w", namespace, a.config.SecretRef, err)
	}
	
	logger.V(1).Info("Loaded secret for JIRA authentication", "secret", secretKey)
	
	// Extract credentials based on auth type
	switch a.config.Type {
	case AuthTypeBasic:
		if err := a.extractBasicCredentials(secret); err != nil {
			return fmt.Errorf("failed to extract basic auth credentials: %w", err)
		}
		
	case AuthTypeBearer:
		if err := a.extractBearerCredentials(secret); err != nil {
			return fmt.Errorf("failed to extract bearer token: %w", err)
		}
		
	case AuthTypeToken:
		if err := a.extractTokenCredentials(secret); err != nil {
			return fmt.Errorf("failed to extract token credentials: %w", err)
		}
		
	default:
		return fmt.Errorf("unsupported authentication type: %s", a.config.Type)
	}
	
	return nil
}

// extractBasicCredentials extracts username and password for basic auth
func (a *authProvider) extractBasicCredentials(secret *corev1.Secret) error {
	usernameKey := a.config.UsernameKey
	if usernameKey == "" {
		usernameKey = "username"
	}
	
	passwordKey := a.config.PasswordKey
	if passwordKey == "" {
		passwordKey = "password"
	}
	
	username, exists := secret.Data[usernameKey]
	if !exists {
		return fmt.Errorf("username key '%s' not found in secret", usernameKey)
	}
	
	password, exists := secret.Data[passwordKey]
	if !exists {
		return fmt.Errorf("password key '%s' not found in secret", passwordKey)
	}
	
	a.username = strings.TrimSpace(string(username))
	a.password = strings.TrimSpace(string(password))
	
	if a.username == "" {
		return fmt.Errorf("username is empty in secret")
	}
	if a.password == "" {
		return fmt.Errorf("password is empty in secret")
	}
	
	return nil
}

// extractBearerCredentials extracts bearer token
func (a *authProvider) extractBearerCredentials(secret *corev1.Secret) error {
	tokenKey := a.config.TokenKey
	if tokenKey == "" {
		tokenKey = "token"
	}
	
	token, exists := secret.Data[tokenKey]
	if !exists {
		return fmt.Errorf("token key '%s' not found in secret", tokenKey)
	}
	
	a.token = strings.TrimSpace(string(token))
	
	if a.token == "" {
		return fmt.Errorf("token is empty in secret")
	}
	
	return nil
}

// extractTokenCredentials extracts username and token for JIRA API token auth
func (a *authProvider) extractTokenCredentials(secret *corev1.Secret) error {
	usernameKey := a.config.UsernameKey
	if usernameKey == "" {
		usernameKey = "username"
	}
	
	tokenKey := a.config.TokenKey
	if tokenKey == "" {
		tokenKey = "token"
	}
	
	username, exists := secret.Data[usernameKey]
	if !exists {
		return fmt.Errorf("username key '%s' not found in secret", usernameKey)
	}
	
	token, exists := secret.Data[tokenKey]
	if !exists {
		return fmt.Errorf("token key '%s' not found in secret", tokenKey)
	}
	
	a.username = strings.TrimSpace(string(username))
	a.token = strings.TrimSpace(string(token))
	
	if a.username == "" {
		return fmt.Errorf("username is empty in secret")
	}
	if a.token == "" {
		return fmt.Errorf("token is empty in secret")
	}
	
	return nil
}

// DefaultAuthConfig returns a default authentication configuration
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		Type:        AuthTypeToken,
		UsernameKey: "username",
		TokenKey:    "token",
	}
}

// ValidateAuthConfig validates an authentication configuration
func ValidateAuthConfig(config AuthConfig) error {
	if config.SecretRef == "" {
		return fmt.Errorf("secretRef is required")
	}
	
	switch config.Type {
	case AuthTypeBasic:
		if config.UsernameKey == "" {
			return fmt.Errorf("usernameKey is required for basic auth")
		}
		if config.PasswordKey == "" {
			return fmt.Errorf("passwordKey is required for basic auth")
		}
		
	case AuthTypeBearer:
		if config.TokenKey == "" {
			return fmt.Errorf("tokenKey is required for bearer auth")
		}
		
	case AuthTypeToken:
		if config.UsernameKey == "" {
			return fmt.Errorf("usernameKey is required for token auth")
		}
		if config.TokenKey == "" {
			return fmt.Errorf("tokenKey is required for token auth")
		}
		
	default:
		return fmt.Errorf("unsupported authentication type: %s", config.Type)
	}
	
	return nil
}

// CreateAuthProviderFromConfig creates an auth provider from configuration
func CreateAuthProviderFromConfig(config AuthConfig, k8sClient client.Client) (AuthProvider, error) {
	if err := ValidateAuthConfig(config); err != nil {
		return nil, fmt.Errorf("invalid auth config: %w", err)
	}
	
	return NewAuthProvider(config, k8sClient), nil
}
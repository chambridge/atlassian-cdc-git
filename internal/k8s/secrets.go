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

package k8s

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// SecretChangeHandler is called when a watched secret changes
type SecretChangeHandler func(ctx context.Context, secretKey types.NamespacedName, secret *corev1.Secret) error

// SecretWatcher watches Kubernetes secrets for changes and notifies handlers
type SecretWatcher interface {
	// WatchSecret registers a handler for secret changes
	WatchSecret(ctx context.Context, secretKey types.NamespacedName, handler SecretChangeHandler) error
	
	// UnwatchSecret stops watching a secret
	UnwatchSecret(ctx context.Context, secretKey types.NamespacedName) error
	
	// GetSecret retrieves the current version of a secret
	GetSecret(ctx context.Context, secretKey types.NamespacedName) (*corev1.Secret, error)
	
	// Start starts the secret watcher
	Start(ctx context.Context) error
	
	// Stop stops the secret watcher
	Stop() error
}

// WatchedSecret represents a secret being watched
type WatchedSecret struct {
	Key       types.NamespacedName
	Handlers  []SecretChangeHandler
	LastSeen  *corev1.Secret
	UpdatedAt time.Time
}

// secretWatcher implements SecretWatcher
type secretWatcher struct {
	client     client.Client
	manager    manager.Manager
	controller controller.Controller
	
	// Watched secrets
	mu      sync.RWMutex
	secrets map[types.NamespacedName]*WatchedSecret
	
	// Lifecycle
	started bool
	stopCh  chan struct{}
}

// NewSecretWatcher creates a new secret watcher
func NewSecretWatcher(mgr manager.Manager) (SecretWatcher, error) {
	w := &secretWatcher{
		client:  mgr.GetClient(),
		manager: mgr,
		secrets: make(map[types.NamespacedName]*WatchedSecret),
		stopCh:  make(chan struct{}),
	}
	
	// Create controller for watching secrets
	ctrl, err := controller.New("secret-watcher", mgr, controller.Options{
		Reconciler: w,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create secret watcher controller: %w", err)
	}
	
	w.controller = ctrl
	
	// Watch secret changes
	if err := ctrl.Watch(
		source.Kind(mgr.GetCache(), &corev1.Secret{}),
		&handler.EnqueueRequestForObject{},
		predicate.NewPredicateFuncs(w.shouldProcessSecret),
	); err != nil {
		return nil, fmt.Errorf("failed to watch secrets: %w", err)
	}
	
	return w, nil
}

// WatchSecret registers a handler for secret changes
func (w *secretWatcher) WatchSecret(ctx context.Context, secretKey types.NamespacedName, handler SecretChangeHandler) error {
	logger := log.FromContext(ctx)
	
	w.mu.Lock()
	defer w.mu.Unlock()
	
	// Get or create watched secret
	watched, exists := w.secrets[secretKey]
	if !exists {
		watched = &WatchedSecret{
			Key:       secretKey,
			Handlers:  []SecretChangeHandler{},
			UpdatedAt: time.Now(),
		}
		w.secrets[secretKey] = watched
		
		// Load initial secret value
		secret := &corev1.Secret{}
		if err := w.client.Get(ctx, secretKey, secret); err != nil {
			logger.V(1).Info("Failed to load initial secret value", "secret", secretKey, "error", err)
		} else {
			watched.LastSeen = secret
		}
		
		logger.Info("Started watching secret", "secret", secretKey)
	}
	
	// Add handler
	watched.Handlers = append(watched.Handlers, handler)
	watched.UpdatedAt = time.Now()
	
	logger.V(1).Info("Added handler for secret", "secret", secretKey, "handlerCount", len(watched.Handlers))
	
	return nil
}

// UnwatchSecret stops watching a secret
func (w *secretWatcher) UnwatchSecret(ctx context.Context, secretKey types.NamespacedName) error {
	logger := log.FromContext(ctx)
	
	w.mu.Lock()
	defer w.mu.Unlock()
	
	if _, exists := w.secrets[secretKey]; exists {
		delete(w.secrets, secretKey)
		logger.Info("Stopped watching secret", "secret", secretKey)
	}
	
	return nil
}

// GetSecret retrieves the current version of a secret
func (w *secretWatcher) GetSecret(ctx context.Context, secretKey types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := w.client.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", secretKey, err)
	}
	return secret, nil
}

// Start starts the secret watcher
func (w *secretWatcher) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)
	
	if w.started {
		return fmt.Errorf("secret watcher is already started")
	}
	
	w.started = true
	logger.Info("Starting secret watcher")
	
	return nil
}

// Stop stops the secret watcher
func (w *secretWatcher) Stop() error {
	if !w.started {
		return nil
	}
	
	close(w.stopCh)
	w.started = false
	
	return nil
}

// Reconcile implements reconcile.Reconciler for secret changes
func (w *secretWatcher) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx).WithValues("secret", req.NamespacedName)
	
	// Check if we're watching this secret
	w.mu.RLock()
	watched, exists := w.secrets[req.NamespacedName]
	if !exists {
		w.mu.RUnlock()
		// Not watching this secret, ignore
		return reconcile.Result{}, nil
	}
	
	// Copy handlers to avoid holding lock during execution
	handlers := make([]SecretChangeHandler, len(watched.Handlers))
	copy(handlers, watched.Handlers)
	w.mu.RUnlock()
	
	// Get current secret
	secret := &corev1.Secret{}
	if err := w.client.Get(ctx, req.NamespacedName, secret); err != nil {
		logger.Error(err, "Failed to get secret")
		
		// If secret was deleted, notify handlers with nil secret
		if client.IgnoreNotFound(err) == nil {
			logger.Info("Secret was deleted, notifying handlers")
			for _, handler := range handlers {
				if err := handler(ctx, req.NamespacedName, nil); err != nil {
					logger.Error(err, "Handler failed for deleted secret")
				}
			}
		}
		
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	
	// Check if secret actually changed
	w.mu.Lock()
	lastSeen := watched.LastSeen
	watched.LastSeen = secret.DeepCopy()
	watched.UpdatedAt = time.Now()
	w.mu.Unlock()
	
	if lastSeen != nil && secretDataEqual(lastSeen, secret) {
		logger.V(1).Info("Secret data unchanged, skipping handlers")
		return reconcile.Result{}, nil
	}
	
	logger.Info("Secret changed, notifying handlers", "handlerCount", len(handlers))
	
	// Notify all handlers
	var errors []error
	for _, handler := range handlers {
		if err := handler(ctx, req.NamespacedName, secret); err != nil {
			logger.Error(err, "Handler failed for secret change")
			errors = append(errors, err)
		}
	}
	
	// Return error if any handlers failed
	if len(errors) > 0 {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, fmt.Errorf("failed to handle secret change: %d handlers failed", len(errors))
	}
	
	return reconcile.Result{}, nil
}

// shouldProcessSecret determines if we should process this secret
func (w *secretWatcher) shouldProcessSecret(obj client.Object) bool {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return false
	}
	
	secretKey := types.NamespacedName{
		Name:      secret.Name,
		Namespace: secret.Namespace,
	}
	
	w.mu.RLock()
	defer w.mu.RUnlock()
	
	_, exists := w.secrets[secretKey]
	return exists
}

// secretDataEqual compares secret data for equality
func secretDataEqual(a, b *corev1.Secret) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	
	if len(a.Data) != len(b.Data) {
		return false
	}
	
	for key, valueA := range a.Data {
		valueB, exists := b.Data[key]
		if !exists {
			return false
		}
		
		if string(valueA) != string(valueB) {
			return false
		}
	}
	
	return true
}

// SecretRotationManager manages credential rotation for authentication providers
type SecretRotationManager struct {
	watcher    SecretWatcher
	providers  map[types.NamespacedName][]CredentialProvider
	mu         sync.RWMutex
}

// CredentialProvider represents a provider that uses credentials from secrets
type CredentialProvider interface {
	// RefreshCredentials refreshes credentials from the secret
	RefreshCredentials(ctx context.Context) error
	
	// GetSecretReference returns the secret reference used by this provider
	GetSecretReference() types.NamespacedName
}

// NewSecretRotationManager creates a new secret rotation manager
func NewSecretRotationManager(watcher SecretWatcher) *SecretRotationManager {
	return &SecretRotationManager{
		watcher:   watcher,
		providers: make(map[types.NamespacedName][]CredentialProvider),
	}
}

// RegisterProvider registers a credential provider for secret rotation
func (m *SecretRotationManager) RegisterProvider(ctx context.Context, provider CredentialProvider) error {
	secretKey := provider.GetSecretReference()
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Add provider to list
	providers, exists := m.providers[secretKey]
	if !exists {
		providers = []CredentialProvider{}
		
		// Start watching the secret
		if err := m.watcher.WatchSecret(ctx, secretKey, m.handleSecretChange); err != nil {
			return fmt.Errorf("failed to watch secret %s: %w", secretKey, err)
		}
	}
	
	m.providers[secretKey] = append(providers, provider)
	
	log.FromContext(ctx).Info("Registered credential provider for secret rotation", 
		"secret", secretKey, "providerCount", len(m.providers[secretKey]))
	
	return nil
}

// UnregisterProvider unregisters a credential provider from secret rotation
func (m *SecretRotationManager) UnregisterProvider(ctx context.Context, provider CredentialProvider) error {
	secretKey := provider.GetSecretReference()
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	providers, exists := m.providers[secretKey]
	if !exists {
		return nil
	}
	
	// Remove provider from list
	newProviders := []CredentialProvider{}
	for _, p := range providers {
		if p != provider {
			newProviders = append(newProviders, p)
		}
	}
	
	if len(newProviders) == 0 {
		// No more providers for this secret, stop watching
		delete(m.providers, secretKey)
		return m.watcher.UnwatchSecret(ctx, secretKey)
	}
	
	m.providers[secretKey] = newProviders
	
	log.FromContext(ctx).Info("Unregistered credential provider from secret rotation", 
		"secret", secretKey, "providerCount", len(m.providers[secretKey]))
	
	return nil
}

// handleSecretChange handles secret changes by refreshing all registered providers
func (m *SecretRotationManager) handleSecretChange(ctx context.Context, secretKey types.NamespacedName, secret *corev1.Secret) error {
	logger := log.FromContext(ctx).WithValues("secret", secretKey)
	
	m.mu.RLock()
	providers, exists := m.providers[secretKey]
	if !exists {
		m.mu.RUnlock()
		return nil
	}
	
	// Copy providers to avoid holding lock during refresh
	providersCopy := make([]CredentialProvider, len(providers))
	copy(providersCopy, providers)
	m.mu.RUnlock()
	
	if secret == nil {
		logger.Info("Secret was deleted, credentials may become invalid")
		return nil
	}
	
	logger.Info("Secret changed, refreshing credentials for providers", "providerCount", len(providersCopy))
	
	// Refresh credentials for all providers
	var errors []error
	for _, provider := range providersCopy {
		if err := provider.RefreshCredentials(ctx); err != nil {
			logger.Error(err, "Failed to refresh credentials for provider")
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to refresh credentials: %d providers failed", len(errors))
	}
	
	logger.Info("Successfully refreshed credentials for all providers")
	return nil
}
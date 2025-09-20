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
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RateLimitConfig contains rate limiting configuration
type RateLimitConfig struct {
	// RequestsPerSecond defines the sustained rate limit
	RequestsPerSecond float64 `json:"requestsPerSecond"`
	
	// BurstSize defines the burst capacity
	BurstSize int `json:"burstSize"`
	
	// ExponentialBackoff configures backoff behavior
	ExponentialBackoff ExponentialBackoffConfig `json:"exponentialBackoff"`
	
	// RespectServerLimits enables server-sent rate limit headers
	RespectServerLimits bool `json:"respectServerLimits"`
	
	// MaxConcurrentRequests limits concurrent requests
	MaxConcurrentRequests int `json:"maxConcurrentRequests"`
}

// ExponentialBackoffConfig contains exponential backoff configuration
type ExponentialBackoffConfig struct {
	// InitialDelay is the initial delay before retry
	InitialDelay time.Duration `json:"initialDelay"`
	
	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration `json:"maxDelay"`
	
	// Multiplier is the backoff multiplier
	Multiplier float64 `json:"multiplier"`
	
	// MaxRetries is the maximum number of retries
	MaxRetries int `json:"maxRetries"`
	
	// Jitter adds randomness to avoid thundering herd
	Jitter bool `json:"jitter"`
}

// RateLimiter handles rate limiting and backoff for JIRA API requests
type RateLimiter interface {
	// Wait waits for permission to make a request
	Wait(ctx context.Context) error
	
	// Allow checks if a request is allowed immediately
	Allow() bool
	
	// HandleResponse handles rate limit information from response
	HandleResponse(resp *http.Response) error
	
	// WithBackoff executes a function with exponential backoff
	WithBackoff(ctx context.Context, fn func() error) error
	
	// GetStats returns rate limiting statistics
	GetStats() RateLimitStats
}

// RateLimitStats contains rate limiting statistics
type RateLimitStats struct {
	RequestsAllowed     int64         `json:"requestsAllowed"`
	RequestsThrottled   int64         `json:"requestsThrottled"`
	ServerLimitHits     int64         `json:"serverLimitHits"`
	CurrentBurstTokens  int           `json:"currentBurstTokens"`
	LastServerResetTime *time.Time    `json:"lastServerResetTime,omitempty"`
	AverageWaitTime     time.Duration `json:"averageWaitTime"`
	BackoffRetries      int64         `json:"backoffRetries"`
}

// rateLimiter implements RateLimiter
type rateLimiter struct {
	config  RateLimitConfig
	limiter *rate.Limiter
	
	// Server-side rate limit tracking
	mu                    sync.RWMutex
	serverRateLimit       int
	serverRateLimitReset  time.Time
	serverRateRemaining   int
	
	// Concurrency control
	concurrentRequests chan struct{}
	
	// Statistics
	stats RateLimitStats
	
	// Wait time tracking for average calculation
	waitTimes []time.Duration
	waitIndex int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config RateLimitConfig) RateLimiter {
	limiter := rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.BurstSize)
	
	var concurrentRequests chan struct{}
	if config.MaxConcurrentRequests > 0 {
		concurrentRequests = make(chan struct{}, config.MaxConcurrentRequests)
	}
	
	return &rateLimiter{
		config:             config,
		limiter:            limiter,
		concurrentRequests: concurrentRequests,
		waitTimes:          make([]time.Duration, 100), // Track last 100 wait times
	}
}

// Wait waits for permission to make a request
func (r *rateLimiter) Wait(ctx context.Context) error {
	logger := log.FromContext(ctx)
	
	// Acquire concurrency slot if limited
	if r.concurrentRequests != nil {
		select {
		case r.concurrentRequests <- struct{}{}:
			// Acquired slot
			defer func() { <-r.concurrentRequests }()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	// Check server-side rate limits first
	if r.config.RespectServerLimits {
		if err := r.waitForServerRateLimit(ctx); err != nil {
			return err
		}
	}
	
	// Wait for client-side rate limiter
	startTime := time.Now()
	
	if err := r.limiter.Wait(ctx); err != nil {
		r.updateStats(false, 0)
		return fmt.Errorf("rate limiter wait failed: %w", err)
	}
	
	waitTime := time.Since(startTime)
	r.updateStats(true, waitTime)
	
	if waitTime > 0 {
		logger.V(1).Info("Rate limited request", "waitTime", waitTime)
	}
	
	return nil
}

// Allow checks if a request is allowed immediately
func (r *rateLimiter) Allow() bool {
	// Check concurrency limit
	if r.concurrentRequests != nil {
		select {
		case r.concurrentRequests <- struct{}{}:
			defer func() { <-r.concurrentRequests }()
		default:
			r.updateStats(false, 0)
			return false
		}
	}
	
	// Check server-side rate limits
	if r.config.RespectServerLimits && !r.serverRateLimitAllows() {
		r.updateStats(false, 0)
		return false
	}
	
	// Check client-side rate limiter
	allowed := r.limiter.Allow()
	r.updateStats(allowed, 0)
	
	return allowed
}

// HandleResponse handles rate limit information from HTTP response headers
func (r *rateLimiter) HandleResponse(resp *http.Response) error {
	if !r.config.RespectServerLimits {
		return nil
	}
	
	// Parse common rate limit headers
	// X-RateLimit-Limit: total limit
	// X-RateLimit-Remaining: remaining requests
	// X-RateLimit-Reset: reset time (Unix timestamp)
	// Retry-After: seconds to wait before next request
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	// Parse rate limit headers
	if limitStr := resp.Header.Get("X-RateLimit-Limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			r.serverRateLimit = limit
		}
	}
	
	if remainingStr := resp.Header.Get("X-RateLimit-Remaining"); remainingStr != "" {
		if remaining, err := strconv.Atoi(remainingStr); err == nil {
			r.serverRateRemaining = remaining
		}
	}
	
	if resetStr := resp.Header.Get("X-RateLimit-Reset"); resetStr != "" {
		if resetTime, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
			r.serverRateLimitReset = time.Unix(resetTime, 0)
		}
	}
	
	// Handle 429 Too Many Requests
	if resp.StatusCode == http.StatusTooManyRequests {
		r.stats.ServerLimitHits++
		
		// Use Retry-After header if available
		if retryAfterStr := resp.Header.Get("Retry-After"); retryAfterStr != "" {
			if retryAfter, err := strconv.Atoi(retryAfterStr); err == nil {
				r.serverRateLimitReset = time.Now().Add(time.Duration(retryAfter) * time.Second)
				r.serverRateRemaining = 0
			}
		}
	}
	
	return nil
}

// WithBackoff executes a function with exponential backoff
func (r *rateLimiter) WithBackoff(ctx context.Context, fn func() error) error {
	logger := log.FromContext(ctx)
	config := r.config.ExponentialBackoff
	
	if config.MaxRetries <= 0 {
		// No backoff configured, execute once
		return fn()
	}
	
	var lastErr error
	delay := config.InitialDelay
	
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute function
		err := fn()
		if err == nil {
			// Success
			if attempt > 0 {
				logger.Info("Function succeeded after retries", "attempt", attempt+1)
			}
			return nil
		}
		
		lastErr = err
		r.stats.BackoffRetries++
		
		// Don't delay after last attempt
		if attempt == config.MaxRetries {
			break
		}
		
		// Calculate delay with jitter if enabled
		actualDelay := delay
		if config.Jitter {
			// Add ±25% jitter
			jitter := time.Duration(float64(delay) * (rand.Float64() - 0.5) * 0.5)
			actualDelay = delay + jitter
		}
		
		// Ensure delay doesn't exceed max
		if actualDelay > config.MaxDelay {
			actualDelay = config.MaxDelay
		}
		
		logger.V(1).Info("Function failed, retrying with backoff", 
			"attempt", attempt+1, "delay", actualDelay, "error", err)
		
		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(actualDelay):
			// Continue to next attempt
		}
		
		// Increase delay for next attempt
		delay = time.Duration(float64(delay) * config.Multiplier)
	}
	
	logger.Error(lastErr, "Function failed after all retries", "attempts", config.MaxRetries+1)
	return fmt.Errorf("function failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

// GetStats returns current rate limiting statistics
func (r *rateLimiter) GetStats() RateLimitStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	stats := r.stats
	stats.CurrentBurstTokens = r.limiter.Tokens()
	
	if !r.serverRateLimitReset.IsZero() {
		resetTime := r.serverRateLimitReset
		stats.LastServerResetTime = &resetTime
	}
	
	return stats
}

// waitForServerRateLimit waits if server rate limit is exceeded
func (r *rateLimiter) waitForServerRateLimit(ctx context.Context) error {
	r.mu.RLock()
	remaining := r.serverRateRemaining
	resetTime := r.serverRateLimitReset
	r.mu.RUnlock()
	
	// If we have remaining requests, no need to wait
	if remaining > 0 {
		return nil
	}
	
	// If no reset time is set, allow the request
	if resetTime.IsZero() {
		return nil
	}
	
	// Calculate wait time
	waitUntil := resetTime
	waitTime := time.Until(waitUntil)
	
	// If reset time is in the past, allow the request
	if waitTime <= 0 {
		return nil
	}
	
	// Wait until reset time
	logger := log.FromContext(ctx)
	logger.Info("Waiting for server rate limit reset", "waitTime", waitTime, "resetTime", resetTime)
	
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(waitTime):
		return nil
	}
}

// serverRateLimitAllows checks if server rate limit allows the request
func (r *rateLimiter) serverRateLimitAllows() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// If we have remaining requests, allow
	if r.serverRateRemaining > 0 {
		return true
	}
	
	// If no reset time is set, allow
	if r.serverRateLimitReset.IsZero() {
		return true
	}
	
	// If reset time has passed, allow
	return time.Now().After(r.serverRateLimitReset)
}

// updateStats updates rate limiting statistics
func (r *rateLimiter) updateStats(allowed bool, waitTime time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if allowed {
		r.stats.RequestsAllowed++
	} else {
		r.stats.RequestsThrottled++
	}
	
	// Update average wait time
	if waitTime > 0 {
		r.waitTimes[r.waitIndex] = waitTime
		r.waitIndex = (r.waitIndex + 1) % len(r.waitTimes)
		
		// Calculate average
		var total time.Duration
		var count int
		for _, wt := range r.waitTimes {
			if wt > 0 {
				total += wt
				count++
			}
		}
		
		if count > 0 {
			r.stats.AverageWaitTime = total / time.Duration(count)
		}
	}
}

// DefaultRateLimitConfig returns a default rate limiting configuration
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond:     10.0, // Conservative default
		BurstSize:            20,
		RespectServerLimits:  true,
		MaxConcurrentRequests: 5,
		ExponentialBackoff: ExponentialBackoffConfig{
			InitialDelay: 1 * time.Second,
			MaxDelay:     30 * time.Second,
			Multiplier:   2.0,
			MaxRetries:   5,
			Jitter:       true,
		},
	}
}

// ValidateRateLimitConfig validates a rate limiting configuration
func ValidateRateLimitConfig(config RateLimitConfig) error {
	if config.RequestsPerSecond <= 0 {
		return fmt.Errorf("requestsPerSecond must be positive")
	}
	
	if config.BurstSize <= 0 {
		return fmt.Errorf("burstSize must be positive")
	}
	
	if config.MaxConcurrentRequests < 0 {
		return fmt.Errorf("maxConcurrentRequests must be non-negative")
	}
	
	backoff := config.ExponentialBackoff
	if backoff.MaxRetries < 0 {
		return fmt.Errorf("maxRetries must be non-negative")
	}
	
	if backoff.InitialDelay < 0 {
		return fmt.Errorf("initialDelay must be non-negative")
	}
	
	if backoff.MaxDelay < backoff.InitialDelay {
		return fmt.Errorf("maxDelay must be >= initialDelay")
	}
	
	if backoff.Multiplier <= 0 {
		return fmt.Errorf("multiplier must be positive")
	}
	
	return nil
}

// AdaptiveRateLimiter automatically adjusts rate limits based on server responses
type AdaptiveRateLimiter struct {
	*rateLimiter
	adaptiveConfig AdaptiveConfig
	
	mu               sync.RWMutex
	successRate      float64
	adjustmentFactor float64
	lastAdjustment   time.Time
}

// AdaptiveConfig contains configuration for adaptive rate limiting
type AdaptiveConfig struct {
	Enabled              bool          `json:"enabled"`
	MinRequestsPerSecond float64       `json:"minRequestsPerSecond"`
	MaxRequestsPerSecond float64       `json:"maxRequestsPerSecond"`
	AdjustmentInterval   time.Duration `json:"adjustmentInterval"`
	SuccessRateThreshold float64       `json:"successRateThreshold"`
	AdjustmentFactor     float64       `json:"adjustmentFactor"`
}

// NewAdaptiveRateLimiter creates a new adaptive rate limiter
func NewAdaptiveRateLimiter(config RateLimitConfig, adaptiveConfig AdaptiveConfig) *AdaptiveRateLimiter {
	baseLimiter := NewRateLimiter(config).(*rateLimiter)
	
	return &AdaptiveRateLimiter{
		rateLimiter:      baseLimiter,
		adaptiveConfig:   adaptiveConfig,
		adjustmentFactor: 1.0,
		successRate:      1.0,
	}
}

// HandleResponse handles server responses and adjusts rate limits adaptively
func (a *AdaptiveRateLimiter) HandleResponse(resp *http.Response) error {
	// Call base implementation
	if err := a.rateLimiter.HandleResponse(resp); err != nil {
		return err
	}
	
	if !a.adaptiveConfig.Enabled {
		return nil
	}
	
	// Track success rate
	success := resp.StatusCode < 400 && resp.StatusCode != http.StatusTooManyRequests
	a.updateSuccessRate(success)
	
	// Adjust rate limit if interval has passed
	a.maybeAdjustRateLimit()
	
	return nil
}

// updateSuccessRate updates the rolling success rate
func (a *AdaptiveRateLimiter) updateSuccessRate(success bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	// Use exponential moving average
	alpha := 0.1
	if success {
		a.successRate = alpha*1.0 + (1-alpha)*a.successRate
	} else {
		a.successRate = alpha*0.0 + (1-alpha)*a.successRate
	}
}

// maybeAdjustRateLimit adjusts the rate limit based on success rate
func (a *AdaptiveRateLimiter) maybeAdjustRateLimit() {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	now := time.Now()
	if now.Sub(a.lastAdjustment) < a.adaptiveConfig.AdjustmentInterval {
		return
	}
	
	a.lastAdjustment = now
	
	// Adjust based on success rate
	currentLimit := float64(a.limiter.Limit())
	newLimit := currentLimit
	
	if a.successRate > a.adaptiveConfig.SuccessRateThreshold {
		// Increase rate limit
		newLimit = currentLimit * (1 + a.adaptiveConfig.AdjustmentFactor)
		if newLimit > a.adaptiveConfig.MaxRequestsPerSecond {
			newLimit = a.adaptiveConfig.MaxRequestsPerSecond
		}
	} else {
		// Decrease rate limit
		newLimit = currentLimit * (1 - a.adaptiveConfig.AdjustmentFactor)
		if newLimit < a.adaptiveConfig.MinRequestsPerSecond {
			newLimit = a.adaptiveConfig.MinRequestsPerSecond
		}
	}
	
	if math.Abs(newLimit-currentLimit) > 0.1 {
		a.limiter.SetLimit(rate.Limit(newLimit))
		log.Log.Info("Adjusted rate limit", 
			"oldLimit", currentLimit, 
			"newLimit", newLimit, 
			"successRate", a.successRate)
	}
}
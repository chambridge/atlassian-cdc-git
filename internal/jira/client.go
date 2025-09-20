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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Config represents the configuration for a JIRA client
type Config struct {
	BaseURL           string
	CredentialsSecret string
	Namespace         string
	ProxyURL          string
	ProxyEnabled      bool
}

// Client represents a JIRA API client with rate limiting
type Client struct {
	config     Config
	httpClient *http.Client
	rateLimiter *rate.Limiter
	k8sClient  client.Client
	
	// Cached credentials
	username string
	token    string
}

// User represents a JIRA user
type User struct {
	Self         string `json:"self"`
	Name         string `json:"name"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
}

// Project represents a JIRA project
type Project struct {
	Self string `json:"self"`
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// Issue represents a JIRA issue
type Issue struct {
	ID     string      `json:"id"`
	Key    string      `json:"key"`
	Self   string      `json:"self"`
	Fields IssueFields `json:"fields"`
}

// IssueFields represents the fields of a JIRA issue
type IssueFields struct {
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Status      StatusField `json:"status"`
	IssueType   IssueType   `json:"issuetype"`
	Assignee    *User       `json:"assignee"`
	Reporter    *User       `json:"reporter"`
	Priority    Priority    `json:"priority"`
	Labels      []string    `json:"labels"`
	Components  []Component `json:"components"`
	FixVersions []Version   `json:"fixVersions"`
	Parent      *IssueRef   `json:"parent,omitempty"`
	Created     string      `json:"created"`
	Updated     string      `json:"updated"`
}

// StatusField represents a JIRA status
type StatusField struct {
	Self string `json:"self"`
	Name string `json:"name"`
	ID   string `json:"id"`
}

// IssueType represents a JIRA issue type
type IssueType struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IconURL     string `json:"iconUrl"`
	Subtask     bool   `json:"subtask"`
}

// Priority represents a JIRA priority
type Priority struct {
	Self    string `json:"self"`
	IconURL string `json:"iconUrl"`
	Name    string `json:"name"`
	ID      string `json:"id"`
}

// Component represents a JIRA component
type Component struct {
	Self string `json:"self"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Version represents a JIRA version
type Version struct {
	Self     string `json:"self"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
	Released bool   `json:"released"`
}

// IssueRef represents a reference to a JIRA issue
type IssueRef struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// SearchRequest represents a JIRA search request
type SearchRequest struct {
	JQL        string   `json:"jql"`
	StartAt    int      `json:"startAt"`
	MaxResults int      `json:"maxResults"`
	Fields     []string `json:"fields"`
	Expand     []string `json:"expand,omitempty"`
}

// SearchResult represents a JIRA search result
type SearchResult struct {
	Expand     string  `json:"expand"`
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	Issues     []Issue `json:"issues"`
}

// NewClient creates a new JIRA client
func NewClient(ctx context.Context, k8sClient client.Client, config Config) (*Client, error) {
	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create rate limiter (10 requests per second with burst of 20)
	rateLimiter := rate.NewLimiter(rate.Limit(10), 20)

	client := &Client{
		config:      config,
		httpClient:  httpClient,
		rateLimiter: rateLimiter,
		k8sClient:   k8sClient,
	}

	// Load credentials
	if err := client.loadCredentials(ctx); err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	return client, nil
}

// loadCredentials loads JIRA credentials from Kubernetes secret
func (c *Client) loadCredentials(ctx context.Context) error {
	var secret corev1.Secret
	secretKey := types.NamespacedName{
		Name:      c.config.CredentialsSecret,
		Namespace: c.config.Namespace,
	}

	if err := c.k8sClient.Get(ctx, secretKey, &secret); err != nil {
		return fmt.Errorf("failed to get credentials secret: %w", err)
	}

	username, exists := secret.Data["username"]
	if !exists {
		return fmt.Errorf("username not found in credentials secret")
	}
	c.username = string(username)

	token, exists := secret.Data["token"]
	if !exists {
		return fmt.Errorf("token not found in credentials secret")
	}
	c.token = string(token)

	return nil
}

// Authenticate verifies the JIRA credentials
func (c *Client) Authenticate(ctx context.Context) error {
	_, err := c.GetCurrentUser(ctx)
	return err
}

// GetCurrentUser returns the current user information
func (c *Client) GetCurrentUser(ctx context.Context) (*User, error) {
	var user User
	if err := c.doRequest(ctx, "GET", "/rest/api/2/myself", nil, &user); err != nil {
		return nil, fmt.Errorf("failed to get current user: %w", err)
	}
	return &user, nil
}

// GetProject returns project information
func (c *Client) GetProject(ctx context.Context, projectKey string) (*Project, error) {
	var project Project
	endpoint := fmt.Sprintf("/rest/api/2/project/%s", projectKey)
	if err := c.doRequest(ctx, "GET", endpoint, nil, &project); err != nil {
		return nil, fmt.Errorf("failed to get project %s: %w", projectKey, err)
	}
	return &project, nil
}

// SearchIssues searches for issues using JQL
func (c *Client) SearchIssues(ctx context.Context, searchRequest SearchRequest) (*SearchResult, error) {
	// Build query parameters
	params := url.Values{}
	params.Set("jql", searchRequest.JQL)
	params.Set("startAt", strconv.Itoa(searchRequest.StartAt))
	params.Set("maxResults", strconv.Itoa(searchRequest.MaxResults))
	
	if len(searchRequest.Fields) > 0 {
		for _, field := range searchRequest.Fields {
			params.Add("fields", field)
		}
	}
	
	if len(searchRequest.Expand) > 0 {
		for _, expand := range searchRequest.Expand {
			params.Add("expand", expand)
		}
	}

	endpoint := "/rest/api/2/search?" + params.Encode()
	
	var result SearchResult
	if err := c.doRequest(ctx, "GET", endpoint, nil, &result); err != nil {
		return nil, fmt.Errorf("failed to search issues: %w", err)
	}
	return &result, nil
}

// GetIssue returns a specific issue by key
func (c *Client) GetIssue(ctx context.Context, issueKey string, fields []string) (*Issue, error) {
	endpoint := fmt.Sprintf("/rest/api/2/issue/%s", issueKey)
	
	if len(fields) > 0 {
		params := url.Values{}
		for _, field := range fields {
			params.Add("fields", field)
		}
		endpoint += "?" + params.Encode()
	}

	var issue Issue
	if err := c.doRequest(ctx, "GET", endpoint, nil, &issue); err != nil {
		return nil, fmt.Errorf("failed to get issue %s: %w", issueKey, err)
	}
	return &issue, nil
}

// GetProjectIssues returns all issues for a project with pagination
func (c *Client) GetProjectIssues(ctx context.Context, projectKey string, startAt, maxResults int, activeOnly bool) (*SearchResult, error) {
	jql := fmt.Sprintf("project = %s", projectKey)
	if activeOnly {
		jql += " AND status != Done AND status != Closed AND status != Resolved"
	}
	
	searchRequest := SearchRequest{
		JQL:        jql,
		StartAt:    startAt,
		MaxResults: maxResults,
		Fields: []string{
			"summary", "description", "status", "issuetype", 
			"assignee", "reporter", "priority", "labels", 
			"components", "fixVersions", "parent", "created", "updated",
		},
	}
	
	return c.SearchIssues(ctx, searchRequest)
}

// doRequest performs an HTTP request with rate limiting and authentication
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body io.Reader, result interface{}) error {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter error: %w", err)
	}

	// Build full URL
	fullURL := c.config.BaseURL + endpoint

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	
	// Set authentication
	req.SetBasicAuth(c.username, c.token)

	// Perform request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("JIRA API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// UpdateConfig updates the client configuration and reloads credentials
func (c *Client) UpdateConfig(config Config) error {
	c.config = config
	// Note: In a real implementation, this would reload credentials
	// For now, we just update the config
	return nil
}

// ValidateWebhookPayload validates a JIRA webhook payload
func (c *Client) ValidateWebhookPayload(payload map[string]interface{}) error {
	// Check required fields
	if _, exists := payload["webhookEvent"]; !exists {
		return fmt.Errorf("missing webhookEvent field")
	}
	
	if _, exists := payload["issue"]; !exists {
		return fmt.Errorf("missing issue field")
	}
	
	return nil
}

// ProcessWebhookEvent processes a JIRA webhook event
func (c *Client) ProcessWebhookEvent(ctx context.Context, payload map[string]interface{}) error {
	// Extract event type
	eventType, ok := payload["webhookEvent"].(string)
	if !ok {
		return fmt.Errorf("invalid webhookEvent type")
	}
	
	// Extract issue data
	issueData, ok := payload["issue"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid issue data")
	}
	
	// Get issue key
	issueKey, ok := issueData["key"].(string)
	if !ok {
		return fmt.Errorf("invalid issue key")
	}
	
	// Log the webhook event (in a real implementation, this would trigger sync operations)
	fmt.Printf("Webhook event: %s for issue %s\n", eventType, issueKey)
	
	return nil
}
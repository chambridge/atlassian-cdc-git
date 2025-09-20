package jira

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_GetIssue(t *testing.T) {
	tests := []struct {
		name           string
		issueKey       string
		responseBody   string
		responseStatus int
		expectedError  bool
	}{
		{
			name:           "successful issue retrieval",
			issueKey:       "PROJ-123",
			responseBody:   `{"key":"PROJ-123","fields":{"summary":"Test Issue","description":"Test Description","status":{"name":"Open"},"assignee":{"displayName":"John Doe"},"reporter":{"displayName":"Jane Smith"},"priority":{"name":"High"},"labels":["bug","urgent"],"components":[{"name":"Backend"}],"fixVersions":[{"name":"1.0.0"}],"parent":{"key":"PROJ-100"},"updated":"2025-09-19T14:30:00.000+0000"}}`,
			responseStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name:           "issue not found",
			issueKey:       "PROJ-999",
			responseBody:   `{"errorMessages":["Issue does not exist or you do not have permission to see it."]}`,
			responseStatus: http.StatusNotFound,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.Path, tt.issueKey)
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			config := Config{
				BaseURL:   server.URL,
				Username:  "test@example.com",
				APIToken:  "test-token",
				UserAgent: "jiracdc-test/1.0.0",
			}

			client, err := NewClient(config)
			require.NoError(t, err)

			ctx := context.Background()
			issue, err := client.GetIssue(ctx, tt.issueKey)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, issue)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, issue)
				assert.Equal(t, tt.issueKey, issue.Key)
			}
		})
	}
}

func TestClient_SearchIssues(t *testing.T) {
	tests := []struct {
		name           string
		jql            string
		responseBody   string
		responseStatus int
		expectedCount  int
		expectedError  bool
	}{
		{
			name:           "successful search",
			jql:            "project = PROJ",
			responseBody:   `{"issues":[{"key":"PROJ-1","fields":{"summary":"Issue 1"}},{"key":"PROJ-2","fields":{"summary":"Issue 2"}}],"total":2,"maxResults":50,"startAt":0}`,
			responseStatus: http.StatusOK,
			expectedCount:  2,
			expectedError:  false,
		},
		{
			name:           "invalid JQL",
			jql:            "invalid jql",
			responseBody:   `{"errorMessages":["The value 'invalid' does not exist for the field 'project'."]}`,
			responseStatus: http.StatusBadRequest,
			expectedCount:  0,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Contains(t, r.URL.RawQuery, "jql=")
				w.WriteHeader(tt.responseStatus)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			config := Config{
				BaseURL:   server.URL,
				Username:  "test@example.com",
				APIToken:  "test-token",
				UserAgent: "jiracdc-test/1.0.0",
			}

			client, err := NewClient(config)
			require.NoError(t, err)

			ctx := context.Background()
			result, err := client.SearchIssues(ctx, tt.jql, 0, 50)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedCount, len(result.Issues))
			}
		})
	}
}

func TestClient_RateLimiting(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount <= 2 {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"errorMessages":["Rate limit exceeded"]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"key":"PROJ-123","fields":{"summary":"Test Issue"}}`))
	}))
	defer server.Close()

	config := Config{
		BaseURL:          server.URL,
		Username:         "test@example.com",
		APIToken:         "test-token",
		UserAgent:        "jiracdc-test/1.0.0",
		RequestsPerMinute: 60,
		MaxRetries:       3,
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	start := time.Now()
	issue, err := client.GetIssue(ctx, "PROJ-123")
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.NotNil(t, issue)
	assert.Greater(t, requestCount, 2, "Should have retried after rate limit")
	assert.Greater(t, elapsed, time.Second, "Should have applied backoff delay")
}

func TestClient_Authentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Basic ") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"errorMessages":["You are not authenticated."]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"key":"PROJ-123","fields":{"summary":"Test Issue"}}`))
	}))
	defer server.Close()

	config := Config{
		BaseURL:   server.URL,
		Username:  "test@example.com",
		APIToken:  "test-token",
		UserAgent: "jiracdc-test/1.0.0",
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	issue, err := client.GetIssue(ctx, "PROJ-123")

	assert.NoError(t, err)
	assert.NotNil(t, issue)
	assert.Equal(t, "PROJ-123", issue.Key)
}

func TestClient_ProxyConfiguration(t *testing.T) {
	config := Config{
		BaseURL:   "https://company.atlassian.net",
		Username:  "test@example.com",
		APIToken:  "test-token",
		UserAgent: "jiracdc-test/1.0.0",
		ProxyURL:  "http://proxy.company.com:8080",
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	// Verify proxy is configured
	transport := client.httpClient.Transport.(*http.Transport)
	assert.NotNil(t, transport.Proxy)
}

func TestClient_Pagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startAt := r.URL.Query().Get("startAt")
		if startAt == "0" {
			w.Write([]byte(`{"issues":[{"key":"PROJ-1"},{"key":"PROJ-2"}],"total":4,"maxResults":2,"startAt":0}`))
		} else if startAt == "2" {
			w.Write([]byte(`{"issues":[{"key":"PROJ-3"},{"key":"PROJ-4"}],"total":4,"maxResults":2,"startAt":2}`))
		}
	}))
	defer server.Close()

	config := Config{
		BaseURL:   server.URL,
		Username:  "test@example.com",
		APIToken:  "test-token",
		UserAgent: "jiracdc-test/1.0.0",
	}

	client, err := NewClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	
	// First page
	result1, err := client.SearchIssues(ctx, "project = PROJ", 0, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, len(result1.Issues))
	assert.Equal(t, 4, result1.Total)

	// Second page
	result2, err := client.SearchIssues(ctx, "project = PROJ", 2, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, len(result2.Issues))
	assert.Equal(t, 4, result2.Total)
}

func TestNewClient_InvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "empty base URL",
			config: Config{
				BaseURL:  "",
				Username: "test@example.com",
				APIToken: "token",
			},
		},
		{
			name: "empty username",
			config: Config{
				BaseURL:  "https://company.atlassian.net",
				Username: "",
				APIToken: "token",
			},
		},
		{
			name: "empty API token",
			config: Config{
				BaseURL:  "https://company.atlassian.net",
				Username: "test@example.com",
				APIToken: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)
			assert.Error(t, err)
			assert.Nil(t, client)
		})
	}
}
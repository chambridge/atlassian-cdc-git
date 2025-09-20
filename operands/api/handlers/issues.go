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

package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"sigs.k8s.io/controller-runtime/pkg/client"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
	"github.com/company/jira-cdc-operator/internal/jira"
	"github.com/company/jira-cdc-operator/internal/git"
)

// IssueHandler handles issue-related API endpoints
type IssueHandler struct {
	k8sClient  client.Client
	jiraClient *jira.Client
	gitManager *git.Manager
}

// NewIssueHandler creates a new issue handler
func NewIssueHandler(k8sClient client.Client, jiraClient *jira.Client, gitManager *git.Manager) *IssueHandler {
	return &IssueHandler{
		k8sClient:  k8sClient,
		jiraClient: jiraClient,
		gitManager: gitManager,
	}
}

// IssueResponse represents an issue for API responses
type IssueResponse struct {
	Key         string            `json:"key"`
	Summary     string            `json:"summary"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	IssueType   string            `json:"issueType"`
	Priority    string            `json:"priority"`
	Assignee    *string           `json:"assignee,omitempty"`
	Reporter    *string           `json:"reporter,omitempty"`
	Labels      []string          `json:"labels"`
	Components  []string          `json:"components"`
	Created     string            `json:"created"`
	Updated     string            `json:"updated"`
	ProjectKey  string            `json:"projectKey"`
	SyncStatus  IssueSyncStatus   `json:"syncStatus"`
	Links       IssueLinks        `json:"links"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// IssueSyncStatus represents the synchronization status of an issue
type IssueSyncStatus struct {
	LastSynced    *string `json:"lastSynced,omitempty"`
	SyncedVersion string  `json:"syncedVersion"`
	GitFilePath   string  `json:"gitFilePath"`
	CommitHash    string  `json:"commitHash,omitempty"`
	Status        string  `json:"status"` // synced, pending, failed, not_synced
	ErrorMessage  string  `json:"errorMessage,omitempty"`
}

// IssueLinks represents links related to an issue
type IssueLinks struct {
	JiraURL string `json:"jiraUrl"`
	GitURL  string `json:"gitUrl,omitempty"`
}

// IssueListResponse represents the response for issue list endpoints
type IssueListResponse struct {
	Issues    []IssueResponse `json:"issues"`
	Total     int             `json:"total"`
	StartAt   int             `json:"startAt"`
	MaxResults int            `json:"maxResults"`
	ProjectKey string         `json:"projectKey"`
}

// GetIssues handles GET /issues
func (h *IssueHandler) GetIssues(c *gin.Context) {
	projectKey := c.Query("projectKey")
	if projectKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "projectKey parameter is required",
		})
		return
	}

	// Parse query parameters
	startAtStr := c.DefaultQuery("startAt", "0")
	maxResultsStr := c.DefaultQuery("maxResults", "50")
	status := c.Query("status")
	assignee := c.Query("assignee")
	search := c.Query("search")

	startAt, err := strconv.Atoi(startAtStr)
	if err != nil || startAt < 0 {
		startAt = 0
	}

	maxResults, err := strconv.Atoi(maxResultsStr)
	if err != nil || maxResults <= 0 {
		maxResults = 50
	}
	if maxResults > 1000 {
		maxResults = 1000 // Cap at 1000
	}

	// Build JQL query
	jql := "project = " + projectKey
	if status != "" {
		jql += " AND status = \"" + status + "\""
	}
	if assignee != "" {
		jql += " AND assignee = \"" + assignee + "\""
	}
	if search != "" {
		jql += " AND (summary ~ \"" + search + "\" OR description ~ \"" + search + "\")"
	}

	// Search issues in JIRA
	searchRequest := jira.SearchRequest{
		JQL:        jql,
		StartAt:    startAt,
		MaxResults: maxResults,
		Fields: []string{
			"summary", "description", "status", "issuetype",
			"assignee", "reporter", "priority", "labels",
			"components", "created", "updated",
		},
	}

	searchResult, err := h.jiraClient.SearchIssues(context.TODO(), searchRequest)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to search issues in JIRA",
			"details": err.Error(),
		})
		return
	}

	// Convert to API response format
	issues := make([]IssueResponse, 0, len(searchResult.Issues))
	for _, issue := range searchResult.Issues {
		issueResponse := h.convertIssueToResponse(&issue, projectKey)
		issues = append(issues, issueResponse)
	}

	response := IssueListResponse{
		Issues:     issues,
		Total:      searchResult.Total,
		StartAt:    searchResult.StartAt,
		MaxResults: searchResult.MaxResults,
		ProjectKey: projectKey,
	}

	c.JSON(http.StatusOK, response)
}

// GetIssue handles GET /issues/:key
func (h *IssueHandler) GetIssue(c *gin.Context) {
	issueKey := c.Param("key")

	// Extract project key from issue key (format: PROJECT-123)
	parts := strings.Split(issueKey, "-")
	if len(parts) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid issue key format",
			"issueKey": issueKey,
		})
		return
	}
	projectKey := parts[0]

	// Get issue from JIRA
	issue, err := h.jiraClient.GetIssue(context.TODO(), issueKey, []string{
		"summary", "description", "status", "issuetype",
		"assignee", "reporter", "priority", "labels",
		"components", "fixVersions", "parent", "created", "updated",
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Issue not found",
			"issueKey": issueKey,
			"details": err.Error(),
		})
		return
	}

	// Convert to API response format
	issueResponse := h.convertIssueToResponse(issue, projectKey)

	c.JSON(http.StatusOK, issueResponse)
}

// PostIssueSync handles POST /issues/:key/sync
func (h *IssueHandler) PostIssueSync(c *gin.Context) {
	issueKey := c.Param("key")

	// Extract project key from issue key
	parts := strings.Split(issueKey, "-")
	if len(parts) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid issue key format",
			"issueKey": issueKey,
		})
		return
	}
	projectKey := parts[0]

	// Get issue from JIRA
	issue, err := h.jiraClient.GetIssue(context.TODO(), issueKey, []string{
		"summary", "description", "status", "issuetype",
		"assignee", "reporter", "priority", "labels",
		"components", "created", "updated",
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Issue not found in JIRA",
			"issueKey": issueKey,
			"details": err.Error(),
		})
		return
	}

	// Convert to git issue data
	issueData := h.convertJiraToGitData(issue)

	// Create or update issue file in git
	err = h.gitManager.CreateIssueFile(context.TODO(), *issueData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to sync issue to git",
			"issueKey": issueKey,
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Issue synced successfully",
		"issueKey": issueKey,
		"gitFilePath": issueKey + ".md",
	})
}

// GetIssueHistory handles GET /issues/:key/history
func (h *IssueHandler) GetIssueHistory(c *gin.Context) {
	issueKey := c.Param("key")

	// For now, return placeholder history
	// In a full implementation, this would get actual history from JIRA or git
	history := []gin.H{
		{
			"timestamp": "2025-01-01T10:00:00Z",
			"field":     "status",
			"from":      "To Do",
			"to":        "In Progress",
			"author":    "john.doe",
		},
		{
			"timestamp": "2025-01-01T09:00:00Z",
			"field":     "assignee",
			"from":      nil,
			"to":        "john.doe",
			"author":    "project.manager",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"issueKey": issueKey,
		"history":  history,
	})
}

// GetIssueComments handles GET /issues/:key/comments
func (h *IssueHandler) GetIssueComments(c *gin.Context) {
	issueKey := c.Param("key")

	// For now, return placeholder comments
	// In a full implementation, this would get actual comments from JIRA
	comments := []gin.H{
		{
			"id":        "12345",
			"author":    "john.doe",
			"body":      "This looks good to me",
			"created":   "2025-01-01T10:30:00Z",
			"updated":   "2025-01-01T10:30:00Z",
		},
		{
			"id":        "12346",
			"author":    "jane.smith",
			"body":      "I agree, let's proceed",
			"created":   "2025-01-01T11:00:00Z",
			"updated":   "2025-01-01T11:00:00Z",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"issueKey": issueKey,
		"comments": comments,
	})
}

// convertIssueToResponse converts a JIRA issue to API response format
func (h *IssueHandler) convertIssueToResponse(issue *jira.Issue, projectKey string) IssueResponse {
	response := IssueResponse{
		Key:         issue.Key,
		Summary:     issue.Fields.Summary,
		Description: issue.Fields.Description,
		Status:      issue.Fields.Status.Name,
		IssueType:   issue.Fields.IssueType.Name,
		Priority:    issue.Fields.Priority.Name,
		Labels:      issue.Fields.Labels,
		Created:     issue.Fields.Created,
		Updated:     issue.Fields.Updated,
		ProjectKey:  projectKey,
		SyncStatus: IssueSyncStatus{
			GitFilePath:   issue.Key + ".md",
			Status:        "unknown", // Would be determined by checking git/sync status
			SyncedVersion: issue.Fields.Updated,
		},
		Links: IssueLinks{
			JiraURL: h.getJiraIssueURL(issue.Key),
			GitURL:  h.getGitIssueURL(issue.Key),
		},
		Metadata: make(map[string]string),
	}

	// Set assignee if available
	if issue.Fields.Assignee != nil {
		response.Assignee = &issue.Fields.Assignee.DisplayName
	}

	// Set reporter if available
	if issue.Fields.Reporter != nil {
		response.Reporter = &issue.Fields.Reporter.DisplayName
	}

	// Set components
	for _, component := range issue.Fields.Components {
		response.Components = append(response.Components, component.Name)
	}

	// Add metadata
	response.Metadata["issueType"] = issue.Fields.IssueType.Name
	if issue.Fields.Parent != nil {
		response.Metadata["parent"] = issue.Fields.Parent.Key
	}

	return response
}

// convertJiraToGitData converts a JIRA issue to git issue data format
func (h *IssueHandler) convertJiraToGitData(issue *jira.Issue) *git.IssueData {
	gitData := &git.IssueData{
		Key:         issue.Key,
		Summary:     issue.Fields.Summary,
		Description: issue.Fields.Description,
		Status:      issue.Fields.Status.Name,
		Priority:    issue.Fields.Priority.Name,
		Labels:      issue.Fields.Labels,
		Created:     parseJiraTime(issue.Fields.Created),
		Updated:     parseJiraTime(issue.Fields.Updated),
	}

	// Set assignee and reporter
	if issue.Fields.Assignee != nil {
		gitData.Assignee = issue.Fields.Assignee.DisplayName
	}
	if issue.Fields.Reporter != nil {
		gitData.Reporter = issue.Fields.Reporter.DisplayName
	}

	// Set components
	for _, component := range issue.Fields.Components {
		gitData.Components = append(gitData.Components, component.Name)
	}

	return gitData
}

// parseJiraTime parses JIRA timestamp format
func parseJiraTime(jiraTime string) time.Time {
	// JIRA uses ISO 8601 format: 2006-01-02T15:04:05.000-0700
	t, err := time.Parse("2006-01-02T15:04:05.000-0700", jiraTime)
	if err != nil {
		// Fallback to basic format
		t, _ = time.Parse(time.RFC3339, jiraTime)
	}
	return t
}

// getJiraIssueURL constructs the JIRA URL for an issue
func (h *IssueHandler) getJiraIssueURL(issueKey string) string {
	// This would use the actual JIRA base URL from configuration
	return "https://your-jira-instance.atlassian.net/browse/" + issueKey
}

// getGitIssueURL constructs the git URL for an issue file
func (h *IssueHandler) getGitIssueURL(issueKey string) string {
	// This would use the actual git repository URL from configuration
	return "https://github.com/your-org/your-repo/blob/main/" + issueKey + ".md"
}
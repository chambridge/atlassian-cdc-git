package crd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	jiradcdv1 "github.com/company/jira-cdc-operator/api/v1"
)

func TestJiraCDCCRDValidation(t *testing.T) {
	tests := []struct {
		name    string
		jiracdc *jiradcdv1.JiraCDC
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid JiraCDC with all required fields",
			jiracdc: &jiradcdv1.JiraCDC{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jiracdc",
					Namespace: "default",
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
						URL:               "git@github.com:test/repo.git",
						CredentialsSecret: "git-creds",
						Branch:            "main",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid JiraCDC missing required jiraInstance",
			jiracdc: &jiradcdv1.JiraCDC{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jiracdc",
					Namespace: "default",
				},
				Spec: jiradcdv1.JiraCDCSpec{
					SyncTarget: jiradcdv1.SyncTargetConfig{
						Type:       "project",
						ProjectKey: "TEST",
					},
					GitRepository: jiradcdv1.GitRepositoryConfig{
						URL:               "git@github.com:test/repo.git",
						CredentialsSecret: "git-creds",
					},
				},
			},
			wantErr: true,
			errMsg:  "jiraInstance is required",
		},
		{
			name: "invalid JiraCDC with invalid sync target type",
			jiracdc: &jiradcdv1.JiraCDC{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jiracdc",
					Namespace: "default",
				},
				Spec: jiradcdv1.JiraCDCSpec{
					JiraInstance: jiradcdv1.JiraInstanceConfig{
						BaseURL:           "https://test.atlassian.net",
						CredentialsSecret: "jira-creds",
					},
					SyncTarget: jiradcdv1.SyncTargetConfig{
						Type: "invalid-type",
					},
					GitRepository: jiradcdv1.GitRepositoryConfig{
						URL:               "git@github.com:test/repo.git",
						CredentialsSecret: "git-creds",
					},
				},
			},
			wantErr: true,
			errMsg:  "syncTarget type must be one of: project, issues, jql",
		},
		{
			name: "invalid JiraCDC with project type but no projectKey",
			jiracdc: &jiradcdv1.JiraCDC{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-jiracdc",
					Namespace: "default",
				},
				Spec: jiradcdv1.JiraCDCSpec{
					JiraInstance: jiradcdv1.JiraInstanceConfig{
						BaseURL:           "https://test.atlassian.net",
						CredentialsSecret: "jira-creds",
					},
					SyncTarget: jiradcdv1.SyncTargetConfig{
						Type: "project",
						// ProjectKey missing for project type
					},
					GitRepository: jiradcdv1.GitRepositoryConfig{
						URL:               "git@github.com:test/repo.git",
						CredentialsSecret: "git-creds",
					},
				},
			},
			wantErr: true,
			errMsg:  "projectKey is required when syncTarget type is 'project'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail initially because JiraCDC types don't exist yet
			err := validateJiraCDC(tt.jiracdc)
			
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// validateJiraCDC validates a JiraCDC resource according to the CRD schema
// This function will be implemented after the CRD types are created
func validateJiraCDC(jiracdc *jiradcdv1.JiraCDC) error {
	// This will fail until we implement the JiraCDC types and validation
	panic("validateJiraCDC not implemented - JiraCDC types need to be created first")
}

func TestJiraCDCStatusValidation(t *testing.T) {
	status := &jiradcdv1.JiraCDCStatus{
		Phase:             "Current",
		SyncedIssueCount:  100,
		LastSyncTime:      &metav1.Time{},
	}

	// This test will fail until JiraCDCStatus is implemented
	err := validateJiraCDCStatus(status)
	require.NoError(t, err)
}

func validateJiraCDCStatus(status *jiradcdv1.JiraCDCStatus) error {
	// This will fail until we implement the JiraCDCStatus types
	panic("validateJiraCDCStatus not implemented - JiraCDCStatus types need to be created first")
}
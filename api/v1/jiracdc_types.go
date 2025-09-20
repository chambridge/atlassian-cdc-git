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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  This is scaffolding for you to own.
// NOTE: json tags are required.  Any new fields you add must have json:"-" or json:"<name>".

// JiraInstanceConfig defines the configuration for connecting to a JIRA instance
type JiraInstanceConfig struct {
	// BaseURL is the JIRA instance base URL
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format=uri
	BaseURL string `json:"baseURL"`

	// CredentialsSecret is the name of the Kubernetes secret containing JIRA credentials
	// +kubebuilder:validation:Required
	CredentialsSecret string `json:"credentialsSecret"`

	// ProxyConfig is optional HTTP proxy configuration for staging environments
	// +optional
	ProxyConfig *ProxyConfig `json:"proxyConfig,omitempty"`
}

// ProxyConfig defines HTTP proxy configuration
type ProxyConfig struct {
	// Enabled indicates whether proxy is enabled
	Enabled bool `json:"enabled"`

	// URL is the proxy server URL
	// +kubebuilder:validation:Format=uri
	URL string `json:"url,omitempty"`

	// CredentialsSecret is the name of the secret containing proxy credentials
	// +optional
	CredentialsSecret string `json:"credentialsSecret,omitempty"`
}

// SyncTargetConfig defines what to synchronize from JIRA
type SyncTargetConfig struct {
	// Type specifies the synchronization target type
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=project;issues;jql
	Type string `json:"type"`

	// ProjectKey is the JIRA project key (required when type=project)
	// +optional
	ProjectKey string `json:"projectKey,omitempty"`

	// IssueKeys are specific issue keys to sync (required when type=issues)
	// +optional
	IssueKeys []string `json:"issueKeys,omitempty"`

	// JQLQuery is a JQL query string (required when type=jql)
	// +optional
	JQLQuery string `json:"jqlQuery,omitempty"`
}

// GitRepositoryConfig defines the target git repository configuration
type GitRepositoryConfig struct {
	// URL is the git repository URL
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// CredentialsSecret is the name of the Kubernetes secret containing git credentials
	// +kubebuilder:validation:Required
	CredentialsSecret string `json:"credentialsSecret"`

	// Branch is the target branch (default: "main")
	// +kubebuilder:default="main"
	// +optional
	Branch string `json:"branch,omitempty"`
}

// OperandConfig defines configuration for managed operands
type OperandConfig struct {
	// API operand configuration
	// +optional
	API *APIOperandConfig `json:"api,omitempty"`

	// UI operand configuration
	// +optional
	UI *UIOperandConfig `json:"ui,omitempty"`
}

// APIOperandConfig defines API operand settings
type APIOperandConfig struct {
	// Enabled indicates whether to deploy the API operand
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Replicas is the number of API replicas
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

// UIOperandConfig defines UI operand settings
type UIOperandConfig struct {
	// Enabled indicates whether to deploy the UI operand
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Replicas is the number of UI replicas
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
}

// SyncConfig defines synchronization behavior
type SyncConfig struct {
	// Interval is the polling interval (default: "5m")
	// +kubebuilder:default="5m"
	// +optional
	Interval string `json:"interval,omitempty"`

	// Bootstrap indicates whether to perform initial bootstrap
	// +kubebuilder:default=true
	// +optional
	Bootstrap *bool `json:"bootstrap,omitempty"`

	// ActiveIssuesOnly indicates whether to sync only non-closed issues
	// +kubebuilder:default=true
	// +optional
	ActiveIssuesOnly *bool `json:"activeIssuesOnly,omitempty"`

	// ForceRefresh indicates whether to ignore last sync timestamps
	// +optional
	ForceRefresh bool `json:"forceRefresh,omitempty"`

	// TriggerBootstrap manually triggers a bootstrap operation
	// +optional
	TriggerBootstrap bool `json:"triggerBootstrap,omitempty"`

	// TriggerReconciliation manually triggers a reconciliation operation
	// +optional
	TriggerReconciliation bool `json:"triggerReconciliation,omitempty"`

	// CancelCurrentTask cancels the currently running task
	// +optional
	CancelCurrentTask bool `json:"cancelCurrentTask,omitempty"`
}

// AgentConfig defines agent integration configuration
type AgentConfig struct {
	// Enabled indicates whether agent integration is enabled
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Submodule configuration for agent access
	// +optional
	Submodule AgentSubmoduleConfig `json:"submodule,omitempty"`

	// Capabilities are the agent capabilities to enable
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`
}

// AgentSubmoduleConfig defines agent submodule configuration
type AgentSubmoduleConfig struct {
	// URL is the agent repository URL
	URL string `json:"url"`

	// Path is the submodule path within the main repository
	// +kubebuilder:default="agents/jira-cdc"
	// +optional
	Path string `json:"path,omitempty"`

	// CredentialsSecret is the name of the secret containing agent repository credentials
	CredentialsSecret string `json:"credentialsSecret"`

	// AutoUpdate indicates whether to automatically update the agent submodule
	// +kubebuilder:default=false
	// +optional
	AutoUpdate bool `json:"autoUpdate,omitempty"`
}

// JiraCDCSpec defines the desired state of JiraCDC
type JiraCDCSpec struct {
	// JiraInstance configuration for connecting to JIRA
	// +kubebuilder:validation:Required
	JiraInstance JiraInstanceConfig `json:"jiraInstance"`

	// SyncTarget defines what to synchronize from JIRA
	// +kubebuilder:validation:Required
	SyncTarget SyncTargetConfig `json:"syncTarget"`

	// GitRepository defines the target git repository
	// +kubebuilder:validation:Required
	GitRepository GitRepositoryConfig `json:"gitRepository"`

	// Operands configuration for managed operands
	// +optional
	Operands *OperandConfig `json:"operands,omitempty"`

	// SyncConfig defines synchronization behavior
	// +optional
	SyncConfig *SyncConfig `json:"syncConfig,omitempty"`

	// AgentConfig defines agent integration configuration
	// +optional
	AgentConfig *AgentConfig `json:"agentConfig,omitempty"`
}

// TaskProgress represents task progress information
type TaskProgress struct {
	// TotalItems is the total number of items to process
	TotalItems int32 `json:"totalItems"`

	// ProcessedItems is the number of items completed
	ProcessedItems int32 `json:"processedItems"`

	// PercentComplete is the calculated percentage
	PercentComplete float32 `json:"percentComplete"`

	// EstimatedTimeRemaining is the estimated completion time
	// +optional
	EstimatedTimeRemaining *metav1.Duration `json:"estimatedTimeRemaining,omitempty"`
}

// TaskInfo represents current task information
type TaskInfo struct {
	// ID is the unique task identifier
	ID string `json:"id"`

	// Type is the task type
	// +kubebuilder:validation:Enum=bootstrap;reconciliation;maintenance
	Type string `json:"type"`

	// Status is the current task status
	// +kubebuilder:validation:Enum=pending;running;completed;failed;cancelled
	Status string `json:"status"`

	// StartedAt is the task start time
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// CompletedAt is the task completion time
	// +optional
	CompletedAt *metav1.Time `json:"completedAt,omitempty"`

	// ErrorMessage contains error details if the task failed
	// +optional
	ErrorMessage string `json:"errorMessage,omitempty"`

	// Progress contains task progress information
	// +optional
	Progress *TaskProgress `json:"progress,omitempty"`
}

// OperandStatus represents the status of a managed operand
type OperandStatus struct {
	// Ready indicates whether the operand is ready
	Ready bool `json:"ready"`

	// Replicas is the number of operand replicas
	Replicas int32 `json:"replicas"`

	// ReadyReplicas is the number of ready replicas
	ReadyReplicas int32 `json:"readyReplicas"`

	// Endpoint is the operand service endpoint
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
}

// OperandsStatus represents the status of all managed operands
type OperandsStatus struct {
	// API operand status
	API OperandStatus `json:"api"`

	// UI operand status
	UI OperandStatus `json:"ui"`
}

// ComponentStatus represents the health status of system components
type ComponentStatus struct {
	// JiraConnection is the JIRA connection health status
	// +kubebuilder:validation:Enum=healthy;unhealthy
	JiraConnection string `json:"jiraConnection"`

	// GitRepository is the git repository connection health status
	// +kubebuilder:validation:Enum=healthy;unhealthy
	GitRepository string `json:"gitRepository"`

	// Kubernetes is the Kubernetes cluster health status
	// +kubebuilder:validation:Enum=healthy;unhealthy
	// +optional
	Kubernetes string `json:"kubernetes,omitempty"`
}

// AgentStatus represents the status of agent integration
type AgentStatus struct {
	// Enabled indicates whether agent integration is enabled
	Enabled bool `json:"enabled"`

	// Status is the agent integration status
	// +kubebuilder:validation:Enum=healthy;unhealthy;disabled
	Status string `json:"status"`

	// Version is the current agent version
	// +optional
	Version string `json:"version,omitempty"`

	// AvailableCapabilities are the currently available agent capabilities
	// +optional
	AvailableCapabilities []string `json:"availableCapabilities,omitempty"`
}

// JiraCDCStatus defines the observed state of JiraCDC
type JiraCDCStatus struct {
	// Phase is the current phase of the JiraCDC resource
	// +kubebuilder:validation:Enum=Pending;Syncing;Current;Error;Paused
	Phase string `json:"phase,omitempty"`

	// LastSyncTime is the timestamp of the last successful sync
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// SyncedIssueCount is the number of issues currently synced
	SyncedIssueCount int32 `json:"syncedIssueCount,omitempty"`

	// DiscoveredIssueCount is the total number of issues discovered in JIRA
	// +optional
	DiscoveredIssueCount int32 `json:"discoveredIssueCount,omitempty"`

	// LastCommitHash is the hash of the last commit made to the git repository
	// +optional
	LastCommitHash string `json:"lastCommitHash,omitempty"`

	// LastReconcileTime is the timestamp of the last controller reconciliation
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// OperandStatus contains the status of managed operands
	// +optional
	OperandStatus OperandsStatus `json:"operandStatus,omitempty"`

	// ComponentStatus contains the health status of system components
	// +optional
	ComponentStatus ComponentStatus `json:"componentStatus,omitempty"`

	// AgentStatus contains the status of agent integration
	// +optional
	AgentStatus AgentStatus `json:"agentStatus,omitempty"`

	// CurrentTask contains information about the currently running task
	// +optional
	CurrentTask *TaskInfo `json:"currentTask,omitempty"`

	// Conditions represent the latest available observations of the JiraCDC's current state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,shortName=jcdc
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Project",type="string",JSONPath=".spec.syncTarget.projectKey"
//+kubebuilder:printcolumn:name="Issues",type="integer",JSONPath=".status.syncedIssueCount"
//+kubebuilder:printcolumn:name="Last Sync",type="date",JSONPath=".status.lastSyncTime"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// JiraCDC is the Schema for the jiracdc API
type JiraCDC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JiraCDCSpec   `json:"spec,omitempty"`
	Status JiraCDCStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// JiraCDCList contains a list of JiraCDC
type JiraCDCList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JiraCDC `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JiraCDC{}, &JiraCDCList{})
}
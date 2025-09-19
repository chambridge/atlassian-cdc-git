# Data Model: JIRA Change Data Capture System

## Custom Resource Definitions (CRDs)

### JiraCDC (Primary CRD)
**Purpose**: Declares desired state for JIRA to Git synchronization
**API Version**: `jiracdc.io/v1`  
**Kind**: `JiraCDC`  

**Spec Fields**:
- `jiraInstance` (object): JIRA instance configuration
  - `baseURL` (string): JIRA instance base URL
  - `credentialsSecret` (string): Name of K8s secret containing credentials
  - `proxyConfig` (object, optional): HTTP proxy configuration for staging
- `syncTarget` (object): What to synchronize
  - `type` (enum): project, issues, jql
  - `projectKey` (string): JIRA project key (for type=project)
  - `issueKeys` ([]string): Specific issue keys (for type=issues)
  - `jqlQuery` (string): JQL query (for type=jql)
- `gitRepository` (object): Target git repository
  - `url` (string): Git repository URL
  - `credentialsSecret` (string): Name of K8s secret for git credentials
  - `branch` (string): Target branch (default: "main")
- `operands` (object): Managed operand configuration
  - `api` (object): API operand settings
    - `enabled` (bool): Whether to deploy API
    - `replicas` (int): Number of API replicas
  - `ui` (object): UI operand settings
    - `enabled` (bool): Whether to deploy UI
    - `replicas` (int): Number of UI replicas
- `syncConfig` (object): Synchronization behavior
  - `interval` (duration): Polling interval (default: "5m")
  - `bootstrap` (bool): Whether to perform initial bootstrap
  - `activeIssuesOnly` (bool): Sync only non-closed issues

**Status Fields**:
- `phase` (enum): Pending, Syncing, Current, Error, Paused
- `lastSyncTime` (timestamp): Last successful sync
- `syncedIssueCount` (int): Number of issues currently synced
- `operandStatus` (object): Status of managed operands
  - `api` (object): API operand status
  - `ui` (object): UI operand status
  - `jobs` ([]object): Current job status
- `conditions` ([]object): Detailed status conditions

## Managed Resources (Operands)

### API Operand (Deployment)
**Purpose**: REST API service for monitoring and task management
**Container**: Go application providing REST endpoints
**Dependencies**: controller-runtime, gin/fiber web framework, go-git
**Ports**: 8080 (HTTP API)
**Endpoints**: /api/v1/* (projects, tasks, health, metrics)
**Configuration**: ConfigMap with JIRA credentials reference, sync settings

### UI Operand (Deployment) 
**Purpose**: React-based monitoring dashboard
**Container**: Node.js serving React 18 + Patternfly 6 + TypeScript application
**Dependencies**: React 18, Patternfly 6, TypeScript, Webpack 5, React Router
**Ports**: 3000 (HTTP UI)
**Features**: Project monitoring, task progress, system health dashboard
**Configuration**: Environment variables for API endpoint discovery

### SyncJob (Job)
**Purpose**: Kubernetes Job for bootstrap or reconciliation operations
**Container**: Go application performing JIRA-to-Git synchronization
**Dependencies**: go-git, JIRA REST client, exponential backoff libraries
**Fields**:
- `Type` (enum): bootstrap, reconciliation, maintenance
- `JiraCDCRef` (object): Reference to parent JiraCDC resource
- `Progress` (object): Job progress tracking
  - `TotalItems` (int): Total items to process
  - `ProcessedItems` (int): Items completed
  - `PercentComplete` (float): Calculated percentage
- `Configuration` (object): Job-specific configuration
  - `IssueFilter` (string): JQL filter for selective sync
  - `ForceRefresh` (bool): Ignore last sync timestamps

### JiraIssue
**Purpose**: Represents a JIRA issue and its git representation
**Fields**:
- `IssueKey` (string): JIRA issue key (e.g., "PROJ-123")
- `ProjectID` (string): Reference to JiraProject
- `IssueType` (string): RFE, Feature, Epic, User Story, Task, Bug
- `Status` (string): JIRA status (Open, In Progress, Done, etc.)
- `Summary` (string): Issue title/summary
- `Description` (text): Issue description content
- `Assignee` (string): Assigned user (optional)
- `Reporter` (string): Issue creator
- `Priority` (string): Priority level
- `Labels` ([]string): Array of labels
- `Components` ([]string): Array of components
- `FixVersions` ([]string): Array of fix versions
- `ParentIssueKey` (string): Parent issue key (for hierarchy)
- `GitFilePath` (string): Relative path in git repository
- `JiraUpdatedAt` (timestamp): Last update time in JIRA
- `GitCommitHash` (string): Last git commit hash for this issue
- `SyncedAt` (timestamp): Last successful sync to git

**Validation Rules**:
- IssueKey must match JIRA key format
- ProjectID must reference valid JiraProject
- GitFilePath must be unique within repository
- JiraUpdatedAt drives sync decisions (newer = needs sync)

### CDCTask
**Purpose**: Represents a unit of work (bootstrap or reconciliation)
**Fields**:
- `ID` (string): Unique task identifier
- `Type` (enum): bootstrap, reconciliation, maintenance
- `ProjectID` (string): Reference to JiraProject
- `Status` (enum): pending, running, completed, failed, cancelled
- `Progress` (object): Progress tracking information
  - `TotalItems` (int): Total number of items to process
  - `ProcessedItems` (int): Number of items completed
  - `PercentComplete` (float): Calculated percentage
  - `EstimatedTimeRemaining` (duration): Estimated completion time
- `StartedAt` (timestamp): Task start time
- `CompletedAt` (timestamp): Task completion time (optional)
- `ErrorMessage` (string): Error details (if failed)
- `Configuration` (object): Task-specific configuration
  - `IssueFilter` (string): JQL filter for selective sync
  - `ForceRefresh` (bool): Whether to ignore last sync timestamps
- `CreatedBy` (string): User or system that created the task

**State Transitions**:
- pending → running (when task execution starts)
- running → completed (when task finishes successfully)
- running → failed (when task encounters error)
- pending → cancelled (when task is cancelled before execution)
- failed → pending (when task is retried)

### SyncOperation
**Purpose**: Tracks individual issue synchronization operations
**Fields**:
- `ID` (string): Unique operation identifier
- `TaskID` (string): Reference to parent CDCTask
- `IssueKey` (string): JIRA issue being synchronized
- `OperationType` (enum): create, update, delete, move
- `Status` (enum): pending, processing, completed, failed, skipped
- `JiraData` (object): JIRA issue data snapshot
- `GitOperation` (object): Git operation details
  - `Action` (string): Git action performed
  - `FilePath` (string): Target file path
  - `CommitMessage` (string): Commit message used
  - `CommitHash` (string): Resulting commit hash
- `ProcessedAt` (timestamp): Operation completion time
- `ErrorDetails` (string): Error information (if failed)
- `RetryCount` (int): Number of retry attempts

### SystemConfiguration
**Purpose**: Global system configuration and state
**Fields**:
- `ConfigVersion` (string): Configuration version for compatibility
- `DefaultSyncInterval` (duration): Default polling interval
- `MaxConcurrentTasks` (int): Maximum concurrent CDC tasks
- `RateLimitSettings` (object): JIRA API rate limiting configuration
  - `RequestsPerMinute` (int): Maximum requests per minute
  - `BurstLimit` (int): Burst request allowance
  - `BackoffMultiplier` (float): Exponential backoff multiplier
- `GitSettings` (object): Git operation configuration
  - `CommitAuthor` (string): Default commit author
  - `CommitEmail` (string): Default commit email
  - `BranchName` (string): Target branch (typically "main")
- `SecuritySettings` (object): Security configuration
  - `TokenRotationInterval` (duration): How often to rotate tokens
  - `MaxSessionDuration` (duration): UI session timeout

## Relationships

### Project → Issues (1:N)
- One JiraProject contains many JiraIssues
- Issues are scoped to a single project
- Cascade delete: Removing project removes all issues

### Project → Tasks (1:N)
- One JiraProject can have multiple CDCTasks
- Tasks operate on a single project
- Historical tasks preserved for audit

### Task → Operations (1:N)
- One CDCTask contains multiple SyncOperations
- Operations are atomic units within a task
- Operations track individual issue synchronizations

### Issue Hierarchy (Self-referential)
- JiraIssue.ParentIssueKey references another JiraIssue.IssueKey
- Supports Epic → Story → Task hierarchies
- Symbolic links in git represent hierarchy (Release 2.0)

## Data Access Patterns

### Real-time Sync
1. Poll JIRA for updated issues
2. Compare JiraUpdatedAt with SyncedAt
3. Create SyncOperation for each outdated issue
4. Process operations and update git
5. Update SyncedAt timestamps

### Bootstrap Operations
1. Create CDCTask with type=bootstrap
2. Query JIRA for all active issues
3. Create SyncOperation for each issue
4. Process operations in batches
5. Update progress tracking

### Reconciliation Operations
1. Compare git repository state with JIRA
2. Identify inconsistencies or missing issues
3. Create SyncOperations to resolve differences
4. Process operations to achieve consistency

### Agent Integration
1. Agents access git repository as read-only submodule
2. File structure: `{IssueKey}.md` with YAML frontmatter
3. Metadata includes sync timestamps and validation state
4. Change history preserved in git commit messages
# Research: JIRA Change Data Capture System

## Technical Stack Decisions

### Go Language Ecosystem
**Decision**: Go 1.21+ with standard library + key dependencies
**Rationale**: Constitutional requirement, excellent concurrency for CDC operations, strong Kubernetes ecosystem
**Alternatives considered**: Node.js (discarded - not constitutional), Python (discarded - performance concerns)

### Git Integration Library
**Decision**: go-git v5 (github.com/go-git/go-git/v5)
**Rationale**: Pure Go implementation, no external dependencies, excellent API for programmatic git operations
**Alternatives considered**: git2go (discarded - CGO dependency), exec git commands (discarded - less reliable)

### JIRA API Integration
**Decision**: Standard HTTP client with JIRA REST API v3
**Rationale**: Flexible, supports both API Token and Personal Access Token authentication patterns
**Alternatives considered**: Third-party JIRA SDK (discarded - adds complexity, constitutional requirement for minimal dependencies)

### Kubernetes Operator Framework
**Decision**: Kubebuilder v3 with controller-runtime for operator development
**Rationale**: Standard operator pattern, declarative configuration via CRDs, automatic reconciliation
**Alternatives considered**: Operator SDK (similar capabilities), raw controller-runtime (more complexity), standard Deployment (lacks declarative management)

### Frontend Technology Stack
**Decision**: React 18 + Patternfly 6 + TypeScript
**Rationale**: Specified in requirements, Patternfly provides enterprise-grade components, TypeScript for reliability
**Alternatives considered**: Vue.js (discarded - React specified), Angular (discarded - complexity)

### CDC Architecture Pattern
**Decision**: Polling + Webhook hybrid with polling as source of truth
**Rationale**: Constitutional requirement (polling-first), webhooks for optimization triggers
**Alternatives considered**: Webhook-only (discarded - constitutional violation), polling-only (discarded - performance)

### Data Storage Strategy
**Decision**: Git repositories as primary storage + in-memory state for operations
**Rationale**: Constitutional requirement, agent-friendly submodule access, version control built-in
**Alternatives considered**: Database + git sync (discarded - complexity), git-only (discarded - operational state needs)

### Authentication & Security
**Decision**: JWT tokens for UI, service accounts for K8s, .env for local development
**Rationale**: Constitutional security requirements, least-privilege patterns
**Alternatives considered**: Basic auth (discarded - security), OAuth (deferred - complexity)

### Monitoring & Observability
**Decision**: Prometheus metrics + structured logging (JSON) + health check endpoints
**Rationale**: Kubernetes-native monitoring, constitutional requirement for troubleshooting
**Alternatives considered**: Custom monitoring (discarded - complexity), ELK stack (deferred)

### Rate Limiting & Resilience
**Decision**: Token bucket algorithm + exponential backoff + circuit breaker pattern
**Rationale**: Constitutional requirement for JIRA API compliance, resilience patterns
**Alternatives considered**: Fixed delays (discarded - inefficient), no rate limiting (discarded - constitutional violation)

## Integration Patterns

### Kubernetes Operator Patterns
- Custom Resource Definition (CRD) for declarative JIRA project configuration
- Controller reconciliation loop for desired state management
- Operand management: API pods, UI pods, Jobs for bootstrap operations
- User-provided secrets referenced by name in CRD
- RBAC for operator and operand permissions

### JIRA API Patterns
- Use JIRA REST API v3 with JQL for issue querying
- Implement pagination for large result sets
- Support proxy configuration for staging environments
- Handle rate limiting with 429 response codes
- JQL query support via CRD specification

### Git Repository Structure
- One file per JIRA issue using issue key as filename
- Conventional commit messages for traceability
- Symbolic links for hierarchy (deferred to Release 2.0)
- Agent-friendly flat structure for submodule consumption

### CRD Configuration Patterns
- Declarative specification of JIRA instance, credentials, and sync targets
- Support for project-level, issue list, or JQL query-based sync
- Reference to existing Kubernetes secrets for credentials
- Status reporting through CRD status subresource

### Agent Integration Patterns
- Git repositories structured as read-only submodules
- Metadata in commit messages for staleness indicators
- Change history preservation for agent memory
- Human-validated vs agent-generated content indicators

## Performance Considerations

### Sync Performance Targets
- Bootstrap: <30 minutes for 1000 active issues
- Real-time sync: <5 minutes from JIRA change to git commit
- System availability: 99.5% during business hours
- Storage efficiency: <10MB per 100 JIRA issues

### Scalability Patterns
- Horizontal scaling via multiple service instances
- Per-project isolation prevents cross-contamination
- Stateless service design for easy scaling
- Checkpointing for resumable operations

## Security Architecture

### Authentication Flows
- JIRA: API Token or Personal Access Token
- Git: SSH keys or HTTPS with tokens
- UI: JWT with session management
- Service-to-service: Kubernetes service accounts

### Secrets Management
- Local: .env files (gitignored)
- Production: Kubernetes mounted secret volumes
- Rotation: Support for credential rotation without downtime
- Least privilege: Bot accounts with minimal required permissions
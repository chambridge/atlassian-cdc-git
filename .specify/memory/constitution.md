<!-- 
SYNC IMPACT REPORT:
Version: 1.0.0 → 1.1.0 (Domain-specific principles added)
Modified principles: Added CDC-specific principles, technology stack constraints
Added sections: CDC System Principles, Technology Stack, Security Standards
Removed sections: N/A
Templates requiring updates: 
  ✅ plan-template.md (updated)
  ✅ spec-template.md (updated)  
  ✅ tasks-template.md (updated)
  ✅ agent-file-template.md (updated)
Follow-up TODOs: None
-->

# Atlassian CDC Git Constitution

## Core Principles

### I. Specification-First Development
Every feature MUST begin with a complete specification in `/specs/[###-feature-name]/spec.md` that defines user scenarios, functional requirements, and acceptance criteria before any planning or implementation begins. Specifications MUST be business-focused, avoiding implementation details, and contain no unresolved [NEEDS CLARIFICATION] markers.

*Rationale: Clear requirements prevent scope creep and ensure all stakeholders understand what will be built before resources are committed.*

### II. Test-Driven Implementation (NON-NEGOTIABLE)
Implementation MUST follow strict TDD: contract tests and integration tests written first → tests MUST fail → then implement to make tests pass. All tests generated from specifications and design contracts during the planning phase.

*Rationale: TDD ensures code meets requirements, prevents regressions, and provides living documentation of system behavior.*

### III. Three-Phase Workflow
Every feature MUST follow: **Phase 1** - Specification (`/spec` command) → **Phase 2** - Planning (`/plan` command) → **Phase 3** - Task execution (`/tasks` command). Each phase has defined inputs, outputs, and gates that MUST pass before proceeding.

*Rationale: Structured workflow prevents incomplete features and ensures consistent quality across all development work.*

### IV. Constitution-Driven Quality Gates
All plans MUST pass Constitution Check gates before and after design phases. Any violations MUST be documented in Complexity Tracking with explicit justification for why simpler alternatives were rejected.

*Rationale: Maintains architectural consistency and prevents unnecessary complexity from accumulating over time.*

### V. Auto-Generated Development Context
Project structure, technology choices, and development guidelines MUST be auto-generated from completed plans and maintained in agent-specific files (e.g., `CLAUDE.md`). Manual additions preserved between marked sections.

*Rationale: Ensures development context stays current with actual implementation and reduces cognitive load for contributors.*

## CDC System Principles

### VI. Polling-First Data Consistency
Polling JIRA APIs MUST be treated as the source of truth over webhook data. All data synchronization MUST prioritize synchronous polling results when conflicts arise. Webhook events serve as optimization triggers but cannot override polling-derived state.

*Rationale: Synchronous polling provides reliable, consistent data while webhooks may be unreliable or out-of-order.*

### VII. Progressive Bootstrap Strategy
Initial JIRA project synchronization MUST prioritize active issues (non-closed/done states) and implement resumable operations. Bootstrap processes MUST respect rate limits, support selective issue filtering, and provide progress tracking. Historical closed issues sync via separate background processes.

*Rationale: Ensures rapid time-to-value while respecting JIRA API limits and providing operational visibility.*

### VIII. Git Storage Consistency
Each JIRA issue MUST map to exactly one file using issue key as primary identifier. Directory structure MUST use symbolic links for hierarchy representation due to JIRA issue mobility. All commits MUST follow conventional commit standards for CDC operations.

*Rationale: Maintains clear data lineage while accommodating JIRA's fluid organizational structure.*

## Technology Stack

### Required Technologies
- **Language**: Go (mandatory for all components)
- **Orchestration**: Kubernetes for deployment and job management
- **Git Integration**: Best-of-breed Go git libraries
- **Configuration**: Environment variables (.env for local) and Kubernetes secrets (production)

### Authentication Standards
- **JIRA Access**: Support both API Token and Personal Access Token flows
- **Git Access**: Bot account with minimal write/push permissions to target repositories
- **Proxy Support**: HTTP SQUID proxy configuration for staging JIRA instances

## Security Standards

### Secrets Management
- Local development MUST support .env file configuration
- Production deployment MUST use Kubernetes mounted secret volumes
- No hardcoded credentials in source code or container images
- Bot accounts MUST operate with least-privilege access patterns

### API Security
- JIRA authentication MUST support token rotation
- Exponential backoff MUST be implemented for rate limit compliance
- Git repository access MUST be limited to designated bot accounts

## Development Workflow

### Planning Standards
- All features MUST have branch name format: `[type]/[###-feature-name]`
- Research phase MUST resolve all "NEEDS CLARIFICATION" markers before design
- Design phase MUST generate failing tests, contracts, and data models
- Task generation MUST follow TDD ordering: setup → tests → implementation → polish

### Quality Requirements
- Contract tests for ALL API endpoints
- Integration tests with mock JIRA APIs for all CDC operations
- End-to-end tests using temporary git repositories (local) and remote git servers (production)
- Models for ALL key entities identified in specifications
- Performance constraints explicitly documented and tested
- Error handling patterns consistently applied
- Progress tracking endpoints for long-running operations (bootstrap sync)
- Observability metrics for CDC health and data consistency

### Documentation Standards
- Specifications written for business stakeholders, not developers
- Plans include technical decisions with rationale and alternatives considered
- Tasks specify exact file paths and parallel execution capabilities
- Agent files updated incrementally with each feature (O(1) operation)

## Quality Standards

### Code Organization
- Single project structure by default: `src/`, `tests/` at repository root
- Web applications: `backend/src/`, `frontend/src/`
- Mobile applications: `api/src/`, platform-specific structure
- Clear separation: models → services → CLI → API layers

### Testing Requirements  
- Tests MUST be written before implementation (TDD enforcement)
- Contract tests for external interfaces
- Integration tests for user workflows
- Unit tests for business logic
- Performance tests for defined constraints

### Complexity Management
- Prefer composition over inheritance
- Extract reusable utilities when patterns emerge
- Document complexity deviations with justification
- Regular refactoring to maintain simplicity

## CDC Deployment Architecture

### Service Topology
- One service instance per JIRA project (no multi-tenancy)
- Support for staging and production JIRA instances
- HTTP SQUID proxy support for staging environments
- Kubernetes job-based bootstrap operations

### Operational Requirements
- Slow and steady processing approach with configurable rate limits
- Progress tracking with "X of Y complete + percentage" for bootstrap operations
- Status endpoints for operational monitoring
- Graceful handling of JIRA API outages with eventual consistency
- Resumable operations for interrupted bootstrap processes

### Configuration Management
- Environment-specific configuration (staging vs production JIRA)
- Per-project deployment isolation
- Scalable architecture for high-volume JIRA projects
- Health check endpoints for Kubernetes readiness/liveness probes

## Governance

### Amendment Process
Constitution changes require:
1. Documented rationale for the change
2. Impact analysis on existing workflows
3. Migration plan for affected features
4. Version bump following semantic versioning

### Compliance Review
- All pull requests MUST verify constitutional compliance
- Complexity deviations MUST be explicitly justified
- Quality gates MUST pass before feature completion
- Regular audits of adherence to principles

### Version Control
- MAJOR: Breaking changes to workflow or principle redefinitions
- MINOR: New principles added or materially expanded guidance  
- PATCH: Clarifications, wording improvements, non-semantic refinements

**Version**: 1.1.0 | **Ratified**: 2025-09-19 | **Last Amended**: 2025-09-19
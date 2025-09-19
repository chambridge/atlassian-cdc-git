# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Atlassian CDC Git is a Change Data Capture system that mirrors JIRA work items (RFEs, Features, Epics, User Stories, Tasks, Bugs) into git repositories. This enables agentic swarm development where autonomous agents access structured project context while maintaining JIRA as the authoritative source of truth. The system supports human-agent collaboration workflows where humans refine agent-generated deliverables in JIRA, and agents consume updated git-mirrored data for implementation tasks.

## Architecture

### Core System Design
- **JIRA CDC Engine**: Continuously monitors JIRA via polling and webhooks, capturing changes into git repositories
- **Git Storage Layer**: One file per JIRA issue using issue key as identifier, structured for read-only submodule consumption by agents
- **Bootstrap Service**: Initial synchronization of active JIRA projects to new git repositories
- **Reconciliation Service**: Detects and corrects data inconsistencies between JIRA and git
- **Monitoring & Health**: Web interface and API endpoints for operational visibility

### Agent Integration
- Git repositories structured as read-only submodules for agent development workflows
- Conventional commit format for traceability and agent memory persistence
- Hierarchical issue relationships maintained via symbolic links (Release 2.0+)
- Validation state metadata to distinguish human-validated vs agent-generated content

## Development Workflow

### Specification-Driven Development (Mandatory)
This project follows strict spec-kit workflow principles:

1. **Specification Phase**: Use `/specify` command to create complete specifications in `/specs/[###-feature-name]/spec.md`
2. **Planning Phase**: Use `/plan` command to generate technical implementation plans
3. **Task Phase**: Use `/tasks` command to break down plans into actionable tasks
4. **Implementation Phase**: Follow TDD with contract tests written first

### Constitution Compliance
All development MUST adhere to the project constitution (`.specify/memory/constitution.md`):
- Test-Driven Implementation (NON-NEGOTIABLE): Tests written first → tests fail → implement to pass
- Constitution Check gates required before and after design phases
- Quality gates must pass before feature completion

## Technology Stack

### Core Technologies (Mandatory)
- **Language**: Go for all components
- **Orchestration**: Kubernetes for deployment and job management
- **Git Integration**: Best-of-breed Go git libraries
- **Configuration**: Environment variables (.env for local), Kubernetes secrets (production)

### Authentication & Security
- JIRA Access: API Token and Personal Access Token flows
- Git Access: Bot account with minimal write/push permissions
- Proxy Support: HTTP SQUID proxy for staging JIRA instances
- Secrets: No hardcoded credentials, .env for local, K8s mounted volumes for production

## Key Commands

### Specify Framework Commands
```bash
# Create new feature specification
/specify "feature description"

# Generate technical implementation plan
/plan

# Create actionable task breakdown
/tasks

# Execute implementation
/implement
```

### Development Setup
Currently in specification phase - no build commands yet. When implementation begins:
- Go modules for dependency management
- Kubernetes manifests for deployment
- Health check endpoints for monitoring

## Current Status

**Active Specification**: `specs/001-create-a-system/` contains the primary system specification and MVP planning document. The system is designed for 12-16 week MVP implementation focusing on core CDC functionality with basic agent integration.

**MVP Constraints**: Single JIRA project support, polling-only synchronization, flat file structure, basic error handling (advanced features deferred to Release 2.0+).

## Agent Workflow Integration

When implementing agent integration features:
- Structure git repositories for submodule consumption
- Preserve JIRA hierarchy and relationships
- Include change history for agent memory continuity
- Support graceful degradation during JIRA outages (agents receive stale-but-functional data)
- Implement change notification strategies for rapid iteration cycles
# JIRA CDC Kubernetes Operator

A Kubernetes operator that provides Change Data Capture (CDC) functionality from JIRA to Git repositories, enabling automated synchronization of JIRA issues to Git-based agent systems.

## Overview

The JIRA CDC Operator implements a polling-first CDC architecture with webhook optimization triggers, designed to maintain current JIRA issue state in Git repositories for consumption by AI agents and other automated systems.

### Architecture

- **Kubernetes Operator**: Declarative configuration via Custom Resource Definitions (CRDs)
- **Operand Management**: Automatic deployment of API and UI operands
- **Sync Engine**: Efficient JIRA-to-Git synchronization with rate limiting and error handling
- **Agent Integration**: Git repositories structured as read-only submodules for agent consumption

### Key Features

- ✅ **Declarative Configuration**: CRD-based project configuration
- ✅ **Real-time Synchronization**: 5-minute default polling with webhook triggers
- ✅ **High Performance**: <30 minutes bootstrap for 1000 active issues
- ✅ **Rate Limit Compliance**: Built-in JIRA API rate limiting and exponential backoff
- ✅ **Agent-Friendly**: Flat file structure with YAML frontmatter and markdown content
- ✅ **Monitoring**: Prometheus metrics, structured logging, and health checks
- ✅ **Security**: Kubernetes secrets integration, least-privilege RBAC

## Quick Start

### Prerequisites

- Kubernetes cluster (1.25+)
- Go 1.21+ (for development)
- kubectl
- Docker
- JIRA instance with API access
- Git repository with write access

### Installation

1. **Install the Operator**

```bash
# Deploy CRDs
kubectl apply -f https://raw.githubusercontent.com/company/jira-cdc-operator/main/config/crd/bases/jiracdc.io_jiracdc.yaml

# Deploy RBAC and operator
kubectl apply -f https://raw.githubusercontent.com/company/jira-cdc-operator/main/config/rbac/
kubectl apply -f https://raw.githubusercontent.com/company/jira-cdc-operator/main/config/manager/manager.yaml
```

2. **Create Secrets**

```bash
# JIRA credentials
kubectl create secret generic jira-credentials \
  --from-literal=username=your-email@company.com \
  --from-literal=apiToken=your-jira-api-token \
  -n jiracdc-system

# Git credentials (SSH)
kubectl create secret generic git-credentials \
  --from-file=ssh-privatekey=/path/to/ssh/key \
  -n jiracdc-system
```

3. **Create JiraCDC Resource**

```yaml
apiVersion: jiracdc.io/v1
kind: JiraCDC
metadata:
  name: my-project
  namespace: jiracdc-system
spec:
  jiraInstance:
    baseURL: "https://company.atlassian.net"
    credentialsSecret: "jira-credentials"
  syncTarget:
    type: "project"
    projectKey: "MYPROJ"
  gitRepository:
    url: "git@github.com:company/myproj-mirror.git"
    credentialsSecret: "git-credentials"
    branch: "main"
  operands:
    api:
      enabled: true
      replicas: 2
    ui:
      enabled: true
      replicas: 1
  syncConfig:
    interval: "5m"
    bootstrap: true
    activeIssuesOnly: true
```

```bash
kubectl apply -f jiracdc-config.yaml
```

4. **Verify Deployment**

```bash
# Check operator status
kubectl get pods -n jiracdc-system
kubectl get jiracdc -n jiracdc-system

# Access API (via port-forward)
kubectl port-forward svc/jiracdc-api-my-project 8080:8080 -n jiracdc-system
curl http://localhost:8080/api/v1/health

# Access UI (via port-forward)
kubectl port-forward svc/jiracdc-ui-my-project 3000:3000 -n jiracdc-system
# Open http://localhost:3000 in browser
```

## Usage

### Monitoring Synchronization

```bash
# Check project status
curl http://localhost:8080/api/v1/projects/MYPROJ

# List sync tasks
curl http://localhost:8080/api/v1/tasks

# Monitor task progress
curl http://localhost:8080/api/v1/tasks/{task-id}
```

### Agent Integration

```bash
# Clone mirrored repository
git clone git@github.com:company/myproj-mirror.git

# Add as submodule to agent project
cd agent-project
git submodule add git@github.com:company/myproj-mirror.git jira-data
git submodule update --init

# Access issue data
cat jira-data/MYPROJ-123.md
```

### Manual Synchronization

```bash
# Trigger bootstrap
curl -X POST http://localhost:8080/api/v1/projects/MYPROJ/sync \
  -H "Content-Type: application/json" \
  -d '{"type": "bootstrap", "forceRefresh": true}'

# Trigger reconciliation
curl -X POST http://localhost:8080/api/v1/projects/MYPROJ/sync \
  -H "Content-Type: application/json" \
  -d '{"type": "reconciliation", "issueFilter": "updated >= -1d"}'
```

## Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/company/jira-cdc-operator.git
cd jira-cdc-operator

# Install dependencies
go mod download

# Generate code and manifests
make generate
make manifests

# Build operator
make build

# Build container images
make docker-build IMG=jiracdc/operator:dev
make docker-build-api IMG=jiracdc/api:dev
make docker-build-ui IMG=jiracdc/ui:dev
```

### Running Tests

```bash
# Unit tests
make test

# Integration tests
make test-integration

# End-to-end tests (requires test environment)
make test-e2e

# Performance tests
make test-performance

# Coverage report
make test-coverage
```

### Local Development

```bash
# Install CRDs
make install

# Run operator locally
make run

# Deploy to development cluster
make deploy IMG=jiracdc/operator:dev
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the Apache License 2.0 - see [LICENSE](LICENSE) for details.

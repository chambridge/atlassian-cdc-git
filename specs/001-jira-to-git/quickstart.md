# Quickstart Guide: JIRA CDC Kubernetes Operator

## Prerequisites

### Required Tools
- Go 1.21+ (for operator development)
- Docker and Kubernetes cluster
- kubectl (for Kubernetes deployment)
- git
- kubebuilder (for operator framework)

### Required Access
- JIRA instance with API Token or Personal Access Token
- Git repository with write access
- Kubernetes cluster with RBAC permissions for operator deployment

## Kubernetes Operator Deployment

### 1. Create Kubernetes Secrets
Create secrets for JIRA and Git credentials:
```bash
# Create namespace
kubectl create namespace jiracdc-system

# Create JIRA credentials secret
kubectl create secret generic jira-credentials \
  --from-literal=username=your-email@company.com \
  --from-literal=apiToken=your-api-token \
  -n jiracdc-system

# Create Git credentials secret (SSH key)
kubectl create secret generic git-credentials \
  --from-file=ssh-privatekey=/path/to/your/ssh/key \
  --from-file=ssh-publickey=/path/to/your/ssh/key.pub \
  -n jiracdc-system

# Or for HTTPS Git access
kubectl create secret generic git-credentials \
  --from-literal=username=git-user \
  --from-literal=token=your-git-token \
  -n jiracdc-system
```

### 2. Deploy the Operator
```bash
# Apply CRD definition
kubectl apply -f config/crd/bases/jiracdc.io_jiracdc.yaml

# Deploy operator
kubectl apply -f config/rbac/
kubectl apply -f config/manager/manager.yaml

# Verify operator is running
kubectl get pods -n jiracdc-system
kubectl logs -n jiracdc-system deployment/jiracdc-controller-manager
```

### 3. Create JiraCDC Resource
Create a JiraCDC custom resource to configure synchronization:
```yaml
# jiracdc-sample.yaml
apiVersion: jiracdc.io/v1
kind: JiraCDC
metadata:
  name: sample-project
  namespace: jiracdc-system
spec:
  jiraInstance:
    baseURL: "https://your-company.atlassian.net"
    credentialsSecret: "jira-credentials"
  syncTarget:
    type: "project"
    projectKey: "PROJ"
  gitRepository:
    url: "git@github.com:company/proj-mirror.git"
    credentialsSecret: "git-credentials"
    branch: "main"
  operands:
    api:
      enabled: true
      replicas: 2
      resources:
        requests:
          cpu: "100m"
          memory: "128Mi"
        limits:
          cpu: "500m"
          memory: "512Mi"
    ui:
      enabled: true
      replicas: 1
      resources:
        requests:
          cpu: "50m"
          memory: "64Mi"
        limits:
          cpu: "200m"
          memory: "256Mi"
  syncConfig:
    interval: "5m"
    bootstrap: true
    activeIssuesOnly: true
```

Apply the resource:
```bash
kubectl apply -f jiracdc-sample.yaml
```

### 4. Verify Deployment
```bash
# Check JiraCDC resource status
kubectl get jiracdc -n jiracdc-system
kubectl describe jiracdc sample-project -n jiracdc-system

# Check managed operands
kubectl get pods -l app.kubernetes.io/managed-by=jiracdc-operator -n jiracdc-system

# Check API operand service
kubectl get svc jiracdc-api-sample-project -n jiracdc-system

# Check UI operand service
kubectl get svc jiracdc-ui-sample-project -n jiracdc-system
```

## Usage Examples

### Access API Operand
```bash
# Port-forward to access API locally
kubectl port-forward svc/jiracdc-api-sample-project 8080:8080 -n jiracdc-system

# Get project status
curl http://localhost:8080/api/v1/projects/PROJ

# List all synchronized projects
curl http://localhost:8080/api/v1/projects
```

### Trigger Manual Synchronization
```bash
# Trigger reconciliation for a project
curl -X POST http://localhost:8080/api/v1/projects/PROJ/sync \
  -H "Content-Type: application/json" \
  -d '{
    "type": "reconciliation",
    "forceRefresh": false,
    "issueFilter": "status != Done AND updated >= -7d"
  }'
```

### Monitor Task Progress
```bash
# List all tasks
curl http://localhost:8080/api/v1/tasks

# Get specific task details
curl http://localhost:8080/api/v1/tasks/{taskId}

# Cancel a running task
curl -X POST http://localhost:8080/api/v1/tasks/{taskId}/cancel
```

### Access Web UI
```bash
# Port-forward to access UI locally
kubectl port-forward svc/jiracdc-ui-sample-project 3000:3000 -n jiracdc-system

# Open browser to http://localhost:3000
```

### Access Mirrored Data as Agent
```bash
# Clone the mirrored repository
git clone git@github.com:company/proj-mirror.git

# Add as submodule to agent project
cd agent-project
git submodule add git@github.com:company/proj-mirror.git jira-data
git submodule update --init

# Access issue data
cat jira-data/PROJ-123.md
```

## Integration Testing

### Test Scenarios

#### 1. Operator Deployment Test
```bash
#!/bin/bash
# Test operator deployment and JiraCDC resource creation

# Verify operator is running
kubectl wait --for=condition=Available deployment/jiracdc-controller-manager \
  -n jiracdc-system --timeout=300s

# Create test JiraCDC resource
kubectl apply -f - <<EOF
apiVersion: jiracdc.io/v1
kind: JiraCDC
metadata:
  name: test-project
  namespace: jiracdc-system
spec:
  jiraInstance:
    baseURL: "https://test.atlassian.net"
    credentialsSecret: "jira-credentials"
  syncTarget:
    type: "project"
    projectKey: "TEST"
  gitRepository:
    url: "git@github.com:company/test-mirror.git"
    credentialsSecret: "git-credentials"
  syncConfig:
    interval: "2m"
    bootstrap: true
EOF

# Wait for operands to be ready
kubectl wait --for=condition=Ready pod \
  -l app.kubernetes.io/managed-by=jiracdc-operator \
  -n jiracdc-system --timeout=300s

echo "Operator deployment test completed successfully"
```

#### 2. Bootstrap Operation Test
```bash
#!/bin/bash
# Test bootstrap operation via API

# Port forward to API
kubectl port-forward svc/jiracdc-api-test-project 8080:8080 -n jiracdc-system &
PORT_FORWARD_PID=$!
sleep 5

# Trigger bootstrap sync
TASK_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/projects/TEST/sync \
  -H "Content-Type: application/json" \
  -d '{"type": "bootstrap", "forceRefresh": true}')

TASK_ID=$(echo $TASK_RESPONSE | jq -r .taskId)
echo "Started bootstrap task: $TASK_ID"

# Monitor task progress
while true; do
  TASK_STATUS=$(curl -s http://localhost:8080/api/v1/tasks/$TASK_ID | jq -r .status)
  PROGRESS=$(curl -s http://localhost:8080/api/v1/tasks/$TASK_ID | jq -r .progress.percentComplete)
  
  echo "Task status: $TASK_STATUS ($PROGRESS% complete)"
  
  if [ "$TASK_STATUS" = "completed" ]; then
    echo "Bootstrap completed successfully"
    break
  elif [ "$TASK_STATUS" = "failed" ]; then
    echo "Bootstrap failed"
    curl -s http://localhost:8080/api/v1/tasks/$TASK_ID | jq .errorMessage
    kill $PORT_FORWARD_PID
    exit 1
  fi
  sleep 10
done

kill $PORT_FORWARD_PID
```

#### 3. Agent Integration Test
```bash
#!/bin/bash
# Test agent submodule integration

# Get git repository URL from JiraCDC resource
GIT_REPO_URL=$(kubectl get jiracdc test-project -n jiracdc-system -o jsonpath='{.spec.gitRepository.url}')

# Create agent project with submodule
mkdir agent-test
cd agent-test
git init
git submodule add $GIT_REPO_URL jira-data
git submodule update --init

# Verify agent can read issue data
if [ -d "jira-data" ] && [ -n "$(find jira-data -name '*.md' | head -1)" ]; then
  SAMPLE_FILE=$(find jira-data -name '*.md' | head -1)
  echo "Agent can access JIRA data via submodule"
  echo "Sample file: $SAMPLE_FILE"
  head -10 "$SAMPLE_FILE"
else
  echo "Agent integration test failed - no issue files found"
  exit 1
fi

# Test staleness detection
SAMPLE_FILE=$(find jira-data -name '*.md' | head -1)
if [ -f "$SAMPLE_FILE" ]; then
  SYNC_TIME=$(grep -o 'syncedAt: .*' "$SAMPLE_FILE" | head -1 | cut -d' ' -f2)
  echo "Last sync: $SYNC_TIME"
fi
```

## Production Deployment

### Deploy to Production Cluster
```bash
# Create production namespace
kubectl create namespace jiracdc-production

# Create production secrets
kubectl create secret generic jira-credentials \
  --from-literal=username=prod-jira@company.com \
  --from-literal=apiToken=$PROD_JIRA_TOKEN \
  -n jiracdc-production

kubectl create secret generic git-credentials \
  --from-file=ssh-privatekey=/secure/path/to/prod-ssh-key \
  -n jiracdc-production

# Deploy operator to production
kubectl apply -f config/crd/bases/jiracdc.io_jiracdc.yaml
kubectl apply -f config/rbac/ -n jiracdc-production
kubectl apply -f config/manager/manager.yaml -n jiracdc-production

# Create production JiraCDC resources
kubectl apply -f production-jiracdc-configs/
```

### Health Checks
```bash
# Check operator health
kubectl get pods -n jiracdc-production
kubectl logs deployment/jiracdc-controller-manager -n jiracdc-production

# Check JiraCDC resources
kubectl get jiracdc -n jiracdc-production
kubectl describe jiracdc -n jiracdc-production

# Check operand health via API
kubectl port-forward svc/jiracdc-api-production 8080:8080 -n jiracdc-production
curl http://localhost:8080/api/v1/health
curl http://localhost:8080/api/v1/metrics
```"

## Troubleshooting

### Common Issues

#### Operator Not Starting
```bash
# Check operator logs
kubectl logs deployment/jiracdc-controller-manager -n jiracdc-system

# Check RBAC permissions
kubectl auth can-i create jiracdc --as=system:serviceaccount:jiracdc-system:jiracdc-controller-manager

# Verify CRD installation
kubectl get crd jiracdc.jiracdc.io
```

#### JiraCDC Resource Issues
```bash
# Check resource status
kubectl describe jiracdc sample-project -n jiracdc-system

# Check operator events
kubectl get events -n jiracdc-system --sort-by='.lastTimestamp'

# Check secret references
kubectl get secret jira-credentials -n jiracdc-system
kubectl get secret git-credentials -n jiracdc-system
```

#### Operand Deployment Failures
```bash
# Check operand pod status
kubectl get pods -l app.kubernetes.io/managed-by=jiracdc-operator -n jiracdc-system
kubectl describe pod -l app.kubernetes.io/managed-by=jiracdc-operator -n jiracdc-system

# Check operand logs
kubectl logs -l app.kubernetes.io/name=jiracdc-api -n jiracdc-system
kubectl logs -l app.kubernetes.io/name=jiracdc-ui -n jiracdc-system
```

#### JIRA Authentication Failed
```bash
# Test JIRA credentials from within cluster
kubectl exec -it deployment/jiracdc-api-sample-project -n jiracdc-system -- \
  curl -u "username:token" "https://company.atlassian.net/rest/api/3/myself"

# Check secret content (base64 encoded)
kubectl get secret jira-credentials -n jiracdc-system -o yaml
```

#### Git Access Issues
```bash
# Test git access from operand pod
kubectl exec -it deployment/jiracdc-api-sample-project -n jiracdc-system -- \
  git clone git@github.com:company/test-repo.git /tmp/test-clone

# Check SSH key format in secret
kubectl get secret git-credentials -n jiracdc-system -o jsonpath='{.data.ssh-privatekey}' | base64 -d | head -1
```

### Debug Commands
```bash
# Access API operand for debugging
kubectl port-forward svc/jiracdc-api-sample-project 8080:8080 -n jiracdc-system

# Check system health
curl http://localhost:8080/api/v1/health

# Monitor task progress
curl http://localhost:8080/api/v1/tasks | jq '.[].status'

# Check specific issue sync status
curl http://localhost:8080/api/v1/issues/PROJ-123
```

## Performance Targets

### Expected Performance
- **Bootstrap**: <30 minutes for 1000 active issues
- **Real-time sync**: <5 minutes from JIRA change to git commit
- **System availability**: 99.5% during business hours
- **Storage efficiency**: <10MB git repository per 100 JIRA issues

### Monitoring
```bash
# Set up Prometheus ServiceMonitor for metrics collection
kubectl apply -f - <<EOF
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: jiracdc-metrics
  namespace: jiracdc-system
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: jiracdc-api
  endpoints:
  - port: http-metrics
    path: /metrics
EOF

# Configure alerts for sync lag > 10 minutes
# Monitor JIRA API rate limit compliance via /metrics endpoint
# Track git repository size growth via operand status
```
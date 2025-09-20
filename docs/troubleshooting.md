# Troubleshooting Guide: JIRA CDC Kubernetes Operator

This guide provides comprehensive troubleshooting information for the JIRA CDC Kubernetes Operator, covering common issues, diagnostic commands, and resolution steps.

## Table of Contents

1. [Operator Deployment Issues](#operator-deployment-issues)
2. [JiraCDC Resource Issues](#jiracdc-resource-issues)
3. [Operand Deployment Problems](#operand-deployment-problems)
4. [Authentication and Authorization](#authentication-and-authorization)
5. [Synchronization Issues](#synchronization-issues)
6. [Performance Problems](#performance-problems)
7. [Network and Connectivity](#network-and-connectivity)
8. [Monitoring and Observability](#monitoring-and-observability)
9. [Data Consistency Issues](#data-consistency-issues)
10. [Recovery Procedures](#recovery-procedures)

## Operator Deployment Issues

### Operator Pod Not Starting

**Symptoms:**
- Operator pod stuck in `Pending`, `CrashLoopBackOff`, or `Error` state
- No controller manager logs available

**Diagnostic Commands:**
```bash
# Check operator pod status
kubectl get pods -n jiracdc-system
kubectl describe pod -l app.kubernetes.io/name=jiracdc-operator -n jiracdc-system

# Check events
kubectl get events -n jiracdc-system --sort-by='.lastTimestamp'

# Check deployment status
kubectl get deployment jiracdc-controller-manager -n jiracdc-system
kubectl describe deployment jiracdc-controller-manager -n jiracdc-system
```

**Common Causes and Solutions:**

1. **Resource Constraints**
   ```bash
   # Check node resources
   kubectl describe nodes
   kubectl top nodes
   
   # Solution: Scale cluster or adjust resource requests
   kubectl patch deployment jiracdc-controller-manager -n jiracdc-system -p '{"spec":{"template":{"spec":{"containers":[{"name":"manager","resources":{"requests":{"memory":"64Mi","cpu":"50m"}}}]}}}}'
   ```

2. **Image Pull Issues**
   ```bash
   # Check image pull secrets
   kubectl get secrets -n jiracdc-system
   
   # Solution: Create image pull secret if needed
   kubectl create secret docker-registry regcred \
     --docker-server=registry.example.com \
     --docker-username=username \
     --docker-password=password \
     -n jiracdc-system
   ```

3. **RBAC Permissions**
   ```bash
   # Check service account
   kubectl get serviceaccount jiracdc-controller-manager -n jiracdc-system
   
   # Check cluster role bindings
   kubectl get clusterrolebinding | grep jiracdc
   
   # Solution: Reapply RBAC manifests
   kubectl apply -f config/rbac/
   ```

### CRD Installation Problems

**Symptoms:**
- `no matches for kind "JiraCDC"` errors
- CRD validation failures

**Diagnostic Commands:**
```bash
# Check CRD installation
kubectl get crd jiracdc.jiracdc.io
kubectl describe crd jiracdc.jiracdc.io

# Check CRD versions
kubectl get crd jiracdc.jiracdc.io -o yaml | grep -A 10 versions
```

**Solutions:**
```bash
# Reinstall CRDs
kubectl apply -f config/crd/bases/jiracdc.io_jiracdc.yaml

# Force update if needed
kubectl replace -f config/crd/bases/jiracdc.io_jiracdc.yaml

# Check API server recognition
kubectl api-resources | grep jiracdc
```

## JiraCDC Resource Issues

### Resource Creation Failures

**Symptoms:**
- `kubectl apply` fails with validation errors
- JiraCDC resource stuck in `Pending` state

**Diagnostic Commands:**
```bash
# Check resource status
kubectl get jiracdc -A
kubectl describe jiracdc <name> -n <namespace>

# Check controller logs
kubectl logs deployment/jiracdc-controller-manager -n jiracdc-system

# Validate resource definition
kubectl apply --dry-run=client -f jiracdc-config.yaml
```

**Common Issues:**

1. **Invalid Configuration**
   ```bash
   # Check required fields
   kubectl explain jiracdc.spec
   
   # Validate JIRA URL format
   curl -I https://company.atlassian.net/rest/api/3/myself
   ```

2. **Secret References**
   ```bash
   # Check secret existence
   kubectl get secret jira-credentials -n <namespace>
   kubectl get secret git-credentials -n <namespace>
   
   # Verify secret format
   kubectl get secret jira-credentials -o yaml
   ```

3. **Namespace Issues**
   ```bash
   # Ensure namespace exists
   kubectl create namespace jiracdc-system
   
   # Check cross-namespace secret access
   kubectl get secret jira-credentials -n jiracdc-system
   ```

### Status Not Updating

**Symptoms:**
- JiraCDC status remains empty or stale
- Phase stuck in `Pending`

**Diagnostic Commands:**
```bash
# Check controller reconciliation
kubectl logs deployment/jiracdc-controller-manager -n jiracdc-system -f

# Check finalizers
kubectl get jiracdc <name> -o yaml | grep finalizers

# Force reconciliation
kubectl annotate jiracdc <name> reconcile.jiracdc.io/trigger="$(date)"
```

**Solutions:**
```bash
# Restart controller
kubectl rollout restart deployment/jiracdc-controller-manager -n jiracdc-system

# Check controller leader election
kubectl get lease -n jiracdc-system
```

## Operand Deployment Problems

### API Operand Issues

**Symptoms:**
- API deployment not created or failing
- API pods crashing or not ready

**Diagnostic Commands:**
```bash
# Check API deployment
kubectl get deployment jiracdc-api-<name> -n <namespace>
kubectl describe deployment jiracdc-api-<name> -n <namespace>

# Check API pods
kubectl get pods -l app.kubernetes.io/component=api -n <namespace>
kubectl logs -l app.kubernetes.io/component=api -n <namespace>

# Check API service
kubectl get service jiracdc-api-<name> -n <namespace>
```

**Common Issues:**

1. **Configuration Errors**
   ```bash
   # Check environment variables
   kubectl describe pod <api-pod-name> -n <namespace>
   
   # Check ConfigMap if used
   kubectl get configmap -n <namespace>
   kubectl describe configmap <configmap-name> -n <namespace>
   ```

2. **Health Check Failures**
   ```bash
   # Test health endpoint locally
   kubectl port-forward svc/jiracdc-api-<name> 8080:8080 -n <namespace>
   curl http://localhost:8080/api/v1/health
   
   # Check readiness/liveness probe configuration
   kubectl describe pod <api-pod-name> -n <namespace> | grep -A 5 "Liveness\|Readiness"
   ```

3. **Resource Limits**
   ```bash
   # Check resource usage
   kubectl top pod <api-pod-name> -n <namespace>
   
   # Adjust resource limits
   kubectl patch deployment jiracdc-api-<name> -n <namespace> -p '{"spec":{"template":{"spec":{"containers":[{"name":"jiracdc-api","resources":{"limits":{"memory":"1Gi","cpu":"1000m"}}}]}}}}'
   ```

### UI Operand Issues

**Symptoms:**
- UI deployment failing
- UI not accessible via browser

**Diagnostic Commands:**
```bash
# Check UI deployment and pods
kubectl get deployment jiracdc-ui-<name> -n <namespace>
kubectl get pods -l app.kubernetes.io/component=ui -n <namespace>
kubectl logs -l app.kubernetes.io/component=ui -n <namespace>

# Test UI accessibility
kubectl port-forward svc/jiracdc-ui-<name> 3000:3000 -n <namespace>
curl -I http://localhost:3000
```

**Common Issues:**

1. **Build or Runtime Errors**
   ```bash
   # Check container logs
   kubectl logs <ui-pod-name> -n <namespace>
   
   # Check for JavaScript errors (if accessible)
   # Open browser dev tools at http://localhost:3000
   ```

2. **API Connectivity**
   ```bash
   # Check API service discovery
   kubectl exec <ui-pod-name> -n <namespace> -- nslookup jiracdc-api-<name>
   
   # Test API connectivity from UI pod
   kubectl exec <ui-pod-name> -n <namespace> -- curl http://jiracdc-api-<name>:8080/api/v1/health
   ```

## Authentication and Authorization

### JIRA Authentication Failures

**Symptoms:**
- Sync operations failing with authentication errors
- `401 Unauthorized` in logs

**Diagnostic Commands:**
```bash
# Check JIRA credentials secret
kubectl get secret jira-credentials -o yaml -n <namespace>

# Test JIRA authentication
kubectl exec <api-pod-name> -n <namespace> -- curl -u "$(kubectl get secret jira-credentials -o jsonpath='{.data.username}' | base64 -d):$(kubectl get secret jira-credentials -o jsonpath='{.data.apiToken}' | base64 -d)" https://company.atlassian.net/rest/api/3/myself

# Check API logs for auth errors
kubectl logs -l app.kubernetes.io/component=api -n <namespace> | grep -i auth
```

**Solutions:**

1. **Invalid Credentials**
   ```bash
   # Update JIRA credentials
   kubectl create secret generic jira-credentials \
     --from-literal=username=new-email@company.com \
     --from-literal=apiToken=new-api-token \
     --dry-run=client -o yaml | kubectl apply -f -
   
   # Restart API pods to pick up new credentials
   kubectl rollout restart deployment jiracdc-api-<name> -n <namespace>
   ```

2. **API Token Expired**
   ```bash
   # Generate new API token in JIRA
   # Update secret with new token
   kubectl patch secret jira-credentials -n <namespace> -p '{"data":{"apiToken":"'$(echo -n 'new-token' | base64)'"}}'
   ```

3. **Permissions Issues**
   ```bash
   # Test specific JIRA project access
   kubectl exec <api-pod-name> -n <namespace> -- curl -u "username:token" "https://company.atlassian.net/rest/api/3/project/PROJ"
   ```

### Git Authentication Failures

**Symptoms:**
- Git operations failing with authentication errors
- SSH key or HTTPS token issues

**Diagnostic Commands:**
```bash
# Check Git credentials secret
kubectl get secret git-credentials -o yaml -n <namespace>

# Test Git access
kubectl exec <api-pod-name> -n <namespace> -- git ls-remote <git-url>

# Check SSH key format (if using SSH)
kubectl get secret git-credentials -o jsonpath='{.data.ssh-privatekey}' -n <namespace> | base64 -d | head -1
```

**Solutions:**

1. **SSH Key Issues**
   ```bash
   # Verify SSH key format
   kubectl get secret git-credentials -o jsonpath='{.data.ssh-privatekey}' -n <namespace> | base64 -d > /tmp/key
   chmod 600 /tmp/key
   ssh -i /tmp/key -T git@github.com
   
   # Update SSH key
   kubectl create secret generic git-credentials \
     --from-file=ssh-privatekey=/path/to/new/key \
     --from-file=ssh-publickey=/path/to/new/key.pub \
     --dry-run=client -o yaml | kubectl apply -f -
   ```

2. **HTTPS Token Issues**
   ```bash
   # Test HTTPS access
   git clone https://username:token@github.com/company/repo.git /tmp/test-clone
   
   # Update HTTPS credentials
   kubectl patch secret git-credentials -n <namespace> -p '{"data":{"token":"'$(echo -n 'new-token' | base64)'"}}'
   ```

## Synchronization Issues

### Sync Operations Failing

**Symptoms:**
- Bootstrap or reconciliation tasks failing
- Issues not appearing in Git repository

**Diagnostic Commands:**
```bash
# Check sync task status
curl http://localhost:8080/api/v1/tasks
curl http://localhost:8080/api/v1/tasks/<task-id>

# Check sync engine logs
kubectl logs -l app.kubernetes.io/component=api -n <namespace> | grep -i sync

# Check JIRA connectivity
curl http://localhost:8080/api/v1/health
```

**Common Issues:**

1. **JIRA API Rate Limiting**
   ```bash
   # Check rate limit status
   kubectl logs -l app.kubernetes.io/component=api -n <namespace> | grep -i "rate limit\|429"
   
   # Adjust rate limit configuration
   kubectl patch jiracdc <name> -n <namespace> --type merge -p '{"spec":{"syncConfig":{"retryPolicy":{"maxRetries":5,"backoffMultiplier":3.0}}}}'
   ```

2. **Git Repository Issues**
   ```bash
   # Check Git repository accessibility
   kubectl exec <api-pod-name> -n <namespace> -- git ls-remote <git-url>
   
   # Check repository permissions
   kubectl exec <api-pod-name> -n <namespace> -- git clone <git-url> /tmp/test-clone
   ```

3. **JQL Query Issues**
   ```bash
   # Test JQL query manually
   curl -u "username:token" "https://company.atlassian.net/rest/api/3/search?jql=project%20%3D%20PROJ"
   
   # Check JQL syntax
   kubectl logs -l app.kubernetes.io/component=api -n <namespace> | grep -i jql
   ```

### Data Inconsistency

**Symptoms:**
- Git repository missing issues that exist in JIRA
- Stale data in Git repository

**Diagnostic Commands:**
```bash
# Check last sync time
kubectl get jiracdc <name> -o jsonpath='{.status.lastSyncTime}' -n <namespace>

# Check sync count vs JIRA count
curl http://localhost:8080/api/v1/projects/<project-key>

# Force full reconciliation
curl -X POST http://localhost:8080/api/v1/projects/<project-key>/sync \
  -H "Content-Type: application/json" \
  -d '{"type": "bootstrap", "forceRefresh": true}'
```

**Solutions:**
```bash
# Trigger manual sync
curl -X POST http://localhost:8080/api/v1/projects/<project-key>/sync \
  -H "Content-Type: application/json" \
  -d '{"type": "reconciliation", "forceRefresh": true}'

# Check for sync configuration issues
kubectl describe jiracdc <name> -n <namespace> | grep -A 10 "Sync Config"
```

## Performance Problems

### Slow Sync Operations

**Symptoms:**
- Bootstrap taking longer than expected
- High CPU or memory usage during sync

**Diagnostic Commands:**
```bash
# Monitor resource usage
kubectl top pod -l app.kubernetes.io/component=api -n <namespace>

# Check sync progress
curl http://localhost:8080/api/v1/tasks | jq '.[] | select(.status == "running")'

# Check performance metrics
curl http://localhost:8080/api/v1/metrics | grep jiracdc_sync
```

**Optimization Steps:**

1. **Adjust Batch Size**
   ```bash
   # Review current configuration
   kubectl get jiracdc <name> -o yaml -n <namespace>
   
   # Optimize batch processing (requires operator update)
   # This would typically be done through sync engine configuration
   ```

2. **Scale API Operand**
   ```bash
   # Increase API replicas
   kubectl patch jiracdc <name> -n <namespace> --type merge -p '{"spec":{"operands":{"api":{"replicas":3}}}}'
   
   # Increase resource limits
   kubectl patch jiracdc <name> -n <namespace> --type merge -p '{"spec":{"operands":{"api":{"resources":{"limits":{"cpu":"2000m","memory":"2Gi"}}}}}}'
   ```

3. **Database Connection Optimization**
   ```bash
   # Monitor connection pool metrics (if applicable)
   curl http://localhost:8080/api/v1/metrics | grep connection
   ```

### Memory Leaks

**Symptoms:**
- Continuously increasing memory usage
- Pods being killed due to OOM

**Diagnostic Commands:**
```bash
# Monitor memory usage over time
while true; do kubectl top pod <api-pod-name> -n <namespace>; sleep 30; done

# Check for memory-related events
kubectl get events -n <namespace> | grep -i "memory\|oom"

# Analyze memory metrics
curl http://localhost:8080/api/v1/metrics | grep -i memory
```

**Solutions:**
```bash
# Restart affected pods
kubectl delete pod <api-pod-name> -n <namespace>

# Implement memory limits
kubectl patch deployment jiracdc-api-<name> -n <namespace> -p '{"spec":{"template":{"spec":{"containers":[{"name":"jiracdc-api","resources":{"limits":{"memory":"1Gi"}}}]}}}}'

# Enable memory profiling (development only)
kubectl port-forward <api-pod-name> 6060:6060 -n <namespace>
go tool pprof http://localhost:6060/debug/pprof/heap
```

## Network and Connectivity

### DNS Resolution Issues

**Symptoms:**
- Cannot resolve JIRA hostname
- Internal service discovery failing

**Diagnostic Commands:**
```bash
# Test DNS resolution from pod
kubectl exec <api-pod-name> -n <namespace> -- nslookup company.atlassian.net
kubectl exec <api-pod-name> -n <namespace> -- nslookup jiracdc-ui-<name>

# Check DNS configuration
kubectl get configmap coredns -n kube-system -o yaml
```

**Solutions:**
```bash
# Use IP address instead of hostname (temporary)
kubectl patch jiracdc <name> -n <namespace> --type merge -p '{"spec":{"jiraInstance":{"baseURL":"https://1.2.3.4"}}}'

# Add custom DNS entry
kubectl patch deployment jiracdc-api-<name> -n <namespace> -p '{"spec":{"template":{"spec":{"hostAliases":[{"ip":"1.2.3.4","hostnames":["company.atlassian.net"]}]}}}}'
```

### Proxy Configuration

**Symptoms:**
- Cannot reach JIRA through corporate proxy
- Proxy authentication failures

**Diagnostic Commands:**
```bash
# Test proxy connectivity
kubectl exec <api-pod-name> -n <namespace> -- curl -v --proxy http://proxy.company.com:8080 https://company.atlassian.net

# Check proxy configuration
kubectl describe jiracdc <name> -n <namespace> | grep -A 5 "Proxy"
```

**Solutions:**
```bash
# Configure proxy in JiraCDC
kubectl patch jiracdc <name> -n <namespace> --type merge -p '{"spec":{"jiraInstance":{"proxyConfig":{"enabled":true,"url":"http://proxy.company.com:8080"}}}}'

# Add proxy credentials if needed
kubectl create secret generic proxy-credentials \
  --from-literal=username=proxy-user \
  --from-literal=password=proxy-pass \
  -n <namespace>
```

## Monitoring and Observability

### Missing Metrics

**Symptoms:**
- Prometheus not scraping metrics
- Metrics endpoint not accessible

**Diagnostic Commands:**
```bash
# Test metrics endpoint
kubectl port-forward svc/jiracdc-api-<name> 8080:8080 -n <namespace>
curl http://localhost:8080/api/v1/metrics

# Check ServiceMonitor (if using Prometheus Operator)
kubectl get servicemonitor -n <namespace>
kubectl describe servicemonitor jiracdc-metrics -n <namespace>
```

**Solutions:**
```bash
# Create ServiceMonitor for Prometheus Operator
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: jiracdc-metrics
  namespace: <namespace>
spec:
  selector:
    matchLabels:
      app.kubernetes.io/component: api
  endpoints:
  - port: http-metrics
    path: /api/v1/metrics
```

### Log Analysis

**Symptoms:**
- Cannot find relevant log entries
- Logs not structured properly

**Diagnostic Commands:**
```bash
# Search logs by component
kubectl logs -l app.kubernetes.io/component=api -n <namespace> | grep ERROR

# Search logs by operation
kubectl logs -l app.kubernetes.io/component=api -n <namespace> | grep "sync\|bootstrap"

# Check log format
kubectl logs <api-pod-name> -n <namespace> | head -10
```

**Log Patterns:**
```bash
# Successful sync operations
kubectl logs -l app.kubernetes.io/component=api -n <namespace> | grep '"level":"info".*"msg":"Sync operation completed"'

# Authentication failures
kubectl logs -l app.kubernetes.io/component=api -n <namespace> | grep '"level":"error".*"auth"'

# Rate limiting events
kubectl logs -l app.kubernetes.io/component=api -n <namespace> | grep '"level":"warn".*"rate.*limit"'
```

## Data Consistency Issues

### Git Repository Corruption

**Symptoms:**
- Git operations failing with corruption errors
- Missing or corrupted issue files

**Diagnostic Commands:**
```bash
# Check Git repository integrity
git clone <git-url> /tmp/repo-check
cd /tmp/repo-check
git fsck --full

# Check recent commits
git log --oneline -10

# Check for conflicting files
git status
git diff
```

**Recovery Steps:**
```bash
# Force push clean state (DESTRUCTIVE - backup first)
git clone <git-url> /tmp/backup-repo
git reset --hard <last-good-commit>
git push --force-with-lease

# Trigger full re-sync
curl -X POST http://localhost:8080/api/v1/projects/<project-key>/sync \
  -H "Content-Type: application/json" \
  -d '{"type": "bootstrap", "forceRefresh": true}'
```

### Issue File Format Issues

**Symptoms:**
- Malformed YAML frontmatter
- Incorrect file encoding

**Diagnostic Commands:**
```bash
# Check file format
git clone <git-url> /tmp/format-check
cd /tmp/format-check
head -20 *.md

# Validate YAML frontmatter
python3 -c "
import yaml
with open('PROJ-123.md', 'r') as f:
    content = f.read()
    parts = content.split('---')
    if len(parts) >= 3:
        yaml.safe_load(parts[1])
    else:
        print('Invalid frontmatter format')
"
```

## Recovery Procedures

### Complete System Recovery

**When to Use:**
- Multiple components failing
- Data corruption detected
- Complete environment rebuild needed

**Steps:**

1. **Backup Current State**
   ```bash
   # Backup Git repositories
   git clone --mirror <git-url> /backup/repo-$(date +%Y%m%d)
   
   # Backup JiraCDC configurations
   kubectl get jiracdc -A -o yaml > /backup/jiracdc-configs-$(date +%Y%m%d).yaml
   
   # Backup secrets
   kubectl get secrets -l app.kubernetes.io/name=jiracdc -A -o yaml > /backup/jiracdc-secrets-$(date +%Y%m%d).yaml
   ```

2. **Clean Environment**
   ```bash
   # Delete all JiraCDC resources
   kubectl delete jiracdc --all -A
   
   # Delete operator
   kubectl delete deployment jiracdc-controller-manager -n jiracdc-system
   
   # Clean up operands (if not automatically cleaned)
   kubectl delete deployment -l app.kubernetes.io/managed-by=jiracdc-operator -A
   ```

3. **Reinstall System**
   ```bash
   # Reinstall CRDs
   kubectl apply -f config/crd/bases/jiracdc.io_jiracdc.yaml
   
   # Reinstall operator
   kubectl apply -f config/rbac/
   kubectl apply -f config/manager/manager.yaml
   
   # Restore secrets
   kubectl apply -f /backup/jiracdc-secrets-$(date +%Y%m%d).yaml
   
   # Restore configurations
   kubectl apply -f /backup/jiracdc-configs-$(date +%Y%m%d).yaml
   ```

4. **Verify Recovery**
   ```bash
   # Check operator status
   kubectl get pods -n jiracdc-system
   
   # Check JiraCDC resources
   kubectl get jiracdc -A
   
   # Trigger bootstrap to rebuild data
   curl -X POST http://localhost:8080/api/v1/projects/<project-key>/sync \
     -H "Content-Type: application/json" \
     -d '{"type": "bootstrap", "forceRefresh": true}'
   ```

### Partial Recovery

**Operand-Only Recovery:**
```bash
# Delete and recreate specific operand
kubectl delete deployment jiracdc-api-<name> -n <namespace>
kubectl annotate jiracdc <name> reconcile.jiracdc.io/trigger="$(date)" -n <namespace>

# Wait for recreation
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=api -n <namespace> --timeout=300s
```

**Data-Only Recovery:**
```bash
# Reset Git repository to clean state
git clone <git-url> /tmp/clean-repo
cd /tmp/clean-repo
git checkout --orphan clean
git rm -rf .
git commit --allow-empty -m "Reset for re-sync"
git push --force-with-lease origin clean:main

# Trigger full bootstrap
curl -X POST http://localhost:8080/api/v1/projects/<project-key>/sync \
  -H "Content-Type: application/json" \
  -d '{"type": "bootstrap", "forceRefresh": true}'
```

## Getting Help

### Collecting Diagnostic Information

**System Information:**
```bash
#!/bin/bash
# diagnostic-collect.sh

NAMESPACE=${1:-jiracdc-system}
OUTPUT_DIR="/tmp/jiracdc-diagnostics-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$OUTPUT_DIR"

# Operator information
kubectl get pods -n $NAMESPACE -o yaml > "$OUTPUT_DIR/operator-pods.yaml"
kubectl logs deployment/jiracdc-controller-manager -n $NAMESPACE > "$OUTPUT_DIR/operator-logs.txt"

# JiraCDC resources
kubectl get jiracdc -A -o yaml > "$OUTPUT_DIR/jiracdc-resources.yaml"

# Operands
kubectl get deployments,services,pods -l app.kubernetes.io/managed-by=jiracdc-operator -A -o yaml > "$OUTPUT_DIR/operands.yaml"

# Events
kubectl get events -A --sort-by='.lastTimestamp' > "$OUTPUT_DIR/events.txt"

# CRD information
kubectl get crd jiracdc.jiracdc.io -o yaml > "$OUTPUT_DIR/crd.yaml"

echo "Diagnostics collected in: $OUTPUT_DIR"
tar -czf "$OUTPUT_DIR.tar.gz" -C "$(dirname $OUTPUT_DIR)" "$(basename $OUTPUT_DIR)"
echo "Archive created: $OUTPUT_DIR.tar.gz"
```

### Support Channels

- **GitHub Issues**: Report bugs and feature requests
- **GitHub Discussions**: Community support and questions
- **Documentation**: Check docs/ directory for detailed guides
- **Enterprise Support**: Contact your Kubernetes platform team

### Best Practices for Issue Reporting

1. **Include Environment Information**
   - Kubernetes version
   - Operator version
   - JIRA instance version
   - Git provider (GitHub, GitLab, etc.)

2. **Provide Reproduction Steps**
   - Exact commands run
   - Configuration files used
   - Expected vs actual behavior

3. **Attach Diagnostics**
   - Relevant logs (sanitized)
   - Resource definitions
   - Error messages

4. **Security Considerations**
   - Remove credentials from logs
   - Sanitize URLs and hostnames
   - Use placeholders for sensitive data
#!/bin/bash
set -euo pipefail

# Security scanning script for JIRA CDC Kubernetes Operator
# This script performs comprehensive security scanning and vulnerability assessment

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "🔒 Starting Security Scanning and Vulnerability Assessment"
echo "Project Root: ${PROJECT_ROOT}"
echo "============================================================"

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Initialize counters
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0
WARNINGS=0

# Function to record check result
record_check() {
    local result=$1
    local message=$2
    
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    case $result in
        "PASS")
            PASSED_CHECKS=$((PASSED_CHECKS + 1))
            print_success "✓ $message"
            ;;
        "FAIL")
            FAILED_CHECKS=$((FAILED_CHECKS + 1))
            print_error "✗ $message"
            ;;
        "WARN")
            WARNINGS=$((WARNINGS + 1))
            print_warning "⚠ $message"
            ;;
    esac
}

# 1. Static Code Analysis Security Tests
print_status "Running static code analysis security tests..."
cd "${PROJECT_ROOT}"

if go test ./test/security/... -v; then
    record_check "PASS" "Static code analysis security tests passed"
else
    record_check "FAIL" "Static code analysis security tests failed"
fi

# 2. Go Security Checker (gosec)
print_status "Checking for gosec installation..."
if ! command_exists gosec; then
    print_warning "gosec not found, installing..."
    if go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; then
        record_check "PASS" "gosec installed successfully"
    else
        record_check "WARN" "Failed to install gosec, skipping security analysis"
    fi
fi

if command_exists gosec; then
    print_status "Running gosec security analysis..."
    if gosec -fmt=text -out=security-report.txt ./...; then
        record_check "PASS" "gosec security analysis completed"
        
        # Check for high severity issues
        if grep -q "Severity: HIGH" security-report.txt 2>/dev/null; then
            record_check "FAIL" "HIGH severity security issues found by gosec"
        else
            record_check "PASS" "No HIGH severity issues found by gosec"
        fi
        
        # Check for medium severity issues
        if grep -q "Severity: MEDIUM" security-report.txt 2>/dev/null; then
            record_check "WARN" "MEDIUM severity security issues found by gosec"
        else
            record_check "PASS" "No MEDIUM severity issues found by gosec"
        fi
    else
        record_check "WARN" "gosec analysis failed or found issues"
    fi
fi

# 3. Dependency Vulnerability Scanning
print_status "Scanning for vulnerable dependencies..."

if command_exists govulncheck; then
    if govulncheck ./...; then
        record_check "PASS" "No vulnerable dependencies found"
    else
        record_check "FAIL" "Vulnerable dependencies detected"
    fi
else
    print_warning "govulncheck not found, installing..."
    if go install golang.org/x/vuln/cmd/govulncheck@latest; then
        if govulncheck ./...; then
            record_check "PASS" "No vulnerable dependencies found"
        else
            record_check "FAIL" "Vulnerable dependencies detected"
        fi
    else
        record_check "WARN" "Failed to install govulncheck, skipping vulnerability scan"
    fi
fi

# 4. RBAC Security Analysis
print_status "Analyzing RBAC permissions..."

RBAC_FILE="${PROJECT_ROOT}/config/rbac/role.yaml"
if [[ -f "$RBAC_FILE" ]]; then
    # Check for wildcard permissions
    if grep -q "- \*" "$RBAC_FILE"; then
        record_check "FAIL" "Wildcard permissions found in RBAC configuration"
    else
        record_check "PASS" "No wildcard permissions in RBAC configuration"
    fi
    
    # Check for excessive secret permissions
    if grep -A5 -B5 "secrets" "$RBAC_FILE" | grep -q "create\|delete\|patch\|update"; then
        record_check "WARN" "Write permissions on secrets detected in RBAC"
    else
        record_check "PASS" "Read-only secret permissions in RBAC"
    fi
    
    # Check for cluster-admin permissions
    if grep -q "cluster-admin" "$RBAC_FILE"; then
        record_check "FAIL" "cluster-admin permissions detected"
    else
        record_check "PASS" "No cluster-admin permissions detected"
    fi
else
    record_check "WARN" "RBAC configuration file not found"
fi

# 5. Hardcoded Secrets Detection
print_status "Scanning for hardcoded secrets..."

# Common secret patterns
SECRET_PATTERNS=(
    'password\s*[:=]\s*["\047][^"\047]{8,}["\047]'
    'token\s*[:=]\s*["\047][^"\047]{10,}["\047]'
    'api[_-]?key\s*[:=]\s*["\047][^"\047]+["\047]'
    'secret\s*[:=]\s*["\047][^"\047]{8,}["\047]'
    'bearer\s+[a-zA-Z0-9_-]{20,}'
    'basic\s+[a-zA-Z0-9+/]{20,}={0,2}'
    '-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----'
)

SECRETS_FOUND=0
for pattern in "${SECRET_PATTERNS[@]}"; do
    if find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -exec grep -l -i -E "$pattern" {} \; 2>/dev/null | head -1 | grep -q .; then
        SECRETS_FOUND=$((SECRETS_FOUND + 1))
    fi
done

if [[ $SECRETS_FOUND -eq 0 ]]; then
    record_check "PASS" "No hardcoded secrets detected"
else
    record_check "FAIL" "Potential hardcoded secrets detected ($SECRETS_FOUND patterns matched)"
fi

# 6. Container Security Best Practices
print_status "Checking container security best practices..."

# Check for Dockerfile security practices
DOCKERFILES=$(find . -name "Dockerfile*" -not -path "./vendor/*")
if [[ -n "$DOCKERFILES" ]]; then
    for dockerfile in $DOCKERFILES; do
        # Check for non-root user
        if grep -q "USER.*[^0]" "$dockerfile"; then
            record_check "PASS" "Non-root user specified in $dockerfile"
        else
            record_check "WARN" "No non-root user specified in $dockerfile"
        fi
        
        # Check for COPY instead of ADD
        if grep -q "^ADD" "$dockerfile"; then
            record_check "WARN" "ADD instruction found in $dockerfile (prefer COPY)"
        else
            record_check "PASS" "No unsafe ADD instructions in $dockerfile"
        fi
    done
else
    record_check "WARN" "No Dockerfiles found for container security analysis"
fi

# 7. TLS/SSL Configuration Check
print_status "Checking TLS/SSL configurations..."

# Check for insecure HTTP usage
if find . -name "*.go" -not -path "./vendor/*" -not -path "./test/*" -exec grep -l "http://" {} \; | head -1 | grep -q .; then
    record_check "WARN" "HTTP URLs detected in source code"
else
    record_check "PASS" "No insecure HTTP URLs detected"
fi

# Check for TLS verification skip
if find . -name "*.go" -not -path "./vendor/*" -exec grep -l "InsecureSkipVerify.*true" {} \; | head -1 | grep -q .; then
    record_check "FAIL" "TLS verification skip detected"
else
    record_check "PASS" "No TLS verification bypass detected"
fi

# 8. Input Validation Security
print_status "Checking input validation security..."

# Check for SQL injection patterns
if find . -name "*.go" -not -path "./vendor/*" -not -path "./test/*" -exec grep -l "fmt\.Sprintf.*SELECT\|fmt\.Sprintf.*INSERT\|fmt\.Sprintf.*UPDATE\|fmt\.Sprintf.*DELETE" {} \; | head -1 | grep -q .; then
    record_check "FAIL" "Potential SQL injection vulnerability detected"
else
    record_check "PASS" "No SQL injection patterns detected"
fi

# Check for command injection patterns
if find . -name "*.go" -not -path "./vendor/*" -not -path "./test/*" -exec grep -l "exec\.Command.*+" {} \; | head -1 | grep -q .; then
    record_check "WARN" "Potential command injection vulnerability detected"
else
    record_check "PASS" "No command injection patterns detected"
fi

# 9. Logging Security
print_status "Checking for secure logging practices..."

# Check for potential secret leakage in logs
SECRET_LOG_PATTERNS=(
    'log.*secret'
    'log.*password'
    'log.*token'
    'fmt\.Printf.*secret'
    'fmt\.Println.*password'
)

LOGGING_ISSUES=0
for pattern in "${SECRET_LOG_PATTERNS[@]}"; do
    if find . -name "*.go" -not -path "./vendor/*" -not -path "./test/*" -exec grep -l -i "$pattern" {} \; 2>/dev/null | head -1 | grep -q .; then
        LOGGING_ISSUES=$((LOGGING_ISSUES + 1))
    fi
done

if [[ $LOGGING_ISSUES -eq 0 ]]; then
    record_check "PASS" "No secret leakage in logging detected"
else
    record_check "WARN" "Potential secret leakage in logging detected"
fi

# 10. Cryptographic Security
print_status "Checking cryptographic security..."

# Check for weak random number generation
if find . -name "*.go" -not -path "./vendor/*" -not -path "./test/*" -exec grep -l "math/rand" {} \; | head -1 | grep -q .; then
    record_check "WARN" "Weak random number generation detected (use crypto/rand for security)"
else
    record_check "PASS" "No weak random number generation detected"
fi

# Check for deprecated crypto functions
DEPRECATED_CRYPTO=(
    'md5\.'
    'sha1\.'
    'rc4\.'
    'des\.'
)

CRYPTO_ISSUES=0
for pattern in "${DEPRECATED_CRYPTO[@]}"; do
    if find . -name "*.go" -not -path "./vendor/*" -not -path "./test/*" -exec grep -l -i "$pattern" {} \; 2>/dev/null | head -1 | grep -q .; then
        CRYPTO_ISSUES=$((CRYPTO_ISSUES + 1))
    fi
done

if [[ $CRYPTO_ISSUES -eq 0 ]]; then
    record_check "PASS" "No deprecated cryptographic functions detected"
else
    record_check "WARN" "Deprecated cryptographic functions detected"
fi

# Generate Security Report
print_status "Generating security report..."

SECURITY_REPORT="${PROJECT_ROOT}/security-assessment-report.md"
cat > "$SECURITY_REPORT" << EOF
# Security Assessment Report

**Date**: $(date)
**Project**: JIRA CDC Kubernetes Operator
**Assessment Type**: Comprehensive Security Scanning

## Executive Summary

- **Total Security Checks**: ${TOTAL_CHECKS}
- **Passed Checks**: ${PASSED_CHECKS}
- **Failed Checks**: ${FAILED_CHECKS}
- **Warnings**: ${WARNINGS}

### Security Status
EOF

if [[ $FAILED_CHECKS -eq 0 ]]; then
    echo "✅ **PASS** - No critical security issues detected" >> "$SECURITY_REPORT"
else
    echo "❌ **FAIL** - Critical security issues require attention" >> "$SECURITY_REPORT"
fi

cat >> "$SECURITY_REPORT" << EOF

## Assessment Categories

### 1. Static Code Analysis
- Comprehensive source code security scanning
- Pattern-based vulnerability detection
- Security best practices validation

### 2. Dependency Security
- Vulnerable dependency detection
- CVE database cross-reference
- Security advisory monitoring

### 3. RBAC Security
- Kubernetes permission analysis
- Principle of least privilege validation
- Excessive permission detection

### 4. Secret Management
- Hardcoded secret detection
- Secure credential handling validation
- Secret leakage prevention

### 5. Container Security
- Dockerfile security best practices
- Non-root user enforcement
- Secure image configuration

### 6. Network Security
- TLS/SSL configuration validation
- Insecure communication detection
- Certificate verification enforcement

### 7. Input Validation
- Injection vulnerability scanning
- Input sanitization validation
- Command execution security

### 8. Logging Security
- Secret leakage in logs prevention
- Secure logging practices validation
- Sensitive data redaction

### 9. Cryptographic Security
- Strong cryptography enforcement
- Deprecated algorithm detection
- Secure random number generation

## Remediation Guidelines

### High Priority (Security Failures)
EOF

if [[ $FAILED_CHECKS -gt 0 ]]; then
    echo "- Address all failed security checks immediately" >> "$SECURITY_REPORT"
    echo "- Review and fix critical vulnerabilities" >> "$SECURITY_REPORT"
    echo "- Implement additional security controls as needed" >> "$SECURITY_REPORT"
else
    echo "- No high priority security issues detected" >> "$SECURITY_REPORT"
fi

cat >> "$SECURITY_REPORT" << EOF

### Medium Priority (Warnings)
EOF

if [[ $WARNINGS -gt 0 ]]; then
    echo "- Review warning-level security findings" >> "$SECURITY_REPORT"
    echo "- Consider implementing recommended security improvements" >> "$SECURITY_REPORT"
    echo "- Monitor for potential security implications" >> "$SECURITY_REPORT"
else
    echo "- No medium priority security issues detected" >> "$SECURITY_REPORT"
fi

cat >> "$SECURITY_REPORT" << EOF

## Security Recommendations

1. **Regular Security Scanning**: Run this assessment regularly as part of CI/CD
2. **Dependency Updates**: Keep all dependencies updated with latest security patches
3. **Security Training**: Ensure development team follows secure coding practices
4. **Incident Response**: Have a plan for addressing security vulnerabilities
5. **Continuous Monitoring**: Implement runtime security monitoring

## Tools Used

- Custom static analysis scanner
- gosec (Go Security Checker)
- govulncheck (Go Vulnerability Database)
- Pattern-based secret detection
- RBAC permission analyzer

---
*This report was generated automatically by the security assessment script.*
EOF

echo ""
echo "============================================================"
print_status "Security Assessment Complete!"
echo ""
print_status "Summary:"
echo "  Total Checks: ${TOTAL_CHECKS}"
print_success "  Passed: ${PASSED_CHECKS}"
print_error "  Failed: ${FAILED_CHECKS}"
print_warning "  Warnings: ${WARNINGS}"
echo ""
print_status "Security report generated: ${SECURITY_REPORT}"

# Clean up temporary files
rm -f security-report.txt 2>/dev/null || true

# Exit with error if any critical checks failed
if [[ $FAILED_CHECKS -gt 0 ]]; then
    print_error "Security assessment failed with ${FAILED_CHECKS} critical issues!"
    exit 1
else
    print_success "Security assessment passed! ✅"
    exit 0
fi
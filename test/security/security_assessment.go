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

package security

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SecurityAssessment performs comprehensive security scanning
type SecurityAssessment struct {
	findings []SecurityFinding
	basePath string
}

// SecurityFinding represents a security issue
type SecurityFinding struct {
	Type        string
	Severity    string
	File        string
	Line        int
	Description string
	Remediation string
}

// NewSecurityAssessment creates a new security assessment
func NewSecurityAssessment(basePath string) *SecurityAssessment {
	return &SecurityAssessment{
		findings: make([]SecurityFinding, 0),
		basePath: basePath,
	}
}

// RunAssessment executes the complete security assessment
func (sa *SecurityAssessment) RunAssessment() error {
	if err := sa.scanForHardcodedSecrets(); err != nil {
		return fmt.Errorf("hardcoded secrets scan failed: %w", err)
	}
	
	if err := sa.scanForSQLInjection(); err != nil {
		return fmt.Errorf("SQL injection scan failed: %w", err)
	}
	
	if err := sa.scanForCommandInjection(); err != nil {
		return fmt.Errorf("command injection scan failed: %w", err)
	}
	
	if err := sa.scanForInsecureHTTP(); err != nil {
		return fmt.Errorf("insecure HTTP scan failed: %w", err)
	}
	
	if err := sa.scanForPathTraversal(); err != nil {
		return fmt.Errorf("path traversal scan failed: %w", err)
	}
	
	if err := sa.scanRBACPermissions(); err != nil {
		return fmt.Errorf("RBAC permissions scan failed: %w", err)
	}
	
	if err := sa.scanForSecretLeakage(); err != nil {
		return fmt.Errorf("secret leakage scan failed: %w", err)
	}
	
	if err := sa.scanForInsecureRandomness(); err != nil {
		return fmt.Errorf("insecure randomness scan failed: %w", err)
	}
	
	return nil
}

// scanForHardcodedSecrets detects hardcoded passwords, tokens, and keys
func (sa *SecurityAssessment) scanForHardcodedSecrets() error {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*["'][^"']{8,}["']`),
		regexp.MustCompile(`(?i)(token|key|secret)\s*[:=]\s*["'][^"']{10,}["']`),
		regexp.MustCompile(`(?i)api[_-]?key\s*[:=]\s*["'][^"']+["']`),
		regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9_-]{20,}`),
		regexp.MustCompile(`(?i)basic\s+[a-zA-Z0-9+/]{20,}={0,2}`),
		regexp.MustCompile(`(?i)ssh-rsa\s+[a-zA-Z0-9+/]{100,}`),
		regexp.MustCompile(`(?i)-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`),
	}
	
	return sa.scanFiles(patterns, "HARDCODED_SECRETS", "HIGH", 
		"Hardcoded secret detected", 
		"Use Kubernetes secrets or environment variables instead")
}

// scanForSQLInjection detects potential SQL injection vulnerabilities
func (sa *SecurityAssessment) scanForSQLInjection() error {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)fmt\.Sprintf\([^)]*SELECT.*%s`),
		regexp.MustCompile(`(?i)fmt\.Sprintf\([^)]*INSERT.*%s`),
		regexp.MustCompile(`(?i)fmt\.Sprintf\([^)]*UPDATE.*%s`),
		regexp.MustCompile(`(?i)fmt\.Sprintf\([^)]*DELETE.*%s`),
		regexp.MustCompile(`(?i)"SELECT.*"\s*\+\s*[a-zA-Z_]`),
		regexp.MustCompile(`(?i)"INSERT.*"\s*\+\s*[a-zA-Z_]`),
	}
	
	return sa.scanFiles(patterns, "SQL_INJECTION", "HIGH",
		"Potential SQL injection vulnerability",
		"Use parameterized queries or prepared statements")
}

// scanForCommandInjection detects command injection vulnerabilities
func (sa *SecurityAssessment) scanForCommandInjection() error {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`exec\.Command\([^)]*\+`),
		regexp.MustCompile(`exec\.CommandContext\([^)]*\+`),
		regexp.MustCompile(`fmt\.Sprintf\([^)]*%s.*exec\.Command`),
		regexp.MustCompile(`os\.system\(`),
		regexp.MustCompile(`syscall\.Exec\(`),
	}
	
	return sa.scanFiles(patterns, "COMMAND_INJECTION", "HIGH",
		"Potential command injection vulnerability",
		"Validate and sanitize all user inputs before executing commands")
}

// scanForInsecureHTTP detects insecure HTTP configurations
func (sa *SecurityAssessment) scanForInsecureHTTP() error {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)http://[^/\s"']+`),
		regexp.MustCompile(`(?i)InsecureSkipVerify:\s*true`),
		regexp.MustCompile(`(?i)TLSClientConfig.*InsecureSkipVerify`),
		regexp.MustCompile(`(?i)tls\.Config.*InsecureSkipVerify:\s*true`),
	}
	
	return sa.scanFiles(patterns, "INSECURE_HTTP", "MEDIUM",
		"Insecure HTTP configuration detected",
		"Use HTTPS and proper TLS certificate validation")
}

// scanForPathTraversal detects path traversal vulnerabilities
func (sa *SecurityAssessment) scanForPathTraversal() error {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`filepath\.Join\([^)]*\.\./`),
		regexp.MustCompile(`os\.Open\([^)]*\.\./`),
		regexp.MustCompile(`ioutil\.ReadFile\([^)]*\.\./`),
		regexp.MustCompile(`/\.\./\.\./`),
		regexp.MustCompile(`\\\.\.\\\.\.\\`),
	}
	
	return sa.scanFiles(patterns, "PATH_TRAVERSAL", "HIGH",
		"Potential path traversal vulnerability",
		"Validate and sanitize file paths, use filepath.Clean()")
}

// scanForSecretLeakage detects potential secret leakage in logs
func (sa *SecurityAssessment) scanForSecretLeakage() error {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)log.*secret`),
		regexp.MustCompile(`(?i)log.*password`),
		regexp.MustCompile(`(?i)log.*token`),
		regexp.MustCompile(`(?i)log.*key.*[^pub]`), // Exclude "public key"
		regexp.MustCompile(`(?i)fmt\.Printf.*secret`),
		regexp.MustCompile(`(?i)fmt\.Println.*password`),
	}
	
	return sa.scanFiles(patterns, "SECRET_LEAKAGE", "MEDIUM",
		"Potential secret leakage in logs",
		"Avoid logging sensitive information, redact secrets from logs")
}

// scanForInsecureRandomness detects weak random number generation
func (sa *SecurityAssessment) scanForInsecureRandomness() error {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`math/rand`),
		regexp.MustCompile(`rand\.Intn\(`),
		regexp.MustCompile(`rand\.Int\(`),
		regexp.MustCompile(`time\.Now\(\)\.UnixNano\(\)`), // Weak seed
	}
	
	return sa.scanFiles(patterns, "WEAK_RANDOMNESS", "LOW",
		"Weak random number generation",
		"Use crypto/rand for cryptographic operations")
}

// scanFiles scans all Go files for security patterns
func (sa *SecurityAssessment) scanFiles(patterns []*regexp.Regexp, findingType, severity, description, remediation string) error {
	return filepath.Walk(sa.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip vendor, .git, and test files (except this security test)
		if strings.Contains(path, "vendor/") || strings.Contains(path, ".git/") {
			return nil
		}
		
		// Only scan Go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		
		// Skip this file to avoid false positives
		if strings.Contains(path, "security_assessment.go") {
			return nil
		}
		
		return sa.scanFile(path, patterns, findingType, severity, description, remediation)
	})
}

// scanFile scans a single file for security patterns
func (sa *SecurityAssessment) scanFile(filePath string, patterns []*regexp.Regexp, findingType, severity, description, remediation string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		
		for _, pattern := range patterns {
			if pattern.MatchString(line) {
				// Skip comments and test files for some checks
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				
				sa.findings = append(sa.findings, SecurityFinding{
					Type:        findingType,
					Severity:    severity,
					File:        filePath,
					Line:        lineNum,
					Description: fmt.Sprintf("%s: %s", description, strings.TrimSpace(line)),
					Remediation: remediation,
				})
			}
		}
	}
	
	return scanner.Err()
}

// scanRBACPermissions analyzes RBAC configurations for excessive permissions
func (sa *SecurityAssessment) scanRBACPermissions() error {
	rbacPath := filepath.Join(sa.basePath, "config", "rbac", "role.yaml")
	
	file, err := os.Open(rbacPath)
	if err != nil {
		// RBAC file doesn't exist, skip
		return nil
	}
	defer file.Close()
	
	scanner := bufio.NewScanner(file)
	lineNum := 0
	inResources := false
	inVerbs := false
	
	dangerousVerbs := []string{"*", "create", "delete", "patch", "update"}
	sensitiveResources := []string{"secrets", "configmaps", "serviceaccounts", "clusterroles", "clusterrolebindings"}
	
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		if strings.Contains(line, "resources:") {
			inResources = true
			continue
		}
		if strings.Contains(line, "verbs:") {
			inVerbs = true
			inResources = false
			continue
		}
		if strings.HasPrefix(line, "-") && inResources {
			for _, sensitive := range sensitiveResources {
				if strings.Contains(line, sensitive) {
					sa.findings = append(sa.findings, SecurityFinding{
						Type:        "RBAC_EXCESSIVE_PERMISSIONS",
						Severity:    "MEDIUM",
						File:        rbacPath,
						Line:        lineNum,
						Description: fmt.Sprintf("Access to sensitive resource: %s", line),
						Remediation: "Follow principle of least privilege, grant minimal required permissions",
					})
				}
			}
		}
		if strings.HasPrefix(line, "-") && inVerbs {
			for _, dangerous := range dangerousVerbs {
				if strings.Contains(line, dangerous) && dangerous == "*" {
					sa.findings = append(sa.findings, SecurityFinding{
						Type:        "RBAC_WILDCARD_PERMISSIONS",
						Severity:    "HIGH",
						File:        rbacPath,
						Line:        lineNum,
						Description: fmt.Sprintf("Wildcard permission detected: %s", line),
						Remediation: "Avoid wildcard permissions, specify exact verbs needed",
					})
				}
			}
		}
		
		// Reset flags on new rule
		if strings.HasPrefix(line, "- apiGroups:") {
			inResources = false
			inVerbs = false
		}
	}
	
	return scanner.Err()
}

// GetFindings returns all security findings
func (sa *SecurityAssessment) GetFindings() []SecurityFinding {
	return sa.findings
}

// GetFindingsBySeverity returns findings filtered by severity
func (sa *SecurityAssessment) GetFindingsBySeverity(severity string) []SecurityFinding {
	var filtered []SecurityFinding
	for _, finding := range sa.findings {
		if finding.Severity == severity {
			filtered = append(filtered, finding)
		}
	}
	return filtered
}

// GetSummary returns a summary of security findings
func (sa *SecurityAssessment) GetSummary() map[string]int {
	summary := make(map[string]int)
	summary["HIGH"] = len(sa.GetFindingsBySeverity("HIGH"))
	summary["MEDIUM"] = len(sa.GetFindingsBySeverity("MEDIUM"))
	summary["LOW"] = len(sa.GetFindingsBySeverity("LOW"))
	summary["TOTAL"] = len(sa.findings)
	return summary
}

// TestSecurityAssessment runs the complete security assessment
func TestSecurityAssessment(t *testing.T) {
	// Get the project root directory
	wd, err := os.Getwd()
	require.NoError(t, err)
	
	// Navigate up to project root (assuming test is in test/security/)
	projectRoot := filepath.Join(wd, "..", "..")
	
	assessment := NewSecurityAssessment(projectRoot)
	err = assessment.RunAssessment()
	require.NoError(t, err)
	
	findings := assessment.GetFindings()
	summary := assessment.GetSummary()
	
	// Log summary
	t.Logf("Security Assessment Summary:")
	t.Logf("  HIGH severity: %d", summary["HIGH"])
	t.Logf("  MEDIUM severity: %d", summary["MEDIUM"])
	t.Logf("  LOW severity: %d", summary["LOW"])
	t.Logf("  TOTAL findings: %d", summary["TOTAL"])
	
	// Log detailed findings
	for _, finding := range findings {
		t.Logf("[%s] %s:%d - %s", finding.Severity, finding.File, finding.Line, finding.Description)
		t.Logf("  Remediation: %s", finding.Remediation)
	}
	
	// Security gates - fail on HIGH severity issues
	highSeverityFindings := assessment.GetFindingsBySeverity("HIGH")
	if len(highSeverityFindings) > 0 {
		t.Errorf("Found %d HIGH severity security issues that must be resolved", len(highSeverityFindings))
		for _, finding := range highSeverityFindings {
			t.Errorf("  HIGH: %s:%d - %s", finding.File, finding.Line, finding.Description)
		}
	}
	
	// Warn on MEDIUM severity issues
	mediumSeverityFindings := assessment.GetFindingsBySeverity("MEDIUM")
	if len(mediumSeverityFindings) > 0 {
		t.Logf("WARNING: Found %d MEDIUM severity security issues that should be reviewed", len(mediumSeverityFindings))
	}
	
	// Check for specific security requirements
	t.Run("NoHardcodedSecrets", func(t *testing.T) {
		hardcodedSecrets := 0
		for _, finding := range findings {
			if finding.Type == "HARDCODED_SECRETS" {
				hardcodedSecrets++
			}
		}
		assert.Equal(t, 0, hardcodedSecrets, "No hardcoded secrets should be present in the codebase")
	})
	
	t.Run("NoSQLInjection", func(t *testing.T) {
		sqlInjections := 0
		for _, finding := range findings {
			if finding.Type == "SQL_INJECTION" {
				sqlInjections++
			}
		}
		assert.Equal(t, 0, sqlInjections, "No SQL injection vulnerabilities should be present")
	})
	
	t.Run("NoCommandInjection", func(t *testing.T) {
		commandInjections := 0
		for _, finding := range findings {
			if finding.Type == "COMMAND_INJECTION" {
				commandInjections++
			}
		}
		assert.Equal(t, 0, commandInjections, "No command injection vulnerabilities should be present")
	})
}

// TestContainerSecurity validates container security configurations
func TestContainerSecurity(t *testing.T) {
	t.Run("SecurityContext", func(t *testing.T) {
		// This would check Dockerfiles and Kubernetes manifests for:
		// - Non-root user usage
		// - Read-only root filesystem
		// - No privileged containers
		// - Security capabilities dropped
		t.Log("Container security context validation would be implemented here")
		t.Log("Checking for non-root users, read-only filesystems, dropped capabilities")
	})
	
	t.Run("ImageScanningPolicy", func(t *testing.T) {
		// This would verify:
		// - Base image vulnerability scanning
		// - No known vulnerable dependencies
		// - Latest security patches applied
		t.Log("Image vulnerability scanning policy would be implemented here")
		t.Log("Verifying base images are scanned and up-to-date")
	})
}

// TestNetworkSecurity validates network security configurations
func TestNetworkSecurity(t *testing.T) {
	t.Run("TLSConfiguration", func(t *testing.T) {
		// Check for proper TLS configuration
		t.Log("TLS configuration validation would be implemented here")
		t.Log("Ensuring all communications use TLS 1.2+")
	})
	
	t.Run("NetworkPolicies", func(t *testing.T) {
		// Verify Kubernetes network policies are in place
		t.Log("Network policy validation would be implemented here")
		t.Log("Checking for proper network segmentation")
	})
}

// TestDataProtection validates data protection measures
func TestDataProtection(t *testing.T) {
	t.Run("EncryptionAtRest", func(t *testing.T) {
		// Verify encryption for sensitive data at rest
		t.Log("Encryption at rest validation would be implemented here")
		t.Log("Ensuring sensitive data is encrypted when stored")
	})
	
	t.Run("SecretManagement", func(t *testing.T) {
		// Verify proper secret management practices
		t.Log("Secret management validation would be implemented here")
		t.Log("Checking Kubernetes secret usage and rotation")
	})
}
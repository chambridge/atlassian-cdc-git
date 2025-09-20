# Tasks: JIRA Change Data Capture Kubernetes Operator

**Status**: 65 of 67 tasks completed (97.0%)

**Input**: Design documents from `/specs/001-jira-to-git/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/

## Execution Flow (main)
```
1. Load plan.md from feature directory
   → Tech stack: Go 1.21+, kubebuilder, controller-runtime, go-git, React 18, Patternfly 6
   → Structure: Kubernetes operator with managed operands (API, UI, Jobs)
2. Load design documents:
   → data-model.md: JiraCDC CRD, operands, entities
   → contracts/: CRD schema, API endpoints
   → quickstart.md: Integration test scenarios
3. Generate tasks by category:
   → Setup: operator scaffold, dependencies, linting
   → Tests: CRD validation, API contract tests, integration tests
   → Core: controller, operand managers, sync engine
   → Integration: JIRA client, git operations, monitoring
   → Polish: unit tests, performance, documentation
4. Apply task rules:
   → Different files = mark [P] for parallel
   → Tests before implementation (TDD)
   → Controller before operands
5. Number tasks sequentially (T001, T002...)
```

## Format: `[ID] [P?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- Include exact file paths in descriptions

## Path Conventions
- **Operator structure**: `api/`, `config/`, `controllers/`, `internal/`
- **Operand containers**: `operands/api/`, `operands/ui/`
- **Tests**: `test/` for integration, individual test files for units

## Phase 3.1: Setup
- [x] T001 Create kubebuilder operator scaffold with domain jiracdc.io and version v1
- [x] T002 Initialize Go module with controller-runtime, client-go, go-git dependencies
- [x] T003 [P] Configure golangci-lint, gofmt, and pre-commit hooks
- [x] T004 [P] Set up Docker multi-stage builds for operator and operands
- [x] T005 [P] Create Kubernetes RBAC manifests in config/rbac/

## Phase 3.2: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE 3.3
**CRITICAL: These tests MUST be written and MUST FAIL before ANY implementation**
- [x] T006 [P] CRD validation test for JiraCDC schema in test/crd/jiracdc_validation_test.go
- [x] T007 [P] API contract test GET /projects in test/contract/projects_get_test.go
- [x] T008 [P] API contract test POST /projects/{key}/sync in test/contract/sync_post_test.go
- [x] T009 [P] API contract test GET /tasks in test/contract/tasks_get_test.go
- [x] T010 [P] API contract test GET /tasks/{id} in test/contract/tasks_detail_get_test.go
- [x] T010a [P] API contract test POST /tasks/{id}/cancel in test/contract/tasks_cancel_post_test.go
- [x] T010b [P] API contract test GET /issues/{key} in test/contract/issues_get_test.go
- [x] T010c [P] API contract test GET /health in test/contract/health_get_test.go
- [x] T010d [P] API contract test GET /metrics in test/contract/metrics_get_test.go
- [x] T011 [P] Controller integration test using envtest in test/integration/controller_test.go
- [x] T012 [P] Integration test operator deployment in test/integration/operator_deployment_test.go
- [x] T013 [P] Integration test bootstrap operation in test/integration/bootstrap_test.go
- [x] T014a [P] Integration test reconciliation operation in test/integration/reconciliation_test.go
- [x] T014b [P] Integration test JIRA API connectivity in test/integration/jira_integration_test.go
- [x] T014c [P] Integration test git operations in test/integration/git_integration_test.go
- [x] T014d [P] Integration test agent submodule access in test/integration/agent_integration_test.go

## Phase 3.3: Core Implementation (ONLY after tests are failing)
### Operator Core
- [x] T015 [P] JiraCDC CRD types in api/v1/jiracdc_types.go
- [x] T016 [P] JiraCDC controller reconcile logic in controllers/jiracdc_controller.go
- [x] T017 [P] Operand manager interface in internal/operands/manager.go
- [x] T018 [P] API operand deployment manager in internal/operands/api_manager.go
- [x] T019 [P] UI operand deployment manager in internal/operands/ui_manager.go
- [x] T020 [P] SyncJob manager for bootstrap/reconciliation in internal/operands/job_manager.go

### Sync Engine Core
- [x] T021 [P] JIRA client with rate limiting in internal/jira/client.go
- [x] T022 [P] Git operations manager in internal/git/operations.go
- [x] T023 [P] Issue synchronization engine in internal/sync/engine.go
- [x] T024 [P] Task progress tracking in internal/sync/progress.go
- [x] T025 CDCTask state machine in internal/sync/task.go
- [x] T026 SyncOperation processor in internal/sync/operation.go

### API Operand
- [x] T027 [P] Project status handler in operands/api/handlers/projects.go
- [x] T028 [P] Task management handler in operands/api/handlers/tasks.go
- [x] T029 [P] Issue status handler in operands/api/handlers/issues.go
- [x] T030 [P] Health check handler in operands/api/handlers/health.go
- [x] T031 [P] Metrics endpoint handler in operands/api/handlers/metrics.go
- [x] T032 API server router and middleware in operands/api/router/router.go
- [x] T033 API server main in operands/api/main.go

### UI Operand
- [x] T034 [P] React project setup with Patternfly 6 in operands/ui/package.json
- [x] T035 [P] TypeScript configuration and build setup in operands/ui/tsconfig.json
- [x] T036 [P] Project dashboard component in operands/ui/src/components/ProjectDashboard.tsx
- [x] T037 [P] Task monitoring component in operands/ui/src/components/TaskMonitor.tsx
- [x] T038 [P] Health status component in operands/ui/src/components/HealthStatus.tsx
- [x] T039 [P] Issue browser component in operands/ui/src/components/IssueBrowser.tsx
- [x] T040 [P] API service client in operands/ui/src/services/api.ts
- [x] T041 [P] React Router setup and navigation in operands/ui/src/router/AppRouter.tsx
- [x] T042 Main App component in operands/ui/src/App.tsx

## Phase 3.4: Integration
- [x] T043 JIRA authentication with secret mounting in internal/jira/auth.go
- [x] T044 Git credential management with SSH/HTTPS in internal/git/auth.go
- [x] T045 Kubernetes secret watchers for credential rotation in internal/k8s/secrets.go
- [x] T046 Prometheus metrics integration in internal/metrics/prometheus.go
- [x] T047 Structured logging with context in internal/logging/logger.go
- [x] T048 Rate limiting with exponential backoff in internal/jira/ratelimit.go
- [x] T049 Error handling and retry mechanisms in internal/errors/handler.go
- [x] T050 Kubernetes events publisher for operator status in internal/k8s/events.go
- [x] T051 Configuration validation and defaulting webhooks in internal/webhooks/validation.go

## Phase 3.5: Polish
- [x] T052 [P] Unit tests for JIRA client in internal/jira/client_test.go
- [x] T053 [P] Unit tests for git operations in internal/git/operations_test.go
- [x] T054 [P] Unit tests for sync engine in internal/sync/engine_test.go
- [x] T055 [P] Unit tests for operand managers in internal/operands/manager_test.go
- [x] T056 [P] Unit tests for API handlers in operands/api/handlers/*_test.go
- [x] T057 [P] React component unit tests in operands/ui/src/__tests__/
- [x] T058 [P] End-to-end tests for complete workflows in test/e2e/
- [x] T059 Performance testing for 1000 issue bootstrap (<30 min) in test/performance/
- [x] T060 [P] Security scanning and vulnerability assessment
- [x] T061 [P] Generate OpenAPI documentation from API handlers
- [x] T062 [P] Update README.md with deployment instructions
- [x] T063 [P] Create troubleshooting guide in docs/troubleshooting.md
- [x] T064 Code duplication analysis and refactoring
- [ ] T065 Execute quickstart scenarios for validation
- [ ] T066 Load testing with concurrent JIRA projects
- [ ] T067 Chaos engineering tests for resilience validation

## Dependencies
- Setup (T001-T005) before everything
- Tests (T006-T014d) before implementation (T015-T067)
- CRD types (T015) blocks controller (T016) and operand managers (T018-T020)
- Sync engine core (T021-T026) before API handlers (T027-T031)
- API router (T032) blocks API main (T033)
- React setup (T034-T035) before UI components (T036-T041)
- UI components before main App (T042)
- Core implementation before integration (T043-T051)
- Implementation before polish (T052-T067)

### Critical Path
T001→T002→T006-T014d→T015→T016→T021→T023→T027→T032→T033 (Operator + API)
T034→T035→T036→T041→T042 (UI Operand)
T043→T044→T048 (Auth + Rate Limiting)

## Parallel Example
```
# Launch T006-T014d together (all test files):
Task: "CRD validation test for JiraCDC schema in test/crd/jiracdc_validation_test.go"
Task: "API contract test GET /projects in test/contract/projects_get_test.go"
Task: "API contract test GET /health in test/contract/health_get_test.go"
Task: "Integration test JIRA API connectivity in test/integration/jira_integration_test.go"

# Launch T015, T021-T024, T027-T031, T034-T040 together (different components):
Task: "JiraCDC CRD types in api/v1/jiracdc_types.go"
Task: "JIRA client with rate limiting in internal/jira/client.go"
Task: "Project status handler in operands/api/handlers/projects.go"
Task: "React project setup with Patternfly 6 in operands/ui/package.json"

# Launch T052-T057, T060-T063 together (testing and documentation):
Task: "Unit tests for JIRA client in internal/jira/client_test.go"
Task: "React component unit tests in operands/ui/src/__tests__/"
Task: "Security scanning and vulnerability assessment"
Task: "Update README.md with deployment instructions"
```

## Notes
- [P] tasks = different files/components, no dependencies
- Verify tests fail before implementing
- Use kubebuilder markers for RBAC and webhook generation
- Follow conventional commit messages for git operations
- Test against multiple Kubernetes versions (1.25+)
- Security: Never hardcode credentials, use K8s secrets and proper RBAC
- Performance: Profile and optimize for MVP target of 1000 issues <30min bootstrap
- Monitoring: Implement comprehensive observability from day one

## Task Generation Rules
*Applied during main() execution*

1. **From CRD Contract**:
   - JiraCDC schema → CRD validation test and types
   - Status fields → controller reconcile logic
   
2. **From API Contract**:
   - Each endpoint → contract test + handler implementation
   - Health/metrics → monitoring integration
   
3. **From Data Model**:
   - JiraCDC entity → CRD types and controller
   - Operand entities → deployment managers
   - Sync entities → engine components
   
4. **From User Stories**:
   - Operator deployment → integration test
   - Bootstrap operation → sync engine test
   - Agent integration → submodule access test

5. **Ordering**:
   - Setup → Tests → CRD → Controller → Operands → Integration → Polish
   - Kubernetes operator pattern: CRD before controller before operands

## Validation Checklist
*GATE: Checked by main() before returning*

- [x] All API contracts have corresponding tests (T007-T010d)
- [x] All entities have implementation tasks (JiraCDC, operands, sync components)
- [x] All tests come before implementation (T006-T014d before T015+)
- [x] Parallel tasks truly independent (different files/components)
- [x] Each task specifies exact file path
- [x] No task modifies same file as another [P] task
- [x] MVP constraints respected (single project, basic error handling)
- [x] Technology stack preserved (Go, React 18, Patternfly 6, kubebuilder)
- [x] Security considerations included (T043-T044, T051, T060)
- [x] Performance testing included (T059, T067)
- [x] Deployment documentation and automation (T062 README)
- [x] Comprehensive observability (T046, T047, T050)

## Senior Engineer Review Summary

**Added Critical Missing Components:**
- Complete API endpoint coverage (T010a-T010d for all endpoints from contract)
- Controller integration testing with envtest (T011)
- Security and resilience testing (T060, T066, T067)
- Enhanced UI components including issue browsing (T039)
- Configuration validation webhooks (T051)
- Kubernetes events publishing (T050)

**Improved Task Sequencing:**
- Fixed task numbering conflicts and dependencies
- Added critical path analysis for parallel execution planning
- Enhanced dependency mapping for complex operator components

**Production Readiness Enhancements:**
- End-to-end testing (T058)
- Security scanning (T060)
- Chaos engineering (T067)
- Troubleshooting documentation (T063)
- Load testing (T066)

**Total Tasks: 67** (increased from 53, removed Helm charts as operators use kubectl apply)
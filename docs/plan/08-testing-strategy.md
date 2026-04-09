# Testing Strategy

## Challenge

Agentic systems are non-deterministic. The same prompt can produce different tool calls, different code, and different outcomes. The testing strategy separates **deterministic orchestration logic** (testable) from **non-deterministic LLM output** (contract-testable).

## Test Layers

### Layer 1: Unit Tests — Deterministic Logic

Test the runtime's decision-making, parsing, and state management without any LLM calls. Written in Go using the standard `testing` package.

**Decision engine (`internal/core/engine_test.go`):**
- `chooseNextAction` with every task status combination
- Priority ordering across multiple ready tasks
- Recovery path selection for stale/failed runs
- Escalation triggering at threshold boundaries
- Status normalization (non-standard statuses mapped correctly)
- Workflow routing (all references go through `workflow.Routing`, no hardcoded strings)

**Parser (`internal/core/parser_test.go`):**
- Markdown table parsing: empty files, missing headers, malformed tables, extra whitespace
- Task metadata extraction: missing fields, extra fields, malformed values
- Backlog parsing edge cases

**State management (`internal/core/executor_test.go`):**
- Task status transitions validated against workflow
- Escalation state machine: create, resolve, supersede, epoch scoping
- Project state updates

**Guard system (`internal/guard/`):**
- Budget: daily rollover, limit enforcement, per-call tracking, cost calculation
- Role boundary matching: glob patterns against file paths
- Violation detection: new files, modified files, removed files
- Manifest snapshot and comparison

**Model tier inference (`internal/agent/prompt_test.go`):**
- Light tier for small-scope tasks
- Standard tier for greenfield tasks
- Explicit override from metadata

**Target: 90%+ coverage on `internal/core/` and `internal/guard/`.**

**Timing: built during Phase 1 alongside each module.** Not deferred to Phase 3.

### Layer 2: Provider Tests

Test each LLM provider against a mock HTTP server.

**Mock server:** a Go `httptest.Server` that serves recorded LLM API responses.

**Per provider (`internal/provider/*_test.go`):**
- Connection validation (healthy, unreachable, auth failure)
- Streaming response parsing
- Tool-use request/response round-trip
- Token count extraction from response metadata
- Error classification (rate limit, auth, model unavailable, context overflow)
- Abort/cancel via context cancellation

**Target: 70% coverage. Some paths require real API calls — tested manually or in integration.**

### Layer 3: Contract Tests — Agent Output Validation

Verify that after an agent runs, the expected artifacts exist with the expected structure — without asserting on content quality.

**Builder contract:**
After a Builder agent completes for task TASK-XXX:
- Task file has `Status: in-review`
- `## Implementation Notes` section is non-empty
- `## Validation` section is non-empty
- At least one acceptance criterion is checked (`- [x]`)
- `## Handoff` section contains `Next Role: Reviewer/QA`

**Reviewer contract:**
After a Reviewer agent completes for task TASK-XXX:
- `reviews/REVIEW-TASK-XXX*.md` exists
- Contains `Outcome: approved|rejected|blocked`
- Contains `## Acceptance Criteria Result`
- Contains `## Final Recommendation`

**These contracts are enforced by the runtime** as post-execution validation and also run as tests.

### Layer 4: Integration Tests — Full Cycle

Test the complete flow without real LLM calls using recorded responses.

**Mock provider:** A test provider that replays recorded LLM responses from JSON files.

**Test scenarios:**
1. Happy path: create project → decompose → build → review → approve → close
2. Rework path: build → review rejects → rework → review approves → close
3. Escalation path: build fails 6 times → escalation raised
4. Budget exhaustion: budget runs out mid-build → graceful abort
5. Multi-project: two projects active, serial execution, budget shared
6. Recovery: agent context cancelled mid-run → next cycle reconciles

**Recording responses:** `--record` flag on real providers writes all API responses to JSON fixtures. These become test data.

**Timing: built during Phase 1.10 (CLI end-to-end test).** Uses recorded responses from real provider testing.

### Layer 5: UI Tests

For the Svelte frontend:
- **Component tests:** Svelte Testing Library + Vitest
- **Snapshot tests:** dashboard renders correctly with mock data
- **Interaction tests:** click "New Project" → wizard opens, click "Abort" → sends abort request
- **Accessibility:** axe-core automated checks on every view
- **Agent output rendering:** verify LLM output is rendered as plain text, not HTML (XSS prevention)

**Timing: built during Phase 2 alongside each component.**

## CI Pipeline

```yaml
on: [push, pull_request]

jobs:
  lint:
    - golangci-lint run ./...
    - cd frontend && npx svelte-check

  unit-tests:
    - go test ./internal/core/... -cover
    - go test ./internal/guard/... -cover
    - go test ./internal/provider/... -cover
    - go test ./internal/agent/... -cover

  integration-tests:
    - go test ./internal/integration/... -tags=integration

  ui-tests:
    - cd frontend && npx vitest run

  build:
    - make build
    - Smoke test: binary starts, health endpoint responds, UI loads

  matrix:
    - Ubuntu 22.04, Ubuntu 24.04, Fedora 39, Arch (latest)
```

## Test Timing Per Phase

| Phase | Tests built |
|---|---|
| 1.2-1.4 (Providers) | Provider unit tests with mock HTTP server |
| 1.5 (Agent executor) | Executor unit tests, tool execution tests |
| 1.6 (Core runtime) | Decision engine tests, parser tests, state management tests |
| 1.7 (Guard) | Budget tests, validation tests |
| 1.10 (CLI E2E) | Integration tests with recorded responses |
| 2.2-2.8 (UI) | Component tests, interaction tests, accessibility checks |
| 3.3 (Test suite) | Gap filling, chaos tests, cross-distro matrix, coverage enforcement |

Tests are **not deferred to Phase 3**. Each module ships with its unit tests. Phase 3 fills gaps and adds chaos/cross-distro testing.

## Coverage Targets

| Module | Target | Rationale |
|---|---|---|
| `internal/core/` | 90% | Deterministic logic, must be reliable |
| `internal/guard/` | 90% | Security-critical |
| `internal/provider/` | 70% | Some paths need real APIs |
| `internal/agent/` | 60% | Non-deterministic LLM interaction |
| `frontend/` | 70% | Component rendering and interaction |

# Dependencies and Phase Plan

## Dependency Graph

```
Phase 1: Foundation (Go rewrite + core)
  ├─ 1.1 Go project scaffold + build pipeline ─────────┐
  ├─ 1.2 Provider interface + Ollama provider ──────────┤
  ├─ 1.3 Provider: OpenAI ─────────────────────────────┤
  ├─ 1.4 Provider: Anthropic ──────────────────────────┤
  ├─ 1.5 Agent executor (tool loop) ◄──── 1.2,1.3,1.4
  ├─ 1.6 Core runtime rewrite (engine + state) ◄──── 1.1
  ├─ 1.7 Guard system (budget + validation) ◄──── 1.6
  ├─ 1.8 SQLite schema + sync layer ◄──── 1.6
  ├─ 1.9 Wire agent executor into runtime ◄── 1.5, 1.6, 1.7
  └─ 1.10 CLI end-to-end test ◄──── 1.9
         (brief → decompose → build → review → close)

Phase 2: Web UI
  ├─ 2.1 HTTP server + SSE + embedded frontend ◄──── 1.9
  ├─ 2.2 Svelte project scaffold ──────────────────────┐
  ├─ 2.3 Config system + first-run wizard ◄──── 2.1, 2.2
  ├─ 2.4 Project list + creation UI ◄──── 2.1, 2.2, 1.8
  ├─ 2.5 Task pipeline visualization ◄──── 2.4
  ├─ 2.6 Agent activity + streaming UI ◄──── 2.1, 1.5
  ├─ 2.7 Budget / cost dashboard ◄──── 2.1, 1.7
  └─ 2.8 Escalation inbox ◄──── 2.1

Phase 3: Distribution
  ├─ 3.1 Binary + AppImage packaging ◄──── 2.*
  ├─ 3.2 Auto-updater ◄──── 3.1
  ├─ 3.3 Test suite ◄──── 1.*, 2.*
  ├─ 3.4 Documentation + example project ◄──── 2.*
  └─ 3.5 Landing page + release ◄──── 3.1, 3.4
```

## Phase 1: Foundation

Go rewrite of the runtime. CLI remains functional throughout — no UI required.

### 1.1 Go Project Scaffold + Build Pipeline
**Output:** `go.mod`, project structure, Makefile with `build`, `dev`, `test` targets. Framework YAML files copied into `framework/`.
**Effort:** 1 day
**Risk:** Low

### 1.2 Provider Interface + Ollama Provider
**Output:** `provider.go` (interface), `ollama.go` (HTTP client with tool use, streaming, token counting)
**Effort:** 2-3 days
**Dependencies:** 1.1
**Risk:** Medium — Ollama's tool-use API may have edge cases
**Acceptance:** Send prompt with tools, receive tool calls, iterate to completion. Token count matches Ollama response metadata.

### 1.3 Provider: OpenAI
**Output:** `openai.go` — OpenAI chat completions with tool use, streaming, token counting
**Effort:** 1-2 days
**Dependencies:** 1.2 (reuses interface)
**Risk:** Low

### 1.4 Provider: Anthropic
**Output:** `anthropic.go` — Claude API with tool use, streaming, token counting
**Effort:** 1-2 days
**Dependencies:** 1.2 (reuses interface)
**Risk:** Low

### 1.5 Agent Executor (Tool Loop)
**Output:** `runner.go` — the core agentic loop: prompt → LLM → tool calls → execute → feed back → repeat
**Effort:** 3-4 days (hardest module)
**Dependencies:** 1.2 (at least one provider)
**Risk:** High — reliability is critical
**Key behaviors:**
- Streaming output forwarded via channel
- Tool execution: read_file, write_file, run_command, list_files
- Budget check per turn
- Timeout enforcement (per-call and per-run)
- Graceful handling of malformed LLM responses
- Abort/cancel via context cancellation
- Max 30 tool calls per run
**Acceptance:** Can autonomously implement a simple task using each provider.

### 1.6 Core Runtime Rewrite
**Output:** `workflow.go`, `project.go`, `parser.go`, `engine.go`, `executor.go`, `escalation.go`
**Effort:** 5-7 days
**Dependencies:** 1.1
**Risk:** Medium — rewriting working logic. Mitigated by existing test + contract tests.
**Acceptance:** Decision engine produces identical routing decisions as the JS version for the same project state.

### 1.7 Guard System
**Output:** `budget.go` (token-level tracking), `validate.go` (file manifest + role boundaries), `hooks.go`
**Effort:** 2-3 days
**Dependencies:** 1.6
**Risk:** Low

### 1.8 SQLite Schema + Sync Layer
**Output:** `db.go`, `sync.go`, `migrations.go`, schema SQL
**Effort:** 2-3 days
**Dependencies:** 1.6
**Risk:** Low

### 1.9 Wire Agent Executor into Runtime
**Output:** Runtime uses agent executor for Builder/Reviewer instead of external CLI.
**Effort:** 2-3 days
**Dependencies:** 1.5, 1.6, 1.7
**Risk:** Medium
**Acceptance:** Full pickup cycle works via direct API calls.

### 1.10 CLI End-to-End Test
**Output:** `warpspawn` CLI can: create a project from a brief, decompose via Mission Control, build via Builder, review via Reviewer, close the task. All via direct LLM API calls.
**Effort:** 1-2 days (mostly testing and fixing)
**Dependencies:** 1.9
**Risk:** Medium — first real end-to-end test
**This is the Phase 1 milestone.** If this works, the foundation is proven.

**Phase 1 total: ~20-28 days (4-6 weeks)**

---

## Phase 2: Web UI

### 2.1 HTTP Server + SSE + Embedded Frontend
**Output:** Go serves the Svelte frontend via embedded static files. REST API handlers. SSE event stream.
**Effort:** 2-3 days
**Dependencies:** 1.9

### 2.2 Svelte Project Scaffold
**Output:** Svelte 5 + Vite + TypeScript. Basic layout, routing, dark theme.
**Effort:** 1-2 days
**Dependencies:** None (can start in parallel with Phase 1)
**Risk:** Low

### 2.3 Config System + First-Run Wizard
**Output:** Settings in `~/.config/warpspawn/`. API keys in OS keyring. Wizard detects Ollama, prompts for cloud keys, assigns models to roles.
**Effort:** 3-4 days
**Dependencies:** 2.1, 2.2

### 2.4 Project List + Creation UI
**Output:** Dashboard showing all projects with status. "New Project" button with brief input.
**Effort:** 2-3 days
**Dependencies:** 2.1, 2.2, 1.8

### 2.5 Task Pipeline Visualization
**Output:** Per-project view showing task lifecycle stages as a visual pipeline.
**Effort:** 2-3 days
**Dependencies:** 2.4

### 2.6 Agent Activity + Streaming UI
**Output:** Panel showing running agent with streaming LLM output via SSE, tool calls inline, abort button.
**Effort:** 3-4 days
**Dependencies:** 2.1, 1.5
**Risk:** Medium — streaming UX

### 2.7 Budget / Cost Dashboard
**Output:** Token usage charts, cost breakdown by project/role/model.
**Effort:** 2-3 days
**Dependencies:** 2.1, 1.7

### 2.8 Escalation Inbox
**Output:** In-app notification center for escalations.
**Effort:** 2 days
**Dependencies:** 2.1

**Phase 2 total: ~17-24 days (3-5 weeks)**

---

## Phase 3: Distribution

### 3.1 Binary + AppImage Packaging
**Output:** `make release` produces Linux amd64/arm64 binaries + optional AppImage
**Effort:** 2-3 days

### 3.2 Auto-Updater
**Output:** On-startup version check against GitHub Releases, download + replace binary
**Effort:** 2 days

### 3.3 Test Suite
**Output:** Unit tests (engine, providers, guard), integration tests (full cycles with recorded responses), contract tests (agent output validation)
**Effort:** 5-7 days

### 3.4 Documentation + Example Project
**Output:** User guide, architecture overview, example project
**Effort:** 3-4 days

### 3.5 Landing Page + Release
**Output:** GitHub repo, itch.io listing, first release
**Effort:** 2 days

**Phase 3 total: ~14-18 days (3-4 weeks)**

---

## Timeline Summary

| Phase | Duration | Cumulative |
|---|---|---|
| Phase 1: Foundation (Go rewrite) | 4-6 weeks | 4-6 weeks |
| Phase 2: Web UI | 3-5 weeks | 7-11 weeks |
| Phase 3: Distribution | 3-4 weeks | 10-15 weeks |

**Total estimated: 10-15 weeks to first public release.**

Key milestone: **Phase 1.10** — CLI end-to-end test proves the foundation works before investing in UI.

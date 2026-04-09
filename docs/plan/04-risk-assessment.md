# Risk Assessment and Mitigation

## R1: Agent Tool Loop Reliability — HIGH

**Risk:** The agent executor is the core. LLMs produce unpredictable output: malformed tool calls, infinite loops, scope violations, hallucinated file paths.

**Impact:** Users lose trust after 2-3 broken runs.

**Mitigation:**
- Hard iteration cap (30 tool calls). Abort and escalate if exceeded.
- Tool call JSON validation before execution. Reject and retry once if malformed.
- File path validation: confirm within project scope before writing.
- Command validation: block dangerous patterns unless explicitly allowed.
- Timeout at two levels: per-call (60s) and per-run (240s).
- Completion signal: agent uses a `task_complete` tool call rather than message parsing.
- Fallback: failed task returns to `ready-for-build` with a failure report.

---

## R2: LLM Provider API Changes — MEDIUM

**Risk:** Ollama, OpenAI, and Anthropic APIs evolve. Breaking changes could break providers.

**Impact:** Provider becomes unusable until fixed.

**Mitigation:**
- Provider abstraction isolates changes to one file.
- No SDK dependencies — raw HTTP calls. Fewer transitive breakage vectors.
- Provider health check on startup: verify connection, model availability, tool-use support.
- Graceful degradation: failed provider marked unavailable in UI, not crash.

---

## R3: Token Cost Overrun — HIGH

**Risk:** Agent consumes excessive tokens. Users exceed budget unexpectedly.

**Impact:** Financial impact, trust erosion.

**Mitigation:**
- Token-level budget with hard enforcement per-call (not just per-run).
- Cost estimation before run shown in UI (based on historical averages once available; honest "unknown" for first run).
- Streaming abort via UI button.
- Agent loop cap: 30 tool calls.
- Lean prompts: no redundant context.
- Model tier auto-selection: cheapest model that fits task complexity.
- Daily/weekly/monthly caps configurable.

---

## R4: File Corruption from Concurrent Writes — MEDIUM

**Risk:** Agent writes a file while file watcher reads it, or power loss during write.

**Impact:** Project state corruption.

**Mitigation:**
- Atomic writes: write to temp file, `os.Rename` (atomic on Linux ext4/btrfs/xfs).
- Agent executor is single-writer per project (enforced by in-process mutex).
- File watcher debounce: 500ms before syncing to SQLite.
- Pre-execution git commit: rollback available.

---

## R5: Cross-Distribution Compatibility — MEDIUM

**Risk:** Go binary or frontend doesn't work on all Linux distros.

**Impact:** App doesn't start or behaves incorrectly.

**Mitigation:**
- Go produces a static binary — no shared library dependencies.
- Frontend runs in the user's browser — no WebKitGTK version issues.
- `modernc.org/sqlite` is pure Go — no C compiler or shared libs needed.
- CI matrix: Ubuntu 22.04+, Fedora 39+, Arch, Debian 12+.
- The only external dependency is a web browser (universally available).

---

## R6: Ollama Integration Complexity — MEDIUM

**Risk:** Ollama setups vary. Tool-use support varies by model.

**Impact:** Setup friction, broken local execution.

**Mitigation:**
- Auto-detect Ollama at `localhost:11434` on startup.
- Configurable URL for non-standard setups.
- Model capability probing: make a test tool-use call during model assignment. If it fails, warn and suggest alternatives.
- Clear error messages: "Model X does not support tool use. Try qwen2.5-coder:7b."
- Tool-use fallback (structured JSON output) planned for v1.1.

---

## R7: Scope Creep / UX Complexity — MEDIUM

**Risk:** The framework's complexity (6 roles, 9 statuses) overwhelms users who just want autonomous delivery.

**Impact:** Poor adoption, high abandonment.

**Mitigation:**
- "Quick mode": one-click project creation → auto-scaffold → auto-shape → start building. Full framework runs underneath, user sees dashboard only.
- Progressive disclosure: simple view (project → tasks → done) with expandable detail.
- Sensible defaults: pre-configured roles, models, budgets. Power users customize.
- First-run target: <10 minutes from install to first autonomous build (for users with API keys ready or Ollama already running).

---

## R8: Security — LLM Executing Arbitrary Commands — HIGH

**Risk:** LLM-generated shell command damages the user's system.

**Impact:** Data loss, security breach.

**Mitigation:**
- Command allowlist/denylist per role.
- Write-path restriction: agents write only within project directory.
- Go's `exec.Command` uses explicit argument arrays, not shell interpolation — prevents command injection by default.
- Three execution modes: unrestricted, restricted (allowlist), approval (user confirms each command).
- All commands logged in SQLite for audit.

---

## R9: Localhost API Security — HIGH (NEW)

**Risk:** The Go backend serves HTTP on a localhost port. Any local process can access it. A malicious website could attempt cross-origin requests. CSRF attacks could trigger agent execution.

**Impact:** Unauthorized agent execution, API key exfiltration, project manipulation.

**Mitigation:**
- Session token generated on startup (`crypto/rand`, 32 bytes).
- Token printed to terminal and embedded in the browser URL.
- All API calls require `Authorization: Bearer <token>`.
- No CORS headers set — same-origin only. Cross-origin requests blocked by browser.
- Content Security Policy headers prevent XSS.
- Token regenerated on every restart.
- SSE connection authenticated via query parameter (EventSource limitation).

---

## R10: SSE Connection Limits — MEDIUM (NEW)

**Risk:** HTTP/1.1 browsers allow max 6 connections per origin. SSE consumes one persistent connection. Multiple tabs exhaust the limit.

**Impact:** App becomes unresponsive in additional browser tabs.

**Mitigation:**
- Detect multiple tabs in the frontend (BroadcastChannel API). Only one tab maintains the SSE connection; others receive events via BroadcastChannel.
- If HTTP/2 is needed: Go can serve HTTP/2 over TLS with a self-signed localhost certificate. Deferred to v1.1 — single-tab + BroadcastChannel is sufficient for v1.

---

## R11: modernc.org/sqlite Performance — MEDIUM (NEW)

**Risk:** Pure Go SQLite is 2-10x slower than CGo sqlite3 for write-heavy workloads. File watcher sync and token tracking generate many writes.

**Impact:** UI lag, slow dashboard queries during agent execution.

**Mitigation:**
- Benchmark during Phase 1.8 with realistic load (100 file changes, 50 token tracking entries).
- If too slow, switch to `mattn/go-sqlite3` (CGo, requires C compiler for building from source — but prebuilt binaries can be distributed).
- Write batching: accumulate file watcher events for 500ms, sync in one transaction.
- WAL mode: `PRAGMA journal_mode=WAL` for concurrent read/write.
- Native SQLite data (runs, token_usage) is append-mostly — writes are small.

---

## R12: Browser Keyboard Shortcut Conflicts — LOW (NEW)

**Risk:** UX design specifies `Ctrl+N`, `Ctrl+R` etc. which are intercepted by the browser.

**Impact:** Keyboard shortcuts don't work as designed.

**Mitigation:**
- Use non-conflicting shortcuts: `Ctrl+Shift+N` (new project), `Ctrl+Shift+R` (run), `Ctrl+Shift+.` (abort).
- Or use single-key shortcuts when a Warpspawn element has focus (not global browser shortcuts).
- Document keyboard shortcuts in the app's help panel.

---

## R13: fsnotify Limitations — LOW (NEW)

**Risk:** fsnotify may hit inotify watch limits on large projects, doesn't work on network filesystems.

**Impact:** File changes not detected, stale dashboard.

**Mitigation:**
- Periodic re-index every 60s as safety net (catches missed events).
- If inotify limit hit: log warning, fall back to periodic-only mode.
- Network filesystem: detect and warn. Recommend local storage.
- Full re-index on startup regardless.

---

## Risk Summary Matrix

| Risk | Likelihood | Impact | Priority |
|---|---|---|---|
| R1: Agent loop reliability | High | Critical | P0 |
| R3: Token cost overrun | High | High | P0 |
| R8: Arbitrary command execution | Medium | Critical | P0 |
| R9: Localhost API security | Medium | High | P0 |
| R2: Provider API changes | Medium | Medium | P1 |
| R4: File corruption | Medium | High | P1 |
| R6: Ollama integration | Medium | Medium | P1 |
| R7: UX complexity | Medium | High | P1 |
| R10: SSE connection limits | Low | Medium | P1 |
| R11: SQLite performance | Medium | Medium | P1 |
| R5: Cross-distro compat | Low | Medium | P2 |
| R12: Keyboard shortcuts | Low | Low | P2 |
| R13: fsnotify limitations | Low | Low | P2 |

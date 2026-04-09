# Architecture Decisions

## ADR-001: Application Architecture — Go Binary + Browser UI

**Context:** The application needs a desktop experience with a modern UI. Options evaluated: Electron (all TypeScript), Tauri v2 (Rust + TypeScript), Go + Tauri (Go + Rust + TypeScript), Go + Browser (Go + TypeScript).

**Decision:** Single Go binary serving an embedded Svelte 5 frontend via localhost HTTP. Opens in the user's default browser.

**Rationale:**
- **Efficiency:** ~15MB binary, ~40MB RAM. Smallest footprint of all options.
- **Simplicity:** One build toolchain (Go + Vite). No Rust, no Electron, no WebView dependency.
- **Security:** Zero npm backend dependencies. Go is memory-safe. Single static binary.
- **Distribution:** Download binary, `chmod +x`, run. No AppImage required (though one can be provided for desktop integration).
- **LLM buildability:** Go is one of the best languages for LLM-generated code — simple, explicit, stdlib-rich, compiles in <2s.
- **Free capabilities:** Headless mode (`--no-browser`), remote access from LAN/phone, SSH tunnel support — all come for free.
- **Future path:** The Svelte frontend can be wrapped in Tauri v2 later for a native window experience without changing any backend or frontend code.

**Trade-offs:**
- Opens in a browser tab, not a native window. Manageable — Grafana, Jupyter, Ollama web UI use the same pattern.
- System tray integration is fragmented on Linux (GNOME removed tray icons). Mitigated by terminal process management.
- Browser tab competes with user's other tabs. Mitigated by clean UI design and optional pinned-tab behavior.

---

## ADR-002: Backend Language — Go

**Context:** The runtime logic (decision engine, agent executor, providers, guard system) needs a language. Options evaluated: TypeScript/Node.js (existing code), Rust (maximum efficiency), Go (balance of efficiency + LLM buildability), Python.

**Decision:** Go.

**Rationale:**
- **LLM code generation quality:** LLMs produce correct Go on the first try far more often than Rust. Rust's borrow checker causes iterative compilation failures that burn tokens. Since AI agents are the primary builders, this is a critical factor.
- **Efficiency:** Single static binary, ~15MB, ~40MB RAM. 90% of Rust's efficiency without the complexity.
- **Stdlib coverage:** net/http, encoding/json, os/exec, embed, testing — covers 90% of needs with zero external dependencies.
- **Zero npm supply chain:** No node_modules, no transitive dependency risk.
- **Compilation speed:** <2s for a full build. Fast iteration cycles for agent-driven development.
- **Concurrency:** Goroutines make concurrent streaming (LLM output + tool execution + UI updates) trivial.

**Key Go dependencies (minimal):**
- `modernc.org/sqlite` — pure Go SQLite, no CGo, no C compiler needed
- `github.com/fsnotify/fsnotify` — file watching for SQLite sync
- Standard library covers everything else (HTTP, JSON, YAML, process exec, file I/O, testing)

**Trade-offs:**
- Full rewrite of the existing ~2000-line JavaScript runtime. Estimated 3-4 weeks.
- Smaller contributor pool than TypeScript (mitigated: frontend is still TypeScript/Svelte).
- No generics-based type inference as rich as TypeScript (mitigated: Go 1.22+ generics are sufficient).

---

## ADR-003: Frontend Framework — Svelte 5

**Context:** The dashboard needs a dynamic, reactive UI. Options: React, Vue, Svelte, vanilla.

**Decision:** Svelte 5 (with SvelteKit for routing if needed).

**Rationale:**
- Smallest bundle size — compiles to vanilla JS, no runtime framework overhead.
- Runes-based reactivity in Svelte 5 is simple and performant.
- Excellent for streaming updates (agent output, budget counters, status changes).
- Fast development velocity.

**Trade-offs:**
- Smaller ecosystem than React (acceptable for a focused dashboard).
- Fewer UI component libraries (the UI is custom by design).

---

## ADR-004: State Architecture — Flat Files + SQLite Index

**Context:** The framework uses flat files (markdown + JSON) as the source of truth. A dashboard UI needs fast queries across projects.

**Decision:** Hybrid — flat files remain authoritative, SQLite serves as a read-optimized index.

**Rationale:**
- Flat files are git-friendly, human-readable, and portable.
- SQLite enables fast queries ("all blocked tasks across all projects") without re-parsing markdown on every render.
- A file watcher (fsnotify) syncs file changes into SQLite.
- Token usage, cost history, and run logs live natively in SQLite (not files).

**Schema boundaries:**
- `projects`, `tasks`, `reviews`, `backlog_items` — indexed from flat files, file path is the primary key
- `runs`, `token_usage`, `cost_entries` — native SQLite data, never written to flat files
- `settings`, `providers`, `roles` — SQLite with export/import capability

**Trade-offs:**
- Two sources of truth for project data (file is authoritative, SQLite is derived).
- File watcher can miss changes if the app isn't running (mitigated: full re-index on startup).

---

## ADR-005: LLM Integration — Direct API, No Agent Framework Dependency

**Context:** The current system shells out to OpenClaw CLI for LLM access, incurring ~5,000-10,000 tokens of overhead per session.

**Decision:** Direct HTTP calls to provider APIs with an in-process tool execution loop.

**Rationale:**
- Eliminates all third-party agent session overhead.
- Full control over what tokens are sent per call.
- Token usage reported directly from API responses.
- Tool execution (file read/write, shell) runs in-process — no detached process PID tracking.

**Implementation:**
- Provider interface in Go: `Complete(ctx, messages, tools, options) -> Stream`
- Built-in tool definitions: `read_file`, `write_file`, `run_command`, `list_files`
- Agent loop: send prompt → receive tool calls → execute tools → feed results back → repeat until done or budget exhausted
- Each provider adapts the interface to its API (Ollama, OpenAI, Anthropic all support tool use).

**Trade-offs:**
- Must implement the agent tool loop (~500-800 lines Go). Well-understood engineering.
- Must handle streaming, abort, retry, and partial responses per provider.

---

## ADR-006: Packaging — Single Binary + Optional AppImage

**Context:** Must be easily installable on any Linux distribution.

**Decision:** Primary distribution is a single Go binary. Optional AppImage for desktop integration (`.desktop` file, icon, file associations).

**Rationale:**
- Go produces a single static binary. No runtime dependencies, no shared libraries.
- `chmod +x warpspawn && ./warpspawn` — the simplest possible install.
- AppImage adds desktop integration for users who want it (menu entry, icon).
- Both distributed via itch.io (5 EUR pay-what-you-want) and GitHub Releases.

**Auto-update:** Binary checks GitHub Releases API on startup. Downloads new version, replaces itself, prompts restart.

---

## ADR-007: Security Model — Defense in Depth

**Context:** The app executes LLM-generated code and shell commands on the user's machine.

**Decision:** Layered defense.

**Layers:**
1. **API key storage** — OS keyring (libsecret on Linux), never plaintext config files
2. **Role boundary enforcement** — pre/post-execution file manifest diffing
3. **Shell execution modes** — configurable: unrestricted (default), restricted (allowlist), approval-required
4. **Token/cost budget enforcement** — hard caps enforced per-call
5. **Atomic file writes** — write to temp file, rename
6. **LLM output sanitization** — validate tool call structure before execution
7. **Path containment** — agents can only write within the project directory

---

## ADR-008: Concurrency Model — Serial Default, Parallel Cloud Optional

**Context:** Local LLMs require exclusive GPU access. Cloud APIs can run in parallel.

**Decision:**
- Local execution (Ollama): strictly serial. Enforced by a semaphore.
- Cloud execution: parallel up to configurable limit (default: 2). Different roles can run simultaneously.
- Mixed mode: local Builder + cloud Reviewer can run concurrently.

**Implementation:** Job queue with configurable concurrency per provider type.

---

## ADR-009: Workflow as Data Structure (v2 Preparation)

**Context:** The decision engine hardcodes task status names and transitions. Custom workflows would require a rewrite. We want v2 custom workflows without a rewrite.

**Decision:** Extract the workflow into a single data structure in v1. The decision engine reads from this structure. v1 ships only the default workflow; custom workflows are v2.

**The data structure:**
```go
type Workflow struct {
    ID          string
    Statuses    map[string]StatusDef
    Transitions []Transition
    Routing     RoutingConfig
}

type StatusDef struct {
    Phase    string // planning, execution, closed
    Terminal bool
    Active   bool
}

type Transition struct {
    From string
    To   string
    Role string
}

type RoutingConfig struct {
    ActiveStatuses   []string
    ReadyStatus      string
    ShapingStatus    string
    TerminalStatuses []string
    BuilderEntry     string
    ReviewEntry      string
}
```

**v1 rules:**
1. No status string literals in the decision engine — always reference `workflow.Routing.*`
2. Transition validation — log warning if a status change isn't in `workflow.Transitions`
3. Export default workflow as YAML in the framework directory

**v2 path:** Load custom `workflow.yaml` from project config. Decision engine already reads from the struct.

**Cost:** Minimal — just discipline in how statuses are referenced.

---

## ADR-010: Communication — HTTP + Server-Sent Events

**Context:** The Svelte frontend needs to communicate with the Go backend. Previous design considered stdio JSON-RPC for a sidecar. With Go + Browser architecture, the communication is HTTP.

**Decision:** REST API for commands, Server-Sent Events (SSE) for streaming.

**Implementation:**
- `POST /api/project/create` — create project
- `GET /api/projects` — list projects
- `GET /api/project/:id` — project detail
- `POST /api/run/start` — start agent execution
- `POST /api/run/abort` — abort running agent
- `GET /api/events` — SSE stream for real-time updates (agent output, status changes, budget updates)

**Rationale:**
- SSE is simpler than WebSocket for server-to-client streaming (which is the dominant direction).
- REST for commands is standard and debuggable (curl, browser dev tools).
- No WebSocket connection management, reconnection logic, or protocol overhead.
- SSE auto-reconnects on connection loss (built into the browser EventSource API).

**Trade-offs:**
- SSE is server-to-client only. Client-to-server uses regular HTTP POST. This is fine — commands are infrequent, streaming is frequent.

---

## ADR-011: Git Integration — Auto-Commit with Branch Safeguard

**Context:** Agents modify project files. Rollback capability is critical for recovery.

**Decision:** Auto-init git in every new project. Auto-commit before and after each agent run. For imported repos with existing `.git`, agent commits go to a dedicated `warpspawn/runs` branch.

**Commit format:** `[warpspawn] pre-execution: Builder TASK-003 (gpt-5.4-mini)`

**Trade-offs:**
- Adds git dependency (acceptable — virtually all developers have git).
- Git history grows with agent runs (mitigated: periodic gc, small commits).

---

## ADR-012: Extension Model — Data-Driven Roles, Pluggable Providers

**Context:** Users need to customize roles, add LLM providers, and extend workflows.

**Decision:**
- **Roles** defined as YAML in `~/.config/warpspawn/roles/` and per-project `config/roles/`. 5 default roles ship as built-in YAML.
- **Providers** conform to a Go interface. Built-in: Ollama, OpenAI, Anthropic. Users can request additional providers via issues/PRs.
- **Hooks** generalize the guard pattern: `pre-spawn`, `post-spawn` hooks configurable per project.
- **Notification channels** abstracted behind an interface. Built-in: in-app, desktop (`notify-send`).

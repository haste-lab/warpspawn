# Technical Architecture

## System Overview

```
┌─────────────────────────────────────────────────┐
│              User's Browser                      │
│  ┌───────────────────────────────────────────┐   │
│  │         Svelte 5 Frontend                  │   │
│  │  ┌──────────┐ ┌──────────┐ ┌───────────┐  │   │
│  │  │ Project  │ │  Agent   │ │  Budget / │  │   │
│  │  │Dashboard │ │ Activity │ │ Cost Panel│  │   │
│  │  └────┬─────┘ └────┬─────┘ └─────┬─────┘  │   │
│  │       │             │             │        │   │
│  │  ┌────▼─────────────▼─────────────▼─────┐  │   │
│  │  │   HTTP REST + SSE EventSource         │  │   │
│  │  └──────────────────┬────────────────────┘  │   │
│  └─────────────────────┼──────────────────────┘   │
└─────────────────────────┼─────────────────────────┘
                          │ localhost:<port>
┌─────────────────────────▼─────────────────────────┐
│            Go Binary (single process)              │
│                                                    │
│  ┌─── HTTP Server ──────────────────────────────┐  │
│  │  REST API + SSE + embedded static assets      │  │
│  └──────────────────────┬───────────────────────┘  │
│                         │                          │
│  ┌──────────────────────▼───────────────────────┐  │
│  │              Core Runtime                     │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────┐  │  │
│  │  │ Decision │ │  Action  │ │  Escalation  │  │  │
│  │  │ Engine   │ │ Executor │ │  State       │  │  │
│  │  └────┬─────┘ └────┬─────┘ └──────────────┘  │  │
│  │       │             │                         │  │
│  │  ┌────▼─────────────▼─────────────────────┐   │  │
│  │  │          Agent Executor                 │   │  │
│  │  │  prompt build · tool loop · streaming   │   │  │
│  │  └──┬──────────┬──────────────┬───────────┘   │  │
│  │     │          │              │               │  │
│  │  ┌──▼────┐ ┌───▼─────┐ ┌─────▼───────┐       │  │
│  │  │Ollama │ │ OpenAI  │ │  Anthropic  │       │  │
│  │  └───────┘ └─────────┘ └─────────────┘       │  │
│  │                                               │  │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────────┐  │  │
│  │  │  SQLite  │ │  Guard   │ │  File-backed │  │  │
│  │  │  Index   │ │  System  │ │  Project State│  │  │
│  │  └──────────┘ └──────────┘ └──────────────┘  │  │
│  └───────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
            Single binary: ~15MB
```

## Project Structure

```
warpspawn/
├── cmd/
│   └── warpspawn/
│       └── main.go                # Entry point, CLI flags, server startup
│
├── internal/
│   ├── server/
│   │   ├── server.go              # HTTP server, routing, middleware
│   │   ├── api.go                 # REST API handlers
│   │   └── sse.go                 # Server-Sent Events stream
│   │
│   ├── core/
│   │   ├── workflow.go            # Workflow data structure (ADR-009)
│   │   ├── project.go             # Project loading, file discovery
│   │   ├── parser.go              # Markdown/YAML parsing
│   │   ├── engine.go              # Decision engine (chooseNextAction)
│   │   ├── executor.go            # Action execution, state mutations
│   │   └── escalation.go          # Escalation state machine
│   │
│   ├── agent/
│   │   ├── runner.go              # Tool loop: prompt → LLM → tools → iterate
│   │   ├── tools.go               # Built-in tools: read, write, shell, list
│   │   └── prompt.go              # Role-specific prompt assembly
│   │
│   ├── provider/
│   │   ├── provider.go            # Provider interface definition
│   │   ├── ollama.go              # Ollama HTTP client
│   │   ├── openai.go              # OpenAI API client
│   │   └── anthropic.go           # Anthropic API client
│   │
│   ├── guard/
│   │   ├── budget.go              # Token-level budget tracking
│   │   ├── validate.go            # File manifest + role boundary check
│   │   └── hooks.go               # Pre/post execution hooks
│   │
│   ├── db/
│   │   ├── db.go                  # SQLite connection, queries
│   │   ├── sync.go                # File watcher → SQLite sync
│   │   └── migrations.go          # Schema migrations
│   │
│   ├── config/
│   │   ├── config.go              # Settings load/save
│   │   └── keyring.go             # OS keyring integration
│   │
│   └── git/
│       └── git.go                 # Auto-commit, branch management
│
├── framework/                     # Reusable delivery framework (as-is)
│   ├── roles/                     # YAML role definitions
│   ├── templates/                 # Project scaffolding templates
│   ├── workflows/                 # Default workflow YAML
│   └── config/                    # Default policies
│
├── frontend/                      # Svelte 5 application
│   ├── src/
│   │   ├── lib/
│   │   │   ├── components/
│   │   │   │   ├── ProjectList.svelte
│   │   │   │   ├── TaskPipeline.svelte
│   │   │   │   ├── AgentActivity.svelte
│   │   │   │   ├── BudgetPanel.svelte
│   │   │   │   ├── EscalationInbox.svelte
│   │   │   │   ├── SettingsPanel.svelte
│   │   │   │   └── SetupWizard.svelte
│   │   │   ├── stores/
│   │   │   │   ├── projects.ts
│   │   │   │   ├── agents.ts
│   │   │   │   └── budget.ts
│   │   │   └── api.ts             # HTTP + SSE client
│   │   ├── routes/
│   │   │   ├── +layout.svelte
│   │   │   ├── +page.svelte       # Dashboard
│   │   │   ├── project/[id]/
│   │   │   └── settings/
│   │   └── app.html
│   ├── package.json
│   └── vite.config.ts
│
├── go.mod
├── go.sum
├── Makefile                       # build, dev, test, release targets
├── LICENSE                        # Apache 2.0
└── README.md
```

## Data Flow

### Project Creation
```
User clicks "New Project" → enters brief text
  → POST /api/project/create { brief, stack, budget }
  → Go scaffolds project directory from framework/templates/
  → Go runs Mission Control (LLM call) for decomposition
  → MC produces: backlog.md, task files, docs
  → File watcher syncs to SQLite
  → SSE event: project.created
  → Frontend renders project dashboard
```

### Autonomous Task Execution
```
User clicks "Run Next Task" (or cron triggers pickup)
  → POST /api/run/start { projectId }
  → Decision engine evaluates project state
  → Selects highest-priority actionable task
  → Infers model tier from task properties
  → Agent runner:
      1. Assembles lean prompt (role instructions + task)
      2. Calls LLM provider (streaming via goroutine)
      3. Receives tool calls (read_file, write_file, run_command)
      4. Executes tools in-process
      5. Feeds results back to LLM
      6. SSE events stream to frontend: agent.chunk, agent.tool, agent.progress
      7. Repeats until agent signals completion or budget/timeout hit
  → Guard validates file changes against role boundaries
  → Git auto-commit (post-execution)
  → Runtime updates task status, writes artifacts
  → File watcher syncs to SQLite
  → SSE event: run.complete, project.updated
```

### Token Tracking Flow
```
Every LLM API call returns token counts:
  → Go records to SQLite: run_id, project, role, task, model, input_tokens, output_tokens, cost
  → Budget check: if cumulative cost > limit → abort execution
  → SSE event: budget.updated
  → Frontend budget panel updates reactively
```

## API Design

### REST Endpoints (Frontend → Backend)

```
Projects:
  POST   /api/project/create          Create from brief
  GET    /api/projects                 List all projects
  GET    /api/project/:id              Project detail + tasks
  DELETE /api/project/:id              Archive project
  POST   /api/project/:id/import      Register external directory

Execution:
  POST   /api/run/start               Start agent run for a project
  POST   /api/run/abort               Abort running agent
  GET    /api/run/history/:projectId   Past runs with token data

Settings:
  GET    /api/settings                 Current config
  PUT    /api/settings                 Update config
  POST   /api/provider/test            Test provider connection
  GET    /api/provider/models/:id      List available models

Budget:
  GET    /api/budget                   Current usage and limits
  GET    /api/budget/history           Historical cost data
```

### SSE Events (Backend → Frontend)

```
agent.chunk     { runId, text }              Streaming LLM output
agent.tool      { runId, tool, args, result } Tool call executed
agent.complete  { runId, result }            Agent finished
project.updated { projectId }                Project state changed
escalation      { projectId, escalation }    Needs human input
budget.updated  { usage }                    Token/cost change
error           { message, severity }        System error
```

## Build Pipeline

```makefile
# Development
make dev          # Start Go server + Vite dev server with hot reload

# Production build
make build        # 1. Vite builds Svelte → frontend/dist/
                  # 2. Go embeds frontend/dist/ via embed.FS
                  # 3. go build → single binary

# Release
make release      # Cross-compile for linux/amd64 and linux/arm64
                  # Create AppImage (optional)
                  # Generate checksums

# Test
make test         # go test ./...
make test-ui      # vitest (frontend)
make test-all     # both
```

## Embedded Frontend

```go
package server

import "embed"

//go:embed all:frontend/dist
var frontendAssets embed.FS

// Served at / — Go's HTTP server serves the Svelte SPA
// API routes take precedence over static assets
```

This means the final binary contains everything — no external files needed.

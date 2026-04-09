# Runtime Topology

Use this file to map the reusable framework roles to actual OpenClaw runtimes and sessions.

## Goal

Keep the framework project agnostic while giving each active project a clear, durable runtime shape.

The framework files define behavior.
Project files define state.
Runtime mapping defines which roles stay persistent and how live sessions attach to that state.

## Default Persistence Model

Default role persistence:
- Mission Control: persistent per project
- Architect: optional persistent per project
- UX: on-demand
- Builder: persistent per project when the work is coding-heavy or iterative
- Reviewer/QA: on-demand by default
- Research: on-demand only

## Why This Default Exists

- Mission Control benefits most from continuity because it owns intake, sequencing, closure, and escalation.
- Architect only needs persistence when the project has ongoing structural decisions.
- UX and Reviewer/QA usually work best as bounded review passes, not idle background sessions.
- Builder benefits from persistence when using an ACP-backed coding runtime over multiple tasks.
- Research should remain temporary so unknowns do not turn into idle agent sprawl.

## Runtime Selection Rules

### Mission Control
Preferred runtime:
- persistent OpenClaw session bound to the project

Responsibilities:
- read and update project files
- decide the next owner role
- enforce stage gates
- close, reopen, block, or escalate work

### Architect
Preferred runtime:
- persistent session for design-heavy projects

Fallback:
- invoke on demand for shaping or blocker resolution

### UX
Preferred runtime:
- on-demand session or task run

Use when:
- flows are unclear
- UI acceptance criteria are missing
- accessibility or interaction design needs shaping

### Builder
Preferred runtime:
- persistent ACP-backed project session when available

Fallback:
- standard OpenClaw local execution with file edits and local validation

Use a persistent Builder when:
- implementation spans multiple tasks
- codebase context is large
- iterative coding is expected
- tool setup or environment warmup would otherwise be repeated

### Reviewer/QA
Preferred runtime:
- on-demand review pass per task or work slice

Optional persistent mode:
- only when a project is review-heavy and repeated context reuse clearly helps

### Research
Preferred runtime:
- on-demand only

Use when:
- project files and trusted docs are insufficient
- external uncertainty is blocking progress

## Session Naming Convention

Use stable, project-scoped names.
Recommended pattern:
- `session:mc:<project-slug>`
- `session:arch:<project-slug>`
- `session:ux:<project-slug>`
- `session:builder:<project-slug>`
- `session:qa:<project-slug>`
- `session:research:<project-slug>`

Examples:
- `session:mc:academy-web-testing-app`
- `session:builder:academy-web-testing-app`

## Source of Truth Rule

Live sessions are helpers, not the source of truth.

Authoritative state must stay in project files:
- `docs/`
- `backlog/`
- `tasks/`
- `reviews/`
- `status/`

Do not rely on chat history alone for:
- task status
- review outcomes
- dependency state
- project decisions
- closure state

## Per-Project Runtime Map

Each project should keep a runtime map file, for example:
- `projects/<project-name>/runtime-map.yaml`

That file should record:
- project identity
- which roles are persistent
- session keys or naming targets
- preferred Builder mode
- any project-specific runtime notes

## Human Interaction Model

Recommended default:
- the human talks to the main session or the project Mission Control session
- Mission Control reads project files and decides the next step
- other roles are invoked through explicit handoff, not ad hoc side conversations

## Activation Sequence for a New Project

1. create the project files from templates
2. fill the project brief and initial backlog
3. create `runtime-map.yaml`
4. designate a persistent Mission Control session for the project
5. optionally designate a persistent Builder ACP session
6. keep all meaningful state changes in project files
7. let Mission Control coordinate the next owner role

## Guardrails

- Do not make every role persistent by default.
- Do not let Builder or Reviewer/QA close work without Mission Control.
- Do not store durable project state only in session memory.
- Do not share one Builder session across unrelated projects.
- Prefer one persistent Mission Control per active project over one giant multi-project control thread.

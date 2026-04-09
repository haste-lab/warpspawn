# Delivery Workflow

## Stage Model

1. Intake
2. Shaping
3. Task Ready
4. Build
5. Review
6. Rework or Done
7. Closure

## Default Flow

- Mission Control -> UX + Architect -> Builder -> Reviewer/QA -> Mission Control

## Stage Entry / Exit Rules

### 1. Intake
Owner: Mission Control

Entry:
- project request exists

Actions:
- create or open project folder
- initialize project files from templates
- capture scope, goals, constraints, risks, and unknowns

Exit requires:
- initial brief written
- backlog initialized
- status log initialized

### 2. Shaping
Owners: UX and Architect

Actions:
- UX defines flows, accessibility expectations, and UI acceptance criteria
- Architect defines solution shape, interfaces, constraints, and NFRs
- decisions and open questions are recorded

Exit requires:
- `docs/ux-spec.md` updated
- `docs/architecture-note.md` updated
- unresolved blockers either cleared or explicitly marked

### 3. Task Ready
Owner: Mission Control

Actions:
- decompose work into bounded tasks
- assign role owner
- add dependencies, acceptance criteria, and links to source specs

Exit requires:
- task file exists
- task status is `ready-for-build`
- Architect and UX inputs are reflected in the task

### 4. Build
Owner: Builder

Actions:
- implement only approved task scope
- update task execution notes and status
- run targeted validation

Exit requires:
- implementation complete
- validation results recorded
- task handed to review

### 5. Review
Owner: Reviewer/QA

Actions:
- validate against acceptance criteria, UX spec, architecture constraints, and tests
- write review report

Exit options:
- `approved` -> Mission Control closure
- `rejected` -> back to Builder as `rework`
- `blocked` -> Mission Control escalation

### 6. Rework or Done
Owner: Mission Control

Actions:
- route rejected work back with explicit defects
- close approved task and update backlog/status

### 7. Closure
Owner: Mission Control

Actions:
- synthesize result
- update status log and backlog state
- record decision or lesson if needed

## Dashboard-Ready Task Fields

Each task should expose:
- `task_id`
- `title`
- `status`
- `priority`
- `owner_role`
- `depends_on`
- `source_files`
- `acceptance_criteria`
- `validation`
- `review_outcome`
- `last_updated`

## Handoff Rules

- Every handoff must name the next owner role.
- Every handoff must link the source task and relevant spec files.
- Builder cannot skip review.
- Reviewer cannot silently fix and approve unless the task explicitly permits micro-fixes.
- Mission Control is the only role that marks final closure.

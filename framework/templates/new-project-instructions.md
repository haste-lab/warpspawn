# New Project Instructions

Use this procedure to create a new project from the framework.

## Create the Folder Structure

Create:
- `projects/<project-name>/docs/`
- `projects/<project-name>/backlog/`
- `projects/<project-name>/tasks/`
- `projects/<project-name>/reviews/`
- `projects/<project-name>/status/`

## Initialize Core Files

Copy or instantiate from templates:
- `templates/mission-control-entry.md` -> `projects/<project-name>/mission-control.md`
- `templates/project-orchestration.md` -> `projects/<project-name>/project-orchestration.md` (optional but recommended)
- `templates/runtime-map.yaml` -> `projects/<project-name>/runtime-map.yaml`
- `templates/project-brief.md` -> `projects/<project-name>/docs/project-brief.md`
- `templates/ux-spec.md` -> `projects/<project-name>/docs/ux-spec.md`
- `templates/architecture-note.md` -> `projects/<project-name>/docs/architecture-note.md`
- `templates/decision-log.md` -> `projects/<project-name>/docs/decision-log.md`
- `templates/backlog.md` -> `projects/<project-name>/backlog/backlog.md`
- `templates/project-state.md` -> `projects/<project-name>/status/project-state.md`
- `templates/status-log.md` -> `projects/<project-name>/status/status-log.md`
- `templates/tasks-readme.md` -> `projects/<project-name>/tasks/README.md`
- `templates/reviews-readme.md` -> `projects/<project-name>/reviews/README.md`

## First Project Fill-In

Mission Control should then:
- fill in the project brief
- write the initial project state
- seed the first backlog items
- fill `runtime-map.yaml` with the chosen runtime model for the project
- designate a persistent Mission Control session for the project
- optionally designate a persistent Builder ACP session for iterative coding work
- request shaping from UX and Architect
- convert shaped backlog items into bounded task files

## Working Rule

Do not start implementation until at least one task is explicitly `ready-for-build`.
Do not rely on live sessions alone for durable state; record runtime intent in `runtime-map.yaml` and project state in the normal project files.

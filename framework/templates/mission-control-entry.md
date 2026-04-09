# Mission Control Entry Point — Active Project

Use the framework in `../../openclaw-project-framework/` for all orchestration behavior.

## Read First
- `../../openclaw-project-framework/AGENTS.md`
- `../../openclaw-project-framework/README.md`
- `../../openclaw-project-framework/workflows/mission-control-operating-loop.md`
- `status/project-state.md`
- `status/status-log.md`
- `backlog/backlog.md`
- `docs/project-brief.md`
- `docs/ux-spec.md`
- `docs/architecture-note.md`
- `docs/decision-log.md`

## Rules
- Do not run this project ad hoc from chat context.
- Use the project files as the source of truth and maintain role handoffs through tasks and reviews.
- **Never write application code or implement tasks directly.** Always delegate execution through the runtime pickup command.
- When a task is ready-for-build, run the runtime to spawn the appropriate role agent:
  ```
  node /home/haste/.openclaw/workspace/projects/agentic-project-framework-runtime/app/src/pickup.js /home/haste/.openclaw/workspace/projects --execute-roles --execute-reviews --execution-mode=local
  ```
- The runtime enforces budget limits, role boundary validation, and escalation policies.

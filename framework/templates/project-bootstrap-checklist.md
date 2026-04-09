# Project Bootstrap Checklist

Use this checklist when creating a new project instance from the framework.

## 1. Project Initialization
- [ ] Create `projects/<project-name>/`
- [ ] Create folders:
  - `docs/`
  - `backlog/`
  - `tasks/`
  - `reviews/`
  - `status/`
- [ ] Create `mission-control.md` from `templates/mission-control-entry.md`
- [ ] Create optional `project-orchestration.md` from `templates/project-orchestration.md`
- [ ] Create `runtime-map.yaml` from `templates/runtime-map.yaml`
- [ ] Create `docs/project-brief.md`
- [ ] Create `docs/ux-spec.md`
- [ ] Create `docs/architecture-note.md`
- [ ] Create `docs/decision-log.md`
- [ ] Create `backlog/backlog.md`
- [ ] Create `status/project-state.md`
- [ ] Create `status/status-log.md`
- [ ] Create `tasks/README.md`
- [ ] Create `reviews/README.md`

## 2. Intake Quality Gate
- [ ] Project name, owner, goal, scope, constraints, and risks are captured
- [ ] At least one success metric is documented
- [ ] Open questions are explicitly listed
- [ ] First backlog slice is visible

## 3. Shaping Gate
- [ ] UX has written primary journeys and UI acceptance criteria
- [ ] Architect has written structure, interfaces, and constraints
- [ ] Open questions are either resolved or marked as blockers
- [ ] Any lasting decision is recorded in `docs/decision-log.md`

## 4. Build Readiness Gate
- [ ] Backlog items are decomposed into bounded task files
- [ ] Each build task includes acceptance criteria and dependencies
- [ ] Each task identifies the next handoff role
- [ ] Initial priority ordering is recorded
- [ ] `runtime-map.yaml` identifies the project runtime model
- [ ] Persistent Mission Control session target is defined
- [ ] Builder runtime choice is documented
- [ ] At least one task has status `ready-for-build`

## 5. Review Readiness Gate
- [ ] Builder recorded validation evidence in the task file
- [ ] Reviewer/QA received task file and implementation context
- [ ] Review report template is ready to instantiate

## 6. Operating Rule
- [ ] Project files, not chat, are treated as the source of truth
- [ ] Live sessions are mapped deliberately and do not replace durable records

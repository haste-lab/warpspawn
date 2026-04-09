# Project Orchestration

## Project Identity
- Name:
- Owner:
- Status:
- Last Updated:

## Active Goal
-

## Role Map
- Mission Control: intake, planning, decomposition, flow control, closure
- Architect: system design, interfaces, non-functional constraints
- UX: user journeys, interaction rules, accessibility, acceptance criteria
- Builder: implementation and validation evidence
- Reviewer/QA: review, defect finding, approval or rejection
- Research: optional support for unresolved external unknowns

## Default Flow
- Mission Control -> UX + Architect -> Builder -> Reviewer/QA -> Mission Control

## Task Lifecycle
- `intake`
- `shaping`
- `ready-for-build`
- `in-build`
- `in-review`
- `rework`
- `blocked`
- `done`
- `archived`

## Routing Rules
- Mission Control is the only role that marks tasks `ready-for-build`, `done`, or `archived`.
- Builder only works from explicit task files.
- Reviewer/QA does not bypass Mission Control for closure.
- UX and Architect shape work before build when requirements or constraints are unclear.
- Research is invoked only when project files and trusted docs are insufficient.

## Parallelism Rules
- Prefer one active build task per bounded workstream.
- Do not open multiple implementation tasks if one unresolved blocker affects all of them.
- Prefer finishing review and rework before starting more build work.

## Escalation Rules
Escalate to the human when:
- scope changes materially
- a product decision is needed
- secrets, production access, or destructive actions are required
- repeated review cycles do not converge
- time/cost/risk trade-offs require owner judgment

## Required Durable Records
Every meaningful project cycle should update at least one of:
- `backlog/backlog.md`
- `status/project-state.md`
- `status/status-log.md`
- `docs/decision-log.md`
- `tasks/<task>.md`
- `reviews/<review>.md`

## Operating Rule
Do not rely on chat alone as the source of truth. Project files are authoritative.

# Task and Review Record Conventions

Use these stable fields in every task file for dashboard and tooling compatibility.

## Task Required Fields
- `Task ID`
- `Title`
- `Status`
- `Priority`
- `Owner Role`
- `Depends On`
- `Source Files`
- `Last Updated`
- `Objective`
- `In Scope`
- `Out of Scope`
- `Acceptance Criteria`
- `Constraints`
- `Implementation Notes`
- `Validation`
- `Handoff`

## Allowed Task Status Values
Use only these values:
- `intake`
- `shaping`
- `ready-for-build`
- `in-build`
- `in-review`
- `blocked`
- `rework`
- `done`
- `archived`

## Task Status Ownership Rules
Use these default ownership rules unless the project explicitly overrides them.

- `intake`
  - set by: Mission Control
  - meaning: work exists, but is not yet properly shaped
- `shaping`
  - set by: Mission Control, UX, Architect
  - meaning: requirements/specs/constraints are still being clarified
- `ready-for-build`
  - set by: Mission Control only
  - meaning: task is bounded, shaped, and approved for execution
- `in-build`
  - set by: Builder only
  - meaning: implementation is actively underway
- `in-review`
  - set by: Builder only, when validation evidence is recorded and handoff is complete
  - meaning: ready for Reviewer/QA evaluation
- `blocked`
  - set by: any role, but blocker owner and reason must be recorded
  - meaning: progress cannot continue without intervention or missing input
- `rework`
  - set by: Mission Control after rejected review
  - meaning: task returns to Builder with explicit defects to address
- `done`
  - set by: Mission Control only after approved review
  - meaning: task is complete and closed in active delivery flow
- `archived`
  - set by: Mission Control only
  - meaning: task retained for record but no longer active in working views

## Required State Transition Rules
Default allowed transitions:
- `intake -> shaping`
- `intake -> blocked`
- `shaping -> ready-for-build`
- `shaping -> blocked`
- `ready-for-build -> in-build`
- `ready-for-build -> blocked`
- `in-build -> in-review`
- `in-build -> blocked`
- `in-review -> done`
- `in-review -> rework`
- `in-review -> blocked`
- `rework -> in-build`
- `rework -> blocked`
- `blocked -> intake`
- `blocked -> shaping`
- `blocked -> ready-for-build`
- `blocked -> in-build`
- `blocked -> archived`
- `done -> archived`

Avoid skipping major gates.
Examples to avoid:
- `ready-for-build -> done`
- `in-build -> done`
- `shaping -> in-review`

## Review Required Fields
- `Review ID`
- `Task ID`
- `Reviewer`
- `Outcome`
- `Date`
- `Checks Reviewed`
- `Acceptance Criteria Result`
- `Defects`
- `Risks / Notes`
- `Required Rework`
- `Final Recommendation`

## Allowed Review Outcomes
- `approved`
- `rejected`
- `blocked`

## Review-to-Task Rules
- `approved` supports `in-review -> done`
- `rejected` supports `in-review -> rework`
- `blocked` supports `in-review -> blocked`
- Reviewer/QA does not mark the task `done`; Mission Control performs closure
- Reviewer/QA should not silently implement missing work unless the task explicitly allows micro-fixes

## Status Log Conventions
Each row should represent a durable state change or meaningful checkpoint, not casual commentary.

Minimum columns:
- Date
- Item
- Status
- Owner Role
- Summary
- Blockers
- Next Step

## Backlog Conventions
Every backlog item should be linkable to either:
- a shaping artifact
- a task file
- a review outcome
- or an explicit blocker/decision entry

Backlog is the planning view.
Task files are the execution units.
Review files are the quality gate evidence.
Status logs are the historical flow record.

# Escalation Lifecycle Policy

Use this policy to decide whether an autonomous runtime should create, keep, suppress, or deliver a human-facing escalation.

## Goal

Escalations should be:
- relevant
- current
- non-duplicative
- bounded to active work

Escalations should not behave like immortal alarms.
A stale escalation artifact must not outrank newer authoritative project state.

## Core Rule

A human-facing escalation is valid only when all are true:
1. the project is active
2. autonomous pickup is enabled
3. escalation delivery is enabled
4. the task or work item is still unresolved
5. no newer authoritative state has superseded the escalation
6. the escalation is not a duplicate for the same task epoch

If any of those are false, suppress the escalation.

## Project Lifecycle Gate

Each project should carry explicit lifecycle state in `status/project-state.md`.

Recommended values:
- `active`
- `paused`
- `inactive`
- `archived`

Default rules:
- `active` -> eligible for pickup and escalation, subject to the remaining gates
- `paused` -> no automatic escalation delivery
- `inactive` -> no pickup, no escalation delivery
- `archived` -> historical only; no pickup, no escalation delivery

A project should also record:
- `Autonomous Pickup: enabled|disabled`
- `Escalation Delivery: enabled|disabled`

## Task-State Precedence

Newer authoritative task state outranks older escalation artifacts.

Suppress escalation if the relevant task is:
- `done`
- `archived`
- superseded by a successor task or phase
- approved in review and already closed or closure-ready
- otherwise terminal according to the project workflow

If a task has both:
- an old `needs-human-review` escalation
- and a newer approved/closed/archived outcome

then the newer task outcome wins.

## Resolution Invalidation

Whenever a newer authoritative state is written, prior unresolved escalations for the same task should be considered resolved or obsolete.

Examples:
- review outcome `approved` -> invalidate unresolved escalation for that task
- task moved to `archived` -> invalidate unresolved escalation for that task
- work reshaped into successor task -> invalidate prior-task escalation unless explicitly carried forward
- task closed by Mission Control -> invalidate unresolved escalation for that task

## Epoch / Phase Scoping

Escalation counting must be bounded to the current task phase or task epoch.

Do not accumulate thresholds forever across:
- old prototype phases
- archived work
- superseded task versions
- previously closed review loops

Recommended reset points:
- task materially reshaped
- task replaced by successor task
- task moved to archived baseline
- project enters a new delivery phase

## Duplicate Suppression

Do not deliver the same escalation repeatedly unless something materially changed.

A stable escalation identity should include at least:
- project
- task id
- escalation kind
- escalation reason
- epoch or phase id

If the same unresolved escalation has already been delivered for the same identity, suppress repeat delivery unless:
- the state changed materially
- the epoch changed
- a reminder policy explicitly allows another delivery

## Delivery Filter

The final message-delivery layer must re-check relevance before sending a human-facing escalation.

That layer should suppress escalations when:
- project is not active
- escalation delivery is disabled
- task is terminal or superseded
- newer state invalidates the escalation
- the escalation is a duplicate already delivered

This prevents stale runtime artifacts from reaching the user even if lower layers emitted them.

## Framework Guidance

Recommended durable project-state fields:
- `Project Lifecycle: active|paused|inactive|archived`
- `Autonomous Pickup: enabled|disabled`
- `Escalation Delivery: enabled|disabled`
- `Current Epoch: <string>`
- `Active Task: <task-id-or-none>`
- `Last Resolved Escalation: <id-or-none>`

## Implementation Guidance

At runtime, evaluate escalation validity in this order:
1. project lifecycle gate
2. task-state precedence gate
3. resolution invalidation gate
4. epoch/phase gate
5. duplicate suppression gate
6. delivery gate

If any gate fails, do not deliver the escalation.

## Anti-Patterns

Do not:
- treat `status/escalation-report.md` as permanently live just because it exists
- deliver escalations from inactive or archived projects
- let historical failure counts override current approved/closed outcomes
- carry prototype-phase escalation history forever into new hardening phases
- assume every project under `projects/` is still active work

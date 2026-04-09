# Definition of Done and Acceptance Gates

## Definition of Done

A task is done only when all of the following are true:
- scope implemented as described in the task file
- acceptance criteria satisfied
- UX expectations met for affected surfaces
- architecture constraints and interfaces respected
- relevant validation executed and results recorded
- review report written
- Mission Control marked task closed in project tracking files

A project milestone is done only when:
- all included tasks are closed
- known defects are either fixed, accepted, or explicitly deferred
- open risks and follow-ups are recorded
- status log reflects milestone completion

## Acceptance Gate Model

### Gate 1 — Intake Complete
Required:
- brief present
- goals and constraints captured
- initial backlog present

### Gate 2 — Shaping Complete
Required:
- UX spec updated
- architecture note updated
- key decisions recorded
- blockers surfaced

### Gate 3 — Build Ready
Required:
- task file approved
- acceptance criteria present
- dependencies identified
- no unresolved blocking ambiguity

### Gate 4 — Review Ready
Required:
- implementation complete
- targeted validation run
- evidence attached in task file

### Gate 5 — Accepted
Required:
- review report outcome is `approved`
- defects resolved or explicitly deferred by Mission Control

### Gate 6 — Closed
Required:
- backlog updated
- status log updated
- final handoff summary written

## Reject Conditions

Reviewer/QA should reject when:
- acceptance criteria are missing or unmet
- UX behavior contradicts the UX spec
- architecture constraints are violated
- validation is missing or insufficient
- defects materially affect correctness, usability, accessibility, or maintainability

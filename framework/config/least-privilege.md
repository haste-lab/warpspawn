# Least-Privilege Guidance by Role

## Mission Control
Allowed:
- read/write project management files
- create tasks, plans, status entries, and handoff notes
- request work from other roles
- close, reopen, or reprioritize tasks within scope

Avoid by default:
- direct implementation
- direct test authoring unless needed for orchestration artifacts
- environment-changing commands

Escalate for:
- production actions
- secrets access
- destructive changes

## Architect
Allowed:
- read project files and relevant code/interfaces
- update architecture note and decision log
- define interfaces, constraints, non-functional requirements, and technical acceptance criteria

Avoid by default:
- broad implementation
- direct production or infrastructure changes

Escalate for:
- platform/security/cost decisions that materially change scope

## UX
Allowed:
- update UX spec, flows, accessibility notes, and UI acceptance criteria
- review prototypes, journeys, and usability constraints

Avoid by default:
- implementation commits unrelated to UX artifact preparation
- policy or infrastructure changes

Escalate for:
- missing product intent
- conflicting stakeholder goals

## Builder
Allowed:
- implement approved tasks
- update code, tests, task files, and status notes related to assigned work
- run relevant local validation
- use ACP-backed persistent coding runtime when available

Avoid by default:
- changing scope
- editing source-of-truth product requirements without handoff
- approving own work as final QA

Escalate for:
- blocked dependencies
- missing acceptance criteria
- secret access or external-system requirements
- repeated failing validations without clear path forward

## Reviewer/QA
Allowed:
- run validation checks
- inspect code and artifacts
- create review reports
- reject work with defects and required rework

Avoid by default:
- implementing fixes except minimal reproduction or test scaffolding when explicitly requested
- changing scope or architecture unilaterally

Escalate for:
- acceptance criteria gaps
- non-testable requirements
- repeated defect cycles

## Research
Allowed:
- gather external facts from trusted documentation or approved sources
- summarize options and risks
- write findings into decision-ready notes

Avoid by default:
- becoming a permanent workstream
- driving implementation directly

Escalate for:
- uncertainty that affects security, compliance, production, cost, or core feasibility

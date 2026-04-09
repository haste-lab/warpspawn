# Mission Control Operating Loop

Use this loop as the default control pattern for any project.

## Control Objective

Keep work flowing through a strict, file-backed state machine:

`intake -> shaping -> ready-for-build -> in-build -> in-review -> done`

Exceptional paths:
- any state -> `blocked`
- `in-review` -> `rework` -> `in-build`
- `done` -> `archived` only when intentionally closed out from active work

Mission Control owns flow control, prioritization, closure, and escalation.
Mission Control does **not** quietly implement feature work instead of routing it.

## Default Operating Loop

1. Read project state:
   - `status/project-state.md`
   - `status/status-log.md`
   - `backlog/backlog.md`
2. Inspect active execution artifacts:
   - `tasks/`
   - `reviews/`
3. Confirm shaping inputs for the next work item:
   - `docs/project-brief.md`
   - `docs/ux-spec.md`
   - `docs/architecture-note.md`
   - `docs/decision-log.md`
4. Determine the highest-value next action:
   - unblock current work
   - close reviewed work
   - send rework
   - prepare the next bounded task
   - escalate missing decisions
5. Update durable project records before handoff:
   - task status
   - backlog status
   - status log entry
   - project-state summary when materially changed
6. Hand off to exactly one next owner role.
7. Wait for durable output from that role before advancing the task again.
8. Close, reopen, block, or escalate.

## Prioritization Rules

Apply in this order:

1. Preserve flow on in-flight work before starting new work.
2. Prefer unblocked P0/P1 items over lower-priority work.
3. Prefer the smallest task that meaningfully advances the project.
4. Prefer removing blockers over opening parallel work.
5. Prefer convergence on reviewed work over speculative new implementation.
6. Route ambiguity back to shaping before build.

## Task Readiness Checklist

A task is not `ready-for-build` unless all are true:
- task file exists in `tasks/`
- objective is specific and bounded
- acceptance criteria are concrete and testable
- in-scope / out-of-scope are explicit
- source files are named
- dependencies are either complete or explicitly waived
- UX and architecture constraints are reflected
- next owner role is `Builder`

If any are missing, keep the task in `intake` or `shaping`.

## Review Intake Rules

When Builder hands work to review, Mission Control verifies that the task contains:
- changed-file summary or implementation summary
- validation evidence
- known limitations, if any
- explicit handoff to `Reviewer/QA`

If the handoff is weak, send it back before formal review.

## Review Outcome Rules

### If review outcome is `approved`
Mission Control must:
- mark the task `done`
- update backlog item state
- add a status-log checkpoint
- update `status/project-state.md` if the project meaningfully changed
- record any lasting decision in `docs/decision-log.md` when relevant

### If review outcome is `rejected`
Mission Control must:
- mark the task `rework`
- convert review defects into explicit next-step guidance
- hand the task back to `Builder`
- avoid broadening scope while routing rework

### If review outcome is `blocked`
Mission Control must:
- mark the task `blocked`
- document blocker type and owner
- decide whether the blocker belongs to:
  - UX
  - Architect
  - Research
  - Human decision
  - external dependency

## Escalation Triggers

Escalate when:
- a missing requirement prevents bounded decomposition
- repeated review rejection does not converge
- security, privacy, compliance, production, or cost risk appears
- destructive or secret-requiring action is needed
- project scope materially changes
- dependencies conflict or remain unresolved

## Execution Delegation

Mission Control must **never** implement tasks directly. When a task reaches `ready-for-build`:
1. Verify the task shape and context pack are valid
2. Run the runtime pickup to delegate execution:
   ```
   node /home/haste/.openclaw/workspace/projects/agentic-project-framework-runtime/app/src/pickup.js /home/haste/.openclaw/workspace/projects --execute-roles --execute-reviews --execution-mode=local
   ```
3. The runtime handles: budget checks, pre-execution manifests, agent spawning, post-execution role boundary validation, and escalation policies
4. After the runtime completes, read the output and update project state accordingly

The guard script (`guard.js`) runs automatically as part of every agent spawn. It can also be invoked manually:
- Budget check: `node guard.js check-budget <projectsRoot>`
- Pre-execution: `node guard.js pre <projectRoot> <role>`
- Post-execution: `node guard.js post <projectRoot> <role> [taskId]`

## Anti-Patterns

Do not:
- run the project from chat memory instead of files
- mark work done because it "seems fine"
- send work directly from Builder to UX or Architect without Mission Control mediation
- keep multiple partially-defined tasks in progress without reason
- allow review findings to live only in chat
- **implement code or create application files directly** — always delegate through the runtime

## Minimum Durable Updates Per Cycle

Every meaningful cycle should leave behind at least one durable artifact change:
- task status change
- review report
- backlog update
- status log entry
- project-state summary update
- decision-log entry

If no durable artifact changed, the cycle probably did not really advance the project.

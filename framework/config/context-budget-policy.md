# Context Budget Policy

Use this policy to keep model context disciplined, relevant, and scalable.

## Goal

Do not try to load the entire framework, full project history, and broad codebase context into every prompt.

Instead:
- keep durable truth in files
- define role-specific context contracts
- pass only the minimum relevant working set
- summarize stable context when raw loading is wasteful
- decompose work when context pressure becomes too high

This policy addresses the root cause of context failure: poor context architecture.

## Core Rule

Long context is a convenience, not the architecture.

The architecture should depend on:
- file-backed state
- bounded task files
- role-specific input contracts
- selective retrieval
- durable summaries for decisions and status

## General Principles

### 1. Load the smallest sufficient context
Only include files that are necessary for the current role and task.

### 2. Prefer current state over long history
Prefer:
- latest project state
- current task file
- current review artifact
- current decision log

over raw historical logs unless history is directly relevant.

### 3. Use summaries for stable docs
When a brief, architecture note, or UX spec becomes long, prefer task-scoped excerpts or summaries rather than reloading the entire document each time.

### 4. Prefer retrieval over preloading
For code and large projects:
- retrieve task-relevant files
- retrieve adjacent dependencies only when needed
- avoid sending the full repo by default

### 5. Decompose before overflowing context
If a role cannot work safely without an oversized prompt, that usually means the task is too broad or the context contract is too loose.

## Recommended Role Budgets

These are working heuristics, not tokenizer-exact guarantees.

### Mission Control
Use mostly project-state and coordination context.

Recommended composition:
- framework/process docs: 20-40%
- project state and active artifacts: 40-60%
- slack/reserve for user request and synthesis: 10-20%

Mission Control should usually load:
- `status/project-state.md`
- `status/status-log.md`
- `backlog/backlog.md`
- active `tasks/*.md`
- relevant `reviews/*.md`
- relevant source specs only when needed
- one or two framework operating docs, not the whole framework

### Builder
Use mostly task scope and code context.

Recommended composition:
- framework/process docs: 5-15%
- task/spec constraints: 15-25%
- code + tests: 60-75%
- slack/reserve: 10-15%

Builder should usually load:
- assigned task file
- task-scoped architecture / UX excerpts
- named source files from the task
- nearest relevant tests
- small adjacent dependency files only when needed

Builder should not preload the full framework or full project history.

### Reviewer/QA
Use mostly acceptance criteria, validation evidence, and changed artifacts.

Recommended composition:
- framework/review rules: 10-20%
- task/spec context: 20-30%
- changed files and validation evidence: 40-60%
- slack/reserve: 10-20%

Reviewer/QA should usually load:
- task file
- review handoff
- validation evidence
- relevant changed files
- acceptance-driving spec excerpts

## Local Model Guidance

For local models with roughly 32k effective context:
- keep Mission Control working sets modest and state-focused
- keep Builder tightly task-scoped
- avoid loading full codebases
- reserve margin for reasoning and output

Safe practical working ranges often look like:
- framework slice: 2k-10k tokens
- project-state slice: 3k-8k tokens
- single task: 1k-3k tokens
- focused code slice: 8k-15k tokens
- nearby tests: 2k-6k tokens

If a role routinely exceeds this, decompose the work or improve retrieval.

## Escalation Trigger for Context Pressure

Escalate or reshape when:
- a task requires too many unrelated files to reason safely
- acceptance criteria depend on too much unstated context
- the role needs broad repo knowledge not captured in task scope
- context pressure forces repeated omission of important constraints

Do not solve this by blindly increasing prompt size if the task architecture is weak.

## Required Durable Artifacts

To reduce context load over time, maintain:
- concise task files
- concise review handoffs
- clear decision-log entries
- current project-state summaries
- runtime maps and lifecycle state

These artifacts should let a new run reconstruct the needed context without replaying large chat histories.

## Recommended Operating Pattern

1. load role-mandatory files
2. load task-mandatory files
3. load minimal code/spec dependencies
4. summarize if needed
5. execute
6. write durable outputs back to files

## Anti-Patterns

Do not:
- preload the whole framework for Builder work
- send the entire codebase for a small task
- rely on chat history as the only memory of constraints
- solve poor task scoping by throwing more tokens at the model
- assume theoretical model context equals practical safe context

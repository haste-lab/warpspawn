# Warpspawn

Autonomous agentic software delivery framework for Linux — spawn AI agents that build your project through structured roles, reviews, and guardrails.

## What is this?

Give Warpspawn a project description. It decomposes the work into bounded tasks, then executes them autonomously through specialized AI agents:

- **Mission Control** — decomposes, prioritizes, delegates, closes
- **Architect** — defines structure, interfaces, constraints
- **UX** — defines journeys, flows, acceptance criteria
- **Builder** — implements the code
- **Reviewer/QA** — validates against acceptance criteria

Each role has explicit boundaries. Tasks pass through shaping and review gates. State is durable and auditable. Guardrails are enforced in code, not by prompt compliance.

## Status

**Pre-release.** Architecture and project plan complete. Implementation starting.

See [docs/plan/](docs/plan/) for the full project plan.

## Architecture

- **Backend:** Go (single static binary, ~15MB)
- **Frontend:** Svelte 5 (embedded in the binary, served via localhost HTTP)
- **LLM Providers:** Ollama (local), OpenAI, Anthropic (cloud) — direct API, no framework overhead
- **State:** Flat files (git-friendly, human-readable) + SQLite index (fast dashboard queries)
- **Distribution:** Single binary for Linux. Optional AppImage for desktop integration.

## License

Apache 2.0 — see [LICENSE](LICENSE).

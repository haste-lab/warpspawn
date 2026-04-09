# Open Questions — All Resolved

## Q1: Product Name — DECIDED

**Decision: Warpspawn**

StarCraft-inspired (warp-in + spawn). Available on GitHub, npm, PyPI. Unique in dev tools, strong SEO. CLI: `warpspawn new "Build a weather dashboard"`

## Q2: License — DECIDED

**Decision: Apache 2.0 for everything.**

Revenue: Paid binary on itch.io (5 EUR, pay-what-you-want). Free to build from source. GitHub Sponsors for recurring support. No feature gating.

## Q3: Frontend-Backend Communication — DECIDED

**Decision: REST API + Server-Sent Events (SSE) over localhost HTTP.**

The Go backend serves the Svelte frontend and exposes REST endpoints for commands and an SSE stream for real-time events. See ADR-010. Authenticated via session token generated at startup.

Note: The original Q3 discussed IPC for a Tauri sidecar (stdio JSON-RPC). That architecture was replaced by Go+Browser. The communication mechanism is now standard HTTP.

## Q4: Ollama Tool-Use Compatibility — DECIDED

**Decision: Tool-use-capable models only for v1.**

Validate via test call at model assignment time. Clear error message for incompatible models. Structured JSON fallback planned for v1.1.

## Q5: Project Storage Location — DECIDED

**Decision: Default location + import registry.**

Default: `~/.local/share/warpspawn/projects/`. "Open Project" registers external directories via a `projects.json` registry. No file copying for imports.

## Q6: Git Integration — DECIDED

**Decision: Auto git with branch safeguard.**

Auto-init git in new projects. Auto-commit before/after each agent run. For imported repos with existing `.git`, agent commits go to a dedicated `warpspawn/runs` branch. User can disable per-project. Git checked at startup — if not installed, feature disabled with a warning (not a fatal error). See ADR-011.

## Q7: Multi-User / Collaboration — DECIDED

**Decision: Single-user with portable architecture.**

No absolute paths in project state. Relative paths only. Projects are self-contained and movable.

## Q8: Monetization — DECIDED

**Decision: Apache 2.0 open source + paid convenience binary.**

Binary on itch.io (5 EUR pay-what-you-want). GitHub Sponsors. Free to build from source.

## Q9: Framework Customization Depth — DECIDED

**v1 allows:**
- Custom roles (YAML in `~/.config/warpspawn/roles/` or per-project)
- Custom templates (user templates override shipped defaults)
- Custom guardrails (per-project `mayEdit`/`mayNotEdit` override, with warning)
- Custom notification channels (Go interface: send a message)

**v1 does NOT allow:**
- Custom LLM providers (providers are built into the Go binary; users submit PRs for new providers)
- Custom workflow stages (task lifecycle is fixed; see ADR-009 for v2 preparation)

Note: The original Q9 mentioned "JS modules conforming to provider interface" — that was from the Node.js architecture. In Go, providers are compiled into the binary. Third-party providers require a PR or a plugin system (v2 consideration).

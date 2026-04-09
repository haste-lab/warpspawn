# Project Plan — Warpspawn

A Linux desktop application for autonomous, role-based software delivery.

**Name:** Warpspawn
**License:** Apache 2.0
**Revenue:** Binary on itch.io (5 EUR pay-what-you-want)
**Stack:** Go backend + Svelte 5 frontend + Browser UI
**Architecture:** Single Go binary (~15MB) with embedded frontend, localhost HTTP + SSE

## Documents

| # | Document | Purpose | Last reviewed |
|---|---|---|---|
| 00 | [Executive Summary](00-executive-summary.md) | Product vision, audience, differentiator | 2026-04-09 |
| 01 | [Architecture Decisions](01-architecture-decisions.md) | 12 ADRs: language, architecture, state, LLM, packaging, security, concurrency, extensions, workflow, communication, git | 2026-04-09 |
| 02 | [Technical Architecture](02-technical-architecture.md) | System diagram, Go module structure, data flow, API design, build pipeline | 2026-04-09 |
| 03 | [Dependencies and Phases](03-dependencies-and-phases.md) | Dependency graph, 3-phase plan, 10-15 week timeline | 2026-04-09 |
| 04 | [Risk Assessment](04-risk-assessment.md) | 13 risks (4 P0, 5 P1, 4 P2) with mitigations | 2026-04-09 |
| 05 | [Security Plan](05-security.md) | Threat model, 9 security controls, localhost auth, CSP, CORS | 2026-04-09 |
| 06 | [Reliability and Maintenance](06-reliability-and-maintenance.md) | Process lifecycle, error recovery, atomic writes, SQLite backup, updates | 2026-04-09 |
| 07 | [UX Design](07-ux-design.md) | 6 views, browser-compatible shortcuts, terminal experience, accessibility | 2026-04-09 |
| 08 | [Testing Strategy](08-testing-strategy.md) | 5 test layers, Go test pipeline, tests built per-phase not deferred | 2026-04-09 |
| 09 | [Open Questions](09-open-questions.md) | All 9 decisions resolved, updated for Go architecture | 2026-04-09 |

## Decisions Summary

| Decision | Choice |
|---|---|
| Product name | **Warpspawn** |
| License | **Apache 2.0** (paid binary on itch.io, 5 EUR PWYW) |
| Backend language | **Go** (LLM-buildable, efficient, zero npm supply chain) |
| Frontend | **Svelte 5 + TypeScript** |
| Desktop shell | **Browser (localhost HTTP)** — single Go binary, no Tauri/Electron |
| Communication | **REST + SSE** (authenticated via session token) |
| State | **Flat files (source of truth) + SQLite index** |
| LLM integration | **Direct HTTP API calls** (no framework dependency) |
| Packaging | **Single binary + optional AppImage** |
| Ollama compatibility | **Tool-use models only (v1)** |
| Project storage | **Default location + import registry** |
| Git integration | **Auto-commit + branch safeguard** (graceful if git missing) |
| Multi-user | **Single-user, portable architecture** |
| Framework customization | **Roles/templates/guardrails: yes. Providers/workflow: v2.** |
| Workflow v2 prep | **Data-driven workflow struct (ADR-009)** |

## Review History

| Date | Review | Findings | Resolution |
|---|---|---|---|
| 2026-04-09 | Initial compilation | 10 documents, 9 open questions | All resolved |
| 2026-04-09 | Architecture pivot | Tauri+TS → Go+Browser | All documents updated |
| 2026-04-09 | Adversarial review | 3 critical, 6 high, 9 medium findings | All addressed — see below |

### Adversarial Review Resolutions

**Critical:**
- C1: Stale Node.js/Tauri content in docs 06, 08 → **Fully rewritten for Go**
- C2: Browser-to-keyring gap → **Resolved: Go backend proxies keyring access, S1 localhost auth designed**
- C3: S8 "binds no ports" false → **Rewritten: localhost binding documented, LAN opt-in, session token auth**

**High:**
- H1: No process lifecycle → **Added: PID file, signal handling, multi-instance prevention, daemon guidance**
- H2: SSE 6-connection limit → **Added R10: BroadcastChannel for multi-tab, HTTP/2 deferred to v1.1**
- H3: modernc.org/sqlite risk → **Added R11: benchmark in Phase 1.8, CGo fallback plan, WAL mode, write batching**
- H4: "10 min" goal unrealistic → **Clarified: <10 min for users with API keys or Ollama already ready**
- H5: Keyboard conflicts → **Fixed: browser-compatible shortcuts (Ctrl+Shift+*), single-key when focused**
- H6: First-run undefined → **Added: terminal banner design, xdg-open failure handling, --no-browser flag**

**Medium:**
- M1: Q3 contradicts ADR-010 → **Fixed: Q3 updated to reference HTTP+SSE**
- M3: fsnotify limits → **Added R13: periodic re-index fallback, inotify warning**
- M4: Git not validated → **Added: startup check, graceful disable if missing**
- M5: Ollama tool-use fragile → **Added: test call during model assignment**
- M8: Tests deferred to Phase 3 → **Fixed: tests built per-phase, Phase 3 fills gaps only**
- M9: No CORS/CSP → **Added: S5 (CSP) and S6 (CORS) controls, XSS prevention in agent output rendering**

## Status

- [x] All documents compiled
- [x] All open questions resolved
- [x] 12 architecture decisions recorded
- [x] Adversarial review completed — all findings addressed
- [x] Feasibility confirmed — 10-15 weeks to first release
- [ ] GitHub repo created
- [ ] Phase 1 implementation started

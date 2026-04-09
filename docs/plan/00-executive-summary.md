# Executive Summary

## Product

A Linux desktop application that enables autonomous, role-based software delivery. The user provides a project goal; the application decomposes it into bounded tasks and executes them through specialized AI agents (Mission Control, Architect, UX, Builder, Reviewer/QA) following a structured delivery lifecycle with enforced guardrails.

## Target Audience

AI enthusiasts and developers on Linux who have access to:
- Local LLMs via Ollama (privacy-first, zero cost per token)
- Cloud models via OpenAI Codex or Anthropic Claude (higher capability, pay-per-token)
- Or a hybrid of both

## Differentiator

Existing tools (CrewAI, AutoGen, Aider, Devin) let agents run without structure. This application enforces a **software delivery lifecycle** — roles have explicit boundaries, tasks pass through shaping and review gates, state is durable and auditable, and guardrails are enforced in code, not by prompt compliance.

## Origin

The delivery framework (roles, workflows, templates, policies) and the runtime decision engine already exist and are validated. The application wraps them in a distributable desktop experience with direct LLM integration, replacing the current dependency on OpenClaw.

## Key Outcomes

1. Single-file AppImage install on any Linux distribution
2. First project running autonomously within 10 minutes of install
3. Full token and cost visibility per project, role, and task
4. 10-15x token efficiency improvement over the current OpenClaw-based execution
5. Extensible via custom roles, providers, and workflows

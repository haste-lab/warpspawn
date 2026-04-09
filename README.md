# Warpspawn

Autonomous agentic software delivery framework for Linux — spawn AI agents that build your project through structured roles, reviews, and guardrails.

## How it works

1. **Describe** your project in plain text
2. **Mission Control** decomposes it into bounded tasks
3. **Builder** agents implement each task autonomously
4. **Reviewer** agents validate the work against acceptance criteria
5. **Guardrails** enforce shell restrictions, file boundaries, and budget limits

Each role has explicit boundaries. Tasks pass through review gates. State is durable and auditable.

## Quick Start

```bash
# Download and install
chmod +x warpspawn
./warpspawn install

# Run
warpspawn
```

The browser opens automatically. The setup wizard detects your LLM providers.

## Requirements

- **Linux** (any distro)
- **LLM provider** — one of:
  - [Ollama](https://ollama.com/) (local, free) — recommended: `qwen3:8b` or `qwen2.5-coder:7b`
  - OpenAI API key
  - Anthropic API key
- **8GB+ VRAM** recommended for local models

## Architecture

- **Backend:** Go (single static binary, ~16MB)
- **Frontend:** Svelte 5 (embedded in the binary, served via localhost HTTP)
- **LLM Providers:** Ollama (local), OpenAI, Anthropic — direct API calls, no framework overhead
- **State:** Flat files (git-friendly, human-readable) + SQLite index
- **Security:** Session auth, shell command allowlist, path containment, role boundary enforcement

## Commands

```bash
warpspawn              # Start the UI server (default)
warpspawn run <path>   # Run one orchestration cycle on a project
warpspawn install      # Install to ~/.local/bin
warpspawn uninstall    # Remove binary and optionally data
warpspawn help         # Show all options
```

## License

Apache 2.0 — see [LICENSE](LICENSE).

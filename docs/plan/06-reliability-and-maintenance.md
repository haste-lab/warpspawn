# Reliability, Maintenance, and Updates

## Reliability

### Process Lifecycle

**Startup:**
```
$ ./warpspawn
Warpspawn v1.0.0
Server: http://localhost:9320
Opening browser...

Press Ctrl+C to stop. Use --no-browser for headless mode.
Use --port=NNNN to set a custom port.
```

- Port defaults to 9320, configurable via `--port` flag or `WARPSPAWN_PORT` env var.
- If port is taken, auto-increment until a free port is found.
- PID file written to `~/.local/share/warpspawn/warpspawn.pid`. If PID file exists and process is alive, print `Already running (PID XXXX) at http://localhost:YYYY` and exit.
- `xdg-open` opens the browser. If it fails (headless, SSH, no browser), print the URL and continue. The `--no-browser` flag skips this step entirely.

**Shutdown signals:**
- `Ctrl+C` (SIGINT): graceful shutdown. Wait up to 30s for running agent to complete. If agent doesn't finish, abort it and write recovery state.
- `SIGHUP` (terminal close): same as SIGINT — graceful shutdown.
- `SIGTERM`: same as SIGINT.
- Cleanup: remove PID file, close SQLite connection, flush logs.

**Background execution:**
- Users who want background mode: `nohup ./warpspawn --no-browser &` or use a systemd user service.
- Ship an example `warpspawn.service` file for systemd user services.

**Multi-instance prevention:**
- PID file + port binding. Two instances cannot bind the same port. PID file provides a clear error message.

### Error Recovery Strategy

| Failure mode | Detection | Recovery |
|---|---|---|
| LLM returns malformed tool call | JSON parse failure in executor | Retry once with error context. If repeated, abort and escalate. |
| LLM infinite loop (keeps calling tools) | Tool call counter exceeds 30 | Abort, mark task as `blocked`, write failure report. |
| LLM timeout (no response) | Per-call timeout (60s) | Retry once. If repeated, abort and try different model tier. |
| Agent timeout (overall run too long) | 240s wallclock via `context.WithTimeout` | Cancel context, mark task for recovery on next cycle. |
| Provider unavailable | Connection error / HTTP 5xx | Mark provider as temporarily unavailable. Try next configured provider if available. |
| Rate limit hit | HTTP 429 | Exponential backoff (1s, 2s, 4s, max 30s). After 3 retries, abort run. |
| Budget exhausted mid-run | Token check before each LLM call | Graceful abort: save partial progress, mark task `blocked` with "budget exhausted" reason. |
| Disk full | Write fails with ENOSPC | Abort, log error, surface in UI with clear message. |
| SQLite corruption (native data) | Query error on startup or during operation | Attempt repair via `PRAGMA integrity_check`. If unrecoverable, rebuild derived data from files. Native-only data (runs, token_usage) logged to fallback JSON for manual recovery. |
| File watcher misses changes | Periodic re-index every 60s as safety net | Full re-index on startup. Periodic light re-index catches external edits. |
| Machine sleep during agent run | LLM call timeout fires after wake | Same as agent timeout — mark for recovery. |
| Terminal closed during agent run | SIGHUP triggers graceful shutdown | Same as Ctrl+C — wait for agent, abort if needed. |
| Browser tab closed | Backend keeps running in terminal | No impact on execution. User reopens browser to same URL. |
| Git not installed | Startup check | Warning in UI: "Git not found. Auto-commit disabled. Install git for rollback support." Proceed without git. |

### Atomic Operations

All file writes follow the safe pattern:
```go
tmpPath := targetPath + ".tmp." + strconv.Itoa(os.Getpid())
os.WriteFile(tmpPath, content, 0644)
os.Rename(tmpPath, targetPath) // atomic on Linux ext4/btrfs/xfs
```

### SQLite Backup

- On startup: copy SQLite DB to `warpspawn.db.bak` (lightweight, takes <1s for typical DB sizes).
- Native-only data (runs, token_usage, cost_entries) that cannot be rebuilt from flat files is backed up.
- If corruption is detected, restore from backup and log what was lost.

### Idempotency

Every runtime cycle is safely re-runnable:
- Decision engine reads current state, produces the same decision for the same state.
- State mutations are conditional (check current status before updating).
- Escalation deduplication prevents repeated notifications.

---

## Maintenance

### Code Organization

- `internal/core/` — pure logic, no I/O side effects. Testable in isolation.
- `internal/provider/` — one file per provider, conforming to interface.
- `internal/agent/` — tool loop. Isolated from provider specifics.
- `internal/server/` — HTTP handlers, SSE. No business logic.
- `internal/guard/` — budget, validation, hooks.
- `internal/db/` — SQLite queries, sync, migrations.
- `frontend/` — Svelte components, no business logic. Calls backend via HTTP.
- `framework/` — delivery methodology as YAML/markdown data files.

### Dependency Management

**Principle:** minimal dependencies.

**Go backend dependencies (expected):**
| Package | Purpose | Risk mitigation |
|---|---|---|
| `modernc.org/sqlite` | Pure Go SQLite, no CGo | Backup + rebuild strategy. Benchmark before committing. If too slow, switch to `mattn/go-sqlite3` (CGo, faster but requires C compiler). |
| `github.com/fsnotify/fsnotify` | File watching | Fallback: periodic re-index every 60s. Handle inotify watch limit with error message. |
| Standard library | HTTP, JSON, YAML, exec, embed, testing | No risk |

**Frontend dependencies:**
| Package | Purpose |
|---|---|
| `svelte` | UI framework |
| `vite` | Build tool |

**No other runtime dependencies.** LLM provider APIs are called via Go's `net/http` (no SDK dependencies).

**Versioning:** `go.sum` committed. Dependabot for security updates. Pin exact versions.

### Logging

Structured logging via Go's `slog` (standard library, no external dependency):
- **Levels:** error, warn, info, debug
- **Default:** info in production, debug with `--debug` flag
- **Format:** JSON lines to stdout (user can redirect to file)
- **Log rotation:** not built-in. Users who need rotation: `./warpspawn 2>&1 | rotatelogs` or systemd journal.
- **Sensitive data:** API keys, tokens, and file content are NEVER logged. Log tool call names, file paths, and token counts.

### Configuration Migration

Settings stored at `~/.config/warpspawn/config.yaml` (YAML, consistent with framework files):
```yaml
config_version: 1
providers:
  ollama:
    url: "http://localhost:11434"
  openai:
    # key stored in OS keyring, not here
    key_ref: "keyring:warpspawn/openai"
budget:
  daily_limit_usd: 10.00
```

On startup, if `config_version` < current, run migration functions sequentially. Old config backed up as `config.v1.backup.yaml`.

### Database Migration

SQLite schema versioned in `schema_version` table. On startup:
```sql
SELECT version FROM schema_version;
-- If version < current, apply migrations in order
```

Each migration runs in a transaction — if it fails, database remains at the previous version.

---

## Updates

### Update Mechanism

1. On startup (configurable: startup / daily / manual), check GitHub Releases API for new version.
2. Compare semver. If update available, show notification in UI with changelog summary.
3. User clicks "Update" → download new binary to temp path.
4. **Verify SHA256 checksum** against the value published in the GitHub Release.
5. Replace current binary via atomic rename.
6. Prompt restart.

**Failure handling:**
- Download interrupted: temp file deleted, no change to current binary.
- Checksum mismatch: abort update, warn user.
- Cannot write to binary path (read-only, wrong permissions): warn user, provide manual download link.
- User declines restart: runs old version until next startup.

### Backward Compatibility

- **Project format:** flat files are stable. New features add optional fields; never remove or rename. A project created in v1.0 must load in v2.0.
- **Config format:** versioned with migrations.
- **SQLite schema:** versioned with migrations.
- **Framework files (roles, templates, workflows):** shipped as defaults. User overrides in config directory are never overwritten by updates.

### Versioning

Semantic versioning: `MAJOR.MINOR.PATCH`
- **MAJOR:** breaking changes to project format or config (requires migration)
- **MINOR:** new features, new providers, UI improvements
- **PATCH:** bug fixes, security updates

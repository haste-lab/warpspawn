# UX Design

## Design Principles

1. **Progressive disclosure** — Simple view by default. Complexity available on demand.
2. **Status at a glance** — Every screen answers "what's happening right now?" without interaction.
3. **Zero-config start** — First project runs with defaults. Customization is optional.
4. **Token awareness** — Cost is always visible. Never surprise the user with a bill.
5. **Dark-first** — AI developers work in dark mode. Light mode as option.

---

## Views

### V1: Setup Wizard (first run only)

```
┌──────────────────────────────────────────┐
│  Welcome to [App Name]                    │
│                                           │
│  Step 1 of 3: LLM Providers              │
│                                           │
│  ┌─ Local (Ollama) ─────────────────────┐│
│  │ ✅ Detected on localhost:11434        ││
│  │    Models: qwen3:8b, qwen2.5-coder   ││
│  └──────────────────────────────────────┘│
│                                           │
│  ┌─ Cloud (optional) ───────────────────┐│
│  │ OpenAI API key:  [••••••••] [Test]   ││
│  │ Anthropic key:   [        ] [Test]   ││
│  └──────────────────────────────────────┘│
│                                           │
│              [Back]  [Next →]             │
└──────────────────────────────────────────┘
```

Steps:
1. **Providers** — auto-detect Ollama, enter cloud API keys with test button
2. **Model assignment** — pre-filled defaults per role, editable dropdowns
3. **Budget** — daily/weekly token or dollar limit, pre-filled with sensible default

### V2: Dashboard (main view)

```
┌─ [App Name] ────────────────────────────────────────────┐
│                                                          │
│  Projects                            Budget: $2.40/$10  │
│  ┌──────────────────────────────────────────────────┐    │
│  │ ● PC Health Dashboard        done    3 tasks     │    │
│  │ ◐ Academy Web App            build   1/5 tasks   │    │
│  │ ○ Chess Engine Refactor      intake  0 tasks     │    │
│  └──────────────────────────────────────────────────┘    │
│                                                          │
│  [+ New Project]                                         │
│                                                          │
│  ┌─ Active Agent ───────────────────────────────────┐    │
│  │  Builder (gpt-5.4-mini) → Academy/TASK-003       │    │
│  │  ▌Writing app/src/routes/quiz.js...              │    │
│  │  Tokens: 1,240 in / 890 out  ($0.03)            │    │
│  │                                    [Abort]       │    │
│  └──────────────────────────────────────────────────┘    │
│                                                          │
│  ┌─ Attention (1) ──────────────────────────────────┐    │
│  │  ⚠ TASK-005 blocked: missing API credentials     │    │
│  │                          [View] [Dismiss]        │    │
│  └──────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────┘
```

Key elements:
- **Project list** with status dot (○ intake, ◐ in progress, ● done, ⚠ needs attention)
- **Budget indicator** always visible in header
- **Active agent panel** with streaming output, model name, token counter, cost, abort button
- **Attention inbox** for escalations and blockers

### V3: Project Detail

```
┌─ Academy Web App ──────────────────────────────────────┐
│                                                         │
│  Pipeline                                               │
│  [intake] → [shaping] → [▶ build] → [review] → [done]  │
│                                                         │
│  Tasks                                                  │
│  ┌────────────────────────────────────────────────────┐ │
│  │ ✅ TASK-001  Setup project scaffold        done    │ │
│  │ ✅ TASK-002  Auth module                   done    │ │
│  │ ▶  TASK-003  Quiz component               build   │ │
│  │ ○  TASK-004  Results dashboard             ready   │ │
│  │ ○  TASK-005  Deploy instructions           ready   │ │
│  └────────────────────────────────────────────────────┘ │
│                                                         │
│  Recent Activity                                        │
│  ┌────────────────────────────────────────────────────┐ │
│  │ 14:32  Builder completed TASK-002 (1,840 tokens)   │ │
│  │ 14:33  Reviewer approved TASK-002                  │ │
│  │ 14:34  Mission Control closed TASK-002             │ │
│  │ 14:35  Builder started TASK-003                    │ │
│  └────────────────────────────────────────────────────┘ │
│                                                         │
│  Cost: $1.20 total  │  Runs: 8  │  Model: gpt-5.4-mini │
│                                                         │
│  [▶ Run Next Task]  [⏸ Pause]  [⚙ Project Settings]    │
└─────────────────────────────────────────────────────────┘
```

### V4: New Project Flow

```
┌─ New Project ────────────────────────────────────────┐
│                                                       │
│  Describe your project:                               │
│  ┌──────────────────────────────────────────────────┐ │
│  │ Build a local weather dashboard that fetches     │ │
│  │ weather data from Open-Meteo API and displays    │ │
│  │ current conditions and 5-day forecast. Use       │ │
│  │ vanilla HTML/CSS/JS. No framework.               │ │
│  └──────────────────────────────────────────────────┘ │
│                                                       │
│  Stack preference: [Auto-detect ▾]                    │
│  Model tier: [Auto ▾]                                 │
│  Budget limit: [$5.00]                                │
│                                                       │
│  [Cancel]                       [Create & Start →]    │
└───────────────────────────────────────────────────────┘
```

On "Create & Start":
1. Scaffold project directory
2. Run Mission Control to decompose brief into backlog + tasks
3. Open project detail view
4. Begin autonomous execution

### V5: Budget / Cost View

```
┌─ Token Usage ────────────────────────────────────────┐
│                                                       │
│  Today: 12,400 tokens ($0.84)   │ This week: $4.20   │
│                                                       │
│  ▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░  42% of daily budget          │
│                                                       │
│  By Project          By Role           By Model       │
│  Academy    $2.80    Builder   $3.10   gpt-5.4  $3.60 │
│  Health     $1.40    Reviewer  $0.70   mini     $0.60 │
│                      MC        $0.40                   │
│                                                       │
│  Recent Runs                                          │
│  ┌──────────────────────────────────────────────────┐ │
│  │ 14:35  Builder  TASK-003  1,240 in / 890 out $03 │ │
│  │ 14:33  Reviewer TASK-002    680 in / 420 out $01 │ │
│  │ 14:30  Builder  TASK-002  2,100 in / 1840 out$05 │ │
│  └──────────────────────────────────────────────────┘ │
│                                                       │
│  Daily limit: [$10.00]  Weekly limit: [$50.00]        │
└───────────────────────────────────────────────────────┘
```

### V6: Settings

```
┌─ Settings ────────────────────────────────────────────┐
│                                                        │
│  Providers                                             │
│  ┌─ Ollama ─────────────────────────────────────────┐  │
│  │ URL: [localhost:11434]  Status: ✅ Connected      │  │
│  │ Models: qwen3:8b, qwen2.5-coder:7b              │  │
│  └──────────────────────────────────────────────────┘  │
│  ┌─ OpenAI ─────────────────────────────────────────┐  │
│  │ API Key: [••••••••]  Status: ✅ Valid             │  │
│  │ [Test Connection]  [Remove Key]                  │  │
│  └──────────────────────────────────────────────────┘  │
│                                                        │
│  Role → Model Assignment                               │
│  Mission Control:  [openai/gpt-5.4       ▾]           │
│  Builder:          [openai/gpt-5.4       ▾]  (auto)   │
│  Builder Light:    [openai/gpt-5.4-mini  ▾]  (auto)   │
│  Reviewer/QA:      [openai/gpt-5.4-mini  ▾]           │
│  Architect:        [openai/gpt-5.4-mini  ▾]           │
│  UX:               [openai/gpt-5.4-mini  ▾]           │
│                                                        │
│  Execution                                             │
│  Agent timeout:  [240] seconds                         │
│  Max tool calls: [30] per run                          │
│  Shell mode:     [Restricted ▾]                        │
│                                                        │
│  Updates                                               │
│  Check for updates: [On startup ▾]                     │
│  Channel: [Stable ▾]                                   │
└────────────────────────────────────────────────────────┘
```

---

## Interaction Patterns

### Agent Streaming
When an agent is running, the Active Agent panel shows a live feed:
- LLM text output streams character-by-character
- Tool calls appear as collapsed cards: `📄 read_file: app/src/server.js` (expandable)
- Write operations show a diff preview
- Shell commands show the command and its output

### Escalation Flow
When the runtime escalates (budget, rework limit, ambiguity):
1. Browser notification appears (if permission granted) + in-app alert
2. Attention inbox item added
3. Clicking it shows: what happened, why, suggested actions
4. User can: approve continuation, reject and archive, edit the task, or override

### Keyboard Navigation (Browser-Compatible)
Browser tabs intercept `Ctrl+N`, `Ctrl+R`, etc. Use non-conflicting shortcuts:
- `Ctrl+Shift+N` — New project
- `Ctrl+Shift+R` — Run next task on selected project
- `Ctrl+Shift+.` — Abort running agent
- `Tab` — Navigate between panels (when Warpspawn element has focus)
- `Enter` — Expand selected item
- `Escape` — Close modal / back to dashboard
- Single-key shortcuts (`n`, `r`, `.`) when the main content area has focus (not when typing in an input)

### Terminal Experience

```
$ ./warpspawn
 __      __                                         
 \ \    / /_ _ _ _ _ __ ___ _ __  __ ___ __ ___ _   
  \ \/\/ / _` | '_| '_ (_-< '_ \/ _` \ V  V / ' \  
   \_/\_/\__,_|_| | .__/__/ .__/\__,_|\_/\_/|_||_| 
                  |_|     |_|   v1.0.0

  Server:  http://localhost:9320?token=a3f8...c2d1
  Browser: opening...

  Press Ctrl+C to stop.
  Use --no-browser for headless mode.
  Use --port=NNNN for a custom port.
  Use --host=0.0.0.0 to allow LAN access (requires token).
```

If `xdg-open` fails:
```
  Browser: could not open automatically.
           Open this URL manually: http://localhost:9320?token=a3f8...c2d1
```

---

## Visual Design

- **Theme:** Dark by default (match IDE aesthetic). Light mode toggle.
- **Colors:** Muted palette with accent colors for status:
  - Green: done, approved, healthy
  - Blue: in progress, active
  - Amber: needs attention, warning
  - Red: blocked, failed, over budget
- **Non-color status indicators:** Every status color is accompanied by an icon or text label (e.g., checkmark for done, spinner for active, warning triangle for blocked). Color is never the sole indicator — accessible for color-blind users.
- **Typography:** System monospace for code/logs, system sans-serif for UI
- **Animation:** Minimal — loading spinners, progress bars, streaming text. No decorative animation.
- **Density:** Compact by default (developer audience expects information density)
- **Contrast:** Minimum 4.5:1 contrast ratio for all text (WCAG AA). Verified via axe-core in UI tests.

# CLI UX Deep Dive: From Utilitarian Output to Design-Forward Interfaces

*Context: The glade codebase at `/Users/matt/Dev/glade` has a single progress reporter (`cliTestProgressReporter` in `internal/gladecli/test_command.go:316`), a 2-function `internal/cliui/` package, and otherwise purely batched output. This analysis bridges from that baseline to the state of the art.*

---

## 1. The Two Paradigms

### Batch (Traditional)
All work completes silently, then a summary is dumped. Glade's `check`, `exec`, `schema`, and `package` commands follow this pattern — `fmt.Print` at the end only.

**Problem**: During a 90-second `glade test` with 2000 cases, the user sees nothing until completion. Uncertainty breeds distrust. Was the process killed? Did it hang? The silence is indistinguishable from a dead process.

### Streaming / Reactive (Modern)
The process emits structured events as work happens. The terminal renders these as a live, evolving surface. This is not decorative — it reduces *perceived latency* (Maister's Law: "occupied time feels shorter than unoccupied time") and provides *situational awareness* (early failure visibility, decision to abort).

---

## 2. Information Architecture for CLIs

CLI output fits a small set of *information layers* that should be rendered at different priority levels:

| Layer | Purpose | Lifetime | Example |
|---|---|---|---|
| **Status bar** | Persistent situational summary | Full command duration | `Pass 42/120 · Fail 2 · 1m05s elapsed` |
| **Activity feed** | Recent notable events | Scrollback, last N lines | `FAIL AccountTest.insertWithBulk · 342ms` |
| **Detail pane** | Context for a selected item | On-demand, transient | Full stack trace for a failure |
| **Structural region** | Logical grouping | Section lifetime | `=== Setup: AccountDomain ===` |

Modern tools render these as *overlapping TUI regions* using alternate screen buffers, cursor positioning, and repaint passes. Tools that target pipes/files degrade each layer to a line prefix or NDJSON event type.

Glade's `--watch` NDJSON output (`watch.started`, `watch.run_finished`) is an embryonic version of this layering — each event type maps to a conceptual layer, but the rendering is left to the consumer.

---

## 3. State Communication Spectrum

### 3.1 What to Communicate

Non-obvious states that tools should surface:

| State | Why it matters | How to surface |
|---|---|---|
| **Idle / waiting for I/O** | User should know the tool isn't hung | Spinner with context label ("Connecting to org…") |
| **Bounded progress (known total)** | Enables time estimation | Progress bar + ETA + rate |
| **Unbounded progress (streaming)** | Shows work is ongoing | Counter + throughput ("1,283 records/s") |
| **Degraded mode** | Explains unexpected slowness | Icon + reason ("⚠ Running without compilation cache") |
| **Failure early-warning** | Lets user abort early | Inline red marker on the bar, then detail below |
| **Completion** | Clear success/failure signal | Summary bar + exit code + actionable next step |

### 3.2 Channels

```
 stdout  →  final output, structured results, JSON
 stderr  →  progress, diagnostics, logs, interactive UI
```

This is a critical convention. Glade currently writes test progress to stderr (correct), but the `cliui/cliui.go` package doesn't enforce or document this split. A dedicated renderer should have `stdout` and `stderr` as explicit, named parameters.

---

## 4. Progress Visualization Methods (Ranked by Capability)

### Level 1 — Periodic text lines (Glade's current approach without `--progress`)
```
PASS AccountTest.insert 42ms
PASS AccountTest.update 38ms
FAIL ContactTest.validate 12ms
```
**Pros**: Simple, grep-friendly, works in any terminal. **Cons**: Scrolls uncontrollably, no aggregate status, hard to scan for the current state.

### Level 2 — In-place line overwrite (Glade's current `--progress` terminal mode)
```
Progress: 42/120 elapsed=1m05s eta=2m30s pass=40 fail=1 error=1 running=AccountTest.insertAccount
```
`\r\x1b[K` — carriage return + clear-to-end-of-line. This is the UNIX archetype (`curl`, `wget`, `ffmpeg`).

**Pros**: Single line, shows ETA, count, current unit. **Cons**: No multi-line layout, no spinner, no failure details inline, flickers on some terminals.

### Level 3 — Multi-line TUI region (Docker BuildKit, Homebrew, Bun)
```
[====================>               ] 58% · 695/1200 tests
  PASS  AccountTest.insert          342ms
  FAIL  ContactTest.validate         FAILED after 12ms
        Expected: "Active", got: "Inactive"
  ▶ Running  PaymentTest.chargeCard
```

**Pros**: Rich failures inline, visual bar, contextual detail without scrolling. Rendered by write + cursor-up sequences on TTY; degrades to line-mode on pipe.

**Implementation pattern**:
```go
type Region struct {
    height int
    w      io.Writer
}

func (r *Region) Render(lines []string) {
    // Clear previous region: cursor up N lines + clear each
    // Write new lines
    // If non-TTY: write lines with timestamp prefix, no ANSI
}
```

### Level 4 — Full alternate-screen TUI (Top, htop, lazydocker)
Switches to alternate screen buffer. Complete control over every cell. Avoid this for a compiler/test-runner CLI — it blocks scrollback and is hostile to piping. Reserve for interactive tools.

### Level 5 — Animated spinners (modern AI CLIs)

Spinners are the **primary UX differentiator** for AI CLIs. They solve *unbounded wait* — when you don't know total work, a progress bar is meaningless.

**Pattern taxonomy**:

| Type | Use case | Tools that use it |
|---|---|---|
| **Simple spinner** (⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏) | Short operations (<5s) | npm, yarn, pip |
| **Labeled spinner** (⠙ Fetching packages…) | Medium operations, multiple phases | Bun, Cargo, uv |
| **Multi-spinner / task tree** | Parallel operations | Docker Build, pnpm, uv (resolver) |
| **Spinner chain** (⠙ Thinking… → ✓ Analyzed → ⠙ Generating…) | Sequential AI pipeline phases | Claude Code, Kilo, Codex |
| **Typing animation / streaming tokens** | LLM response streaming | ChatGPT CLI, Claude Code, Copilot |

**The AI CLI signature pattern**: A spinner with a changing label that communicates *what the model is doing right now* — not just "loading." This transforms the wait from opaque to transparent:

```
⠋ Thinking about the test failures…
   → Found 3 failing tests in AccountDomain
⠙ Analyzing root cause…
   → Null pointer in AccountTrigger.handleAfterInsert
⠹ Generating fix…
```

This pattern is why AI CLIs feel *conversational and alive* rather than *batch and dead*. The spinner label serves as a narrative device.

---

## 5. Engineering a Progress System

### 5.1 The Event Model

The foundational shift is from `func() -> Result` to `func(onEvent) -> Result`. Glade's `apextest.Options.Progress` callback already follows this pattern — the architecture is correct. The gap is that:

1. Only `test` uses it
2. The callback emits test-specific events, not general progress primitives
3. No renderer abstraction exists between events and the terminal

A general model:

```go
// Progress event (renderer-agnostic)
type Event struct {
    Kind    EventKind   // PhaseStart, PhaseEnd, Tick, Info, Warn, Fail
    Phase   string      // "setup", "compile", "execute"
    Current int
    Total   int         // -1 = unbounded
    Label   string      // human-readable, changes on Tick
    Detail  string      // optional extra line
    Data    any         // structured payload for custom renderers
}

type Renderer interface {
    Render(Event)
    Finish(Result)
}
```

### 5.2 Renderer Decomposition

```go
// TTYRenderer — multi-line region with spinner, bar, and activity feed
type TTYRenderer struct {
    out io.Writer  // stderr for progress
    spinner    *Spinner
    bar        *ProgressBar
    activity   *ActivityFeed  // last N lines
    region     *Region        // manages cursor positioning
    phases     []PhaseSpan    // timing for each phase
}

// LineRenderer — degrades to log lines for pipes/CI
type LineRenderer struct {
    out io.Writer
}

// NDJSONRenderer — structured machine-readable events
type NDJSONRenderer struct {
    out io.Writer
}

// NullRenderer — silent (when stderr is not a TTY and user opted out)
type NullRenderer struct{}
```

### 5.3 Spinner Implementation

```go
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type Spinner struct {
    frames   []string
    interval time.Duration  // typically 80ms
    label    string
    done     bool
    result   string // "✓" or "✗"
}
```

The key implementation detail: the spinner **ticks on a timer goroutine, not on events**. The event system updates the label atomically; the timer goroutine redraws the frame periodically. This decouples event frequency from animation smoothness.

### 5.4 Progress Bar Resolution

For bounded operations, the bar should reflect **not just completion count but wall-clock progress** when individual unit durations vary widely (e.g., one test takes 10s, 99 take 10ms). A simple `done/total` bar would stall at 99% for 10 seconds. Solutions:
- **Time-weighted bar**: estimate remaining time per remaining tests using running average duration
- **Phase-weighted bar**: assign weight to phases (setup 10%, compile 30%, execute 60%)
- **Hybrid**: bar shows phase position weighting, with test count as supplementary text

### 5.5 Graceful Degradation

```go
func NewRenderer(opts RendererOpts) Renderer {
    if opts.JSON {
        return &NDJSONRenderer{out: opts.Stderr}
    }
    if !isTTY(opts.Stderr) {
        return &LineRenderer{out: opts.Stderr}
    }
    return &TTYRenderer{out: opts.Stderr}
}
```

Every renderer satisfies the same interface. Commands don't branch on output mode — they emit events uniformly.

---

## 6. Industry Pattern Catalog

| Tool | Signature UX | Technique | Applicable to glade? |
|---|---|---|---|
| **Bun** | Multi-line TUI with spinner, bar, activity feed during install/test | ANSI cursor control, zone-based rendering | Yes — `glade test` should adopt this |
| **uv (pip)** | Spinner chain ("Resolving…" → "Downloading…" → "Installing…"), clear phase transitions | Phase-labeled spinner with timing per phase | Yes — `glade check` (parse → index → sema) should show phases |
| **Docker BuildKit** | Multi-line build steps with per-step spinners, caching indicators, timing | Per-line ANSI updates, diff-based terminal writes | Partially — `glade test` class-level progress |
| **Homebrew** | Poured/unpoured emoji (🍺/⏳), formula names streamed as installed | Emoji as compact status glyphs, streaming line-by-line | Yes — compact status glyphs in watch mode |
| **Cargo** | Colored compilation lines (green=compiling, red=error, yellow=warning), spinner for "Updating crates.io" | Color-coded line prefixes, single spinner for unbounded phases | Yes — color prefixes in `glade check` output |
| **Claude Code / Kilo** | Spinner chains with narrative labels ("Thinking…" → "Analyzing codebase…" → "Writing…"), streaming token output with cursor positioning | Spinner with dynamic label, multi-phase progression | Partially — spinner for index/sema phases |
| **Pulumi** | Resource tree with per-resource spinners that resolve to ✓/✗, diff preview inline | Tree-structured progress with per-node resolution state | Yes — class/suite tree for `glade test` |
| **Turborepo** | Task graph with per-task status indicators, cached/hit/miss markers | Per-task line with status icon and timing | Yes — test suite with per-class/method status |
| **Vitest** | Interactive test dashboard with filter, file list, and test detail panel | Full TUI with keyboard navigation | Not yet — would be a `glade test --ui` feature |

---

## 7. Concrete Recommendations for glade

### Phase 1 — Foundational (low risk, high impact)

**A. Upgrade `internal/cliui` from 11 lines to a real package:**
- `Spinner` type with configurable frames, label, tick interval
- `ProgressBar` with known-total and unbounded modes
- `Renderer` interface with TTY and line-based implementations
- `PhaseTracker` for timing and displaying named phases
- `ActivityFeed` for the last N log lines below the bar

**B. Add `--progress` to `glade check`:**
- Phases: `loading metadata` → `parsing` → `indexing` → `checking` → `done`
- Each phase gets a spinner with label and elapsed time
- Errors appear inline in the activity feed as they're found

**C. Add a spinner to `glade schema`:**
- Currently synchronous (fast), but the pattern establishes the standard

### Phase 2 — Multi-line TUI for `glade test`

Replace the single-line `\r\x1b[K` reporter with:
- Top bar: `[========>           ] 58% · 695/1200 · 1m05s elapsed · 2m30s remaining`
- Activity feed (last 5 events): failures and slow tests surfaced inline
- Per-suite section headers: `=== AccountDomain (12 tests)` → resolves to `✓ 10 passed, 2 failed`
- Autowatch: detect TTY resize for dynamic width

### Phase 3 — Machine-readable streaming

- Unify `--watch` NDJSON with a general `--progress-json` flag for all commands
- Make all long-running commands emit the same event schema
- Consumers (CI systems, editors, `glade dev`) get structured progress without parsing ANSI

### Phase 4 — Interactive dashboard

- `glade test --ui` launches a full-screen TUI test dashboard (Vitest-like)
- Keyboard navigation: filter by file, jump to failure, re-run single test
- This is a separate rendering backend, not a different event model

---

## 8. Key Architectural Principles

1. **Emit events, not bytes.** Commands should never write ANSI codes directly. They emit structured progress events. Renderers translate events to terminal output.

2. **TTY-detection is a renderer concern, not a command concern.** The command shouldn't branch on `isTTY()` — the renderer does.

3. **Stderr is the UI channel, stdout is the data channel.** This preserves piping: `glade test --json | jq '.summary'` should work while the spinner runs on stderr.

4. **Always degrading is better than sometimes failing.** If the terminal width is unknown, use 80 columns. If the terminal doesn't support cursor movement, use line mode. If stderr is a pipe, downgrade to periodic log lines. Never fail to render.

5. **Progress is opt-out, not opt-in.** The `--progress` flag is backward. Progress should be the default on TTYs. Users should opt *out* with `--no-progress` or `--quiet`. Glade currently requires `--progress` to even see the basic line-overwrite — this should be inverted.

6. **Phase transitions carry meaning.** "Compiling…" → "✓ Compiled (1.2s)" → "⠙ Executing tests…" tells a story. The resolved checkmark (✓) after the spinner provides closure and timing. AI CLIs lean heavily on this — the spinner label is the primary narrative device for communicating what the system is doing.

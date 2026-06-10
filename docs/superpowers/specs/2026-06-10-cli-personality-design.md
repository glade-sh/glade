# CLI Personality Design

**Date:** 2026-06-10  
**Status:** Approved  
**Goal:** Give glade a cohesive, modern CLI personality — hybrid of build-tool result formatting (Cargo/uv) and alive progress narration (AI CLIs) — across progress, results, help, doctor, and errors.

---

## Context

Prior work established a renderer-agnostic progress system in `internal/cliui` (events, TTY/line/NDJSON renderers, activity feed, progress bar) and wired it into `glade test`, `glade check`, and `glade schema load`. See `docs/research/CLI_UX_DESIGN.md` and `docs/superpowers/plans/2026-06-10-cli-ux-progress-system.md`.

The infrastructure works but the surface is still ASCII and utilitarian: `|/-\` spinners, `v`/`x` completion markers, plain `[===>    ]` bars, and unstyled stdout (`PASS`, `project:`, `Result:`). This spec defines the presentation layer that sits on top of the existing event model.

**Constraints for v1:**

- Project is pre-release; stdout/stderr format may change freely (no backward-compat requirement).
- Standard library only — no Bubble Tea, lipgloss, termenv, or other TUI dependencies.
- Progress stays on **stderr**; command results stay on **stdout**.
- Commands emit structured events; ANSI and layout live in `internal/cliui` only.
- JSON, JUnit, and NDJSON machine output paths are unchanged.

**Aesthetic:** Hybrid **C** — Cargo-style scannable results plus AI-style animated progress and phase resolution during long runs.

---

## Approach

**Unified `internal/cliui` theme (recommended architecture).**

One package owns theme tokens, live progress renderers, and static formatters (help, doctor, test summary, diagnostics). Commands and reporters call `cliui` instead of raw `fmt`. Alternative splits (`cliformat` package, semantic writer middleware) add boundary complexity without benefit at current scale.

---

## Visual Language

**Personality:** Confident build tool, not chatbot. Progress narrates phases; results are scannable blocks.

| Element | TTY + color | Plain / pipe |
|--------|-------------|--------------|
| Spinner | Braille `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏` | `…` or omitted |
| Phase done | `✓ Label (1.2s)` green | `✓ Label (1.2s)` |
| Phase fail | `✗ Label` red | `✗ Label` |
| Progress bar | `██████░░░░` cyan fill | `[======    ]` |
| PASS | `✓` green + dim timing | `PASS` + timing |
| FAIL | `✗` red + bold name | `FAIL` + name |
| Warning | `⚠` yellow | `WARN` |
| Error code | `GLADESEMA002` magenta | same, no color |
| Muted meta | dim gray (elapsed, counts) | plain text |

**Degradation rules:**

- `NO_COLOR` set (any value): no ANSI; keep Unicode glyphs where the terminal supports them.
- Non-TTY stdout/stderr: no ANSI; simplified layout (no boxes if width unknown); same information density.
- Piped stderr with explicit `--progress`: line renderer, no spinner animation.

---

## Progress Layer (stderr)

Upgrade the existing TTY renderer. Example target:

```
⠹ Checking Apex semantics · 142/380 types · 12s
  ✓ AccountService.cls
  ✓ ContactTrigger.cls
  ✗ OrderService.cls — unknown type MissingType
```

**Changes:**

1. **Spinner** — Braille frames; tick on existing 150ms goroutine (decoupled from event rate).
2. **Progress bar** — Block characters with percent when `total > 0`; unbounded mode keeps sliding indicator.
3. **Status line** — `·` separators instead of `elapsed=`; dim styling for metadata.
4. **Activity feed** — Color by event kind (pass green, fail red, warn yellow, info default).
5. **Phase resolution** — On `EventPhaseEnd`, show `✓ Phase name (duration)` before the next `EventPhaseStart` (brief narrative chain like uv/Cargo).
6. **Line renderer** (`--progress`) — Same semantics, no ANSI, structured prefixes for CI logs.

Existing event kinds and renderer selection (`ProgressAuto`, `--no-progress`, `--progress-json`) are unchanged.

---

## Command Results (stdout)

### `glade test`

```
╭─ Tests ─────────────────────────────────────╮
│  12 selected · 11 passed · 1 failed · 1.2s  │
╰─────────────────────────────────────────────╯

  ✓  AccountTest.insertBulk          342ms
  ✓  AccountTest.update              38ms
  ✗  ContactTest.validate             12ms

  ContactTest.validate
  Assertion failed: Expected "Active", got "Inactive"
  at ContactTest.validate:42

╭─ Result ────────────────────────────────────╮
│  11 passed · 1 failed · 12 total · 1.2s   │
╰─────────────────────────────────────────────╯
```

Replaces flat `PASS`/`FAIL` lines and comma-separated `Result:` line. `testreport.WriteJSON` and JUnit output unchanged.

### `glade check`

```
╭─ Check ─────────────────────────────────────╮
│  project: ./force-app                       │
│  142 types · 3 triggers · 28 objects      │
│  2 diagnostics (1 error, 1 warning)       │
╰─────────────────────────────────────────────╯

  ✗  AccountService.cls:24:8
     error[GLADESEMA002]: Unknown type MissingType

  ⚠  ContactTrigger.cls:10:1
     warning[GLADESEMA101]: Unused variable x
```

Replaces `project:` / `types:` / `diagnostics:` key-value dump.

### `glade doctor`

```
╭─ glade doctor ──────────────────────────────╮
│  glade 0.0.0-dev                            │
╰─────────────────────────────────────────────╯

  ✓  go              go1.26.3
  ✓  parser          tree-sitter
  ✓  config          ./glade.yml
  ✓  project.root    ./force-app
  ─────────────────────────────────────────
  ✓  All checks passed
```

Parser or config failures use `✗` with inline explanation. Exit code semantics unchanged.

### `glade` help

```
  glade — local Apex runtime

  Usage
    glade <command> [flags]

  Commands
    test       Discover and run Apex tests
    check      Semantic checks over a project
    …
```

Subtle color on TTY: command names emphasized, descriptions dim. No box around help (too noisy for long command lists). One-line tagline under title.

### CLI errors

```
  ✗  glade: unknown command "foo"

  Run glade help for usage.
```

Replace bare `glade: %v` where glade controls the message. Structured diagnostics (`GLADECLI001`) keep using the diagnostic formatter.

---

## Architecture

### New and modified files under `internal/cliui`

| File | Responsibility |
|------|----------------|
| `theme.go` | `Theme` struct, ANSI helpers, glyph selection, `NO_COLOR`/TTY detection |
| `format.go` | Boxes, rows, separators, width-aware truncation, duration display |
| `help.go` | `WriteHelp`, `WriteTestHelp`, topic routing |
| `doctor.go` | `WriteDoctor` status grid |
| `diagnostic.go` | `WriteDiagnostics` for CLI presentation |
| `test_result.go` | `WriteTestRun` — console test report |
| `tty_renderer.go` | Polish pass (spinner, bar, feed colors, phase resolution) |
| `line_renderer.go` | Align with new semantics, no ANSI |
| `progressbar.go` | Block bar rendering |

### Wiring

| Consumer | Change |
|----------|--------|
| `internal/gladecli/cli.go` | `printHelp` → `cliui.WriteHelp`; `runDoctor` → `cliui.WriteDoctor`; check summary → `cliui.WriteCheckResult`; errors → `cliui.WriteError` |
| `internal/gladecli/test_command.go` | `testreport.WriteConsole` → `cliui.WriteTestRun` |
| `internal/diagnostic` | `Report.WriteText` retained for non-CLI callers; CLI uses `cliui.WriteDiagnostics` |
| `internal/testreport` | `WriteConsole` may delegate to `cliui` or remain as thin wrapper for tests |

### Invariants

1. Commands never write ANSI directly.
2. TTY detection is a renderer/theme concern.
3. Machine-readable output (`--json`, `--junit`, `--progress-json`, watch NDJSON) is never styled.
4. Box drawing uses light Unicode (`╭─╮│╰─╯`); ASCII fallback `+-|` when `Theme.Plain` or narrow width.

---

## Testing

- Unit tests for `Theme` with `NO_COLOR=1` and simulated TTY via `io.Writer` type assertions.
- Golden string tests for formatted blocks in **plain mode** (stable, no ANSI in goldens).
- Separate tests assert ANSI substrings only when color explicitly enabled in test opts.
- Update `internal/gladecli/cli_test.go` contract tests for new stdout/stderr shapes.
- Run: `go test ./internal/cliui ./internal/gladecli ./internal/testreport`

---

## v1 Scope

| In scope | Out of scope (see Phase 2+) |
|----------|----------------------------|
| Theme + format primitives | Full-screen TUI dashboard |
| TTY + line progress polish | External TUI libraries |
| Test / check / doctor / help / CLI errors | Every command restyled |
| `NO_COLOR` and non-TTY degradation | Animations beyond spinner |

---

## Future Phases

### Phase 2 — Remaining commands

**Goal:** Apply the same theme and formatters to all user-facing commands so glade feels consistent end-to-end.

**Commands to restyle:**

| Command | Presentation needs |
|---------|-------------------|
| `exec` | Result block for return value / debug output; spinner during VM execution |
| `parse` | Per-file pass/fail lines; diagnostic blocks matching `check` |
| `inspect` | Summary box (symbol counts, risk flags); optional table for hotspots |
| `schema` | Load summary box (objects, fields loaded); progress already partially wired |
| `package` | Phase chain (validate → compile → bundle); artifact path highlight |
| `profile` | Flame-graph path or summary stats box |
| `debug` | Subcommand headers; log parse progress; repro snippet in code block style |
| `report` | List runs as table; `show` uses shared result formatters |
| `db` | Seed/reset confirmation prompts styled; row counts in summary box |
| `server` / `playground` | Startup banner with URL highlight, port, ready checkmark |
| `dev` | Orchestrates check + test; reuse their formatters, add cockpit header |
| `editor` | Install status per editor (✓/✗ rows like doctor) |

**Architecture:** No new packages. Extend `cliui` with small formatter functions per domain (`WriteExecResult`, `WriteServerBanner`, …). Long-running commands get progress events using the v1 event model.

**Prerequisite:** Phase 1 theme and `format.go` primitives stable.

---

### Phase 3 — `glade test --ui` interactive dashboard

**Goal:** Vitest-like full-screen test dashboard for local development: filter by file, jump to failures, re-run single test.

**UX target:**

```
┌ glade test ─────────────────────────────────────────┐
│ ▶ Running · 42/120 · 2 failed · filter: Account*    │
├─────────────────────────────────────────────────────┤
│ ✓ AccountTest.insertBulk                    342ms   │
│ ✗ ContactTest.validate                       12ms ◀ │
│   Expected "Active", got "Inactive"                 │
│ ○ PaymentTest.chargeCard                    (queued)  │
├─────────────────────────────────────────────────────┤
│ [f] filter  [r] rerun  [q] quit  [↑↓] navigate      │
└─────────────────────────────────────────────────────┘
```

**Architecture:**

- **Same event model** as stderr progress — `--ui` is an alternate `Renderer` backend, not a second instrumentation path.
- New `internal/cliui/dashboard_renderer.go` using alternate screen buffer (`\x1b[?1049h` / restore on exit).
- Keyboard input via raw terminal mode (`golang.org/x/term` is acceptable here if stdlib is insufficient; evaluate in plan).
- Does not replace default TTY progress; opt-in flag only.

**Constraints:**

- Blocks scrollback while active — document clearly in help.
- Disabled when stdout is not a TTY or when `--json` / `--watch` is set.
- Exit restores terminal state even on SIGINT.

**Prerequisite:** Phase 1 progress + test result formatters; stable test progress events including per-method timing.

---

### Phase 4 — Optional TUI library evaluation

**Goal:** Decide whether to adopt a TUI framework for Phase 3+ or stay on hand-rolled ANSI.

**Candidates:**

| Library | Pros | Cons |
|---------|------|------|
| **Bubble Tea** | Rich components, community patterns | Dependency, learning curve, model/update overhead for a compiler CLI |
| **lipgloss** | Layout and styles without full TUI | Still a dependency; layout only |
| **Hand-rolled (current)** | Zero deps, full control, matches AGENTS.md | More code for dashboard, resize, input |

**Decision criteria:**

1. Can dashboard be built in &lt;500 lines hand-rolled? If yes, stay stdlib.
2. Does `x/term` for raw mode count as acceptable minimal dependency?
3. Binary size and `go mod` policy — glade has avoided UI deps so far.

**Default recommendation:** Hand-rolled for Phase 3 unless dashboard complexity exceeds one focused file; defer Bubble Tea unless `--ui` grows keyboard workflows beyond filter/rerun/quit.

---

### Phase 5 — Motion and micro-interactions

**Goal:** Subtle motion beyond the spinner tick — without sacrificing pipe safety or CI stability.

**Ideas (TTY-only, opt-in or automatic on TTY):**

| Effect | Use case | Implementation sketch |
|--------|----------|----------------------|
| **Spinner label crossfade** | Phase label changes | Debounce label updates; don't animate text chars (causes flicker) |
| **Progress bar ease** | Bounded test runs | Interpolate displayed percent toward actual (max 200ms lag) |
| **Completion pulse** | All tests pass | Brief green flash on result box border (one frame) |
| **Failure emphasis** | First FAIL in feed | Bold + red for 500ms then settle |
| **Throughput counter** | Large test suites | `· 42 tests/s` next to elapsed |

**Explicitly not in scope:**

- Streaming token-style animation (LLM output) — not applicable to glade.
- Sound or desktop notifications.
- Alternate-screen effects outside `--ui`.

**Flags:**

- `--no-motion` disables ease/pulse (spinner remains or use `--no-progress`).
- All motion off when `NO_COLOR` or non-TTY.

**Prerequisite:** Phase 1 TTY renderer with stable redraw; Phase 5 is polish only.

---

### Phase 6 — Structured output unification

**Goal:** One NDJSON event schema for `--progress-json`, `--watch`, and future editor/CI integrations.

**Work:**

- Document event schema in `docs/` (kinds, phases, payloads).
- Align `watch.*` events with `cliui.Event` field names where possible.
- Add `--output=jsonl` global mode for unified machine consumption (optional).
- Editor extensions and `glade dev` consume the same stream.

**Prerequisite:** Phase 1 event model; existing `--progress-json` and watch NDJSON in codebase.

---

## Implementation Notes

- Width-aware boxes: read terminal width when TTY (`ioctl` or default 80); truncate inner content with `…`.
- Count visible width excluding ANSI sequences when truncating (existing `truncateCell` may need ANSI-aware variant).
- Keep `internal/testreport` free of CLI assumptions — presentation import direction is `gladecli` → `cliui`, optionally `testreport` → `cliui` for `WriteConsole` delegation only.

---

## References

- `docs/research/CLI_UX_DESIGN.md` — research and pattern catalog
- `docs/superpowers/plans/2026-06-10-cli-ux-progress-system.md` — progress infrastructure plan (largely implemented)
- `internal/cliui/` — current renderer package

# CLI Personality Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the approved CLI personality spec (`docs/superpowers/specs/2026-06-10-cli-personality-design.md`) — theme, polished progress, and redesigned stdout for test, check, doctor, help, and errors.

**Architecture:** Extend `internal/cliui` with a `Theme` and format helpers. Polish existing TTY/line renderers. Wire `gladecli` and test output through `cliui` formatters. Stdlib only; progress on stderr, results on stdout.

**Tech Stack:** Go 1.26, standard library, existing `internal/cliui`, `internal/gladecli`, `internal/testreport`, `internal/diagnostic`.

**Spec:** `docs/superpowers/specs/2026-06-10-cli-personality-design.md`  
**Future work:** Phases 2–6 in spec (not this plan).

---

## File Structure

| File | Action | Role |
|------|--------|------|
| `internal/cliui/theme.go` | Create | Colors, glyphs, `Theme`, `NewTheme(w)` |
| `internal/cliui/format.go` | Create | Boxes, rows, width, ANSI strip/width |
| `internal/cliui/help.go` | Create | `WriteHelp`, `WriteTestHelp` |
| `internal/cliui/doctor.go` | Create | `WriteDoctor` |
| `internal/cliui/diagnostic.go` | Create | `WriteDiagnostics` |
| `internal/cliui/test_result.go` | Create | `WriteTestRun` |
| `internal/cliui/check_result.go` | Create | `WriteCheckResult` |
| `internal/cliui/error.go` | Create | `WriteCLIError` |
| `internal/cliui/tty_renderer.go` | Modify | Braille spinner, block bar, feed colors, phase resolution |
| `internal/cliui/line_renderer.go` | Modify | Align prefixes with new semantics |
| `internal/cliui/progressbar.go` | Modify | Block bar + plain ASCII bar |
| `internal/cliui/theme_test.go` | Create | Theme/plain/color tests |
| `internal/cliui/format_test.go` | Create | Box and row goldens (plain mode) |
| `internal/cliui/renderer_test.go` | Modify | Update for braille, colors, phase resolution |
| `internal/gladecli/cli.go` | Modify | Help, doctor, check, errors → cliui |
| `internal/gladecli/cli_test.go` | Modify | New output contracts |
| `internal/testreport/reporters.go` | Modify | Optional: delegate `WriteConsole` to cliui |

---

## Task 1: Theme and Format Primitives

**Files:**
- Create: `internal/cliui/theme.go`, `internal/cliui/format.go`
- Create: `internal/cliui/theme_test.go`, `internal/cliui/format_test.go`

- [ ] **Step 1: Write theme tests**

```go
func TestThemePlainWhenNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	th := NewTheme(nil) // non-TTY
	if th.Color {
		t.Fatal("expected plain theme")
	}
	if th.GlyphPass != "✓" { // glyphs stay unicode
		t.Fatalf("pass glyph = %q", th.GlyphPass)
	}
}

func TestThemeANSIWhenTTYAndColor(t *testing.T) {
	// use pipe pair or mock: Color true, Green("x") contains \x1b[
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./internal/cliui -run 'TestTheme' -count=1
```

- [ ] **Step 3: Implement `theme.go`**

```go
type Theme struct {
	Color      bool
	GlyphPass  string // "✓"
	GlyphFail  string // "✗"
	GlyphWarn  string // "⚠"
	GlyphSpin  []string // braille frames
	// methods: Green, Red, Yellow, Cyan, Dim, Bold, Reset string wrappers
}

func NewTheme(w io.Writer) Theme
```

Respect `NO_COLOR` and `IsTerminalWriter(w)`.

- [ ] **Step 4: Write format tests (plain goldens)**

```go
func TestFormatBoxPlain(t *testing.T) {
	th := Theme{Color: false}
	got := FormatBox(th, "Tests", "12 selected · 11 passed")
	// golden: lines start with ╭ or + depending on PlainBoxes setting
}
```

- [ ] **Step 5: Implement `format.go`**

- `FormatBox(title, body string) string` — unicode box, ASCII fallback when `!Color` or narrow
- `FormatRow(icon, label, value string) string` — aligned columns
- `VisibleWidth(s string) int` — strip ANSI for truncation
- `TruncateVisible(s string, max int) string`

- [ ] **Step 6: Run tests**

```bash
go test ./internal/cliui -run 'TestTheme|TestFormat' -count=1
```

Expected: PASS.

---

## Task 2: Polish Progress Renderers

**Files:**
- Modify: `internal/cliui/progressbar.go`, `internal/cliui/tty_renderer.go`, `internal/cliui/line_renderer.go`
- Modify: `internal/cliui/renderer_test.go`

- [ ] **Step 1: Update progress bar tests for block style**

```go
func TestProgressBarBlockStyle(t *testing.T) {
	th := Theme{Color: true}
	got := RenderProgressBarStyled(th, 5, 10, 10)
	if !strings.Contains(got, "█") {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./internal/cliui -run TestProgressBar -count=1
```

- [ ] **Step 3: Add `RenderProgressBarStyled(Theme, current, total, width)`**

Keep `RenderProgressBar` as plain ASCII wrapper for existing callers or replace usages.

- [ ] **Step 4: Update TTY renderer**

- Braille `spinnerFrames` from `Theme.GlyphSpin`
- Use `Theme` for bar, icons (`✓`/`✗`), dim elapsed
- Status line: `label · count · elapsed` with `·` separators
- Color activity feed lines by `EventKind`
- Phase resolution: track phase start time; on `EventPhaseEnd` emit resolved line in feed

- [ ] **Step 5: Update line renderer**

- Prefixes: `✓`/`✗`/`⚠` instead of bare `FAIL`/`WARN` where appropriate
- Keep machine-readable structure

- [ ] **Step 6: Run renderer tests**

```bash
go test ./internal/cliui -count=1
```

Expected: PASS (update existing test expectations for braille and new layout).

---

## Task 3: Test Result Formatter

**Files:**
- Create: `internal/cliui/test_result.go`
- Create tests in `internal/cliui/format_test.go` or new `test_result_test.go`
- Modify: `internal/gladecli/test_command.go`

- [ ] **Step 1: Write test for `WriteTestRun` plain golden**

Use `testreport.Run` fixture (copy minimal struct from `testreport/reporters_test.go` sample or import test helper).

- [ ] **Step 2: Run — expect FAIL**

- [ ] **Step 3: Implement `WriteTestRun(w io.Writer, run testreport.Run, opts ConsoleOptions)`**

Layout per spec: summary box, per-case rows, first problem detail, result box.

- [ ] **Step 4: Wire `test_command.go`**

Replace `testreport.WriteConsole` / `WriteConsoleWithOptions` calls with `cliui.WriteTestRun`.

- [ ] **Step 5: Update `internal/gladecli/cli_test.go` test expectations**

- [ ] **Step 6: Run**

```bash
go test ./internal/cliui ./internal/gladecli -run TestRunTest -count=1
```

---

## Task 4: Check Result and Diagnostics

**Files:**
- Create: `internal/cliui/diagnostic.go`, `internal/cliui/check_result.go`
- Modify: `internal/gladecli/cli.go` (`runCheck`)

- [ ] **Step 1: Write `WriteDiagnostics` test** — plain golden with file:line and severity

- [ ] **Step 2: Write `WriteCheckResult` test** — summary box + diagnostics

- [ ] **Step 3: Implement formatters**

- [ ] **Step 4: Replace check stdout block in `runCheck`** (lines ~791–799) with `cliui.WriteCheckResult`

- [ ] **Step 5: Update `TestRunCheck*` in `cli_test.go`**

- [ ] **Step 6: Run**

```bash
go test ./internal/gladecli -run TestRunCheck -count=1
```

---

## Task 5: Doctor and Help

**Files:**
- Create: `internal/cliui/doctor.go`, `internal/cliui/help.go`
- Modify: `internal/gladecli/cli.go`

- [ ] **Step 1: Tests for `WriteDoctor` and `WriteHelp` (plain goldens)**

- [ ] **Step 2: Implement formatters**

`WriteDoctor(w, DoctorInfo{...})` — pass structured fields from `runDoctor` instead of interleaved `fmt.Fprintf`.

- [ ] **Step 3: Replace `printHelp`, `printTestHelp`, `runDoctor` output**

- [ ] **Step 4: Update `TestRunTopLevelHelpAlignment` if column layout changes**

Help may use two-column layout with different spacing — update test to match new design (still aligned).

- [ ] **Step 5: Run**

```bash
go test ./internal/gladecli -run 'TestRunTopLevelHelp|TestRunDoctor' -count=1
```

---

## Task 6: CLI Errors

**Files:**
- Create: `internal/cliui/error.go`
- Modify: `internal/gladecli/cli.go` (error paths)

- [ ] **Step 1: Test `WriteCLIError`**

- [ ] **Step 2: Implement**

- [ ] **Step 3: Replace `fmt.Fprintf(stderr, "glade: %v\n", err)`** with `cliui.WriteCLIError(stderr, err)` where message is user-facing (not diagnostic reports).

Keep `diagnostic.Report.WriteText` for `GLADECLI001` unknown command (already structured).

- [ ] **Step 4: Run gladecli tests**

```bash
go test ./internal/gladecli -count=1
```

---

## Task 7: Delegate testreport.WriteConsole (optional cleanup)

**Files:**
- Modify: `internal/testreport/reporters.go`

- [ ] **Step 1: Change `WriteConsole` to call `cliui.WriteTestRun`**

Avoid duplicating two console formatters. Update `reporters_test.go` expected strings.

- [ ] **Step 2: Run**

```bash
go test ./internal/testreport -count=1
```

Note: creates `testreport` → `cliui` import. Acceptable per spec.

---

## Task 8: Full Verification

- [ ] **Step 1: Package tests**

```bash
go test ./internal/cliui ./internal/gladecli ./internal/testreport -count=1
```

- [ ] **Step 2: Manual smoke (TTY)**

```bash
go run ./cmd/glade check --project internal/gladecli/testdata/simple
go run ./cmd/glade test --project internal/gladecli/testdata/simple --filter Sample
go run ./cmd/glade doctor
go run ./cmd/glade help
```

- [ ] **Step 3: Pipe / NO_COLOR**

```bash
NO_COLOR=1 go run ./cmd/glade check --project internal/gladecli/testdata/simple 2>/dev/null | head
go run ./cmd/glade check --project internal/gladecli/testdata/simple --progress 2>&1 | head
```

- [ ] **Step 4: JSON unchanged**

```bash
go run ./cmd/glade check --project internal/gladecli/testdata/simple --json | jq type
go run ./cmd/glade test --project internal/gladecli/testdata/simple --json | jq type
```

Expected: `"object"` for both.

---

## Implementation Rules

- No new `go.mod` dependencies.
- Commands do not import `"fmt"` for user-facing styled output once cliui helpers exist (errors/diagnostics only).
- Goldens use plain theme (`Color: false`) for CI stability.
- Do not change JSON/JUnit/progress-json schemas in this plan.

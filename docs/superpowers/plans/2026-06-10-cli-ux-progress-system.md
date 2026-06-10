# CLI UX Progress System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the CLI UX research in `docs/research/CLI_UX_DESIGN.md` into a polished progress system for `glade test`, `glade check`, and `glade schema load`.

**Architecture:** Add renderer-agnostic progress events to `internal/cliui`, then render those events through line, TTY-region, and NDJSON backends. Keep progress on `stderr`, final command data on `stdout`, and make commands emit events instead of ANSI bytes.

**Tech Stack:** Go 1.26, standard library only, existing `internal/cliui`, `internal/gladecli`, `internal/project`, `internal/schema`, `internal/typesys`, `internal/sema`, and `internal/apextest`.

---

## Review Notes From `docs/research/CLI_UX_DESIGN.md`

- The research doc is accurate about the current shape: `internal/cliui/cliui.go` has only `ColorEnabled` and `Row`, and `internal/gladecli/test_command.go` owns the single progress reporter.
- `Run()` currently passes `stderr` only into `runTest()`. `runCheck()` and `runSchema()` must receive a progress writer before those commands can show progress.
- `runCheck()` hides the natural phases behind `loadIndex(root)`. Split that work enough to report `loading project`, `loading metadata`, `indexing`, and `checking`.
- `glade test --progress` already writes to `stderr`. Preserve that contract while moving rendering out of `internal/gladecli`.
- The repo has broad in-flight work. Before implementation, start from a clean worktree or an isolated `codex/` worktree. Do not overwrite unrelated editor, DAP, debug, or docs changes.

## File Structure

- Create `internal/cliui/event.go`: shared event model, event kinds, phase summaries, and result summaries.
- Create `internal/cliui/renderer.go`: renderer interface, options, renderer selection, clock injection, and TTY detection helpers.
- Create `internal/cliui/line_renderer.go`: pipe/CI renderer that writes stable text lines to `stderr`.
- Create `internal/cliui/ndjson_renderer.go`: machine-readable progress event writer.
- Create `internal/cliui/tty_renderer.go`: bounded multi-line terminal renderer with spinner, progress bar, activity feed, and cleanup.
- Create `internal/cliui/progressbar.go`: width-aware bar rendering and ETA formatting.
- Create `internal/cliui/activity.go`: fixed-size recent activity feed.
- Create `internal/cliui/renderer_test.go`: unit tests for line rendering, JSON rendering, TTY rendering, bars, feed trimming, and stdout/stderr separation.
- Modify `internal/cliui/cliui.go`: keep `ColorEnabled` and `Row`; add small exported helpers only if shared tests prove they are useful.
- Modify `internal/gladecli/cli.go`: pass `stderr` into `runCheck()` and `runSchema()`, parse progress flags, and emit phase events.
- Modify `internal/gladecli/test_command.go`: replace `cliTestProgressReporter` with an adapter that emits `cliui.Event`.
- Modify `internal/gladecli/cli_test.go`: add command contract tests for progress defaults, `--no-progress`, `--progress-json`, and clean `stdout`.

## Implementation Rules

- Do not add Bubble Tea, termenv, lipgloss, or another TUI dependency. The first pass is small enough for standard library ANSI rendering.
- Do not put ANSI strings in command code. ANSI belongs in `internal/cliui`.
- Do not write progress to `stdout`. `stdout` is for JSON, console reports, and command results.
- Do not make alternate-screen UI part of this plan. `glade test --ui` stays future work.
- Default progress should appear only on an interactive terminal. Tests can keep explicit `--progress` because byte buffers are non-TTY.
- `--json` must not suppress progress on `stderr` when progress is explicitly requested.
- `--progress-json` writes NDJSON events to `stderr`. It does not change the command result on `stdout`.
- `NO_COLOR` disables color. It does not disable progress text.

---

## Task 1: Build The Progress Event Model

**Files:**
- Create: `internal/cliui/event.go`
- Modify: `internal/cliui/cliui_test.go`
- Test: `internal/cliui/renderer_test.go`

- [ ] **Step 1: Write the event model tests**

Create `internal/cliui/renderer_test.go` with this first test:

```go
package cliui

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventJSONIsStable(t *testing.T) {
	ev := Event{
		Kind:    EventPhaseStart,
		Phase:   "checking",
		Label:   "Checking Apex semantics",
		Current: 3,
		Total:   7,
		Detail:  "AccountService.cls",
		At:      time.Unix(100, 250000000).UTC(),
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"kind":"phase_start","phase":"checking","label":"Checking Apex semantics","detail":"AccountService.cls","current":3,"total":7,"at":"1970-01-01T00:01:40.25Z"}`
	if string(data) != want {
		t.Fatalf("json = %s, want %s", data, want)
	}
}
```

- [ ] **Step 2: Run the focused test and watch it fail**

Run:

```bash
go test ./internal/cliui -run TestEventJSONIsStable -count=1
```

Expected: FAIL with undefined `Event` and `EventPhaseStart`.

- [ ] **Step 3: Add the event types**

Create `internal/cliui/event.go`:

```go
package cliui

import "time"

type EventKind string

const (
	EventPhaseStart EventKind = "phase_start"
	EventPhaseTick  EventKind = "phase_tick"
	EventPhaseEnd   EventKind = "phase_end"
	EventInfo       EventKind = "info"
	EventWarn       EventKind = "warn"
	EventFail       EventKind = "fail"
	EventDone       EventKind = "done"
)

type Event struct {
	Kind    EventKind `json:"kind"`
	Phase   string    `json:"phase,omitempty"`
	Label   string    `json:"label,omitempty"`
	Detail  string    `json:"detail,omitempty"`
	Current int       `json:"current,omitempty"`
	Total   int       `json:"total,omitempty"`
	At      time.Time `json:"at,omitempty"`
}

type Result struct {
	OK       bool          `json:"ok"`
	Label    string        `json:"label,omitempty"`
	Elapsed  time.Duration `json:"-"`
	ExitCode int           `json:"exitCode,omitempty"`
}
```

- [ ] **Step 4: Run the focused test**

Run:

```bash
go test ./internal/cliui -run TestEventJSONIsStable -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the event model**

Run:

```bash
git add internal/cliui/event.go internal/cliui/renderer_test.go
git commit -m "feat: add cli progress event model"
```

Expected: commit succeeds. If the worktree contains unrelated changes, stage only these two files.

---

## Task 2: Add Renderer Selection And Line Rendering

**Files:**
- Create: `internal/cliui/renderer.go`
- Create: `internal/cliui/line_renderer.go`
- Modify: `internal/cliui/renderer_test.go`

- [ ] **Step 1: Add tests for renderer selection and line output**

Append to `internal/cliui/renderer_test.go`:

```go
func TestNewRendererUsesLineRendererForNonTTY(t *testing.T) {
	var out bytes.Buffer
	r := NewRenderer(RendererOptions{Stderr: &out, Mode: ProgressLine})
	if _, ok := r.(*LineRenderer); !ok {
		t.Fatalf("renderer = %T, want *LineRenderer", r)
	}
}

func TestLineRendererWritesReadableProgress(t *testing.T) {
	var out bytes.Buffer
	r := NewLineRenderer(&out, fixedClock(time.Unix(10, 0).UTC()))
	r.Render(Event{Kind: EventPhaseStart, Phase: "checking", Label: "Checking Apex semantics"})
	r.Render(Event{Kind: EventPhaseTick, Phase: "checking", Label: "Analyzed AccountService", Current: 2, Total: 5})
	r.Render(Event{Kind: EventFail, Phase: "checking", Label: "GLADESEMA002", Detail: "Unknown type MissingType"})
	r.Finish(Result{OK: false, Label: "check failed", ExitCode: 1})

	got := out.String()
	for _, want := range []string{
		"checking: Checking Apex semantics",
		"checking: 2/5 Analyzed AccountService",
		"FAIL checking: GLADESEMA002 - Unknown type MissingType",
		"done: check failed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("line output missing %q:\n%s", want, got)
		}
	}
}

type fixedClock time.Time

func (c fixedClock) Now() time.Time {
	return time.Time(c)
}
```

Add these imports to the test file:

```go
import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)
```

- [ ] **Step 2: Run the tests and verify failure**

Run:

```bash
go test ./internal/cliui -run 'Test(NewRendererUsesLineRendererForNonTTY|LineRendererWritesReadableProgress)' -count=1
```

Expected: FAIL with undefined renderer types.

- [ ] **Step 3: Add renderer selection**

Create `internal/cliui/renderer.go`:

```go
package cliui

import (
	"io"
	"os"
	"time"
)

type ProgressMode string

const (
	ProgressAuto ProgressMode = "auto"
	ProgressTTY  ProgressMode = "tty"
	ProgressLine ProgressMode = "line"
	ProgressJSON ProgressMode = "json"
	ProgressOff  ProgressMode = "off"
)

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type Renderer interface {
	Render(Event)
	Finish(Result)
}

type RendererOptions struct {
	Stderr io.Writer
	Mode   ProgressMode
	Clock  Clock
}

func NewRenderer(opts RendererOptions) Renderer {
	if opts.Stderr == nil || opts.Mode == ProgressOff {
		return NullRenderer{}
	}
	if opts.Clock == nil {
		opts.Clock = systemClock{}
	}
	switch opts.Mode {
	case ProgressJSON:
		return NewNDJSONRenderer(opts.Stderr)
	case ProgressTTY:
		return NewTTYRenderer(opts.Stderr, opts.Clock)
	case ProgressLine:
		return NewLineRenderer(opts.Stderr, opts.Clock)
	case ProgressAuto, "":
		if IsTerminalWriter(opts.Stderr) {
			return NewTTYRenderer(opts.Stderr, opts.Clock)
		}
		return NewLineRenderer(opts.Stderr, opts.Clock)
	default:
		return NewLineRenderer(opts.Stderr, opts.Clock)
	}
}

type NullRenderer struct{}

func (NullRenderer) Render(Event)  {}
func (NullRenderer) Finish(Result) {}

func IsTerminalWriter(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok || file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
```

Add `time` to the import list in `renderer.go`.

- [ ] **Step 4: Add the line renderer**

Create `internal/cliui/line_renderer.go`:

```go
package cliui

import (
	"fmt"
	"io"
	"strings"
)

type LineRenderer struct {
	w     io.Writer
	clock Clock
}

func NewLineRenderer(w io.Writer, clock Clock) *LineRenderer {
	if clock == nil {
		clock = systemClock{}
	}
	return &LineRenderer{w: w, clock: clock}
}

func (r *LineRenderer) Render(ev Event) {
	if r == nil || r.w == nil {
		return
	}
	phase := strings.TrimSpace(ev.Phase)
	label := strings.TrimSpace(ev.Label)
	detail := strings.TrimSpace(ev.Detail)
	switch ev.Kind {
	case EventFail:
		if detail != "" {
			fmt.Fprintf(r.w, "FAIL %s: %s - %s\n", phase, label, detail)
			return
		}
		fmt.Fprintf(r.w, "FAIL %s: %s\n", phase, label)
	case EventWarn:
		fmt.Fprintf(r.w, "WARN %s: %s\n", phase, label)
	case EventPhaseTick:
		if ev.Total > 0 {
			fmt.Fprintf(r.w, "%s: %d/%d %s\n", phase, ev.Current, ev.Total, label)
			return
		}
		fmt.Fprintf(r.w, "%s: %s\n", phase, label)
	default:
		if label == "" {
			return
		}
		fmt.Fprintf(r.w, "%s: %s\n", phase, label)
	}
}

func (r *LineRenderer) Finish(result Result) {
	if r == nil || r.w == nil || strings.TrimSpace(result.Label) == "" {
		return
	}
	fmt.Fprintf(r.w, "done: %s\n", result.Label)
}
```

- [ ] **Step 5: Run the renderer tests**

Run:

```bash
go test ./internal/cliui -count=1
```

Expected: PASS after import fixes.

- [ ] **Step 6: Commit line rendering**

Run:

```bash
git add internal/cliui/renderer.go internal/cliui/line_renderer.go internal/cliui/renderer_test.go
git commit -m "feat: add cli progress line renderer"
```

Expected: commit succeeds with only `internal/cliui` files staged.

---

## Task 3: Add NDJSON Rendering

**Files:**
- Create: `internal/cliui/ndjson_renderer.go`
- Modify: `internal/cliui/renderer_test.go`

- [ ] **Step 1: Add the NDJSON renderer test**

Append to `internal/cliui/renderer_test.go`:

```go
func TestNDJSONRendererWritesOneEventPerLine(t *testing.T) {
	var out bytes.Buffer
	r := NewNDJSONRenderer(&out)
	r.Render(Event{Kind: EventPhaseStart, Phase: "schema", Label: "Loading metadata"})
	r.Finish(Result{OK: true, Label: "schema loaded"})

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2:\n%s", len(lines), out.String())
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatalf("invalid json line %q", line)
		}
	}
	if !strings.Contains(lines[0], `"kind":"phase_start"`) || !strings.Contains(lines[1], `"kind":"done"`) {
		t.Fatalf("unexpected ndjson:\n%s", out.String())
	}
}
```

- [ ] **Step 2: Run the test and verify failure**

Run:

```bash
go test ./internal/cliui -run TestNDJSONRendererWritesOneEventPerLine -count=1
```

Expected: FAIL with undefined `NewNDJSONRenderer`.

- [ ] **Step 3: Add the NDJSON renderer**

Create `internal/cliui/ndjson_renderer.go`:

```go
package cliui

import (
	"encoding/json"
	"io"
	"time"
)

type NDJSONRenderer struct {
	enc *json.Encoder
}

func NewNDJSONRenderer(w io.Writer) *NDJSONRenderer {
	return &NDJSONRenderer{enc: json.NewEncoder(w)}
}

func (r *NDJSONRenderer) Render(ev Event) {
	if r == nil || r.enc == nil {
		return
	}
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	_ = r.enc.Encode(ev)
}

func (r *NDJSONRenderer) Finish(result Result) {
	if r == nil || r.enc == nil {
		return
	}
	_ = r.enc.Encode(Event{
		Kind:  EventDone,
		Label: result.Label,
		At:    time.Now().UTC(),
	})
}
```

- [ ] **Step 4: Run all cliui tests**

Run:

```bash
go test ./internal/cliui -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit NDJSON rendering**

Run:

```bash
git add internal/cliui/ndjson_renderer.go internal/cliui/renderer_test.go
git commit -m "feat: add ndjson progress renderer"
```

Expected: commit succeeds.

---

## Task 4: Add Activity Feed And Progress Bars

**Files:**
- Create: `internal/cliui/activity.go`
- Create: `internal/cliui/progressbar.go`
- Modify: `internal/cliui/renderer_test.go`

- [ ] **Step 1: Add tests for feed trimming and bar shape**

Append to `internal/cliui/renderer_test.go`:

```go
func TestActivityFeedKeepsLastNEvents(t *testing.T) {
	feed := NewActivityFeed(3)
	for _, label := range []string{"one", "two", "three", "four"} {
		feed.Add(Event{Kind: EventInfo, Label: label})
	}
	got := feed.Lines()
	want := []string{"two", "three", "four"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("feed = %#v, want %#v", got, want)
	}
}

func TestProgressBarRendersBoundedAndUnbounded(t *testing.T) {
	if got := RenderProgressBar(5, 10, 10); got != "[===>    ]" {
		t.Fatalf("bounded bar = %q", got)
	}
	if got := RenderProgressBar(3, 0, 10); got != "[   >    ]" {
		t.Fatalf("unbounded bar = %q", got)
	}
}

func TestFormatDurationForProgress(t *testing.T) {
	cases := map[time.Duration]string{
		1500 * time.Millisecond: "2s",
		65 * time.Second:       "1m05s",
		2*time.Hour + time.Minute: "2h01m",
	}
	for input, want := range cases {
		if got := FormatDuration(input); got != want {
			t.Fatalf("FormatDuration(%s) = %s, want %s", input, got, want)
		}
	}
}
```

Add `reflect` to the imports.

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/cliui -run 'Test(ActivityFeed|ProgressBar|FormatDuration)' -count=1
```

Expected: FAIL with undefined helpers.

- [ ] **Step 3: Add the activity feed**

Create `internal/cliui/activity.go`:

```go
package cliui

import "strings"

type ActivityFeed struct {
	limit int
	lines []string
}

func NewActivityFeed(limit int) *ActivityFeed {
	if limit <= 0 {
		limit = 5
	}
	return &ActivityFeed{limit: limit}
}

func (f *ActivityFeed) Add(ev Event) {
	if f == nil {
		return
	}
	line := strings.TrimSpace(ev.Label)
	if ev.Detail != "" {
		line += " - " + strings.TrimSpace(ev.Detail)
	}
	if line == "" {
		return
	}
	f.lines = append(f.lines, line)
	if len(f.lines) > f.limit {
		f.lines = append([]string(nil), f.lines[len(f.lines)-f.limit:]...)
	}
}

func (f *ActivityFeed) Lines() []string {
	if f == nil {
		return nil
	}
	return append([]string(nil), f.lines...)
}
```

- [ ] **Step 4: Add progress bar helpers**

Create `internal/cliui/progressbar.go`:

```go
package cliui

import (
	"fmt"
	"strings"
	"time"
)

func RenderProgressBar(current, total, width int) string {
	if width < 4 {
		width = 4
	}
	inner := width - 2
	pos := 0
	if total > 0 {
		if current < 0 {
			current = 0
		}
		if current > total {
			current = total
		}
		pos = current * (inner - 1) / total
	} else {
		pos = current % inner
	}
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < inner; i++ {
		switch {
		case i < pos && total > 0:
			b.WriteByte('=')
		case i == pos:
			b.WriteByte('>')
		default:
			b.WriteByte(' ')
		}
	}
	b.WriteByte(']')
	return b.String()
}

func FormatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(d/time.Minute), int((d%time.Minute)/time.Second))
	}
	return fmt.Sprintf("%dh%02dm", int(d/time.Hour), int((d%time.Hour)/time.Minute))
}
```

- [ ] **Step 5: Run cliui tests**

Run:

```bash
go test ./internal/cliui -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit feed and bar helpers**

Run:

```bash
git add internal/cliui/activity.go internal/cliui/progressbar.go internal/cliui/renderer_test.go
git commit -m "feat: add progress bar and activity feed"
```

Expected: commit succeeds.

---

## Task 5: Add The TTY Region Renderer

**Files:**
- Create: `internal/cliui/tty_renderer.go`
- Modify: `internal/cliui/renderer_test.go`

- [ ] **Step 1: Add TTY renderer tests**

Append to `internal/cliui/renderer_test.go`:

```go
func TestTTYRendererUsesANSIRegionAndActivityFeed(t *testing.T) {
	var out bytes.Buffer
	r := NewTTYRenderer(&out, fixedClock(time.Unix(20, 0).UTC()))
	r.Render(Event{Kind: EventPhaseStart, Phase: "test", Label: "Running tests", Total: 2})
	r.Render(Event{Kind: EventPhaseTick, Phase: "test", Label: "PASS AccountTest.creates", Current: 1, Total: 2})
	r.Render(Event{Kind: EventFail, Phase: "test", Label: "FAIL ContactTest.validates", Detail: "Expected Active"})
	r.Finish(Result{OK: false, Label: "1 passed, 1 failed", ExitCode: 1})

	got := out.String()
	for _, want := range []string{
		"\r\x1b[K",
		"[",
		"Running tests",
		"PASS AccountTest.creates",
		"FAIL ContactTest.validates - Expected Active",
		"1 passed, 1 failed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("tty output missing %q:\n%q", want, got)
		}
	}
}
```

- [ ] **Step 2: Run the test and verify failure**

Run:

```bash
go test ./internal/cliui -run TestTTYRendererUsesANSIRegionAndActivityFeed -count=1
```

Expected: FAIL with undefined `NewTTYRenderer`.

- [ ] **Step 3: Add TTY renderer**

Create `internal/cliui/tty_renderer.go`:

```go
package cliui

import (
	"fmt"
	"io"
	"strings"
	"time"
)

var spinnerFrames = []string{"|", "/", "-", "\\"}

type TTYRenderer struct {
	w       io.Writer
	clock   Clock
	feed    *ActivityFeed
	started time.Time
	last    Event
	frame   int
	lines   int
}

func NewTTYRenderer(w io.Writer, clock Clock) *TTYRenderer {
	if clock == nil {
		clock = systemClock{}
	}
	return &TTYRenderer{
		w:       w,
		clock:   clock,
		feed:    NewActivityFeed(5),
		started: clock.Now(),
	}
}

func (r *TTYRenderer) Render(ev Event) {
	if r == nil || r.w == nil {
		return
	}
	if ev.At.IsZero() {
		ev.At = r.clock.Now()
	}
	if ev.Kind == EventInfo || ev.Kind == EventWarn || ev.Kind == EventFail || ev.Kind == EventPhaseTick {
		r.feed.Add(ev)
	}
	r.last = ev
	r.frame++
	r.draw("")
}

func (r *TTYRenderer) Finish(result Result) {
	if r == nil || r.w == nil {
		return
	}
	r.draw(strings.TrimSpace(result.Label))
	fmt.Fprint(r.w, "\n")
	r.lines = 0
}

func (r *TTYRenderer) draw(doneLabel string) {
	if r.lines > 0 {
		for i := 0; i < r.lines; i++ {
			fmt.Fprint(r.w, "\x1b[1A\r\x1b[K")
		}
	} else {
		fmt.Fprint(r.w, "\r\x1b[K")
	}
	lines := r.renderLines(doneLabel)
	for _, line := range lines {
		fmt.Fprintln(r.w, line)
	}
	r.lines = len(lines)
}

func (r *TTYRenderer) renderLines(doneLabel string) []string {
	ev := r.last
	elapsed := FormatDuration(r.clock.Now().Sub(r.started))
	bar := RenderProgressBar(ev.Current, ev.Total, 22)
	icon := spinnerFrames[r.frame%len(spinnerFrames)]
	label := strings.TrimSpace(ev.Label)
	if doneLabel != "" {
		icon = "x"
		if ev.Kind != EventFail {
			icon = "v"
		}
		label = doneLabel
	}
	count := ""
	if ev.Total > 0 {
		count = fmt.Sprintf(" %d/%d", ev.Current, ev.Total)
	}
	lines := []string{fmt.Sprintf("%s %s%s %s elapsed=%s", icon, bar, count, label, elapsed)}
	for _, line := range r.feed.Lines() {
		lines = append(lines, "  "+line)
	}
	return lines
}
```

This first renderer uses ASCII spinner frames and `v`/`x` final markers to keep output stable across terminals. Add color and Unicode glyphs only after the ASCII renderer is green.

- [ ] **Step 4: Run cliui tests**

Run:

```bash
go test ./internal/cliui -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit TTY rendering**

Run:

```bash
git add internal/cliui/tty_renderer.go internal/cliui/renderer_test.go
git commit -m "feat: add tty progress renderer"
```

Expected: commit succeeds.

---

## Task 6: Move `glade test --progress` Onto `cliui`

**Files:**
- Modify: `internal/gladecli/test_command.go`
- Modify: `internal/gladecli/cli_test.go`
- Test: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Update the CLI progress contract test**

In `internal/gladecli/cli_test.go`, update `TestRunTestProgressWritesToStderr` expected strings from the old `Progress:` line to the new renderer language:

```go
for _, want := range []string{"test:", "1/1", "elapsed=", "SampleTest.passes"} {
	if !strings.Contains(got, want) {
		t.Fatalf("stderr missing %q:\n%s", want, got)
	}
}
```

Then add this test:

```go
func TestRunTestProgressJSONWritesNDJSONToStderr(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/SampleTest.cls"), `
@isTest
private class SampleTest {
  @isTest static void passes() {
    System.assertEquals(2, 1 + 1);
  }
}
`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"test", "--project", root, "--progress-json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "Result: 1 passed") {
		t.Fatalf("stdout did not include console result: %q", stdout.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatalf("stderr line is not json: %q\nall stderr:\n%s", line, stderr.String())
		}
	}
	for _, want := range []string{`"kind":"phase_start"`, `"kind":"phase_tick"`, `"kind":"done"`} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}
```

Add `encoding/json` to the imports if it is not already present.

- [ ] **Step 2: Run tests and verify failure**

Run:

```bash
go test ./internal/gladecli -run 'TestRunTestProgress(WritesToStderr|JSONWritesNDJSONToStderr)' -count=1
```

Expected: FAIL because `--progress-json` does not exist and old reporter output still starts with `Progress:`.

- [ ] **Step 3: Import `cliui` and parse the new flag**

In `internal/gladecli/test_command.go`, add:

```go
"github.com/glade-sh/glade/internal/cliui"
```

Replace:

```go
progress := false
```

with:

```go
progressMode := cliui.ProgressOff
```

Replace the flag parse case:

```go
case "--progress":
	progress = true
```

with:

```go
case "--progress":
	progressMode = cliui.ProgressLine
case "--progress-json":
	progressMode = cliui.ProgressJSON
```

Keep `--progress` as line mode for byte-buffer tests and CI stability. Later tasks make interactive TTY auto-progress.

- [ ] **Step 4: Replace the old reporter construction**

Replace:

```go
var progressReporter *cliTestProgressReporter
if progress {
	progressReporter = newCLITestProgressReporter(progressW)
	defer progressReporter.finish()
}

testOpts := apextest.Options{
	Filter:          filter,
	ParallelMethods: parallelMethods,
	Parallelism:     parallelism,
	Timeout:         testTimeout,
}
if progress {
	testOpts.Progress = progressReporter.handle
}
```

with:

```go
var progressReporter *cliTestProgressReporter
if progressMode != cliui.ProgressOff {
	progressReporter = newCLITestProgressReporter(cliui.NewRenderer(cliui.RendererOptions{
		Stderr: progressW,
		Mode:   progressMode,
	}))
	defer progressReporter.finish()
}

testOpts := apextest.Options{
	Filter:          filter,
	ParallelMethods: parallelMethods,
	Parallelism:     parallelism,
	Timeout:         testTimeout,
}
if progressReporter != nil {
	testOpts.Progress = progressReporter.handle
}
```

- [ ] **Step 5: Replace the reporter type**

In `internal/gladecli/test_command.go`, replace the whole `cliTestProgressReporter` type and its methods with:

```go
type cliTestProgressReporter struct {
	renderer cliui.Renderer
	started  time.Time
	total    int
	done     int
	passed   int
	failed   int
	errors   int
	mu       sync.Mutex
	finished bool
}

func newCLITestProgressReporter(renderer cliui.Renderer) *cliTestProgressReporter {
	r := &cliTestProgressReporter{renderer: renderer, started: time.Now()}
	r.renderer.Render(cliui.Event{
		Kind:  cliui.EventPhaseStart,
		Phase: "test",
		Label: "Discovering tests",
	})
	return r
}

func (r *cliTestProgressReporter) setTotal(total int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.total = total
	r.renderer.Render(cliui.Event{
		Kind:  cliui.EventPhaseTick,
		Phase: "test",
		Label: "Running tests",
		Total: total,
	})
}

func (r *cliTestProgressReporter) handle(progress apextest.TestProgress) {
	if r == nil || r.renderer == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	name := progress.ClassName
	if progress.MethodName != "" {
		name += "." + progress.MethodName
	}

	switch progress.Event {
	case "test_start", "setup_start":
		r.renderer.Render(cliui.Event{
			Kind:    cliui.EventPhaseTick,
			Phase:   "test",
			Label:   name,
			Current: r.done,
			Total:   r.total,
		})
	case "test_done":
		r.done++
		kind := cliui.EventInfo
		label := "PASS " + name
		switch testreport.Status(progress.Status) {
		case testreport.StatusPass:
			r.passed++
		case testreport.StatusFail:
			r.failed++
			kind = cliui.EventFail
			label = "FAIL " + name
		case testreport.StatusCompileError, testreport.StatusRuntimeError, testreport.StatusUnsupported:
			r.errors++
			kind = cliui.EventFail
			label = string(progress.Status) + " " + name
		}
		r.renderer.Render(cliui.Event{
			Kind:    kind,
			Phase:   "test",
			Label:   label,
			Current: r.done,
			Total:   r.total,
		})
	case "setup_done":
		if progress.Status != "pass" {
			r.errors++
			r.renderer.Render(cliui.Event{
				Kind:    cliui.EventFail,
				Phase:   "test",
				Label:   "setup failed " + name,
				Current: r.done,
				Total:   r.total,
			})
		}
	}
}

func (r *cliTestProgressReporter) finish() {
	if r == nil || r.renderer == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finished {
		return
	}
	r.finished = true
	ok := r.failed == 0 && r.errors == 0
	r.renderer.Finish(cliui.Result{
		OK:    ok,
		Label: fmt.Sprintf("%d passed, %d failed, %d errors", r.passed, r.failed, r.errors),
	})
}
```

Then delete the old `writeLine`, `countText`, `etaText`, `formatProgressDuration`, and `isTerminalWriter` methods from `test_command.go`. `FormatDuration` and `IsTerminalWriter` now live in `internal/cliui`.

- [ ] **Step 6: Run the focused gladecli tests**

Run:

```bash
go test ./internal/gladecli -run 'TestRunTestProgress(WritesToStderr|JSONWritesNDJSONToStderr)' -count=1
```

Expected: PASS.

- [ ] **Step 7: Run related package tests**

Run:

```bash
go test ./internal/cliui ./internal/gladecli -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit test progress migration**

Run:

```bash
git add internal/gladecli/test_command.go internal/gladecli/cli_test.go
git commit -m "feat: render test progress through cliui"
```

Expected: commit succeeds.

---

## Task 7: Add `check` Progress Phases

**Files:**
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Add `check --progress` contract test**

Append near the existing check tests in `internal/gladecli/cli_test.go`:

```go
func TestRunCheckProgressWritesPhasesToStderr(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello { public void run() {} }")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "diagnostics: 0") {
		t.Fatalf("stdout missing check result: %q", stdout.String())
	}
	for _, want := range []string{
		"check: Loading project",
		"check: Loading metadata",
		"check: Indexing Apex symbols",
		"check: Running semantic checks",
		"done: check complete",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}
```

- [ ] **Step 2: Add `check --progress-json` contract test**

Append:

```go
func TestRunCheckProgressJSONKeepsStdoutJSON(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello { public void run() {} }")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--json", "--progress-json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not json: %q", stdout.String())
	}
	for _, line := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
		if !json.Valid([]byte(line)) {
			t.Fatalf("stderr line is not json: %q\nall stderr:\n%s", line, stderr.String())
		}
	}
	if !strings.Contains(stderr.String(), `"phase":"check"`) {
		t.Fatalf("stderr missing check phase:\n%s", stderr.String())
	}
}
```

- [ ] **Step 3: Run the tests and verify failure**

Run:

```bash
go test ./internal/gladecli -run 'TestRunCheckProgress' -count=1
```

Expected: FAIL because `check` does not parse progress flags.

- [ ] **Step 4: Pass `stderr` into `runCheck`**

In `Run()` inside `internal/gladecli/cli.go`, change:

```go
result, err := runCheck(ctx, args[1:], stdout)
```

to:

```go
result, err := runCheck(ctx, args[1:], stdout, stderr)
```

Change the function signature:

```go
func runCheck(ctx context.Context, args []string, w io.Writer, progressW io.Writer) (sema.Result, error)
```

- [ ] **Step 5: Add progress flag parsing**

Inside `runCheck`, replace:

```go
root, jsonOut, err := parseProjectFlags(args)
```

with:

```go
root, jsonOut, progressMode, err := parseProjectProgressFlags(args)
```

Add this helper near `parseProjectFlags`:

```go
func parseProjectProgressFlags(args []string) (root string, jsonOut bool, progressMode cliui.ProgressMode, err error) {
	root = "."
	progressMode = cliui.ProgressOff
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--progress":
			progressMode = cliui.ProgressLine
		case "--progress-json":
			progressMode = cliui.ProgressJSON
		case "--no-progress", "--quiet":
			progressMode = cliui.ProgressOff
		case "--project":
			if i+1 >= len(args) {
				return "", false, cliui.ProgressOff, errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		default:
			return "", false, cliui.ProgressOff, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return root, jsonOut, progressMode, nil
}
```

Add the `cliui` import to `internal/gladecli/cli.go`.

- [ ] **Step 6: Split `runCheck` into explicit phases**

Replace:

```go
index, err := loadIndex(root)
if err != nil {
	return sema.Result{}, err
}
result := sema.Analyze(index)
```

with:

```go
renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: progressMode})
renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "check", Label: "Loading project"})
p, err := project.Load(root)
if err != nil {
	renderer.Finish(cliui.Result{OK: false, Label: "check failed"})
	return sema.Result{}, err
}

renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "check", Label: "Loading metadata", Current: 1, Total: 4})
s, err := gladeschema.LoadProject(p)
var index typesys.Index
if err != nil {
	index = typesys.Build(p, gladeschema.Schema{})
	index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "GLADESCHEMA001",
		Message:  fmt.Sprintf("metadata schema load failed: %v", err),
	})
} else {
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "check", Label: "Indexing Apex symbols", Current: 2, Total: 4})
	index = typesys.Build(p, s)
}

renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "check", Label: "Running semantic checks", Current: 3, Total: 4})
result := sema.Analyze(index)
renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: "check", Label: "Semantic checks complete", Current: 4, Total: 4})
renderer.Finish(cliui.Result{OK: !result.HasErrors(), Label: "check complete"})
```

This keeps malformed metadata behavior identical to `loadProjectIndex`: schema load errors become diagnostics, not hard command errors.

- [ ] **Step 7: Run focused check tests**

Run:

```bash
go test ./internal/gladecli -run 'TestRunCheck(Progress|JSON|UnknownType|ReportsMalformedMetadata)' -count=1
```

Expected: PASS.

- [ ] **Step 8: Run related package tests**

Run:

```bash
go test ./internal/cliui ./internal/gladecli -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit check progress**

Run:

```bash
git add internal/gladecli/cli.go internal/gladecli/cli_test.go
git commit -m "feat: add progress phases to check"
```

Expected: commit succeeds.

---

## Task 8: Add `schema load` Progress

**Files:**
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Add schema progress tests**

Append near `TestRunSchemaLoad`:

```go
func TestRunSchemaLoadProgressWritesToStderr(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/objects/Thing__c/Thing__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Thing</label></CustomObject>`)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"schema", "load", "--project", root, "--progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "Thing__c") {
		t.Fatalf("stdout missing schema result: %q", stdout.String())
	}
	for _, want := range []string{"schema: Loading project", "schema: Loading metadata", "done: schema loaded"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}
```

- [ ] **Step 2: Run and verify failure**

Run:

```bash
go test ./internal/gladecli -run TestRunSchemaLoadProgressWritesToStderr -count=1
```

Expected: FAIL because `schema load` does not parse `--progress`.

- [ ] **Step 3: Pass `stderr` into `runSchema`**

In `Run()` change:

```go
if err := runSchema(ctx, args[1:], stdout); err != nil {
```

to:

```go
if err := runSchema(ctx, args[1:], stdout, stderr); err != nil {
```

Change the function signature:

```go
func runSchema(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error
```

- [ ] **Step 4: Parse progress flags for schema**

Inside `runSchema`, replace:

```go
root, jsonOut, err := parseProjectFlags(args[1:])
```

with:

```go
root, jsonOut, progressMode, err := parseProjectProgressFlags(args[1:])
```

Then insert before `project.Load(root)`:

```go
renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: progressMode})
renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "schema", Label: "Loading project"})
```

Insert before `gladeschema.LoadProject(p)`:

```go
renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "schema", Label: "Loading metadata", Current: 1, Total: 2})
```

Insert after successful metadata load:

```go
renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: "schema", Label: "Metadata loaded", Current: 2, Total: 2})
renderer.Finish(cliui.Result{OK: true, Label: "schema loaded"})
```

On each hard error before return, call:

```go
renderer.Finish(cliui.Result{OK: false, Label: "schema failed"})
```

- [ ] **Step 5: Run schema tests**

Run:

```bash
go test ./internal/gladecli -run 'TestRunSchemaLoad' -count=1
```

Expected: PASS.

- [ ] **Step 6: Run related package tests**

Run:

```bash
go test ./internal/cliui ./internal/gladecli -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit schema progress**

Run:

```bash
git add internal/gladecli/cli.go internal/gladecli/cli_test.go
git commit -m "feat: add progress phases to schema load"
```

Expected: commit succeeds.

---

## Task 9: Make Interactive Progress The Default

**Files:**
- Modify: `internal/gladecli/test_command.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Add opt-out tests**

Append to `internal/gladecli/cli_test.go`:

```go
func TestRunCheckNoProgressSuppressesProgress(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/classes/Hello.cls"), "public class Hello {}")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"check", "--project", root, "--no-progress"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
```

- [ ] **Step 2: Add auto-mode behavior**

In `parseProjectProgressFlags`, initialize:

```go
progressMode = cliui.ProgressAuto
```

instead of `ProgressOff`.

In `runTest`, initialize:

```go
progressMode := cliui.ProgressAuto
```

instead of `ProgressOff`.

Keep explicit `--no-progress` and `--quiet` as `ProgressOff`.

- [ ] **Step 3: Preserve non-TTY silence unless explicitly requested**

Update `NewRenderer` in `internal/cliui/renderer.go`:

```go
case ProgressAuto, "":
	if IsTerminalWriter(opts.Stderr) {
		return NewTTYRenderer(opts.Stderr, opts.Clock)
	}
	return NullRenderer{}
```

The older line-renderer behavior remains available through `--progress`. This matches the design: progress is default for real terminals, quiet for pipes unless requested.

- [ ] **Step 4: Run focused tests**

Run:

```bash
go test ./internal/cliui ./internal/gladecli -run 'TestRun(CheckNoProgress|CheckProgress|TestProgress|SchemaLoadProgress)|TestNewRenderer' -count=1
```

Expected: PASS. Existing explicit `--progress` tests still write to byte buffers.

- [ ] **Step 5: Update help text**

In `printTestHelp`, replace:

```text
  --progress                Print bounded progress to stderr.
```

with:

```text
  --progress                Print line progress to stderr, even when not attached to a terminal.
  --progress-json           Print NDJSON progress events to stderr.
  --no-progress, --quiet    Disable terminal progress.
```

In `printHelp`, add no global flags. Keep command-specific help simple until `check` and `schema` get their own help topics.

- [ ] **Step 6: Commit auto progress**

Run:

```bash
git add internal/cliui/renderer.go internal/gladecli/test_command.go internal/gladecli/cli.go internal/gladecli/cli_test.go
git commit -m "feat: enable terminal progress by default"
```

Expected: commit succeeds.

---

## Task 10: Polish The TTY Surface

**Files:**
- Modify: `internal/cliui/tty_renderer.go`
- Modify: `internal/cliui/progressbar.go`
- Modify: `internal/cliui/renderer_test.go`

- [ ] **Step 1: Add polish tests for width and truncation**

Append to `internal/cliui/renderer_test.go`:

```go
func TestTTYRendererTruncatesLongLabels(t *testing.T) {
	var out bytes.Buffer
	r := NewTTYRenderer(&out, fixedClock(time.Unix(30, 0).UTC()))
	r.SetWidthForTest(50)
	r.Render(Event{
		Kind:    EventPhaseTick,
		Phase:   "test",
		Label:   "Running VeryLongAccountDomainServiceTestName.withAnEquallyLongMethodName",
		Current: 1,
		Total:   2,
	})
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if len(stripANSI(line)) > 50 {
			t.Fatalf("line too wide (%d): %q", len(stripANSI(line)), line)
		}
	}
}

func stripANSI(s string) string {
	replacer := strings.NewReplacer("\r", "", "\x1b[K", "", "\x1b[1A", "")
	return replacer.Replace(s)
}
```

- [ ] **Step 2: Add width control and truncation**

In `TTYRenderer`, add:

```go
	width int
```

Add this method:

```go
func (r *TTYRenderer) SetWidthForTest(width int) {
	r.width = width
}
```

Add helpers:

```go
func (r *TTYRenderer) terminalWidth() int {
	if r.width > 0 {
		return r.width
	}
	return 80
}

func truncateCell(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}
```

Keep the truncation marker ASCII. This repo does not need a Unicode ellipsis for progress output.

In `renderLines`, after building each line, truncate to `terminalWidth()`.

- [ ] **Step 3: Improve final markers with color-gated glyphs**

Add to `TTYRenderer`:

```go
	color bool
```

Set it in `NewTTYRenderer`:

```go
color: ColorEnabled(IsTerminalWriter(w), os.Getenv("NO_COLOR")),
```

Use ASCII markers by default in tests. If `color` is true, wrap failure labels with `\x1b[31m...\x1b[0m` and success labels with `\x1b[32m...\x1b[0m`. Do not color the whole line; color only the marker or first word.

- [ ] **Step 4: Run cliui tests**

Run:

```bash
go test ./internal/cliui -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit polish**

Run:

```bash
git add internal/cliui/tty_renderer.go internal/cliui/progressbar.go internal/cliui/renderer_test.go
git commit -m "feat: polish cli progress tty output"
```

Expected: commit succeeds.

---

## Task 11: Full Verification

**Files:**
- No code edits unless verification exposes a defect.

- [ ] **Step 1: Run focused package tests**

Run:

```bash
go test ./internal/cliui ./internal/gladecli ./internal/apextest ./internal/testreport -count=1
```

Expected: PASS.

- [ ] **Step 2: Run command smoke checks**

Run:

```bash
go run ./cmd/glade test --project internal/gladecli/testdata/simple --progress
go run ./cmd/glade check --project internal/gladecli/testdata/simple --progress
go run ./cmd/glade schema load --project internal/gladecli/testdata/simple --progress
```

Expected: each command writes progress to `stderr` and final command output to `stdout`. If `internal/gladecli/testdata/simple` does not exist in the checkout, create a temporary Salesforce project in `/tmp/glade-cliui-smoke` with `sfdx-project.json`, one Apex class, one Apex test, and one custom object XML.

- [ ] **Step 3: Verify JSON channel separation**

Run:

```bash
tmp="$(mktemp -d)"
cat > "$tmp/sfdx-project.json" <<'JSON'
{"packageDirectories":[{"path":"force-app","default":true}]}
JSON
mkdir -p "$tmp/force-app/main/classes"
cat > "$tmp/force-app/main/classes/Hello.cls" <<'APEX'
public class Hello { public void run() {} }
APEX
go run ./cmd/glade check --project "$tmp" --json --progress-json > "$tmp/stdout.json" 2> "$tmp/progress.ndjson"
jq type "$tmp/stdout.json"
awk 'NF { print }' "$tmp/progress.ndjson" | while read -r line; do echo "$line" | jq type >/dev/null; done
```

Expected: `jq type "$tmp/stdout.json"` prints `"object"`, and every progress line parses as JSON.

- [ ] **Step 4: Run smoke script if the checkout is clean enough**

Run:

```bash
scripts/smoke.sh
```

Expected: PASS. If it fails from unrelated dirty worktree changes, record the exact failing command and rerun the focused tests from Step 1.

- [ ] **Step 5: Final review**

Run:

```bash
git diff --stat HEAD~10..HEAD
git diff --check
```

Expected: no whitespace errors. Diff should show progress code under `internal/cliui` and command wiring under `internal/gladecli`, with no maintenance-tool code added to `glade`.

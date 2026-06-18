# CLI Output Consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Glade CLI output consistent, quiet by default, bounded for large human-readable lists, and visibly active during background work.

**Architecture:** Keep the existing `internal/cliui` renderer as the one progress system. All command progress goes to stderr. Stdout stays for final results, requested machine data, or explicit raw output. Human text lists get a shared output budget, with complete data available through JSON, file output, or an explicit full mode.

**Tech Stack:** Go, `internal/gladecli`, `internal/cliui`, `internal/testreport`, focused `go test ./internal/gladecli ./internal/cliui ./internal/testreport`.

---

## Live Audit

### Already consistent enough to build on

- `internal/cliui` has TTY, line, and NDJSON renderers in `internal/cliui/renderer.go`, `internal/cliui/tty_renderer.go`, `internal/cliui/line_renderer.go`, and `internal/cliui/ndjson_renderer.go`.
- `glade test` has the best model. It buffers progress, throttles renders, sends progress to stderr, and keeps final test output on stdout. See `internal/gladecli/test_command.go:960`.
- Test result output already caps long pass listings at 80 cases. See `internal/testreport/reporters.go:27`.
- `parse`, `schema load`, `check`, `package build`, `test`, and `db seed` already emit progress events.

### Inconsistencies to fix

- `--progress` means "line renderer" today. In a TTY it should show the progress bar. Current code returns `ProgressLine` in `internal/gladecli/cli.go:83` and `internal/gladecli/test_command.go:997`.
- `db seed` parses progress flags by hand instead of using the shared helper. See `internal/gladecli/db_command.go:43`.
- `glade test --services` writes a status line straight to `progressW`, outside the renderer and outside `--no-progress`. See `internal/gladecli/test_command.go:290`.
- `glade dev test` calls `runTest` with `progressW == nil`, so it loses the progress behavior users see in `glade test`. See `internal/gladecli/dev_command.go:226`.
- `inspect symbols`, `schema load`, `db inspect`, `refactor rename`, `exec`, `dev vf`, `dev lwc`, and playground/server startup can print long lists with no shared cap. Examples: symbols at `internal/gladecli/cli.go:1041`, schema objects at `internal/gladecli/cli.go:1627`, debug output at `internal/gladecli/cli.go:2148`, db objects at `internal/gladecli/db_command.go:303`, refactor edits at `internal/gladecli/refactor_command.go:196`, and LWC preview routes at `internal/gladecli/dev_lwc_command.go:424`.
- `inspect symbols`, `inspect graph`, `inspect definition`, `inspect references`, `refactor rename`, `debug explain`, `debug repro`, `profile analyze`, `dev status`, `dev vf`, `dev lwc`, `server`, `playground`, and plugin registry/install paths do background project loads, index builds, network fetches, or file writes with no progress. Examples: `internal/gladecli/cli.go:1007`, `internal/gladecli/refactor_command.go:61`, `internal/gladecli/dev_lwc_command.go:34`, `internal/gladecli/plugins_command.go:267`, and `internal/gladecli/server_command.go:94`.
- Long-running server-style commands mix startup summary, warning lines, watch lines, and duplicate status lines on stdout. See `internal/gladecli/dev_vf_command.go:83`, `internal/gladecli/dev_lwc_command.go:56`, `internal/gladecli/server_command.go:126`, and `internal/gladecli/server_command.go:360`.
- Machine output is not shaped the same everywhere. Some commands use `writeCLIJSONEnvelope`; others emit raw structs with `json.NewEncoder`. That is acceptable for stable old surfaces, but new output work should not make the split worse.

### Latest landed work considered

Commit `b6c8ccd3` landed the priority LWC project support lane. It changes this plan in one place: `glade dev lwc`.

- `internal/gladecli/dev_lwc_command.go` now accepts `--flow` and `--flow-input`, resolves selected context and URL, and writes `selectedUrl`, `selectedContext`, and complete `routes` to the ready file.
- LWC startup can now discover generated routes for component previews, URL-addressable targets, app pages, record pages, home pages, tabs, record actions, utility-bar contexts, Flow screens, Flow actions, and Community pages.
- Do not rebuild that routing work in this plan. Use it. The output work should add startup progress, keep the selected URL visible, cap the stdout route list, keep the complete route list in the ready file and `/lightning/local/context.json`, and move transient warnings/progress to stderr.

## Output Contract

Implement these rules first. Then apply them command by command.

- Stdout: final human summary, requested JSON/SARIF/GitHub output, explicit raw logs, or generated source.
- Stderr: progress, transient warnings, server watch reload notices, and background status.
- Default progress: TTY progress bar when attached to a terminal; no progress when redirected.
- `--progress`: force visible human progress. TTY gets the progress bar. Non-TTY gets line progress.
- `--progress-json`: force NDJSON progress on stderr.
- `--no-progress` and `--quiet`: no progress or transient notices.
- `--json`: disable progress by default. Allow `--progress-json` for tools that want both JSON stdout and event stderr.
- Human list budget: show 80 rows by default. Print a final omitted-count line and point to `--json`, `--output`, or `--full` for complete data.
- Raw firehose output must require an explicit raw flag, `--output -`, or an existing command whose job is raw output.

## Task 1: Centralize Progress Semantics

**Files:**
- Modify: `internal/cliui/renderer.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/test_command.go`
- Modify: `internal/gladecli/db_command.go`
- Modify: `internal/cliui/help.go`
- Test: `internal/cliui/renderer_test.go`
- Test: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Write renderer tests for forced visible progress**

Add this test to `internal/cliui/renderer_test.go`:

```go
func TestNewRendererVisibleUsesLineRendererWhenNotTTY(t *testing.T) {
	var out bytes.Buffer
	r := NewRenderer(RendererOptions{Stderr: &out, Mode: ProgressVisible})
	if _, ok := r.(*LineRenderer); !ok {
		t.Fatalf("renderer = %T, want *LineRenderer", r)
	}
}
```

- [ ] **Step 2: Add `ProgressVisible`**

Change `internal/cliui/renderer.go`:

```go
const (
	ProgressAuto    ProgressMode = "auto"
	ProgressTTY     ProgressMode = "tty"
	ProgressVisible ProgressMode = "visible"
	ProgressLine    ProgressMode = "line"
	ProgressJSON    ProgressMode = "json"
	ProgressOff     ProgressMode = "off"
)
```

Add this case in `NewRenderer`:

```go
case ProgressVisible:
	if IsTerminalWriter(opts.Stderr) {
		return NewTTYRenderer(opts.Stderr, opts.Clock)
	}
	return NewLineRenderer(opts.Stderr, opts.Clock)
```

- [ ] **Step 3: Change shared flag mapping**

Change `internal/gladecli/cli.go`:

```go
func progressModeForFlags(jsonOut, progress, progressJSON, noProgress bool) cliui.ProgressMode {
	switch {
	case progressJSON:
		return cliui.ProgressJSON
	case progress:
		return cliui.ProgressVisible
	case noProgress || jsonOut:
		return cliui.ProgressOff
	default:
		return cliui.ProgressAuto
	}
}
```

- [ ] **Step 4: Remove the test-only progress parser drift**

Change `progressModeFromArgs` in `internal/gladecli/test_command.go`:

```go
func progressModeFromArgs(args []string, fallback cliui.ProgressMode) cliui.ProgressMode {
	progress := false
	progressJSON := false
	noProgress := false
	for _, arg := range args {
		name := arg
		if before, _, ok := strings.Cut(arg, "="); ok {
			name = before
		}
		switch name {
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet", "-q":
			noProgress = true
		}
	}
	if !progress && !progressJSON && !noProgress {
		return fallback
	}
	return progressModeForFlags(false, progress, progressJSON, noProgress)
}
```

- [ ] **Step 5: Make `db seed` use the shared helper**

Replace the manual progress state in `internal/gladecli/db_command.go` with booleans:

```go
progress := false
progressJSON := false
noProgress := false
```

Set those booleans in the flag switch. After parsing, compute:

```go
progressMode := progressModeForFlags(jsonOut, progress, progressJSON, noProgress)
```

Keep the existing JSON default behavior through the helper.

- [ ] **Step 6: Fix help text**

In `internal/cliui/help.go` and `internal/gladecli/test_command.go`, change progress descriptions to:

```text
--progress                Show progress on stderr; uses a progress bar on TTY and line output when redirected.
--progress-json           Print NDJSON progress events to stderr.
--no-progress, --quiet    Disable progress.
```

- [ ] **Step 7: Run focused tests**

Run:

```bash
go test ./internal/cliui ./internal/gladecli
```

Expected: pass.

## Task 2: Add a Shared Output Budget

**Files:**
- Create: `internal/cliui/output_budget.go`
- Create: `internal/cliui/output_budget_test.go`
- Modify: `internal/testreport/reporters.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/db_command.go`
- Modify: `internal/gladecli/refactor_command.go`
- Test: `internal/cliui/output_budget_test.go`
- Test: `internal/gladecli/cli_test.go`
- Test: `internal/testreport/reporters_test.go`

- [ ] **Step 1: Write budget tests**

Create `internal/cliui/output_budget_test.go`:

```go
package cliui

import (
	"bytes"
	"strings"
	"testing"
)

func TestOutputBudgetCapsRows(t *testing.T) {
	budget := DefaultOutputBudget()
	if got := budget.VisibleRows(100); got != DefaultDetailLimit {
		t.Fatalf("visible rows = %d, want %d", got, DefaultDetailLimit)
	}
	if got := budget.HiddenRows(100); got != 20 {
		t.Fatalf("hidden rows = %d, want 20", got)
	}
}

func TestOutputBudgetFullShowsAllRows(t *testing.T) {
	budget := DefaultOutputBudget()
	budget.Full = true
	if got := budget.VisibleRows(100); got != 100 {
		t.Fatalf("visible rows = %d, want 100", got)
	}
}

func TestWriteOmittedLine(t *testing.T) {
	var out bytes.Buffer
	if err := WriteOmittedLine(&out, 3, "symbols", "glade inspect symbols --json"); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "3 more symbols omitted") || !strings.Contains(got, "glade inspect symbols --json") {
		t.Fatalf("omitted line = %q", got)
	}
}
```

- [ ] **Step 2: Add the helper**

Create `internal/cliui/output_budget.go`:

```go
package cliui

import (
	"fmt"
	"io"
)

const DefaultDetailLimit = 80

type OutputBudget struct {
	DetailLimit int
	Full        bool
}

func DefaultOutputBudget() OutputBudget {
	return OutputBudget{DetailLimit: DefaultDetailLimit}
}

func (b OutputBudget) limit() int {
	if b.DetailLimit <= 0 {
		return DefaultDetailLimit
	}
	return b.DetailLimit
}

func (b OutputBudget) VisibleRows(total int) int {
	if total <= 0 {
		return 0
	}
	if b.Full {
		return total
	}
	limit := b.limit()
	if total < limit {
		return total
	}
	return limit
}

func (b OutputBudget) HiddenRows(total int) int {
	visible := b.VisibleRows(total)
	if total > visible {
		return total - visible
	}
	return 0
}

func WriteOmittedLine(w io.Writer, hidden int, noun string, completeCommand string) error {
	if hidden <= 0 {
		return nil
	}
	_, err := fmt.Fprintf(w, "  ... %d more %s omitted. Use `%s` for complete output.\n", hidden, noun, completeCommand)
	return err
}
```

- [ ] **Step 3: Reuse the budget in test output**

In `internal/testreport/reporters.go`, replace `consoleDetailLimit` with:

```go
const consoleDetailLimit = cliui.DefaultDetailLimit
```

- [ ] **Step 4: Cap `inspect symbols` human output**

Add `--full` to `parseInspectSymbolsFlags`. Return a `cliui.OutputBudget`.

Apply it when writing `index.Types`, triggers, and objects. Count the filtered rows first. Then print only the visible rows and one omitted line:

```go
type symbolOutputRow struct {
	kind string
	name string
	file string
}

rows := make([]symbolOutputRow, 0, len(index.Types)+len(index.Triggers)+len(index.Objects))
for _, typ := range index.Types {
	if kindFilter != "" && kindFilter != string(typ.Kind) {
		continue
	}
	file := typ.File
	if !fullPaths {
		file = cliui.ProjectRelativePath(index.Project.Root, file)
	}
	rows = append(rows, symbolOutputRow{kind: string(typ.Kind), name: typ.Name, file: file})
}
for _, trigger := range index.Triggers {
	if kindFilter != "" && kindFilter != "trigger" {
		continue
	}
	file := trigger.File
	if !fullPaths {
		file = cliui.ProjectRelativePath(index.Project.Root, file)
	}
	rows = append(rows, symbolOutputRow{kind: "trigger", name: trigger.Name, file: file})
}
for _, object := range index.Objects {
	if kindFilter != "" && kindFilter != "object" && kindFilter != "sobject" {
		continue
	}
	rows = append(rows, symbolOutputRow{kind: "object", name: object.Name, file: "local schema"})
}
budget := cliui.DefaultOutputBudget()
budget.Full = full
visible := budget.VisibleRows(len(rows))
for _, row := range rows[:visible] {
	fmt.Fprintf(w, "  %-8s %-16s %s\n", row.kind, row.name, row.file)
}
_ = cliui.WriteOmittedLine(w, budget.HiddenRows(len(rows)), "symbols", "glade inspect symbols --project . --json")
```

- [ ] **Step 5: Cap `schema load` object list**

In `runSchema`, print the summary counts as now. Cap the `Objects:` list to 80 rows and write:

```text
  ... 17 more objects omitted. Use `glade schema load --project . --json` for complete output.
```

- [ ] **Step 6: Cap `db inspect` object counts**

In `writeDBInspect`, cap `summary.ByObject` rows to 80. Keep totals untouched.

- [ ] **Step 7: Cap `refactor rename` edits**

Add `--full` to `parseRefactorRenameFlags`. In `writeRefactorRenameText`, print only the first 80 edits unless `--full` is set. Keep JSON complete.

- [ ] **Step 8: Cap `exec` debug lines**

Add `DebugLineLimit int` to an `execTextOptions` struct and use `cliui.DefaultDetailLimit`.

Change `writeExecSummary` so it prints the first 80 `USER_DEBUG` lines, then:

```text
  ... 25 more debug lines omitted. See the debug log path below for complete output.
```

Do not cap `--debug-log raw`, `--debug-log -`, JSON, or written log files.

- [ ] **Step 9: Run focused tests**

Run:

```bash
go test ./internal/cliui ./internal/testreport ./internal/gladecli
```

Expected: pass.

## Task 3: Add Progress to Project-Index Commands

**Files:**
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/refactor_command.go`
- Modify: `internal/gladecli/debug_command.go`
- Modify: `internal/gladecli/enterprise_report_command.go`
- Modify: `internal/cliui/help.go`
- Test: `internal/gladecli/cli_test.go`
- Test: `internal/gladecli/refactor_command_test.go`

- [ ] **Step 1: Add a project-index progress helper**

Add this helper near `loadProjectIndex` in `internal/gladecli/cli.go`:

```go
func loadProjectIndexWithProgress(root, phase string, renderer cliui.Renderer) (project.Project, typesys.Index, error) {
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: phase, Label: "Loading project"})
	p, err := project.Load(root)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: phase + " failed"})
		return project.Project{}, typesys.Index{}, err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: phase, Label: "Loading metadata", Current: 1, Total: 3})
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		index := typesys.Build(p, gladeschema.Schema{})
		index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESCHEMA001",
			Message:  fmt.Sprintf("metadata schema load failed: %v", err),
		})
		renderer.Render(cliui.Event{Kind: cliui.EventWarn, Phase: phase, Label: "metadata load failed", Detail: err.Error(), Current: 1, Total: 3})
		return p, index, nil
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: phase, Label: "Indexing Apex symbols", Current: 2, Total: 3})
	index := typesys.Build(p, s)
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: phase, Label: "Index ready", Current: 3, Total: 3})
	return p, index, nil
}
```

- [ ] **Step 2: Add progress flags to inspect subcommands**

Add `--progress`, `--progress-json`, `--no-progress`, and `--quiet` to:

- `glade inspect symbols`
- `glade inspect graph`
- `glade inspect definition`
- `glade inspect references`

Use stderr from the top-level call. This means changing `runInspect(ctx, args, stdout)` to `runInspect(ctx, args, stdout, stderr)` and passing `stderr` from `Run`.

- [ ] **Step 3: Use the helper in `inspect symbols`**

Replace the direct `project.Load`, `gladeschema.LoadProject`, and `typesys.Build` sequence at `internal/gladecli/cli.go:1007` with `loadProjectIndexWithProgress`.

Finish the renderer after output preparation:

```go
renderer.Finish(cliui.Result{OK: !index.HasErrors(), Label: "inspect complete", ExitCode: exitCodeForOK(!index.HasErrors())})
```

- [ ] **Step 4: Add progress to definition and references**

Use the same helper before resolving symbols. Add phase names:

- `inspect definition`
- `inspect references`

Keep JSON stdout clean.

- [ ] **Step 5: Add progress to `refactor rename`**

Change `runRefactor(args, stdout)` to `runRefactor(args, stdout, stderr)` and add progress flags to `rename`. Use `loadProjectIndexWithProgress(flags.root, "refactor", renderer)`.

- [ ] **Step 6: Add progress to debug and profile analysis**

For `debug profile`, `debug explain`, `debug repro`, and `profile analyze`, add progress phases:

- Read log or trace.
- Load project/index when needed.
- Analyze or annotate.
- Write output.

Default progress should be auto. `--json` should turn it off unless `--progress-json` is set.

- [ ] **Step 7: Add progress to enterprise reports**

For `report assess`, `report cruft`, `report refactor-proof`, and `inspect graph`, emit coarse phases:

- Load context.
- Build graph.
- Analyze report.
- Write artifact.

Keep report JSON/HTML/Markdown on stdout or `--out` only.

- [ ] **Step 8: Run focused tests**

Run:

```bash
go test ./internal/gladecli
```

Expected: pass.

## Task 4: Carry Test Progress into `glade dev`

**Files:**
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/dev_command.go`
- Modify: `internal/gladecli/test_command.go`
- Test: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Thread stderr through `dev`**

Change the top-level call in `internal/gladecli/cli.go`:

```go
result, ranTests, err := runDev(ctx, args[1:], stdout, stderr)
```

Change the signature:

```go
func runDev(ctx context.Context, args []string, w io.Writer, progressW io.Writer) (testreport.Run, bool, error)
```

- [ ] **Step 2: Pass progress into `dev test`**

Change `runDevTest`:

```go
func runDevTest(ctx context.Context, args []string, w io.Writer, progressW io.Writer) (testreport.Run, error)
```

Then replace:

```go
result, err := runTest(ctx, testArgs, io.Discard, nil)
```

with:

```go
result, err := runTest(ctx, testArgs, io.Discard, progressW)
```

- [ ] **Step 3: Add progress flags to dev test and watch**

Allow `--progress`, `--progress-json`, `--no-progress`, and `--quiet` on `glade dev test` and `glade dev watch`. Forward them to `runTest`.

- [ ] **Step 4: Keep final dev output clean**

Keep `testreport.WriteConsoleWithOptions` as the only final stdout test summary. Progress stays on stderr.

- [ ] **Step 5: Test it**

Add a focused test that runs:

```go
code := Run(context.Background(), []string{"dev", "test", "--project", root, "--progress"}, &stdout, &stderr)
```

Assert stdout contains `Glade test` and stderr contains `test`.

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/gladecli -run 'TestRunDev|TestRunTestProgress'
```

Expected: pass.

## Task 5: Add Startup Progress to Servers and Previews

**Files:**
- Modify: `internal/gladecli/dev_vf_command.go`
- Modify: `internal/gladecli/dev_lwc_command.go`
- Modify: `internal/gladecli/server_command.go`
- Modify: `internal/gladecli/org_command.go`
- Modify: `internal/cliui/help.go`
- Reference: `internal/lwcshell/context_preset.go`
- Reference: `internal/lwcshell/workbench.go`
- Reference: `internal/server/lwc_shell.go`
- Reference: `internal/server/lwc_shell_assets.go`
- Test: `internal/gladecli/dev_vf_command_test.go`
- Test: `internal/gladecli/dev_lwc_command_test.go`
- Test: `internal/gladecli/org_command_test.go`

- [ ] **Step 1: Thread progress writers**

Change signatures so server-like commands can write progress to stderr:

```go
func runServer(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error
func runPlayground(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error
func runDevVF(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error
func runDevLWC(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error
```

- [ ] **Step 2: Add progress flags**

Add `--progress`, `--progress-json`, `--no-progress`, and `--quiet` to:

- `glade server`
- `glade playground`
- `glade dev vf`
- `glade dev lwc`
- `glade org start`

- [ ] **Step 3: Emit startup phases**

Use these phase names:

- `server`: opening database, loading project, indexing symbols, starting listener.
- `playground`: opening workspace, preparing database, starting workbench.
- `dev vf`: loading project, indexing symbols, applying data fixtures, starting server.
- `dev lwc`: loading project, loading LWC context presets, resolving selected context, indexing symbols, applying data fixtures, building route/workbench metadata, starting shell.

For `dev lwc`, do not generate route behavior twice. The landed LWC shell path already owns Flow, Community, utility-bar, record, app, home, tab, action, URL-addressable, and component routes. Count and summarize the routes after `devLWCPreviewRoutes` has the data.

- [ ] **Step 4: Move transient warnings to stderr**

The toolchain warning lines currently go to stdout. Move them through the renderer as warning events or write them to `progressW` when progress is enabled. Do this in `dev vf` and `dev lwc`.

- [ ] **Step 5: Keep startup summaries on stdout**

After progress finishes, print one stable summary on stdout. Remove duplicate machine-ish lines:

- `glade server: <url>`
- `glade playground: <url>`
- duplicate `state: memory-only`

For `dev lwc`, stdout should keep the selected route visible and cap the rest:

```text
LWC dev shell: http://127.0.0.1:7357/
Selected route: http://127.0.0.1:7357/lightning/c/MyComponent
Routes:
  - http://127.0.0.1:7357/lightning/c/MyComponent
  - http://127.0.0.1:7357/lightning/r/Account/001000000000001AAA/view
  ... 17 more routes omitted. See the ready file or /lightning/local/context.json for complete output.
Watching LWC sources. Press Ctrl-C to stop.
```

- [ ] **Step 6: Keep watch reload lines considerate**

For live reload notices, write one concise line per relevant change group to stderr:

```text
reload: Visualforce metadata updated (3 changes)
reload: failed: <error>
```

Do not print per-file chatter unless a future `--verbose` flag exists.

- [ ] **Step 7: Test startup output**

Use existing ready-file tests where possible. Assert:

- stdout contains the final URL summary.
- stderr contains progress when `--progress` is passed.
- `--no-progress` suppresses progress and reload setup chatter.
- `glade dev lwc --flow Example_Flow --flow-input stage=join --ready-file <path> --progress` keeps progress on stderr, keeps stdout bounded, and writes `selectedUrl`, `selectedContext`, and complete `routes` to the ready-file JSON.

- [ ] **Step 8: Run tests**

Run:

```bash
go test ./internal/gladecli -run 'TestRunDevVF|TestRunDevLWC|TestRunOrg|TestRunPlayground'
```

Expected: pass.

## Task 6: Add Progress to Plugin Operations

**Files:**
- Modify: `internal/gladecli/plugins_command.go`
- Modify: `internal/pluginhost/store.go`
- Modify: `internal/pluginhost/registry.go`
- Modify: `internal/cliui/help.go`
- Test: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Add plugin progress flags where work can block**

Support `--progress`, `--progress-json`, `--no-progress`, and `--quiet` on:

- `glade plugins install`
- `glade plugins restore`
- `glade plugins search`
- `glade plugins available`
- `glade plugins info`
- `glade plugins doctor`

- [ ] **Step 2: Emit coarse events without changing pluginhost first**

In `runPluginsInstall`, emit:

- resolving plugin target
- fetching registry, when registry install
- downloading archive, when archive or remote archive
- installing plugin
- validating manifest
- writing plugin state

If `pluginhost` cannot expose a finer event yet, keep the event coarse.

- [ ] **Step 3: Keep results on stdout**

Keep the final installed/restored/search result on stdout. Keep community/unlisted trust warnings on stderr.

- [ ] **Step 4: Consider pluginhost callbacks only if coarse events lie**

If `store.InstallRemoteArchive` blocks long enough that coarse events do not update during download, add:

```go
type InstallProgress struct {
	Phase   string
	Label   string
	Current int64
	Total   int64
}

type InstallOptions struct {
	Progress func(InstallProgress)
}
```

Do not expose plugin internals in base `glade` output.

- [ ] **Step 5: Test progress and JSON split**

Add tests for:

- `plugins install --progress` writes progress to stderr and final result to stdout.
- `plugins doctor --json --progress-json` keeps stdout valid JSON and stderr valid NDJSON.
- `plugins search --no-progress` prints no progress.

- [ ] **Step 6: Run tests**

Run:

```bash
go test ./internal/gladecli -run 'TestRunPlugins'
```

Expected: pass.

## Task 7: Normalize Human Result Formatting

**Files:**
- Modify: `internal/cliui/human.go`
- Modify: `internal/cliui/check_result.go`
- Modify: `internal/cliui/doctor.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/db_command.go`
- Modify: `internal/gladecli/plugins_command.go`
- Modify: `internal/gladecli/org_command.go`
- Test: `internal/cliui/*_test.go`
- Test: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Add common section helpers**

Extend `internal/cliui/human.go` with helpers:

```go
func WriteSection(w io.Writer, title string) error {
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, title+":")
	return err
}

func WriteKeyValue(w io.Writer, key string, value any) error {
	return WriteAlignedKV(w, key, fmt.Sprint(value))
}
```

Use existing alignment helpers where present.

- [ ] **Step 2: Normalize headings**

Use this shape for human summaries:

```text
Glade <command>

<one-line result>

Summary:
  Key  value

Artifacts:
  Kind  path

Next:
  command
```

Apply it first to `exec`, `schema load`, `db inspect`, `package build`, `refactor rename`, `plugins list`, and `org create`.

- [ ] **Step 3: Keep machine formats stable**

Do not change existing JSON field names unless a test already asserts the envelope. Human text gets polish first.

- [ ] **Step 4: Update snapshots/assertions**

Update only focused tests that assert exact strings. Prefer contains-style tests for layout where exact spacing is not the behavior under test.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/cliui ./internal/gladecli
```

Expected: pass.

## Task 8: Add a CLI Output Surface Guard

**Files:**
- Create: `internal/gladecli/output_contract_test.go`
- Modify: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Add JSON cleanliness tests**

Create table-driven tests for command pairs that support JSON plus progress:

```go
type jsonProgressCase struct {
	name string
	args []string
}
```

Cover:

- `check --json --progress-json`
- `test --json --progress-json`
- `schema load --json --progress-json`
- `package build --json --progress-json`
- `db seed --json --progress-json`

Assert stdout is valid JSON and stderr is valid line-delimited JSON.

- [ ] **Step 2: Add no-progress tests**

For each command with progress flags, run `--no-progress` and assert stderr is empty unless the command fails.

- [ ] **Step 3: Add bounded-output tests**

Create fixtures with more than 80:

- Apex classes for `inspect symbols`.
- Schema objects for `schema load`.
- DB object counts for `db inspect`.
- Refactor edits for `refactor rename`.
- `System.debug` lines for `exec`.
- LWC preview routes for `dev lwc`, with complete route data still present in the ready-file JSON.

Assert the human output includes an omitted-count line and JSON output remains complete.

- [ ] **Step 4: Run the guard**

Run:

```bash
go test ./internal/gladecli -run 'TestCLIOutputContract|TestRun(Check|Test|Schema|Package|DB|Inspect|Exec|Refactor)'
```

Expected: pass.

## Verification

Run these before calling the branch done:

```bash
go test ./internal/cliui ./internal/testreport ./internal/gladecli
go test ./internal/gladecli -run 'TestRunDevLWC|TestCLIOutputContract'
git diff --check
```

If the touched command signatures are broad, run:

```bash
go test ./...
```

Final manual smoke from a TTY:

```bash
glade test --project testdata/local-tests/basic --progress
glade check --project testdata/local-tests/basic --progress
glade inspect symbols --project testdata/local-tests/basic --progress
glade exec --project testdata/local-tests/basic "for (Integer i = 0; i < 100; i++) System.debug('line ' + i);"
```

Expected:

- TTY progress uses the animated bar.
- Redirected progress uses line events.
- JSON stdout stays clean.
- Long human output shows a cap and a way to get complete data.

## Commit Plan

- Commit 1: `fix(cli): centralize progress mode semantics`
- Commit 2: `feat(cli): add shared human output budget`
- Commit 3: `feat(cli): add progress to project indexing commands`
- Commit 4: `feat(cli): carry progress through dev workflows`
- Commit 5: `feat(cli): add progress to server and plugin startup work`
- Commit 6: `test(cli): guard output contract`

## Scope Guard

Do not add maintenance scanners or plugin-internal commands to base `glade`. Plugin install progress is product-facing and belongs here. Plugin implementation progress belongs in the plugin.

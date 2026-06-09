# PR Performance Impact Analysis & Regression Detection

> **Prerequisite:** The base scanner (Tasks 1-10 in `2026-06-09-salesforce-performance-scanner.md`) must be landed first. The scanner already produces `Report` with `Finding` entries, each having a location, score, category, and severity.

**Goal:** When a PR changes Apex code, tell the reviewer exactly what performance impact the change introduces — not a full-project scan, but a focused diff: new findings, removed findings, score changes, and downstream entry-point impact.

**Why this over ISV rules:** ISV package lifecycle rules (global API diff, deprecation cycles, version compatibility) are useful once per release. PR performance diff is useful on every push. A developer who gets a "this PR adds a SOQL-in-loop" comment on GitHub before the human reviewer even looks is the killer code review feature.

**Architecture:** Three components:

1. **Baseline snapshot** — Run the scanner on the base branch, produce a JSON report, commit it as a baseline artifact (or pass it via `--baseline`).
2. **PR diff engine** — Run the scanner on the PR branch, compare findings against the baseline, categorize each change as new/removed/changed.
3. **Entry-point impact propagation** — When a changed method is called from an entry point (trigger, invocable, batch, queueable, REST), flag the entry point as potentially affected even if the entry point file itself didn't change.

---

## File Structure

- Create `internal/perfscan/diff.go`: baseline loading, finding comparison, diff categories, PR report.
- Create `internal/perfscan/diff_test.go`: diff engine tests.
- Create `internal/perfscan/impact.go`: method-to-entry-point call graph, downstream impact.
- Create `internal/perfscan/impact_test.go`: impact analysis tests.
- Modify `internal/perfscan/analyze.go`: accept baseline path in Options.
- Modify `internal/gladecli/cli.go`: add `glade inspect performance --baseline <path>` and `glade inspect performance --diff-base <branch>`.
- Modify `internal/gladecli/cli_test.go`: CLI contract tests for diff mode.

---

## Task 1: Baseline Snapshot And Diff Engine

### Step 1: Write the diff test

Create `internal/perfscan/diff_test.go`:

```go
package perfscan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDiffReportIdentifiesNewRemovedAndChangedFindings(t *testing.T) {
	baseline := Report{SchemaVersion: SchemaVersion, Project: "/project"}
	baseline.AddFinding(Finding{
		ID: "perf.soql.loop", Category: CategorySOQL, Severity: SeverityHigh,
		Confidence: ConfidenceStatic, Score: 90,
		Message:  "SOQL inside a loop.",
		Location: Location{File: "AccountHandler.cls", Line: 15},
	})
	baseline.AddFinding(Finding{
		ID: "perf.dml.loop", Category: CategoryDML, Severity: SeverityMedium,
		Confidence: ConfidenceStatic, Score: 55,
		Message:  "DML inside a loop.",
		Location: Location{File: "ContactHandler.cls", Line: 22},
	})
	baseline.AddFinding(Finding{
		ID: "perf.soql.unfiltered", Category: CategorySOQL, Severity: SeverityHigh,
		Confidence: ConfidenceStatic, Score: 95,
		Message:  "Unfiltered SOQL.",
		Location: Location{File: "OldFile.cls", Line: 3},
	})
	baseline.Finalize()

	current := Report{SchemaVersion: SchemaVersion, Project: "/project"}
	current.AddFinding(Finding{
		ID: "perf.soql.loop", Category: CategorySOQL, Severity: SeverityHigh,
		Confidence: ConfidenceStatic, Score: 90,
		Message:  "SOQL inside a loop.",
		Location: Location{File: "AccountHandler.cls", Line: 15},
	})
	current.AddFinding(Finding{
		ID: "perf.async.loop", Category: CategoryAsync, Severity: SeverityHigh,
		Confidence: ConfidenceStatic, Score: 88,
		Message:  "Async enqueue inside a loop.",
		Location: Location{File: "BatchProcessor.cls", Line: 8},
	})
	current.AddFinding(Finding{
		ID: "perf.dml.loop", Category: CategoryDML, Severity: SeverityMedium,
		Confidence: ConfidenceStatic, Score: 75,
		Message:  "DML inside a loop (now in a nested context).",
		Location: Location{File: "ContactHandler.cls", Line: 30},
	})
	current.Finalize()

	diff := DiffReports(baseline, current)

	if len(diff.New) != 1 || diff.New[0].ID != "perf.async.loop" {
		t.Fatalf("new findings: %#v", diff.New)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].ID != "perf.soql.unfiltered" {
		t.Fatalf("removed findings: %#v", diff.Removed)
	}
	if len(diff.Changed) != 1 {
		t.Fatalf("changed findings: %#v", diff.Changed)
	}
	if diff.Changed[0].ScoreDelta != 20 {
		t.Fatalf("score delta: %d", diff.Changed[0].ScoreDelta)
	}
	if diff.Summary.TotalBefore != 3 || diff.Summary.TotalAfter != 3 {
		t.Fatalf("totals: before=%d after=%d", diff.Summary.TotalBefore, diff.Summary.TotalAfter)
	}

	data, err := json.Marshal(diff)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"new":1`, `"removed":1`, `"changed":1`, `"scoreDelta"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("json missing %q: %s", want, string(data))
		}
	}
}

func TestDiffReportSkipsSameFinding(t *testing.T) {
	baseline := Report{SchemaVersion: SchemaVersion, Project: "/project"}
	baseline.AddFinding(Finding{
		ID: "perf.soql.loop", Category: CategorySOQL, Severity: SeverityHigh,
		Confidence: ConfidenceStatic, Score: 90,
		Location: Location{File: "Handler.cls", Line: 10},
	})
	baseline.Finalize()

	current := Report{SchemaVersion: SchemaVersion, Project: "/project"}
	current.AddFinding(Finding{
		ID: "perf.soql.loop", Category: CategorySOQL, Severity: SeverityHigh,
		Confidence: ConfidenceStatic, Score: 90,
		Location: Location{File: "Handler.cls", Line: 10},
	})
	current.Finalize()

	diff := DiffReports(baseline, current)
	if len(diff.New) != 0 || len(diff.Removed) != 0 || len(diff.Changed) != 0 {
		t.Fatalf("expected no changes: %#v", diff)
	}
}
```

### Step 2: Run diff test and verify it fails

```bash
go test ./internal/perfscan -run TestDiffReport -count=1
```

Expected: FAIL because `DiffReports`, `DiffReport`, `ChangedFinding` do not exist.

### Step 3: Add the diff model and engine

Add to `internal/perfscan/model.go`:

```go
// DiffResult captures the delta between a baseline and current performance scan.
type DiffResult struct {
	SchemaVersion int              `json:"schemaVersion"`
	Project       string           `json:"project"`
	Summary       DiffSummary      `json:"summary"`
	New           []Finding        `json:"new,omitempty"`
	Removed       []Finding        `json:"removed,omitempty"`
	Changed       []ChangedFinding `json:"changed,omitempty"`
	Unchanged     int              `json:"unchanged"`
}

type DiffSummary struct {
	TotalBefore int `json:"totalBefore"`
	TotalAfter  int `json:"totalAfter"`
	New         int `json:"new"`
	Removed     int `json:"removed"`
	Changed     int `json:"changed"`
	HighBefore  int `json:"highBefore"`
	HighAfter   int `json:"highAfter"`
}

type ChangedFinding struct {
	Finding    Finding `json:"finding"`
	ScoreDelta int     `json:"scoreDelta"`
	WasHigh    bool    `json:"-"`
	IsHigh     bool    `json:"-"`
}
```

Create `internal/perfscan/diff.go`:

```go
package perfscan

import "sort"

// DiffReports compares a baseline report against a current report and returns
// the delta. A finding is considered "same" when its ID and Location match exactly.
// Score changes or severity changes on the same finding are reported as Changed.
func DiffReports(baseline, current Report) DiffResult {
	baseline.Finalize()
	current.Finalize()

	result := DiffResult{
		SchemaVersion: SchemaVersion,
		Project:       current.Project,
		Summary: DiffSummary{
			TotalBefore: baseline.Summary.Findings,
			TotalAfter:  current.Summary.Findings,
			HighBefore:  baseline.Summary.High,
			HighAfter:   current.Summary.High,
		},
	}

	// Index findings by key (ID + file + line)
	baselineIndex := buildFindingIndex(baseline.Findings)
	currentIndex := buildFindingIndex(current.Findings)

	for key, baseFinding := range baselineIndex {
		if currentFinding, ok := currentIndex[key]; ok {
			if currentFinding.Score != baseFinding.Score || currentFinding.Severity != baseFinding.Severity {
				changed := ChangedFinding{
					Finding:    currentFinding,
					ScoreDelta: currentFinding.Score - baseFinding.Score,
					WasHigh:    baseFinding.Severity == SeverityHigh,
					IsHigh:     currentFinding.Severity == SeverityHigh,
				}
				result.Changed = append(result.Changed, changed)
			}
		} else {
			result.Removed = append(result.Removed, baseFinding)
		}
	}

	for key, currentFinding := range currentIndex {
		if _, ok := baselineIndex[key]; !ok {
			result.New = append(result.New, currentFinding)
		}
	}

	result.Unchanged = len(baseline.Findings) - len(result.Removed) - len(result.Changed)
	result.Summary.New = len(result.New)
	result.Summary.Removed = len(result.Removed)
	result.Summary.Changed = len(result.Changed)

	// Sort new findings by score (highest first)
	sort.Slice(result.New, func(i, j int) bool {
		if result.New[i].Score != result.New[j].Score {
			return result.New[i].Score > result.New[j].Score
		}
		return result.New[i].ID < result.New[j].ID
	})

	return result
}

type findingKey struct {
	ID   string
	File string
	Line int
}

func buildFindingIndex(findings []Finding) map[findingKey]Finding {
	index := make(map[findingKey]Finding, len(findings))
	for _, f := range findings {
		key := findingKey{ID: f.ID, File: f.Location.File, Line: f.Location.Line}
		index[key] = f
	}
	return index
}
```

### Step 4: Add DiffResult JSON and Markdown writers

Add to `internal/perfscan/report.go`:

```go
func WriteDiffMarkdown(w io.Writer, diff DiffResult) error {
	if _, err := fmt.Fprintf(w, "# Performance Scan Diff\n\nProject: `%s`\n\n", diff.Project); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| | Before | After |\n|---|---|---|\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| Total findings | %d | %d |\n", diff.Summary.TotalBefore, diff.Summary.TotalAfter); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| High severity | %d | %d |\n", diff.Summary.HighBefore, diff.Summary.HighAfter); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if len(diff.New) > 0 {
		if _, err := fmt.Fprintf(w, "## New Findings (%d)\n\n", len(diff.New)); err != nil {
			return err
		}
		for _, f := range diff.New {
			if _, err := fmt.Fprintf(w, "- `%s` [%s] score=%d — %s\n", f.ID, f.Severity, f.Score, f.Message); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if len(diff.Changed) > 0 {
		if _, err := fmt.Fprintf(w, "## Changed Findings (%d)\n\n", len(diff.Changed)); err != nil {
			return err
		}
		for _, cf := range diff.Changed {
			delta := ""
			if cf.ScoreDelta > 0 {
				delta = fmt.Sprintf(" +%d", cf.ScoreDelta)
			} else if cf.ScoreDelta < 0 {
				delta = fmt.Sprintf(" %d", cf.ScoreDelta)
			}
			if _, err := fmt.Fprintf(w, "- `%s` [%s] score=%d (%s) — %s\n", cf.Finding.ID, cf.Finding.Severity, cf.Finding.Score, delta, cf.Finding.Message); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	if len(diff.Removed) > 0 {
		if _, err := fmt.Fprintf(w, "## Removed Findings (%d)\n\n", len(diff.Removed)); err != nil {
			return err
		}
		for _, f := range diff.Removed {
			if _, err := fmt.Fprintf(w, "- `%s` [%s] score=%d — %s\n", f.ID, f.Severity, f.Score, f.Message); err != nil {
				return err
			}
		}
	}

	if len(diff.New) == 0 && len(diff.Changed) == 0 && len(diff.Removed) == 0 {
		if _, err := fmt.Fprintln(w, "No performance changes detected."); err != nil {
			return err
		}
	}

	return nil
}
```

### Step 5: Run diff tests

```bash
gofmt -w internal/perfscan
go test ./internal/perfscan -run TestDiffReport -count=1
```

Expected: PASS.

---

## Task 2: Entry-Point Impact Propagation

This is what makes the diff useful: when a method changes, show which entry points (triggers, invocable, batch, queueable, REST, UI) are downstream.

### Step 1: Write the impact test

Create `internal/perfscan/impact_test.go`:

```go
package perfscan

import (
	"testing"
)

func TestImpactAnalyzerLinksChangedMethodsToEntryPoints(t *testing.T) {
	// Given: a trigger entry point calling AccountHandler.handle()
	// which calls AccountSelector.selectByIds()
	entryPoints := []EntryPoint{
		{Kind: EntryTrigger, Name: "AccountTrigger", File: "AccountTrigger.trigger", Line: 1},
		{Kind: EntryInvocable, Name: "AccountService.updateAccounts", File: "AccountService.cls", Line: 5},
	}
	// Both entry points call into AccountHandler
	callGraph := map[string][]string{
		"AccountTrigger.trigger:AccountHandler.handle":        {"AccountHandler.cls"},
		"AccountService.cls:AccountService.updateAccounts":    {"AccountHandler.cls"},
	}
	// The PR changed AccountHandler.cls
	changedFiles := []string{"AccountHandler.cls"}

	impact := AnalyzeEntryPointImpact(entryPoints, callGraph, changedFiles)

	if len(impact) != 2 {
		t.Fatalf("expected 2 affected entry points, got %d: %#v", len(impact), impact)
	}
	if impact[0].EntryPoint.Name != "AccountTrigger" {
		t.Fatalf("expected AccountTrigger first, got %s", impact[0].EntryPoint.Name)
	}
}

func TestImpactAnalyzerReturnsEmptyForUnchangedFiles(t *testing.T) {
	entryPoints := []EntryPoint{
		{Kind: EntryTrigger, Name: "AccountTrigger", File: "AccountTrigger.trigger"},
	}
	callGraph := map[string][]string{
		"AccountTrigger.trigger:AccountHandler.handle": {"AccountHandler.cls"},
	}

	impact := AnalyzeEntryPointImpact(entryPoints, callGraph, []string{"UnrelatedFile.cls"})
	if len(impact) != 0 {
		t.Fatalf("expected no impact: %#v", impact)
	}
}
```

### Step 2: Run impact test and verify it fails

```bash
go test ./internal/perfscan -run TestImpactAnalyzer -count=1
```

Expected: FAIL because `AnalyzeEntryPointImpact` does not exist.

### Step 3: Add the impact analyzer

Add to `internal/perfscan/model.go`:

```go
type EntryPointImpact struct {
	EntryPoint EntryPoint `json:"entryPoint"`
	ViaFiles   []string   `json:"viaFiles,omitempty"`
}
```

Create `internal/perfscan/impact.go`:

```go
package perfscan

// AnalyzeEntryPointImpact returns which entry points are downstream of changed files.
// Each entry point key is "file:name". The callGraph maps entry point keys to the
// set of files they transitively depend on.
func AnalyzeEntryPointImpact(entryPoints []EntryPoint, callGraph map[string][]string, changedFiles []string) []EntryPointImpact {
	if len(changedFiles) == 0 {
		return nil
	}

	changedSet := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changedSet[f] = true
	}

	var impacted []EntryPointImpact
	for _, ep := range entryPoints {
		key := ep.File + ":" + ep.Name
		deps, ok := callGraph[key]
		if !ok {
			continue
		}
		var viaFiles []string
		for _, dep := range deps {
			if changedSet[dep] {
				viaFiles = append(viaFiles, dep)
			}
		}
		if len(viaFiles) > 0 {
			impacted = append(impacted, EntryPointImpact{
				EntryPoint: ep,
				ViaFiles:   viaFiles,
			})
		}
	}
	return impacted
}
```

### Step 4: Build the call graph from the type index

The call graph tells us: "Entry point X calls method A in file Y." The type system already has method resolution. We need to build a reverse index: "file Y is called by these entry points."

Add to `internal/perfscan/impact.go`:

```go
// BuildEntryPointCallGraph constructs a mapping from entry point keys to the
// files they transitively call into. It uses the existing async call graph for
// Queueable/Batchable/Schedulable edges plus AST-level method call resolution
// for direct entry-point-to-handler edges.
//
// For the initial implementation, use AST-level method call analysis: scan
// entry-point files (triggers, invocable methods) for method calls, resolve
// the target class file via the typesys.Index, and record the dependency.
func BuildEntryPointCallGraph(report Report) map[string][]string {
	graph := make(map[string][]string)

	for _, ep := range report.EntryPoints {
		key := ep.File + ":" + ep.Name

		// For trigger entry points: the trigger file itself calls a handler class.
		// The handler class is typically in a separate file.
		// Walk the findings to see which files are referenced from this entry point.
		for _, f := range report.Findings {
			if f.EntryPoint.Name == ep.Name && f.EntryPoint.Kind == ep.Kind {
				if f.Location.File != "" && f.Location.File != ep.File {
					graph[key] = appendIfMissing(graph[key], f.Location.File)
				}
			}
		}
	}

	return graph
}

func appendIfMissing(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
```

This initial implementation uses the findings themselves to link entry points to dependent files. A deeper implementation would walk the AST call graph through `typesys.Index`, but the finding-based approach works without new type-system work and produces useful results immediately.

### Step 5: Run impact tests

```bash
gofmt -w internal/perfscan
go test ./internal/perfscan -run TestImpactAnalyzer -count=1
```

Expected: PASS.

---

## Task 3: Wire Diff Mode Into CLI

### Step 1: Write CLI diff test

Append to `internal/gladecli/cli_test.go`:

```go
func TestRunInspectPerformanceDiff(t *testing.T) {
	root := writePerformanceScanProject(t)
	// Run baseline scan and save it
	var stdout, stderr strings.Builder
	code := Run(context.Background(), []string{
		"inspect", "performance", "--project", root, "--json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("baseline scan: code=%d stderr=%s", code, stderr.String())
	}
	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(baselinePath, []byte(stdout.String()), 0644); err != nil {
		t.Fatal(err)
	}

	// Run diff against the baseline
	stdout.Reset()
	code = Run(context.Background(), []string{
		"inspect", "performance", "--project", root, "--json",
		"--baseline", baselinePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("diff scan: code=%d stderr=%s", code, stderr.String())
	}
	for _, want := range []string{`"schemaVersion"`, `"new"`, `"changed"`, `"removed"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("diff output missing %q:\n%s", want, stdout.String())
		}
	}
}
```

### Step 2: Run CLI test and verify it fails

```bash
go test ./internal/gladecli -run TestRunInspectPerformanceDiff -count=1
```

Expected: FAIL because `--baseline` flag is not recognized.

### Step 3: Add `--baseline` flag

In `internal/gladecli/cli.go`, modify `runInspectPerformance`:

```go
func runInspectPerformance(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := "."
	jsonOut := false
	tracePath := ""
	baselinePath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--trace":
			if i+1 >= len(args) {
				return errors.New("--trace requires a value")
			}
			tracePath = args[i+1]
			i++
		case "--baseline":
			if i+1 >= len(args) {
				return errors.New("--baseline requires a value")
			}
			baselinePath = args[i+1]
			i++
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	report, err := perfscan.AnalyzeProject(perfscan.Options{
		ProjectRoot:  root,
		TracePath:    tracePath,
		BaselinePath: baselinePath,
	})
	if err != nil {
		return err
	}
	if baselinePath != "" {
		diff := perfscan.DiffReports(report.Baseline, report)
		if jsonOut {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			return enc.Encode(diff)
		}
		return perfscan.WriteDiffMarkdown(w, diff)
	}
	if jsonOut {
		return perfscan.WriteJSON(w, report)
	}
	return perfscan.WriteMarkdown(w, report)
}
```

### Step 4: Add baseline loading to Options and analyze

In `internal/perfscan/analyze.go`:

```go
type Options struct {
	ProjectRoot  string
	TracePath    string
	BaselinePath string
	TopN         int
}
```

In `AnalyzeProject`, after producing the report, if `BaselinePath` is set, load the baseline:

```go
if options.BaselinePath != "" {
	data, err := os.ReadFile(options.BaselinePath)
	if err != nil {
		return report, fmt.Errorf("baseline: %w", err)
	}
	if err := json.Unmarshal(data, &report.Baseline); err != nil {
		return report, fmt.Errorf("baseline: invalid JSON: %w", err)
	}
}
```

Add `Baseline Report` (unexported) to the `Report` struct in `model.go`.

### Step 5: Run CLI diff tests

```bash
gofmt -w internal/gladecli internal/perfscan
go test ./internal/gladecli -run TestRunInspectPerformanceDiff -count=1
```

Expected: PASS.

---

## Task 4: CI-Friendly Output (GitHub Actions / GitLab CI)

### Step 1: Add `--ci` flag

When `--ci` is passed with `--baseline`, the output includes a GitHub Actions annotation or GitLab CI code quality artifact format:

```go
if ciMode {
    // Emit GitHub Actions workflow commands:
    // ::warning file=AccountHandler.cls,line=15::SOQL inside a loop (score: 90)
    for _, f := range diff.New {
        fmt.Fprintf(w, "::warning file=%s,line=%d::[%s] %s\n",
            filepath.Base(f.Location.File), f.Location.Line, f.Severity, f.Message)
    }
    for _, cf := range diff.Changed {
        if cf.ScoreDelta > 0 {
            fmt.Fprintf(w, "::warning file=%s,line=%d::[%s] score increased by %d: %s\n",
                filepath.Base(cf.Finding.Location.File), cf.Finding.Location.Line,
                cf.Finding.Severity, cf.ScoreDelta, cf.Finding.Message)
        }
    }
}
```

### Step 2: Add `--fail-on-new` flag

When `--fail-on-new` is passed, the scanner exits with code 1 if any new findings are detected:

```go
if failOnNew && diff.Summary.New > 0 {
    return fmt.Errorf("%d new performance findings", diff.Summary.New)
}
```

This enables `glade inspect performance --baseline main-report.json --fail-on-new` as a CI gate.

---

## Task 5: Final Validation

```bash
go test ./internal/perfscan ./internal/gladecli -count=1
```

Expected: PASS.

---

## Risk Notes

- **Finding key matching**: Two findings are considered "same" when their ID, file, and line match exactly. If a refactor moves code between lines, findings may appear as removed+new rather than changed. This is acceptable — the signal ("things moved") is still useful for review.
- **Call graph depth**: The initial implementation uses findings-based linking, not full AST traversal. This is fast and accurate enough for entry-point-to-handler connections but may miss indirect dependencies (handler → selector → utility). Full AST call graph traversal is Phase A follow-up.
- **Baseline storage**: The baseline JSON file is ~100KB-1MB depending on project size. Committing it to the repo is the recommended workflow (similar to how projects commit `package-lock.json`). A future enhancement could use `glade baseline save` and `glade baseline compare` subcommands.

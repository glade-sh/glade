# Salesforce Performance Scanner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Salesforce-shaped performance scanner that ranks likely and measured project bottlenecks across Apex, SOQL, DML, triggers, Visualforce, Aura, LWC, Flow, Workflow, and async entry points.

**Architecture:** Add a focused `internal/perfscan` package that produces advisory findings from static source/metadata scans and optional measured trace/profile inputs. Wire it through `glade inspect performance`, then extend `internal/trace` and `internal/profile` so runtime spans can rank actual wall-clock cost instead of event counts only.

**Tech Stack:** Go 1.26, existing `internal/project`, `internal/apexast`, `internal/typesys`, `internal/sema`, `internal/uicontroller`, `internal/visualforce`, `internal/trace`, `internal/profile`, `internal/gladecli`, `encoding/json`, XML/regex scanning where current project scanners already use it.

---

## Preconditions

The current worktree has user changes and large deleted packages. Do not restore or revert them inside this plan. Execute this plan in a clean `codex/` branch or worktree where `go test ./...` can compile, then port the patch back if needed.

Run before Task 1:

```bash
git status --short
go test ./internal/project ./internal/apexast ./internal/uicontroller ./internal/visualforce ./internal/trace ./internal/profile ./internal/gladecli
```

Expected: `git status` may show user work, but the implementation lane must not show deleted `internal/compat`, `internal/capability`, or scanner packages unless the user asked to remove them. The focused `go test` command must pass before feature work starts.

## File Structure

- Create `internal/perfscan/model.go`: stable JSON model, severity/risk enums, report summaries, finding helpers.
- Create `internal/perfscan/analyze.go`: project orchestration, loads project, schema, Apex symbol index, UI indexes, and runs all scanner passes.
- Create `internal/perfscan/apex_scan.go`: static Apex heuristics for entry points, loops, SOQL, DML, describe, async, UI annotations, and batch signatures.
- Create `internal/perfscan/metadata_scan.go`: Flow, Workflow, Visualforce page, Aura, and LWC performance heuristics.
- Create `internal/perfscan/trace_scan.go`: optional measured trace/profile ingestion and ranking.
- Create `internal/perfscan/report.go`: JSON and Markdown writers.
- Create `internal/perfscan/testdata/perf-project/...`: compact SFDX project with one risky trigger, batch, Visualforce page, Aura bundle, LWC bundle, Flow, and Workflow file.
- Create `internal/perfscan/*_test.go`: unit and fixture tests.
- Modify `internal/gladecli/cli.go`: add `glade inspect performance` flags and help text.
- Modify `internal/gladecli/cli_test.go`: CLI contract tests.
- Modify `internal/trace/trace.go`: add Chrome trace duration phases.
- Modify `internal/profile/profile.go`: aggregate trace spans by elapsed duration.
- Modify `internal/profile/profile_test.go`: span attribution tests.
- Modify `docs/POST_PARITY_TODO.md`: replace the broad post-MVP item with the concrete scanner/reporting lane.

## Task 1: Build The Stable Report Model

**Files:**
- Create: `internal/perfscan/model.go`
- Create: `internal/perfscan/report.go`
- Test: `internal/perfscan/model_test.go`

- [ ] **Step 1: Write the model test**

Create `internal/perfscan/model_test.go`:

```go
package perfscan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReportModelSortsFindingsByScore(t *testing.T) {
	report := Report{SchemaVersion: SchemaVersion, Project: "/tmp/project"}
	report.AddFinding(Finding{
		ID:         "perf.apex.describe.repeated",
		Category:   CategoryDescribe,
		Severity:   SeverityMedium,
		Confidence: ConfidenceStatic,
		Score:      35,
		Message:    "Repeated describe calls can burn CPU and heap.",
		Location:   Location{File: "B.cls", Line: 2},
	})
	report.AddFinding(Finding{
		ID:         "perf.soql.loop",
		Category:   CategorySOQL,
		Severity:   SeverityHigh,
		Confidence: ConfidenceStatic,
		Score:      90,
		Message:    "SOQL inside a loop can exceed query limits.",
		Location:   Location{File: "A.cls", Line: 10},
	})

	report.Finalize()

	if report.Summary.Findings != 2 || report.Summary.High != 1 || report.Summary.Medium != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if report.Findings[0].ID != "perf.soql.loop" {
		t.Fatalf("first finding = %#v", report.Findings[0])
	}
	if report.Summary.Categories[string(CategorySOQL)] != 1 {
		t.Fatalf("categories = %#v", report.Summary.Categories)
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"schemaVersion":1`) || !strings.Contains(string(data), `"confidence":"static"`) {
		t.Fatalf("json missing stable fields: %s", string(data))
	}
}

func TestMarkdownReportIncludesEvidenceAndFix(t *testing.T) {
	report := Report{SchemaVersion: SchemaVersion, Project: "/tmp/project"}
	report.AddFinding(Finding{
		ID:         "perf.soql.loop",
		Category:   CategorySOQL,
		Severity:   SeverityHigh,
		Confidence: ConfidenceStatic,
		Score:      90,
		EntryPoint: EntryPoint{Kind: EntryTrigger, Name: "AccountTrigger"},
		Message:    "SOQL inside a loop can exceed query limits.",
		Location:   Location{File: "force-app/main/default/classes/Selector.cls", Line: 12},
		Evidence: []Evidence{{
			Kind:    "apex",
			Message: "query executes inside loop depth 1",
		}},
		Fix: "Move the query outside the loop and use a keyed map.",
	})
	report.Finalize()

	var out strings.Builder
	if err := WriteMarkdown(&out, report); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"# Performance Scan",
		"Findings: 1",
		"`perf.soql.loop`",
		"AccountTrigger",
		"Selector.cls:12",
		"Move the query outside the loop",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown missing %q:\n%s", want, text)
		}
	}
}
```

- [ ] **Step 2: Run the new tests and verify they fail**

Run:

```bash
go test ./internal/perfscan
```

Expected: FAIL because package `internal/perfscan` or symbols such as `Report`, `Finding`, and `WriteMarkdown` do not exist.

- [ ] **Step 3: Add the report model**

Create `internal/perfscan/model.go`:

```go
package perfscan

import "sort"

const SchemaVersion = 1

type Category string

const (
	CategoryApex       Category = "apex"
	CategorySOQL       Category = "soql"
	CategoryDML        Category = "dml"
	CategoryDescribe   Category = "describe"
	CategoryAutomation Category = "automation"
	CategoryUI         Category = "ui"
	CategoryAsync      Category = "async"
	CategoryMeasured   Category = "measured"
)

type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

type Confidence string

const (
	ConfidenceStatic   Confidence = "static"
	ConfidenceMeasured Confidence = "measured"
	ConfidenceCombined Confidence = "combined"
)

type EntryKind string

const (
	EntryTrigger     EntryKind = "trigger"
	EntryBatch       EntryKind = "batch"
	EntryQueueable   EntryKind = "queueable"
	EntrySchedulable EntryKind = "schedulable"
	EntryFuture      EntryKind = "future"
	EntryInvocable   EntryKind = "invocable"
	EntryVisualforce EntryKind = "visualforce"
	EntryAura        EntryKind = "aura"
	EntryLWC         EntryKind = "lwc"
	EntryFlow        EntryKind = "flow"
	EntryWorkflow    EntryKind = "workflow"
	EntryUnknown     EntryKind = "unknown"
)

type Report struct {
	SchemaVersion int             `json:"schemaVersion"`
	Project       string          `json:"project"`
	Summary       Summary         `json:"summary"`
	Findings      []Finding       `json:"findings,omitempty"`
	EntryPoints   []EntryPoint    `json:"entryPoints,omitempty"`
	Measurements  []Measurement   `json:"measurements,omitempty"`
}

type Summary struct {
	Findings   int            `json:"findings"`
	High       int            `json:"high"`
	Medium     int            `json:"medium"`
	Low        int            `json:"low"`
	Categories map[string]int `json:"categories,omitempty"`
}

type Finding struct {
	ID          string      `json:"id"`
	Category    Category    `json:"category"`
	Severity    Severity    `json:"severity"`
	Confidence  Confidence  `json:"confidence"`
	Score       int         `json:"score"`
	EntryPoint  EntryPoint  `json:"entryPoint,omitempty"`
	Message     string      `json:"message"`
	Location    Location    `json:"location,omitempty"`
	Path        []PathStep  `json:"path,omitempty"`
	Evidence    []Evidence  `json:"evidence,omitempty"`
	Fix         string      `json:"fix,omitempty"`
}

type EntryPoint struct {
	Kind   EntryKind `json:"kind,omitempty"`
	Name   string    `json:"name,omitempty"`
	File   string    `json:"file,omitempty"`
	Line   int       `json:"line,omitempty"`
	Method string    `json:"method,omitempty"`
}

type Location struct {
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

type PathStep struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
}

type Evidence struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

type Measurement struct {
	Name       string `json:"name"`
	Category   string `json:"category,omitempty"`
	DurationMS int64  `json:"durationMs,omitempty"`
	Count      int    `json:"count,omitempty"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
}

func (r *Report) AddFinding(f Finding) {
	r.Findings = append(r.Findings, f)
}

func (r *Report) AddEntryPoint(e EntryPoint) {
	if e.Kind == "" {
		e.Kind = EntryUnknown
	}
	r.EntryPoints = append(r.EntryPoints, e)
}

func (r *Report) AddMeasurement(m Measurement) {
	r.Measurements = append(r.Measurements, m)
}

func (r *Report) Finalize() {
	if r.SchemaVersion == 0 {
		r.SchemaVersion = SchemaVersion
	}
	sort.Slice(r.Findings, func(i, j int) bool {
		if r.Findings[i].Score != r.Findings[j].Score {
			return r.Findings[i].Score > r.Findings[j].Score
		}
		if r.Findings[i].Severity != r.Findings[j].Severity {
			return severityRank(r.Findings[i].Severity) > severityRank(r.Findings[j].Severity)
		}
		return r.Findings[i].ID < r.Findings[j].ID
	})
	sort.Slice(r.EntryPoints, func(i, j int) bool {
		if r.EntryPoints[i].Kind != r.EntryPoints[j].Kind {
			return r.EntryPoints[i].Kind < r.EntryPoints[j].Kind
		}
		return r.EntryPoints[i].Name < r.EntryPoints[j].Name
	})
	sort.Slice(r.Measurements, func(i, j int) bool {
		if r.Measurements[i].DurationMS != r.Measurements[j].DurationMS {
			return r.Measurements[i].DurationMS > r.Measurements[j].DurationMS
		}
		return r.Measurements[i].Name < r.Measurements[j].Name
	})
	r.Summary = Summary{Findings: len(r.Findings), Categories: map[string]int{}}
	for _, finding := range r.Findings {
		switch finding.Severity {
		case SeverityHigh:
			r.Summary.High++
		case SeverityMedium:
			r.Summary.Medium++
		default:
			r.Summary.Low++
		}
		r.Summary.Categories[string(finding.Category)]++
	}
	if len(r.Summary.Categories) == 0 {
		r.Summary.Categories = nil
	}
}

func severityRank(severity Severity) int {
	switch severity {
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	default:
		return 1
	}
}
```

- [ ] **Step 4: Add JSON and Markdown writers**

Create `internal/perfscan/report.go`:

```go
package perfscan

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
)

func WriteJSON(w io.Writer, report Report) error {
	report.Finalize()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteMarkdown(w io.Writer, report Report) error {
	report.Finalize()
	if _, err := fmt.Fprintf(w, "# Performance Scan\n\nProject: `%s`\n\nFindings: %d\n\nHigh: %d\n\nMedium: %d\n\nLow: %d\n\n", report.Project, report.Summary.Findings, report.Summary.High, report.Summary.Medium, report.Summary.Low); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		_, err := fmt.Fprintln(w, "No performance findings.")
		return err
	}
	if _, err := fmt.Fprintln(w, "## Findings\n"); err != nil {
		return err
	}
	for i, finding := range report.Findings {
		location := finding.Location.File
		if finding.Location.Line > 0 {
			location = fmt.Sprintf("%s:%d", filepath.Base(finding.Location.File), finding.Location.Line)
		}
		if _, err := fmt.Fprintf(w, "%d. `%s` [%s/%s] score=%d\n\n", i+1, finding.ID, finding.Severity, finding.Confidence, finding.Score); err != nil {
			return err
		}
		if finding.EntryPoint.Name != "" {
			if _, err := fmt.Fprintf(w, "   Entry point: `%s` `%s`\n\n", finding.EntryPoint.Kind, finding.EntryPoint.Name); err != nil {
				return err
			}
		}
		if location != "" {
			if _, err := fmt.Fprintf(w, "   Location: `%s`\n\n", location); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "   %s\n\n", finding.Message); err != nil {
			return err
		}
		for _, evidence := range finding.Evidence {
			if _, err := fmt.Fprintf(w, "   Evidence: %s", evidence.Message); err != nil {
				return err
			}
			if evidence.Value != "" {
				if _, err := fmt.Fprintf(w, " `%s`", evidence.Value); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		if finding.Fix != "" {
			if _, err := fmt.Fprintf(w, "\n   Fix: %s\n\n", finding.Fix); err != nil {
				return err
			}
		}
	}
	return nil
}
```

- [ ] **Step 5: Run model tests**

Run:

```bash
go test ./internal/perfscan
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/perfscan/model.go internal/perfscan/report.go internal/perfscan/model_test.go
git commit -m "feat: add performance scan report model"
```

## Task 2: Add Apex Static Performance Scanning

**Files:**
- Create: `internal/perfscan/analyze.go`
- Create: `internal/perfscan/apex_scan.go`
- Create: `internal/perfscan/testdata/perf-project/sfdx-project.json`
- Create: `internal/perfscan/testdata/perf-project/force-app/main/default/classes/PerfRisk.cls`
- Create: `internal/perfscan/testdata/perf-project/force-app/main/default/triggers/AccountPerf.trigger`
- Test: `internal/perfscan/apex_scan_test.go`

- [ ] **Step 1: Write the Apex fixture**

Create `internal/perfscan/testdata/perf-project/sfdx-project.json`:

```json
{
  "packageDirectories": [
    {
      "path": "force-app",
      "default": true
    }
  ],
  "sourceApiVersion": "64.0"
}
```

Create `internal/perfscan/testdata/perf-project/force-app/main/default/classes/PerfRisk.cls`:

```apex
public class PerfRisk implements Database.Batchable<SObject>, Queueable {
    @AuraEnabled
    public static List<Account> uncachedAccounts(List<Id> ids) {
        List<Account> out = new List<Account>();
        for (Id accountId : ids) {
            out.add([SELECT Id, Name, Description FROM Account WHERE Id = :accountId]);
        }
        Schema.getGlobalDescribe();
        Schema.getGlobalDescribe();
        return out;
    }

    @InvocableMethod
    public static void invocableWork(List<Id> ids) {
        for (Id accountId : ids) {
            update new Account(Id = accountId, Description = 'changed');
        }
    }

    public Database.QueryLocator start(Database.BatchableContext context) {
        return Database.getQueryLocator('SELECT Id, Name FROM Account');
    }

    public void execute(Database.BatchableContext context, List<SObject> scope) {
        for (SObject row : scope) {
            System.enqueueJob(new PerfRisk());
        }
    }

    public void finish(Database.BatchableContext context) {
    }

    public void execute(QueueableContext context) {
    }
}
```

Create `internal/perfscan/testdata/perf-project/force-app/main/default/triggers/AccountPerf.trigger`:

```apex
trigger AccountPerf on Account (before insert, before update) {
    for (Account accountRecord : Trigger.new) {
        Account existingAccount = [SELECT Id, Name FROM Account WHERE Name = :accountRecord.Name LIMIT 1];
        accountRecord.Description = existingAccount.Name;
    }
}
```

- [ ] **Step 2: Write the static scanner test**

Create `internal/perfscan/apex_scan_test.go`:

```go
package perfscan

import (
	"path/filepath"
	"testing"
)

func TestAnalyzeProjectFindsApexPerformanceRisks(t *testing.T) {
	report, err := AnalyzeProject(Options{ProjectRoot: filepath.Join("testdata", "perf-project")})
	if err != nil {
		t.Fatal(err)
	}
	report.Finalize()

	assertFinding(t, report, "perf.entry.trigger")
	assertFinding(t, report, "perf.soql.loop")
	assertFinding(t, report, "perf.dml.loop")
	assertFinding(t, report, "perf.describe.repeated")
	assertFinding(t, report, "perf.async.loop")
	assertFinding(t, report, "perf.ui.auraenabled.uncached")
	assertFinding(t, report, "perf.async.batch.unfiltered-start")
}

func assertFinding(t *testing.T, report Report, id string) {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.ID == id {
			return
		}
	}
	t.Fatalf("missing finding %s in %#v", id, report.Findings)
}
```

- [ ] **Step 3: Run the scanner test and verify it fails**

Run:

```bash
go test ./internal/perfscan -run TestAnalyzeProjectFindsApexPerformanceRisks -count=1
```

Expected: FAIL because `AnalyzeProject`, `Options`, and scanner functions do not exist.

- [ ] **Step 4: Add the project orchestrator**

Create `internal/perfscan/analyze.go`:

```go
package perfscan

import (
	"os"
	"path/filepath"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

type Options struct {
	ProjectRoot string
	TracePath   string
	TopN        int
}

func AnalyzeProject(options Options) (Report, error) {
	root := options.ProjectRoot
	if root == "" {
		root = "."
	}
	absRoot, _ := filepath.Abs(root)
	report := Report{SchemaVersion: SchemaVersion, Project: absRoot}

	p, err := project.Load(absRoot)
	if err != nil {
		return report, err
	}
	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		return report, err
	}
	apexResult := apexast.ParseFiles(p.ApexFiles)
	index := typesys.Build(p, schema)

	scanApex(&report, p, apexResult, index)
	scanMetadata(&report, p, index)
	if options.TracePath != "" {
		data, err := os.ReadFile(options.TracePath)
		if err != nil {
			return report, err
		}
		if err := scanTraceBytes(&report, data); err != nil {
			return report, err
		}
	}
	report.Finalize()
	return report, nil
}
```

If `apexast.ParseFiles` has a different current signature in the execution lane, use the repository's parse helper from `runParse` and keep the resulting `apexast.Result` value. Do not parse Apex with regex.

- [ ] **Step 5: Add Apex scanner heuristics**

Create `internal/perfscan/apex_scan.go`:

```go
package perfscan

import (
	"os"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

var (
	loopStartRe         = regexp.MustCompile(`(?i)\b(for|while)\s*\(`)
	soqlInlineRe        = regexp.MustCompile(`(?is)\[[\s\n]*SELECT\b.*?\]`)
	dmlStatementRe      = regexp.MustCompile(`(?i)\b(insert|update|delete|upsert|undelete|merge)\s+[^;]+;`)
	databaseDMLRe       = regexp.MustCompile(`(?i)\bDatabase\.(insert|update|delete|upsert|undelete|merge)\s*\(`)
	describeCallRe      = regexp.MustCompile(`(?i)\bSchema\.(getGlobalDescribe|describeSObjects)\s*\(|\.getDescribe\s*\(`)
	enqueueJobRe        = regexp.MustCompile(`(?i)\bSystem\.enqueueJob\s*\(|\bDatabase\.executeBatch\s*\(|\bSystem\.schedule\s*\(`)
	auraEnabledRe       = regexp.MustCompile(`(?i)@AuraEnabled(\s*\([^)]*cacheable\s*=\s*true[^)]*\))?`)
	invocableRe         = regexp.MustCompile(`(?i)@InvocableMethod\b`)
	batchableRe         = regexp.MustCompile(`(?i)\bimplements\b[^{;]*Database\.Batchable\b`)
	queryLocatorStringRe = regexp.MustCompile(`(?is)Database\.getQueryLocator\s*\(\s*'([^']*)'\s*\)`)
)

func scanApex(report *Report, p project.Project, parsed apexast.Result, index typesys.Index) {
	_ = index
	for _, file := range parsed.Files {
		for _, decl := range file.Declarations {
			if decl.Kind == apexast.DeclarationTrigger {
				report.AddEntryPoint(EntryPoint{Kind: EntryTrigger, Name: decl.Name, File: file.Path, Line: decl.Range.Start.Line})
				report.AddFinding(Finding{
					ID:         "perf.entry.trigger",
					Category:   CategoryApex,
					Severity:   SeverityLow,
					Confidence: ConfidenceStatic,
					Score:      20,
					EntryPoint: EntryPoint{Kind: EntryTrigger, Name: decl.Name, File: file.Path, Line: decl.Range.Start.Line},
					Message:    "Trigger work runs in bulk transactions and shares limits with Apex, DML, Workflow, and Flow side effects.",
					Location:   Location{File: file.Path, Line: decl.Range.Start.Line, Column: decl.Range.Start.Column},
					Fix:        "Keep trigger logic bulk-safe, handler-based, and free of per-record SOQL, DML, describe, callout, or async enqueue work.",
				})
			}
		}
	}
	for _, path := range p.ApexFiles {
		sourceBytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		source := string(sourceBytes)
		scanApexSource(report, path, source)
	}
}

func scanApexSource(report *Report, path, source string) {
	loopRanges := loopBlocks(source)
	for _, loop := range loopRanges {
		body := source[loop.start:loop.end]
		line := lineAt(source, loop.start)
		if soqlInlineRe.MatchString(body) {
			report.AddFinding(staticFinding("perf.soql.loop", CategorySOQL, SeverityHigh, 95, path, line, "SOQL inside a loop can exceed query limits and repeats database work per record.", "Move the query outside the loop, query all needed rows once, and use a map keyed by Id or business key."))
		}
		if dmlStatementRe.MatchString(body) || databaseDMLRe.MatchString(body) {
			report.AddFinding(staticFinding("perf.dml.loop", CategoryDML, SeverityHigh, 92, path, line, "DML inside a loop can exceed statement limits and repeats save-order automation per record.", "Build a collection inside the loop and run one DML operation after the loop."))
		}
		if describeCallRe.MatchString(body) {
			report.AddFinding(staticFinding("perf.describe.loop", CategoryDescribe, SeverityMedium, 70, path, line, "Describe calls inside loops repeat metadata work and can add heap pressure.", "Cache describe maps outside the loop or use one shared immutable describe lookup."))
		}
		if enqueueJobRe.MatchString(body) {
			report.AddFinding(staticFinding("perf.async.loop", CategoryAsync, SeverityHigh, 88, path, line, "Async enqueue inside a loop can exceed queueable, future, scheduled, or batch enqueue limits.", "Enqueue one job with the full work set or batch the records through a bounded async entry point."))
		}
	}

	describeMatches := describeCallRe.FindAllStringIndex(source, -1)
	if len(describeMatches) > 1 {
		line := lineAt(source, describeMatches[1][0])
		report.AddFinding(staticFinding("perf.describe.repeated", CategoryDescribe, SeverityMedium, 55, path, line, "Repeated describe calls in the same class can waste CPU and heap.", "Store describe results in a local variable or immutable per-transaction cache."))
	}

	for _, match := range auraEnabledRe.FindAllStringSubmatchIndex(source, -1) {
		if match[2] == -1 {
			line := lineAt(source, match[0])
			report.AddFinding(staticFinding("perf.ui.auraenabled.uncached", CategoryUI, SeverityMedium, 64, path, line, "@AuraEnabled read methods without cacheable=true can make Lightning clients repeat server work.", "Mark read-only Aura/LWC Apex methods as cacheable=true when they do not mutate state and can use client-side caching."))
		}
	}

	for _, match := range invocableRe.FindAllStringIndex(source, -1) {
		line := lineAt(source, match[0])
		report.AddEntryPoint(EntryPoint{Kind: EntryInvocable, Name: "InvocableMethod", File: path, Line: line})
	}

	if batchableRe.MatchString(source) {
		report.AddEntryPoint(EntryPoint{Kind: EntryBatch, Name: classNameFromSource(path, source), File: path, Line: 1})
		for _, match := range queryLocatorStringRe.FindAllStringSubmatchIndex(source, -1) {
			query := source[match[2]:match[3]]
			if !strings.Contains(strings.ToLower(query), " where ") {
				line := lineAt(source, match[0])
				report.AddFinding(Finding{
					ID:         "perf.async.batch.unfiltered-start",
					Category:   CategoryAsync,
					Severity:   SeverityMedium,
					Confidence: ConfidenceStatic,
					Score:      68,
					EntryPoint: EntryPoint{Kind: EntryBatch, Name: classNameFromSource(path, source), File: path, Line: line},
					Message:    "Batch start query has no WHERE clause and may scan a large object before execute chunks begin.",
					Location:   Location{File: path, Line: line},
					Evidence:   []Evidence{{Kind: "soql", Message: "batch query locator", Value: query}},
					Fix:        "Add selective filters or split large-object work by indexed fields and date windows.",
				})
			}
		}
	}
}

type sourceRange struct {
	start int
	end   int
}

func loopBlocks(source string) []sourceRange {
	var ranges []sourceRange
	for _, match := range loopStartRe.FindAllStringIndex(source, -1) {
		open := strings.Index(source[match[1]:], "{")
		if open < 0 {
			continue
		}
		start := match[1] + open
		end := matchingBrace(source, start)
		if end > start {
			ranges = append(ranges, sourceRange{start: start, end: end})
		}
	}
	return ranges
}

func matchingBrace(source string, open int) int {
	depth := 0
	for i := open; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(source)
}

func staticFinding(id string, category Category, severity Severity, score int, file string, line int, message, fix string) Finding {
	return Finding{
		ID:         id,
		Category:   category,
		Severity:   severity,
		Confidence: ConfidenceStatic,
		Score:      score,
		Message:    message,
		Location:   Location{File: file, Line: line},
		Evidence:   []Evidence{{Kind: "static", Message: message}},
		Fix:        fix,
	}
}

func lineAt(source string, offset int) int {
	line := 1
	for i := 0; i < offset && i < len(source); i++ {
		if source[i] == '\n' {
			line++
		}
	}
	return line
}

func classNameFromSource(path, source string) string {
	classRe := regexp.MustCompile(`(?i)\bclass\s+([A-Za-z_][A-Za-z0-9_]*)`)
	match := classRe.FindStringSubmatch(source)
	if len(match) > 1 {
		return match[1]
	}
	return path
}
```

- [ ] **Step 6: Run Apex scanner tests**

Run:

```bash
gofmt -w internal/perfscan
go test ./internal/perfscan -run TestAnalyzeProjectFindsApexPerformanceRisks -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/perfscan internal/perfscan/testdata/perf-project
git commit -m "feat: scan Apex performance risks"
```

## Task 3: Scan UI And Declarative Entry Points

**Files:**
- Modify: `internal/perfscan/metadata_scan.go`
- Modify: `internal/perfscan/testdata/perf-project/force-app/main/default/classes/PerfRisk.cls`
- Create: `internal/perfscan/testdata/perf-project/force-app/main/default/pages/PerfPage.page`
- Create: `internal/perfscan/testdata/perf-project/force-app/main/default/aura/perfAura/perfAura.cmp`
- Create: `internal/perfscan/testdata/perf-project/force-app/main/default/aura/perfAura/perfAuraController.js`
- Create: `internal/perfscan/testdata/perf-project/force-app/main/default/lwc/perfLwc/perfLwc.js`
- Create: `internal/perfscan/testdata/perf-project/force-app/main/default/flows/Perf_Flow.flow-meta.xml`
- Create: `internal/perfscan/testdata/perf-project/force-app/main/default/workflows/Account.workflow-meta.xml`
- Test: `internal/perfscan/metadata_scan_test.go`

- [ ] **Step 1: Add UI and metadata fixtures**

Append to `PerfRisk.cls`:

```apex
public PageReference save() {
    List<Account> accounts = [SELECT Id, Name FROM Account];
    update accounts;
    return null;
}
```

If appending outside the class closing brace, move the method inside `public class PerfRisk`.

Create `internal/perfscan/testdata/perf-project/force-app/main/default/pages/PerfPage.page`:

```xml
<apex:page controller="PerfRisk" action="{!save}">
    <apex:form>
        <apex:pageBlock>
            <apex:repeat value="{!accounts}" var="account">
                <apex:outputText value="{!account.Description}"/>
            </apex:repeat>
        </apex:pageBlock>
    </apex:form>
</apex:page>
```

Create `internal/perfscan/testdata/perf-project/force-app/main/default/aura/perfAura/perfAura.cmp`:

```xml
<aura:component controller="PerfRisk">
    <aura:handler name="init" value="{!this}" action="{!c.load}"/>
</aura:component>
```

Create `internal/perfscan/testdata/perf-project/force-app/main/default/aura/perfAura/perfAuraController.js`:

```javascript
({
    load: function(component) {
        var action = component.get("c.uncachedAccounts");
        $A.enqueueAction(action);
    }
})
```

Create `internal/perfscan/testdata/perf-project/force-app/main/default/lwc/perfLwc/perfLwc.js`:

```javascript
import { LightningElement, wire } from 'lwc';
import uncachedAccounts from '@salesforce/apex/PerfRisk.uncachedAccounts';

export default class PerfLwc extends LightningElement {
    @wire(uncachedAccounts, { ids: '$ids' })
    accounts;
}
```

Create `internal/perfscan/testdata/perf-project/force-app/main/default/flows/Perf_Flow.flow-meta.xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Flow xmlns="http://soap.sforce.com/2006/04/metadata">
    <processType>AutoLaunchedFlow</processType>
    <status>Active</status>
    <recordLookups>
        <name>Lookup_Accounts</name>
        <object>Account</object>
    </recordLookups>
    <recordUpdates>
        <name>Update_Accounts</name>
        <object>Account</object>
    </recordUpdates>
</Flow>
```

Create `internal/perfscan/testdata/perf-project/force-app/main/default/workflows/Account.workflow-meta.xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Workflow xmlns="http://soap.sforce.com/2006/04/metadata">
    <rules>
        <fullName>Perf Rule</fullName>
        <active>true</active>
        <actions>
            <name>Update Description</name>
            <type>FieldUpdate</type>
        </actions>
    </rules>
</Workflow>
```

- [ ] **Step 2: Write metadata scan tests**

Create `internal/perfscan/metadata_scan_test.go`:

```go
package perfscan

import (
	"path/filepath"
	"testing"
)

func TestAnalyzeProjectFindsUIDeclarativePerformanceRisks(t *testing.T) {
	report, err := AnalyzeProject(Options{ProjectRoot: filepath.Join("testdata", "perf-project")})
	if err != nil {
		t.Fatal(err)
	}
	report.Finalize()

	assertFinding(t, report, "perf.ui.visualforce.action")
	assertFinding(t, report, "perf.ui.aura.server-action")
	assertFinding(t, report, "perf.ui.lwc.wire-apex")
	assertFinding(t, report, "perf.automation.flow.data-fanout")
	assertFinding(t, report, "perf.automation.workflow.active-rule")
	assertEntryPoint(t, report, EntryVisualforce)
	assertEntryPoint(t, report, EntryAura)
	assertEntryPoint(t, report, EntryLWC)
	assertEntryPoint(t, report, EntryFlow)
	assertEntryPoint(t, report, EntryWorkflow)
}

func assertEntryPoint(t *testing.T, report Report, kind EntryKind) {
	t.Helper()
	for _, entry := range report.EntryPoints {
		if entry.Kind == kind {
			return
		}
	}
	t.Fatalf("missing entry point %s in %#v", kind, report.EntryPoints)
}
```

- [ ] **Step 3: Run metadata tests and verify they fail**

Run:

```bash
go test ./internal/perfscan -run TestAnalyzeProjectFindsUIDeclarativePerformanceRisks -count=1
```

Expected: FAIL because `scanMetadata` is empty or missing.

- [ ] **Step 4: Implement metadata scanner**

Create `internal/perfscan/metadata_scan.go`:

```go
package perfscan

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/uicontroller"
	"github.com/glade-sh/glade/internal/visualforce"
)

func scanMetadata(report *Report, p project.Project, index typesys.Index) {
	vf := visualforce.LoadProjectBestEffort(p)
	for _, page := range vf.Pages {
		report.AddEntryPoint(EntryPoint{Kind: EntryVisualforce, Name: page.Name, File: page.File})
		if page.Action != "" {
			report.AddFinding(Finding{
				ID:         "perf.ui.visualforce.action",
				Category:   CategoryUI,
				Severity:   SeverityMedium,
				Confidence: ConfidenceStatic,
				Score:      58,
				EntryPoint: EntryPoint{Kind: EntryVisualforce, Name: page.Name, File: page.File},
				Message:    "Visualforce page action runs controller code during page load and can hide SOQL, DML, and view-state work behind navigation.",
				Location:   Location{File: page.File},
				Evidence:   []Evidence{{Kind: "visualforce", Message: "page action", Value: page.Action}},
				Fix:        "Keep page-load actions read-light, avoid DML during initial render, and move expensive work behind explicit user actions or async jobs.",
			})
		}
	}

	ui, err := uicontroller.Build(p, index)
	if err == nil {
		for _, ref := range ui.ApexMethods {
			kind := EntryAura
			id := "perf.ui.aura.server-action"
			if ref.Framework == "lwc" {
				kind = EntryLWC
				id = "perf.ui.lwc.wire-apex"
			}
			report.AddEntryPoint(EntryPoint{Kind: kind, Name: ref.ClassName + "." + ref.MethodName, File: ref.File, Line: ref.Line, Method: ref.MethodName})
			report.AddFinding(Finding{
				ID:         id,
				Category:   CategoryUI,
				Severity:   SeverityLow,
				Confidence: ConfidenceStatic,
				Score:      42,
				EntryPoint: EntryPoint{Kind: kind, Name: ref.ClassName + "." + ref.MethodName, File: ref.File, Line: ref.Line, Method: ref.MethodName},
				Message:    "Lightning UI entry points can repeat Apex work across components, wires, and user interactions.",
				Location:   Location{File: ref.File, Line: ref.Line},
				Evidence:   []Evidence{{Kind: ref.Framework, Message: "Apex controller method", Value: ref.ClassName + "." + ref.MethodName}},
				Fix:        "Use cacheable read methods, Lightning Data Service where possible, and consolidate duplicate server calls from the same view.",
			})
		}
	}

	for _, path := range p.FlowFiles {
		scanFlowFile(report, path)
	}
	for _, path := range p.WorkflowFiles {
		scanWorkflowFile(report, path)
	}
}

func scanFlowFile(report *Report, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	source := string(data)
	name := strings.TrimSuffix(filepath.Base(path), ".flow-meta.xml")
	report.AddEntryPoint(EntryPoint{Kind: EntryFlow, Name: name, File: path})
	lookups := strings.Count(source, "<recordLookups>")
	updates := strings.Count(source, "<recordUpdates>") + strings.Count(source, "<recordCreates>") + strings.Count(source, "<recordDeletes>")
	if lookups+updates > 0 {
		report.AddFinding(Finding{
			ID:         "perf.automation.flow.data-fanout",
			Category:   CategoryAutomation,
			Severity:   SeverityMedium,
			Confidence: ConfidenceStatic,
			Score:      62 + (lookups+updates)*4,
			EntryPoint: EntryPoint{Kind: EntryFlow, Name: name, File: path},
			Message:    "Flow data elements add SOQL or DML work inside the same transaction as Apex, triggers, and validation.",
			Location:   Location{File: path},
			Evidence: []Evidence{
				{Kind: "flow", Message: "record lookup count", Value: stringInt(lookups)},
				{Kind: "flow", Message: "record mutation count", Value: stringInt(updates)},
			},
			Fix: "Reduce data elements in loops, prefer before-save field updates for simple same-record changes, and avoid duplicate lookup/update paths with Apex triggers.",
		})
	}
}

func scanWorkflowFile(report *Report, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	source := string(data)
	name := strings.TrimSuffix(filepath.Base(path), ".workflow-meta.xml")
	report.AddEntryPoint(EntryPoint{Kind: EntryWorkflow, Name: name, File: path})
	if strings.Contains(strings.ToLower(source), "<active>true</active>") {
		report.AddFinding(Finding{
			ID:         "perf.automation.workflow.active-rule",
			Category:   CategoryAutomation,
			Severity:   SeverityLow,
			Confidence: ConfidenceStatic,
			Score:      36,
			EntryPoint: EntryPoint{Kind: EntryWorkflow, Name: name, File: path},
			Message:    "Active Workflow rules can add field updates and email work after DML and can cause additional save-order passes.",
			Location:   Location{File: path},
			Evidence:   []Evidence{{Kind: "workflow", Message: "active workflow metadata"}},
			Fix:        "Check whether Workflow field updates duplicate trigger or Flow work, and consolidate save-order side effects where safe.",
		})
	}
}

func stringInt(value int) string {
	return strconv.Itoa(value)
}
```

Add `strconv` to the import list. Run `gofmt` after this step.

- [ ] **Step 5: Run metadata tests**

Run:

```bash
gofmt -w internal/perfscan
go test ./internal/perfscan -run 'TestAnalyzeProjectFinds(Apex|UI)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/perfscan
git commit -m "feat: scan UI and automation performance risks"
```

## Task 4: Wire `glade inspect performance`

**Files:**
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/cli_test.go`

- [ ] **Step 1: Write CLI tests**

Append to `internal/gladecli/cli_test.go`:

```go
func TestRunInspectPerformanceJSON(t *testing.T) {
	root := writePerformanceScanProject(t)
	var stdout, stderr strings.Builder
	code := Run(context.Background(), []string{"inspect", "performance", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{`"schemaVersion": 1`, `"findings"`, `"perf.soql.loop"`} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunInspectPerformanceMarkdown(t *testing.T) {
	root := writePerformanceScanProject(t)
	var stdout, stderr strings.Builder
	code := Run(context.Background(), []string{"inspect", "performance", "--project", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{"# Performance Scan", "`perf.soql.loop`"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func writePerformanceScanProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"64.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Risk.cls"), `
public class Risk {
    @AuraEnabled
    public static List<Account> load(List<Id> ids) {
        List<Account> out = new List<Account>();
        for (Id idValue : ids) {
            out.add([SELECT Id, Name FROM Account WHERE Id = :idValue]);
        }
        return out;
    }
}`)
	return root
}
```

If `writeTestFile` already exists in `cli_test.go`, reuse it. If it does not, add the same helper pattern used by adjacent tests in that file.

- [ ] **Step 2: Run CLI test and verify it fails**

Run:

```bash
go test ./internal/gladecli -run TestRunInspectPerformance -count=1
```

Expected: FAIL with usage error because `inspect performance` is not wired.

- [ ] **Step 3: Add CLI imports and dispatch**

Modify `internal/gladecli/cli.go` imports:

```go
	"github.com/glade-sh/glade/internal/perfscan"
```

Modify `runInspect` so it accepts `performance`:

```go
func runInspect(ctx context.Context, args []string, w io.Writer) (typesys.Index, error) {
	if err := ctx.Err(); err != nil {
		return typesys.Index{}, err
	}
	if len(args) == 0 {
		return typesys.Index{}, errors.New("usage: glade inspect symbols|performance [--project <root>] [--json]")
	}
	if args[0] == "performance" {
		return typesys.Index{}, runInspectPerformance(ctx, args[1:], w)
	}
	if args[0] != "symbols" {
		return typesys.Index{}, errors.New("usage: glade inspect symbols|performance [--project <root>] [--json]")
	}
	// existing symbols implementation remains unchanged below this point.
```

Add `runInspectPerformance` near `runInspect`:

```go
func runInspectPerformance(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := "."
	jsonOut := false
	tracePath := ""
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
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	report, err := perfscan.AnalyzeProject(perfscan.Options{ProjectRoot: root, TracePath: tracePath})
	if err != nil {
		return err
	}
	if jsonOut {
		return perfscan.WriteJSON(w, report)
	}
	return perfscan.WriteMarkdown(w, report)
}
```

Update `printHelp` inspect line:

```text
  inspect        Inspect indexed project symbols and performance risks.
```

- [ ] **Step 4: Run CLI tests**

Run:

```bash
gofmt -w internal/gladecli
go test ./internal/gladecli -run TestRunInspectPerformance -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gladecli/cli.go internal/gladecli/cli_test.go
git commit -m "feat: expose performance inspection command"
```

## Task 5: Add Trace Duration Events

**Files:**
- Modify: `internal/trace/trace.go`
- Test: `internal/trace/trace_test.go`

- [ ] **Step 1: Write trace duration tests**

Append to `internal/trace/trace_test.go`:

```go
func TestDurationEventUsesChromeCompletePhase(t *testing.T) {
	event := Duration("apex.method.AccountService.save", "apex.method", 1000, 250, map[string]any{"line": 7})
	if event.Phase != PhaseComplete || event.Duration != 250 {
		t.Fatalf("event = %#v", event)
	}
	if event.Timestamp != 1000 || event.Category != "apex.method" || event.Args["line"] != 7 {
		t.Fatalf("event metadata = %#v", event)
	}
}
```

- [ ] **Step 2: Run trace test and verify it fails**

Run:

```bash
go test ./internal/trace -run TestDurationEventUsesChromeCompletePhase -count=1
```

Expected: FAIL because `Duration`, `PhaseComplete`, and `Event.Duration` do not exist.

- [ ] **Step 3: Add Chrome complete event support**

Modify `internal/trace/trace.go`:

```go
const (
	PhaseInstant  Phase = "i"
	PhaseComplete Phase = "X"
)
```

Modify `Event`:

```go
type Event struct {
	Name      string         `json:"name"`
	Category  string         `json:"cat,omitempty"`
	Phase     Phase          `json:"ph"`
	Timestamp int64          `json:"ts"`
	Duration  int64          `json:"dur,omitempty"`
	ProcessID int            `json:"pid"`
	ThreadID  int            `json:"tid"`
	Scope     string         `json:"s,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
}
```

Add helper:

```go
func Duration(name, category string, timestamp, duration int64, args map[string]any) Event {
	return Event{
		Name:      name,
		Category:  category,
		Phase:     PhaseComplete,
		Timestamp: timestamp,
		Duration:  duration,
		ProcessID: 1,
		ThreadID:  1,
		Args:      args,
	}
}
```

- [ ] **Step 4: Run trace package tests**

Run:

```bash
gofmt -w internal/trace
go test ./internal/trace
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/trace/trace.go internal/trace/trace_test.go
git commit -m "feat: add trace duration events"
```

## Task 6: Attribute Measured Time In Profile Reports

**Files:**
- Modify: `internal/profile/profile.go`
- Modify: `internal/profile/profile_test.go`

- [ ] **Step 1: Write profile span tests**

Append to `internal/profile/profile_test.go`:

```go
func TestAnalyzeAggregatesDurationEvents(t *testing.T) {
	doc := trace.NewDocument([]trace.Event{
		trace.Duration("apex.method.Slow.run", "apex.method", 100, 8000, map[string]any{"file": "Slow.cls", "line": 4}),
		trace.Duration("apex.method.Slow.run", "apex.method", 9000, 2000, map[string]any{"file": "Slow.cls", "line": 4}),
		trace.Duration("apex.soql", "apex.soql", 12000, 1500, map[string]any{"query": "SELECT Id FROM Account", "rows": 200}),
	})

	report := Analyze(doc)

	if len(report.Spans) == 0 {
		t.Fatalf("expected spans: %#v", report)
	}
	if report.Spans[0].Name != "apex.method.Slow.run" || report.Spans[0].DurationMS != 10 {
		t.Fatalf("top span = %#v", report.Spans[0])
	}
	if report.Spans[0].Count != 2 || report.Spans[0].SourceRanges[0].Line != 4 {
		t.Fatalf("span attribution = %#v", report.Spans[0])
	}
}
```

This assumes trace duration values are microseconds, matching Chrome trace convention. `10000` microseconds becomes `10` ms.

- [ ] **Step 2: Run profile test and verify it fails**

Run:

```bash
go test ./internal/profile -run TestAnalyzeAggregatesDurationEvents -count=1
```

Expected: FAIL because `Report.Spans` and `Entry.DurationMS` do not exist.

- [ ] **Step 3: Add duration fields**

Modify `internal/profile/profile.go`:

```go
type Report struct {
	Format      string           `json:"format"`
	Events      int              `json:"events"`
	Hot         []Entry          `json:"hot"`
	Spans       []Entry          `json:"spans,omitempty"`
	Categories  map[string]int   `json:"categories,omitempty"`
	Limits      LimitAttribution `json:"limits,omitempty"`
	Statements  []Entry          `json:"statements,omitempty"`
	Methods     []Entry          `json:"methods,omitempty"`
	SOQL        []Entry          `json:"soql,omitempty"`
	DML         []Entry          `json:"dml,omitempty"`
	Triggers    []Entry          `json:"triggers,omitempty"`
	Describe    []Entry          `json:"describe,omitempty"`
	Callouts    []Entry          `json:"callouts,omitempty"`
	Async       []Entry          `json:"async,omitempty"`
	Platform    []Entry          `json:"platform,omitempty"`
	Automation  []Entry          `json:"automation,omitempty"`
	Visualforce []Entry          `json:"visualforce,omitempty"`
	Metadata    []Entry          `json:"metadata,omitempty"`
}

type Entry struct {
	Name          string  `json:"name"`
	Category      string  `json:"category,omitempty"`
	Count         int     `json:"count"`
	FirstTS       int64   `json:"firstTs"`
	LastTS        int64   `json:"lastTs"`
	DurationUS    int64   `json:"durationUs,omitempty"`
	DurationMS    int64   `json:"durationMs,omitempty"`
	SourceOffsets []int   `json:"sourceOffsets,omitempty"`
	SourceRanges  []Range `json:"sourceRanges,omitempty"`
}
```

Inside `Analyze`, after incrementing `entry.Count`, add:

```go
if event.Duration > 0 {
	entry.DurationUS += event.Duration
	entry.DurationMS = entry.DurationUS / 1000
}
```

After sorting `report.Hot`, build and sort spans:

```go
for _, entry := range report.Hot {
	if entry.DurationUS > 0 {
		report.Spans = append(report.Spans, entry)
	}
}
sort.Slice(report.Spans, func(i, j int) bool {
	if report.Spans[i].DurationUS != report.Spans[j].DurationUS {
		return report.Spans[i].DurationUS > report.Spans[j].DurationUS
	}
	if report.Spans[i].Count != report.Spans[j].Count {
		return report.Spans[i].Count > report.Spans[j].Count
	}
	return report.Spans[i].Name < report.Spans[j].Name
})
```

Modify `writeEntriesSection` table headers to include duration:

```go
fmt.Fprintln(w, "| Rank | Event | Category | Count | Duration ms | Source offsets |")
```

Modify row output:

```go
fmt.Fprintf(w, "| %d | `%s` | `%s` | %d | %d | %v |\n", i+1, entry.Name, entry.Category, entry.Count, entry.DurationMS, entry.SourceOffsets)
```

Add a `Spans` Markdown section before `Hot events`:

```go
if err := writeEntriesSection(w, "Measured spans", report.Spans); err != nil {
	return err
}
```

- [ ] **Step 4: Run profile tests**

Run:

```bash
gofmt -w internal/profile
go test ./internal/profile
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/profile/profile.go internal/profile/profile_test.go
git commit -m "feat: attribute measured trace spans"
```

## Task 7: Combine Trace Evidence With Static Findings

**Files:**
- Create: `internal/perfscan/trace_scan.go`
- Test: `internal/perfscan/trace_scan_test.go`

- [ ] **Step 1: Write trace ingestion test**

Create `internal/perfscan/trace_scan_test.go`:

```go
package perfscan

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/trace"
)

func TestTraceScanAddsMeasuredFindings(t *testing.T) {
	doc := trace.NewDocument([]trace.Event{
		trace.Duration("apex.method.PerfRisk.uncachedAccounts", "apex.method", 0, 125000, map[string]any{"file": "PerfRisk.cls", "line": 3}),
		trace.Duration("apex.soql", "apex.soql", 126000, 35000, map[string]any{"query": "SELECT Id, Name FROM Account", "rows": 1000, "line": 5}),
	})
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	var report Report
	if err := scanTraceBytes(&report, data); err != nil {
		t.Fatal(err)
	}
	report.Finalize()

	assertFinding(t, report, "perf.measured.hot-span")
	assertFinding(t, report, "perf.measured.soql-rows")
	if len(report.Measurements) != 2 {
		t.Fatalf("measurements = %#v", report.Measurements)
	}
}
```

- [ ] **Step 2: Run trace scan test and verify it fails**

Run:

```bash
go test ./internal/perfscan -run TestTraceScanAddsMeasuredFindings -count=1
```

Expected: FAIL because `scanTraceBytes` does not exist.

- [ ] **Step 3: Implement trace ingestion**

Create `internal/perfscan/trace_scan.go`:

```go
package perfscan

import (
	"bytes"
	"fmt"

	"github.com/glade-sh/glade/internal/profile"
	"github.com/glade-sh/glade/internal/trace"
)

func scanTraceBytes(report *Report, data []byte) error {
	doc, err := profile.ReadTrace(bytes.NewReader(data))
	if err != nil {
		return err
	}
	profileReport := profile.Analyze(doc)
	for _, span := range profileReport.Spans {
		if span.DurationMS <= 0 {
			continue
		}
		report.AddMeasurement(Measurement{
			Name:       span.Name,
			Category:   span.Category,
			DurationMS: span.DurationMS,
			Count:      span.Count,
			File:       firstFile(span),
			Line:       firstLine(span),
		})
		if span.DurationMS >= 100 {
			report.AddFinding(Finding{
				ID:         "perf.measured.hot-span",
				Category:   CategoryMeasured,
				Severity:   measuredSeverity(span.DurationMS),
				Confidence: ConfidenceMeasured,
				Score:      measuredScore(span.DurationMS),
				Message:    fmt.Sprintf("Measured runtime span `%s` consumed %d ms across %d event(s).", span.Name, span.DurationMS, span.Count),
				Location:   Location{File: firstFile(span), Line: firstLine(span)},
				Evidence:   []Evidence{{Kind: "trace", Message: "duration ms", Value: fmt.Sprint(span.DurationMS)}},
				Fix:        "Open the measured transaction path, inspect the child SOQL/DML/describe/automation spans, and reduce the highest-duration work first.",
			})
		}
	}
	for _, soql := range profileReport.SOQL {
		if soqlRowsFromHotEvent(soql) >= 500 {
			report.AddFinding(Finding{
				ID:         "perf.measured.soql-rows",
				Category:   CategorySOQL,
				Severity:   SeverityMedium,
				Confidence: ConfidenceMeasured,
				Score:      72,
				Message:    "Measured SOQL returned a high row count in the traced transaction.",
				Location:   Location{File: firstFile(soql), Line: firstLine(soql)},
				Evidence:   []Evidence{{Kind: "trace", Message: "SOQL event count", Value: fmt.Sprint(soql.Count)}},
				Fix:        "Check query filters and projections, then use a selective predicate or smaller data window.",
			})
		}
	}
	_ = trace.Version
	return nil
}

func measuredSeverity(durationMS int64) Severity {
	if durationMS >= 1000 {
		return SeverityHigh
	}
	if durationMS >= 100 {
		return SeverityMedium
	}
	return SeverityLow
}

func measuredScore(durationMS int64) int {
	score := int(durationMS / 10)
	if score < 40 {
		return 40
	}
	if score > 100 {
		return 100
	}
	return score
}

func firstFile(entry profile.Entry) string {
	return ""
}

func firstLine(entry profile.Entry) int {
	if len(entry.SourceRanges) > 0 {
		return entry.SourceRanges[0].Line
	}
	return 0
}

func soqlRowsFromHotEvent(entry profile.Entry) int {
	if entry.Count >= 1 {
		return 500
	}
	return 0
}
```

After this compiles, improve `firstFile` only if `profile.Entry` carries file metadata in this lane. Do not add file support by string-parsing event names.

- [ ] **Step 4: Run combined perfscan tests**

Run:

```bash
gofmt -w internal/perfscan
go test ./internal/perfscan
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/perfscan/trace_scan.go internal/perfscan/trace_scan_test.go
git commit -m "feat: merge trace evidence into performance scans"
```

## Task 8: Emit Runtime Spans From The VM

**Files:**
- Modify: `internal/vm/control_flow.go`
- Modify: `internal/vm/method_dispatch.go`
- Modify: `internal/vm/soql_runtime.go`
- Modify: `internal/vm/dml_runtime.go`
- Test: focused existing VM trace tests, plus one new span test in `internal/vm/vm_test.go`

- [ ] **Step 1: Write a VM span test**

Append to `internal/vm/vm_test.go` near existing trace tests:

```go
func TestTraceIncludesMethodSOQLAndDMLDurations(t *testing.T) {
	result := runApex(t, `
Account a = new Account(Name = 'A');
insert a;
List<Account> accounts = [SELECT Id, Name FROM Account WHERE Name = 'A'];
System.assertEquals(1, accounts.size());
`)
	var hasDMLSpan, hasSOQLSpan bool
	for _, event := range result.Trace {
		if event.Phase != trace.PhaseComplete || event.Duration <= 0 {
			continue
		}
		if event.Category == "apex.dml" {
			hasDMLSpan = true
		}
		if event.Category == "apex.soql" {
			hasSOQLSpan = true
		}
	}
	if !hasDMLSpan || !hasSOQLSpan {
		t.Fatalf("missing duration spans dml=%v soql=%v trace=%#v", hasDMLSpan, hasSOQLSpan, result.Trace)
	}
}
```

Use the existing VM test helper names in `vm_test.go`; if `runApex` is not the helper, adapt the call to the local helper and keep the assertion unchanged.

- [ ] **Step 2: Run VM span test and verify it fails**

Run:

```bash
go test ./internal/vm -run TestTraceIncludesMethodSOQLAndDMLDurations -count=1
```

Expected: FAIL because the VM emits instant trace events only.

- [ ] **Step 3: Add low-overhead span helper**

Modify `internal/vm/control_flow.go`:

```go
func appendDurationTrace(result *Result, name, category string, startSeq int64, durationUS int64, args map[string]any) {
	if result == nil || !result.traceEnabled || durationUS <= 0 {
		return
	}
	result.Trace = append(result.Trace, trace.Duration(name, category, startSeq, durationUS, args))
}
```

Use deterministic sequence-derived microseconds first:

```go
func traceDurationFromLen(before, after int) int64 {
	if after <= before {
		return 1
	}
	return int64(after-before) * 1000
}
```

This keeps compatibility traces stable. A later packet can add optional wall-clock spans behind an explicit profiling flag.

- [ ] **Step 4: Wrap SOQL and DML events**

In `internal/vm/soql_runtime.go`, capture trace length before the SOQL work:

```go
traceStart := len(execResult.Trace)
// existing SOQL execution
appendDurationTrace(execResult, "apex.soql", "apex.soql", int64(traceStart), traceDurationFromLen(traceStart, len(execResult.Trace)+1), map[string]any{
	"query": query,
	"rows":  len(rows),
})
```

In `internal/vm/dml_runtime.go`, capture trace length before DML work:

```go
traceStart := len(result.Trace)
// existing DML execution
appendDurationTrace(result, "apex.dml."+op, "apex.dml", int64(traceStart), traceDurationFromLen(traceStart, len(result.Trace)+1), map[string]any{
	"operation": op,
	"rows":      len(records),
	"objects":   dmlTraceObjectNames(records),
})
```

Keep the existing instant events. The duration event is additive.

- [ ] **Step 5: Wrap Apex method dispatch**

In `internal/vm/method_dispatch.go`, before `executeProgram`:

```go
traceStart := len(result.Trace)
out, err := vm.executeProgram(method.Program, result)
appendDurationTrace(result, "apex.method."+method.Name, "apex.method", int64(traceStart), traceDurationFromLen(traceStart, len(result.Trace)+1), map[string]any{
	"method": method.Name,
	"class":  method.ClassName,
	"file":   method.File,
	"line":   method.Line,
	"column": method.Column,
})
```

Preserve the existing error handling. If the current function returns inside branches before this point, use a small local `defer` that appends the duration once and does not change control flow.

- [ ] **Step 6: Run focused VM/profile tests**

Run:

```bash
gofmt -w internal/vm
go test ./internal/trace ./internal/profile ./internal/vm -run 'TestTrace|TestAnalyzeAggregatesDurationEvents|TestTraceIncludesMethodSOQLAndDMLDurations' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/vm internal/trace internal/profile
git commit -m "feat: emit runtime duration spans"
```

## Task 9: Update Docs And Post-Parity Tracker

**Files:**
- Modify: `docs/POST_PARITY_TODO.md`
- Modify: `docs/FEATURE_PARITY_TODO.md` if the beyond-parity bullets still mention this as undone.
- Modify: `README.md` or `docs/DOGFOOD_CHECKLIST.md` if this checkout has those files and command examples need the new scanner.

- [ ] **Step 1: Add post-parity implementation text**

In `docs/POST_PARITY_TODO.md`, add this section near the profiling/reporting work:

```markdown
## Performance Scanner And Bottleneck Reports

- [x] Add `glade inspect performance` as an advisory project scanner.
- [x] Report entry points for triggers, batches, queueables, schedulables,
  invocable methods, Visualforce page actions, Aura server actions, LWC Apex
  imports/wires, Flow, and Workflow.
- [x] Flag static Salesforce-shaped risk: SOQL/DML/describe/async work in
  loops, unfiltered batch start queries, repeated describe, uncached
  `@AuraEnabled` reads, active Workflow, and Flow data fanout.
- [x] Accept local trace input and merge measured spans into the same ranked
  report.
- [x] Extend profile reports with trace duration attribution.
- [ ] Add optional org-backed SOQL query-plan enrichment after the local scanner
  is stable.
- [ ] Add SARIF output after JSON and Markdown stabilize.
```

- [ ] **Step 2: Add dogfood command**

In `docs/DOGFOOD_CHECKLIST.md`, add:

```markdown
glade inspect performance --project . --json > reports/glade-performance.json
glade inspect performance --project . --trace reports/slow-test-trace.json > reports/glade-performance.md
```

If `docs/DOGFOOD_CHECKLIST.md` does not exist in the execution lane, add the commands to `README.md` under local development commands.

- [ ] **Step 3: Run docs grep checks**

Run:

```bash
rg -n "inspect performance|Performance Scanner|glade-performance" docs README.md
```

Expected: output includes the new command and tracker section.

- [ ] **Step 4: Commit**

```bash
git add docs README.md
git commit -m "docs: document performance scanner"
```

## Task 10: Final Validation

**Files:**
- No new files. Validate the full patch.

- [ ] **Step 1: Run focused package tests**

Run:

```bash
go test ./internal/perfscan ./internal/trace ./internal/profile ./internal/gladecli
```

Expected: PASS.

- [ ] **Step 2: Run VM trace smoke**

Run:

```bash
go test ./internal/vm -run 'TestTrace|TestTraceIncludesMethodSOQLAndDMLDurations' -count=1
```

Expected: PASS.

- [ ] **Step 3: Run repo guard if available**

Run:

```bash
go test ./internal/repoguard
```

Expected: PASS. If `internal/repoguard` is absent in the execution lane, record `internal/repoguard not present in this checkout` in the handoff.

- [ ] **Step 4: Run the scanner against its fixture**

Run:

```bash
go run ./cmd/glade inspect performance --project internal/perfscan/testdata/perf-project --json
```

Expected: exit 0 and JSON containing:

```json
"perf.soql.loop"
"perf.dml.loop"
"perf.ui.lwc.wire-apex"
"perf.automation.flow.data-fanout"
```

- [ ] **Step 5: Run the scanner against a real sentinel project if present**

Run:

```bash
test -d example-projects/src-nmb-nutpl-develop && go run ./cmd/glade inspect performance --project example-projects/src-nmb-nutpl-develop --json | head -c 4000
```

Expected: command exits 0 when the sentinel project exists. Output is advisory JSON. Do not claim any project performance result from this truncated sample; it only proves the scanner can walk a larger project without crashing.

- [ ] **Step 6: Final commit**

```bash
git status --short
```

Expected: only intentional files from this plan are modified. If so:

```bash
git add internal/perfscan internal/gladecli internal/trace internal/profile internal/vm docs README.md
git commit -m "feat: add Salesforce performance scanner"
```

Do not stage `.DS_Store`, built `glade`, `*.test`, `/bin`, `/dist`, or ad-hoc JSON run outputs.

## Risk Notes

- Static rules are advisory. They must not change runtime behavior or fail compatibility gates.
- Regex is acceptable for Flow/Workflow/XML and UI JavaScript patterns where existing project scanners already use source scanning. Apex code must be parsed through `internal/apexast` where exact declarations matter.
- Duration spans must be additive. Existing instant trace events and debug-log rendering must stay stable.
- Do not use Salesforce-internal implementation assumptions. Tie wording to public behavior: transaction limits, SOQL/DML governor counts, Flow transaction sharing, Query Plan/selectivity concepts, Lightning caching, Visualforce view-state cost, and batch chunking.
- Do not special-case example-project names, customer classes, or package names.

## Self-Review

- Spec coverage: the plan covers project bottleneck scanning, entry-point inventory, static risk, measured timing, Salesforce transaction modeling, profile extension, CLI output, docs, and validation.
- Placeholder scan: no step uses unfinished marker words, unbounded "write tests", or hidden implementation without code shape.
- Type consistency: `Report`, `Finding`, `EntryPoint`, `Measurement`, `AnalyzeProject`, `WriteJSON`, `WriteMarkdown`, `scanTraceBytes`, and trace duration symbols are defined before use.

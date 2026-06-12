# Enterprise Modernization Workbench Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn Glade's existing parser, indexer, local test runner, trace, and report pieces into a first enterprise Apex modernization workbench that can assess a codebase, find conservative cruft candidates, and prove branch changes with local evidence.

**Architecture:** Build on existing product seams instead of adding a second tool. Keep implementation in `internal/gladecli`, `internal/enterprise`, `internal/enterprisegraph`, `internal/enterpriseassess`, `internal/enterprisecruft`, `internal/refactorproof`, and focused docs/fixtures. Reuse `project.Load`, `schema.LoadProject`, `typesys.Build`, `sema.Analyze`, `watch.GitChangesSince`, `apextest.RunCasesContext`, `internal/trace`, `internal/testreport`, and `internal/diagnostic`.

**Tech Stack:** Go 1.26, Glade's existing custom flag parser, custom `glade.yml` YAML subset parser, JSON/HTML/Markdown renderers, Chrome trace-event compatible runtime traces, current Apex parser/type index/sema/test runner, and existing CLI test harness.

---

## Review Of The Imported Plan

The imported plan was directionally useful but not wired to this repository.

Keep these ideas:

- Enterprise positioning: local proof engine for large Apex teams.
- Shared report schema with severity, confidence, evidence, recommendations, and artifacts.
- Conservative cruft classification. Never call public or global symbols safe to delete.
- Refactor proof as evidence collection, not automated rewriting.
- Report UX that starts with decisions and evidence.
- Fixture-backed service seams as later proof amplifiers.

Cut or defer these ideas:

- Do not create `cmd/glade/assess.go`, `cmd/glade/cruft.go`, or `cmd/glade/refactor.go`. This repo keeps `cmd/glade/main.go` thin and routes product commands through `internal/gladecli`.
- Do not build a new parser or broad AST engine. Use `internal/apexast`, `internal/typesys`, `internal/sema`, and file-level text scans where the current AST model does not expose bodies.
- Do not add all proposed top-level command roots in the first packet. Start under existing product roots where possible, then add short aliases only after the product surface proves itself.
- Do not treat service virtualization as overnight scope. The VM already emits traces for SOQL, DML, async, callouts, email, flows, Visualforce, and limits. First normalize and consume that evidence.
- Do not put compatibility ledgers, Salesforce docs inventory, or support-surface scanners back into base Glade. Those stay in first-party plugins.

Change the command shape:

```text
Phase 1 product surface:
  glade inspect graph --project . --json
  glade report assess --project . --format json|html|md --out reports/glade-assessment.html
  glade report cruft --project . --format json|html|md --out reports/glade-cruft.html
  glade report refactor-proof --project . --since origin/main --format json|html|md --out reports/glade-refactor-proof.html

Phase 2 optional aliases, after the root command review:
  glade assess --project .
  glade cruft scan --project .
  glade refactor prove --project . --since origin/main
```

This keeps the first implementation aligned with the current `inspect` and `report` product roots. The aliases can be thin wrappers once the evidence model works.

## Current Repo Facts

The current checkout is dirty and has unresolved merge conflicts in source files. Do not implement this plan in that worktree until conflicts are resolved or a fresh worktree is created from the intended base branch.

Observed unresolved paths:

```text
internal/apextest/runner.go
internal/apextest/runner_test.go
internal/cliui/doctor.go
internal/cliui/help.go
internal/gladecli/cli.go
scripts/release-build.sh
```

Useful existing pieces:

- `internal/project.Project` already inventories Apex, metadata, Visualforce, Aura, LWC, flows, workflows, named credentials, remote sites, profiles, permissions, layouts, tabs, and package dependencies.
- `internal/typesys.Index` already contains project info, Apex types, members, triggers, schema objects, dependencies, and diagnostics.
- `internal/sema.Analyze` already produces semantic diagnostics from the type index.
- `internal/watch.RefGraph` already builds a conservative Apex dependency graph for affected tests.
- `internal/watch.GitChangesSince` already collects tracked, staged, unstaged, and untracked changes since a ref.
- `internal/trace` already defines Chrome trace-event JSON. VM/test code already emits SOQL, DML, async, callout, event bus, flow, Visualforce, email, method, trigger, and limits events.
- `internal/testreport` already carries per-test trace/profile data.
- `internal/diagnostic` already writes JSON, text, SARIF, and GitHub annotations.
- `testdata/local-tests/enterprise-composed` already demonstrates enterprise-adjacent metadata, Visualforce, Aura component, flow, workflow, custom metadata, labels, and Apex.

## Product Boundary

Base Glade may own this plan because it analyzes and proves a user's local project. It is product work.

Base Glade must not absorb maintenance-only support machinery:

- No compat fixture runners.
- No surface ledger refresh/packet/post-parity commands.
- No Salesforce docs inventory mining.
- No generated compatibility dashboard commands.
- No project-corpus readiness scans.

If a task needs those, route it to the first-party compat or performance plugin and link from docs. The enterprise report may link to the support map; it must not regenerate the support map.

## Phase Order

Work phases in order. Each phase leaves a working product surface.

1. Phase 0: Prepare clean implementation ground.
2. Phase 1: Shared enterprise schema and report renderers.
3. Phase 2: Enterprise graph and `glade inspect graph`.
4. Phase 3: Assessment report and `glade report assess`.
5. Phase 4: Cruft scanner and `glade report cruft`.
6. Phase 5: Refactor proof and `glade report refactor-proof`.
7. Phase 6: Service config and trace normalization, no deep VM virtualization yet.
8. Phase 7: Docs, fixture, demo script, and alias decision.

Use parallel subagents inside each phase only after Phase 1 contracts pass. Shared files likely to conflict are `internal/gladecli/cli.go`, `internal/cliui/help.go`, and report renderers. One integration worker owns those files.

## File Map

Create these packages:

- `internal/enterprise/schema.go`: shared report, finding, severity, confidence, location, evidence, artifact, status, and trace-summary types.
- `internal/enterprise/render_json.go`: shared JSON writer.
- `internal/enterprise/render_markdown.go`: Markdown report writer used by PR summaries.
- `internal/enterprise/render_html.go`: dependency-free HTML report writer.
- `internal/enterprise/project.go`: helper that loads project, schema, index, sema, and project inventory.
- `internal/enterprise/trace_summary.go`: converts existing `trace.Event` values into enterprise trace counts.
- `internal/enterprisegraph/graph.go`: graph node/edge model.
- `internal/enterprisegraph/build.go`: graph builder from `typesys.Index`, `project.Project`, and shallow source scans.
- `internal/enterprisegraph/impact.go`: blast-radius helper from graph and git changes.
- `internal/enterprisegraph/metadata_refs.go`: metadata reference scans for flows, pages, components, Aura, LWC, labels, and named credentials.
- `internal/enterprisegraph/fflib.go`: fflib and layered-architecture indicators.
- `internal/enterpriseassess/assess.go`: assessment orchestration.
- `internal/enterpriseassess/risk.go`: transparent risk scoring.
- `internal/enterpriseassess/review_patterns.go`: first enterprise review checks from `docs/research/enterprise-code-review-patterns.md`.
- `internal/enterprisecruft/scan.go`: cruft scanner orchestration.
- `internal/enterprisecruft/classify.go`: conservative classification rules.
- `internal/enterprisecruft/dynamic_refs.go`: dynamic-reference risk scan.
- `internal/refactorproof/prove.go`: proof orchestration and stage status.
- `internal/refactorproof/gitdiff.go`: wrapper around `watch.GitChangesSince`.
- `internal/refactorproof/tests.go`: affected-test selection and optional local test run.
- `internal/refactorproof/api_surface.go`: public/global API delta warnings.
- `internal/gladecli/enterprise_graph_command.go`: `glade inspect graph`.
- `internal/gladecli/enterprise_report_command.go`: `glade report assess|cruft|refactor-proof`.

Modify these files:

- `internal/gladecli/cli.go`: route new `inspect graph` and `report` subcommands, later optional aliases.
- `internal/gladecli/cli_test.go`: CLI behavior tests.
- `internal/gladecli/test_command.go`: only in Phase 6, to add `--trace <path>` and optional `--services <path>` if the service packet proceeds.
- `internal/cliui/help.go`: help entries for `inspect graph` and new `report` subcommands.
- `docs/ARCHITECTURE.md`: short architecture note for enterprise reports.
- `docs/RICH_LOCAL_WORKFLOWS.md`: enterprise workflow entry point.
- `site/docs-src/guide/rich-local-workflows.md`: site copy for enterprise workflow.
- `site/docs-src/guide/cli-reference.md`: command reference.

Add or expand fixtures:

- `testdata/enterprise/mri-basic`: small static assessment fixture.
- `testdata/enterprise/cruft-basic`: conservative cruft fixture.
- `testdata/enterprise/refactor-proof`: git-diff and API-surface fixture.
- Reuse `testdata/local-tests/enterprise-composed` for end-to-end local runtime smoke.

## Phase 0: Clean Ground

**Files:**
- Modify none in this phase.

- [ ] **Step 1: Confirm current checkout status**

Run:

```bash
git status --short
```

Expected if implementing in the current checkout today: unresolved `UU` entries are present. Stop source implementation in this worktree until those are resolved.

- [ ] **Step 2: Create an isolated worktree**

Use the intended base. In this repo, local `main` is usually the right base unless the user names another branch.

```bash
git fetch --all --prune
git worktree add /tmp/glade-enterprise-workbench -b codex/enterprise-workbench main
cd /tmp/glade-enterprise-workbench
git status --short
```

Expected: clean status or only files created by the worktree setup.

- [ ] **Step 3: Run baseline focused tests**

```bash
go test ./internal/gladecli ./internal/cliui ./internal/project ./internal/typesys ./internal/sema ./internal/watch ./internal/testreport ./internal/trace -count=1
```

Expected: pass. If baseline fails, record the exact package and first failure. Do not repair unrelated failures in Phase 0.

- [ ] **Step 4: Check command baseline**

```bash
go run ./cmd/glade inspect symbols --project . --json >/tmp/glade-inspect-symbols.json
go run ./cmd/glade report --help >/tmp/glade-report-help.txt
go run ./cmd/glade test --project testdata/local-tests/basic --json >/tmp/glade-basic-tests.json
```

Expected: commands exit 0 and output files are nonempty.

- [ ] **Step 5: Commit nothing**

No code changes happen in Phase 0. It only proves the bench is level.

## Phase 1: Shared Enterprise Contracts

**Files:**
- Create: `internal/enterprise/schema.go`
- Create: `internal/enterprise/schema_test.go`
- Create: `internal/enterprise/render_json.go`
- Create: `internal/enterprise/render_json_test.go`
- Create: `internal/enterprise/render_markdown.go`
- Create: `internal/enterprise/render_markdown_test.go`
- Create: `internal/enterprise/render_html.go`
- Create: `internal/enterprise/render_html_test.go`
- Create: `internal/enterprise/project.go`
- Create: `internal/enterprise/project_test.go`
- Create: `internal/enterprise/trace_summary.go`
- Create: `internal/enterprise/trace_summary_test.go`

- [ ] **Step 1: Write schema tests**

Add `internal/enterprise/schema_test.go`:

```go
package enterprise

import (
	"encoding/json"
	"testing"
)

func TestReportJSONShape(t *testing.T) {
	report := Report{
		SchemaVersion: SchemaVersion,
		Command:       "glade report assess --project .",
		Project: ProjectSummary{
			Root:        ".",
			ApexClasses: 2,
			Triggers:    1,
			Tests:       1,
		},
		Findings: []Finding{{
			ID:         "ENT-RISK-001",
			Category:   CategoryArchitecture,
			Severity:   SeverityHigh,
			Confidence: ConfidenceMedium,
			Title:      "Trigger handler has high fan-out",
			Summary:    "AccountTriggerHandler has 12 downstream dependencies.",
			Symbol:     "AccountTriggerHandler",
			Location:   Location{File: "force-app/main/default/classes/AccountTriggerHandler.cls", LineStart: 10, LineEnd: 80},
			Evidence:   []Evidence{{Type: EvidenceGraph, Message: "12 outbound references found."}},
			Recommendation: "Add characterization tests before splitting the handler.",
			NextActions: []string{"glade inspect graph --project . --json"},
			Tags:        []string{"trigger-path", "fan-out"},
		}},
	}
	report.RefreshSummary()

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if got["schema_version"] != SchemaVersion {
		t.Fatalf("schema_version = %v", got["schema_version"])
	}
	summary := got["summary"].(map[string]any)
	if summary["high"].(float64) != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestSeverityAndConfidenceValidate(t *testing.T) {
	for _, severity := range []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo} {
		if !severity.Valid() {
			t.Fatalf("severity %q should be valid", severity)
		}
	}
	for _, confidence := range []Confidence{ConfidenceHigh, ConfidenceMedium, ConfidenceLow, ConfidenceUnknown} {
		if !confidence.Valid() {
			t.Fatalf("confidence %q should be valid", confidence)
		}
	}
	if Severity("urgent").Valid() {
		t.Fatalf("unexpected severity accepted")
	}
	if Confidence("certain").Valid() {
		t.Fatalf("unexpected confidence accepted")
	}
}
```

- [ ] **Step 2: Implement shared schema**

Add `internal/enterprise/schema.go`:

```go
package enterprise

import "time"

const SchemaVersion = "glade.enterprise.report/v0"

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

func (s Severity) Valid() bool {
	switch s {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo:
		return true
	default:
		return false
	}
}

type Confidence string

const (
	ConfidenceHigh    Confidence = "high"
	ConfidenceMedium  Confidence = "medium"
	ConfidenceLow     Confidence = "low"
	ConfidenceUnknown Confidence = "unknown"
)

func (c Confidence) Valid() bool {
	switch c {
	case ConfidenceHigh, ConfidenceMedium, ConfidenceLow, ConfidenceUnknown:
		return true
	default:
		return false
	}
}

type Category string

const (
	CategoryArchitecture Category = "architecture"
	CategoryCruft        Category = "cruft"
	CategoryRefactor     Category = "refactor_proof"
	CategoryRuntime      Category = "runtime"
	CategorySupport      Category = "support"
	CategoryInventory    Category = "inventory"
)

type EvidenceType string

const (
	EvidenceGraph    EvidenceType = "graph"
	EvidenceMetadata EvidenceType = "metadata"
	EvidenceRuntime  EvidenceType = "runtime"
	EvidenceSema     EvidenceType = "sema"
	EvidenceGit      EvidenceType = "git"
	EvidenceHeuristic EvidenceType = "heuristic"
)

type Status string

const (
	StatusPass        Status = "pass"
	StatusWarn        Status = "warn"
	StatusFail        Status = "fail"
	StatusNotRun      Status = "not_run"
	StatusUnsupported Status = "unsupported"
)

type Location struct {
	File      string `json:"file,omitempty"`
	LineStart int    `json:"line_start,omitempty"`
	LineEnd   int    `json:"line_end,omitempty"`
	ColumnStart int  `json:"column_start,omitempty"`
	ColumnEnd   int  `json:"column_end,omitempty"`
}

type Evidence struct {
	Type     EvidenceType       `json:"type"`
	Message  string             `json:"message"`
	Location *Location          `json:"location,omitempty"`
	Details  map[string]any     `json:"details,omitempty"`
}

type Finding struct {
	ID             string     `json:"id"`
	Category       Category   `json:"category"`
	Severity       Severity   `json:"severity"`
	Confidence     Confidence `json:"confidence"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	Symbol         string     `json:"symbol,omitempty"`
	Location       Location   `json:"location,omitempty"`
	Evidence       []Evidence `json:"evidence"`
	Recommendation string     `json:"recommendation"`
	NextActions    []string   `json:"next_actions,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
}

type ProjectSummary struct {
	Root          string `json:"root"`
	Namespace     string `json:"namespace,omitempty"`
	SourceAPIVersion string `json:"source_api_version,omitempty"`
	ApexClasses   int    `json:"apex_classes"`
	Triggers      int    `json:"triggers"`
	Tests         int    `json:"tests"`
	MetadataFiles int    `json:"metadata_files"`
}

type Summary struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
}

type Section struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Summary  string         `json:"summary,omitempty"`
	Items    []SectionItem  `json:"items,omitempty"`
}

type SectionItem struct {
	Label   string         `json:"label"`
	Value   string         `json:"value,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

type Artifact struct {
	Type string `json:"type"`
	Path string `json:"path"`
}

type TraceSummary struct {
	Events         int            `json:"events"`
	ByCategory     map[string]int `json:"by_category,omitempty"`
	ByName         map[string]int `json:"by_name,omitempty"`
	SOQLStatements int            `json:"soql_statements,omitempty"`
	DMLOperations  int            `json:"dml_operations,omitempty"`
	AsyncEvents    int            `json:"async_events,omitempty"`
	Callouts       int            `json:"callouts,omitempty"`
}

type Report struct {
	SchemaVersion string         `json:"schema_version"`
	GeneratedAt   time.Time      `json:"generated_at"`
	Command       string         `json:"command"`
	Project       ProjectSummary `json:"project"`
	Status        Status         `json:"status,omitempty"`
	Summary       Summary        `json:"summary"`
	Sections      []Section      `json:"sections"`
	Findings      []Finding      `json:"findings"`
	Artifacts     []Artifact     `json:"artifacts,omitempty"`
	Trace         *TraceSummary  `json:"trace,omitempty"`
	Limitations   []string       `json:"limitations,omitempty"`
}

func NewReport(command string, project ProjectSummary) Report {
	return Report{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Command:       command,
		Project:       project,
		Status:        StatusPass,
	}
}

func (r *Report) RefreshSummary() {
	r.Summary = Summary{}
	status := StatusPass
	for _, finding := range r.Findings {
		switch finding.Severity {
		case SeverityCritical:
			r.Summary.Critical++
			status = StatusFail
		case SeverityHigh:
			r.Summary.High++
			if status != StatusFail {
				status = StatusWarn
			}
		case SeverityMedium:
			r.Summary.Medium++
			if status == StatusPass {
				status = StatusWarn
			}
		case SeverityLow:
			r.Summary.Low++
		case SeverityInfo:
			r.Summary.Info++
		}
	}
	r.Status = status
}
```

- [ ] **Step 3: Write JSON renderer tests**

Add `internal/enterprise/render_json_test.go`:

```go
package enterprise

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	report := NewReport("glade report assess --project .", ProjectSummary{Root: ".", ApexClasses: 1})
	var buf bytes.Buffer
	if err := WriteJSON(&buf, report); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if !strings.Contains(buf.String(), "\n  ") {
		t.Fatalf("expected indented JSON, got %q", buf.String())
	}
	var decoded Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q", decoded.SchemaVersion)
	}
}
```

- [ ] **Step 4: Implement JSON renderer**

Add `internal/enterprise/render_json.go`:

```go
package enterprise

import (
	"encoding/json"
	"io"
)

func WriteJSON(w io.Writer, report Report) error {
	report.RefreshSummary()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
```

- [ ] **Step 5: Write Markdown and HTML renderer tests**

Add `internal/enterprise/render_markdown_test.go`:

```go
package enterprise

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteMarkdownIncludesEvidence(t *testing.T) {
	report := NewReport("glade report cruft --project .", ProjectSummary{Root: ".", ApexClasses: 1})
	report.Findings = []Finding{{
		ID: "ENT-CRUFT-001", Category: CategoryCruft, Severity: SeverityMedium, Confidence: ConfidenceHigh,
		Title: "Private method has no inbound references",
		Summary: "LegacyDiscountService.oldPath has no inbound graph references.",
		Evidence: []Evidence{{Type: EvidenceGraph, Message: "0 inbound references found."}},
		Recommendation: "Delete after affected tests pass.",
	}}
	var buf bytes.Buffer
	if err := WriteMarkdown(&buf, report); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"# Glade Enterprise Report", "ENT-CRUFT-001", "0 inbound references found.", "Delete after affected tests pass."} {
		if !strings.Contains(out, want) {
			t.Fatalf("markdown missing %q:\n%s", want, out)
		}
	}
}
```

Add `internal/enterprise/render_html_test.go`:

```go
package enterprise

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteHTMLEscapesFindingText(t *testing.T) {
	report := NewReport("glade report assess --project .", ProjectSummary{Root: "."})
	report.Findings = []Finding{{
		ID: "ENT-RISK-HTML", Category: CategoryArchitecture, Severity: SeverityHigh, Confidence: ConfidenceMedium,
		Title: "<script>alert(1)</script>",
		Summary: "Unsafe text must be escaped.",
		Evidence: []Evidence{{Type: EvidenceHeuristic, Message: "Observed <tag> in title."}},
		Recommendation: "Escape report fields.",
	}}
	var buf bytes.Buffer
	if err := WriteHTML(&buf, report); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Fatalf("HTML was not escaped:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("escaped title missing:\n%s", out)
	}
}
```

- [ ] **Step 6: Implement Markdown and HTML renderers**

Keep both renderers dependency-free. The HTML renderer should use `html/template`. It should not pull in a frontend stack.

Required functions:

```go
func WriteMarkdown(w io.Writer, report Report) error
func WriteHTML(w io.Writer, report Report) error
```

The Markdown layout must include:

```text
# Glade Enterprise Report
Command
Project
Status
Summary
Findings
Limitations
```

The HTML layout must include:

```text
header
dashboard counts
top findings
section list
limitations
raw artifact links
```

- [ ] **Step 7: Write project context tests**

Add `internal/enterprise/project_test.go`:

```go
package enterprise

import (
	"path/filepath"
	"testing"
)

func TestLoadContextBuildsProjectSummary(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	ctx, err := LoadContext(root)
	if err != nil {
		t.Fatalf("LoadContext: %v", err)
	}
	if ctx.Project.Root == "" {
		t.Fatalf("project root missing")
	}
	summary := ctx.Summary()
	if summary.ApexClasses == 0 {
		t.Fatalf("expected Apex classes in summary: %#v", summary)
	}
	if summary.MetadataFiles == 0 {
		t.Fatalf("expected metadata files in summary: %#v", summary)
	}
}
```

- [ ] **Step 8: Implement project context**

Add `internal/enterprise/project.go`:

```go
package enterprise

import (
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

type Context struct {
	Project project.Project
	Schema  gladeschema.Schema
	Index   typesys.Index
	Sema    sema.Result
}

func LoadContext(root string) (Context, error) {
	p, err := project.Load(root)
	if err != nil {
		return Context{}, err
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		return Context{}, err
	}
	idx := typesys.Build(p, s)
	return Context{Project: p, Schema: s, Index: idx, Sema: sema.Analyze(idx)}, nil
}

func (c Context) Summary() ProjectSummary {
	tests := 0
	apexClasses := 0
	for _, typ := range c.Index.Types {
		if typ.Dependency {
			continue
		}
		if typ.IsTest {
			tests++
		}
		if string(typ.Kind) == "class" {
			apexClasses++
		}
	}
	return ProjectSummary{
		Root:             c.Project.Root,
		Namespace:        c.Project.Namespace,
		SourceAPIVersion: c.Project.SourceAPIVersion,
		ApexClasses:      apexClasses,
		Triggers:         len(c.Index.Triggers),
		Tests:            tests,
		MetadataFiles:    countMetadataFiles(c.Project),
	}
}

func countMetadataFiles(p project.Project) int {
	return len(p.ObjectFiles) + len(p.FieldFiles) + len(p.FieldSetFiles) +
		len(p.RecordTypeFiles) + len(p.ValidationRuleFiles) + len(p.LabelFiles) +
		len(p.TranslationFiles) + len(p.StaticResourceFiles) + len(p.StaticResourceMetas) +
		len(p.DataWeaveFiles) + len(p.DataWeaveMetas) + len(p.ContentAssetFiles) +
		len(p.ContentAssetMetas) + len(p.EmailTemplateFiles) + len(p.FolderFiles) +
		len(p.NamedCredentialFiles) + len(p.RemoteSiteFiles) + len(p.CustomMetadataFiles) +
		len(p.WorkflowFiles) + len(p.FlowFiles) + len(p.ProfileFiles) +
		len(p.PermissionSetFiles) + len(p.PermissionSetGroupFiles) +
		len(p.PermissionAssignmentFiles) + len(p.ListViewFiles) + len(p.LayoutFiles) +
		len(p.CompactLayoutFiles) + len(p.TabFiles) + len(p.WebLinkFiles) +
		len(p.QuickActionFiles) + len(p.GlobalValueSetFiles) + len(p.StandardValueSetFiles) +
		len(p.FlexiPageFiles) + len(p.ApplicationFiles) + len(p.VisualforcePageFiles) +
		len(p.VisualforceComponentFiles) + len(p.AuraFiles) + len(p.LWCFiles) +
		len(p.LWCHTMLFiles) + len(p.LWCCSSFiles) + len(p.LWCMetaFiles)
}
```

- [ ] **Step 9: Write trace summary tests**

Add `internal/enterprise/trace_summary_test.go`:

```go
package enterprise

import (
	"testing"

	"github.com/glade-sh/glade/internal/trace"
)

func TestSummarizeTrace(t *testing.T) {
	events := []trace.Event{
		trace.Instant("apex.soql", "apex.soql", 1, nil),
		trace.Instant("apex.dml.insert", "apex.dml", 2, nil),
		trace.Instant("apex.async.enqueue", "apex.async", 3, nil),
		trace.Instant("apex.callout.http", "apex.callout", 4, nil),
	}
	got := SummarizeTrace(events)
	if got.Events != 4 || got.SOQLStatements != 1 || got.DMLOperations != 1 || got.AsyncEvents != 1 || got.Callouts != 1 {
		t.Fatalf("summary = %#v", got)
	}
	if got.ByCategory["apex.soql"] != 1 {
		t.Fatalf("category counts = %#v", got.ByCategory)
	}
}
```

- [ ] **Step 10: Implement trace summary**

Add `internal/enterprise/trace_summary.go`:

```go
package enterprise

import (
	"strings"

	"github.com/glade-sh/glade/internal/trace"
)

func SummarizeTrace(events []trace.Event) TraceSummary {
	out := TraceSummary{
		Events:     len(events),
		ByCategory: make(map[string]int),
		ByName:     make(map[string]int),
	}
	for _, event := range events {
		out.ByCategory[event.Category]++
		out.ByName[event.Name]++
		switch {
		case strings.HasPrefix(event.Category, "apex.soql"):
			out.SOQLStatements++
		case strings.HasPrefix(event.Category, "apex.dml"):
			out.DMLOperations++
		case strings.HasPrefix(event.Category, "apex.async"):
			out.AsyncEvents++
		case strings.HasPrefix(event.Category, "apex.callout"):
			out.Callouts++
		}
	}
	return out
}
```

- [ ] **Step 11: Run Phase 1 tests**

```bash
go test ./internal/enterprise -count=1
git diff --check
```

Expected: pass.

- [ ] **Step 12: Commit Phase 1**

```bash
git add internal/enterprise
git commit -m "feat: add enterprise report contracts"
```

## Phase 2: Enterprise Graph And `glade inspect graph`

**Files:**
- Create: `internal/enterprisegraph/graph.go`
- Create: `internal/enterprisegraph/graph_test.go`
- Create: `internal/enterprisegraph/build.go`
- Create: `internal/enterprisegraph/build_test.go`
- Create: `internal/enterprisegraph/impact.go`
- Create: `internal/enterprisegraph/impact_test.go`
- Create: `internal/enterprisegraph/metadata_refs.go`
- Create: `internal/enterprisegraph/metadata_refs_test.go`
- Create: `internal/enterprisegraph/fflib.go`
- Create: `internal/enterprisegraph/fflib_test.go`
- Create: `internal/gladecli/enterprise_graph_command.go`
- Create: `internal/gladecli/enterprise_graph_command_test.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/cliui/help.go`

- [ ] **Step 1: Write graph model tests**

Add `internal/enterprisegraph/graph_test.go`:

```go
package enterprisegraph

import "testing"

func TestGraphAddNodeAndEdgeDedupe(t *testing.T) {
	var g Graph
	g.AddNode(Node{ID: "class:AccountService", Kind: NodeClass, Name: "AccountService"})
	g.AddNode(Node{ID: "class:AccountService", Kind: NodeClass, Name: "AccountService"})
	g.AddEdge(Edge{From: "class:AccountService", To: "class:AccountSelector", Kind: EdgeReferences})
	g.AddEdge(Edge{From: "class:AccountService", To: "class:AccountSelector", Kind: EdgeReferences})
	if len(g.Nodes) != 1 {
		t.Fatalf("nodes = %d", len(g.Nodes))
	}
	if len(g.Edges) != 1 {
		t.Fatalf("edges = %d", len(g.Edges))
	}
}
```

- [ ] **Step 2: Implement graph model**

Add `internal/enterprisegraph/graph.go` with these exported types and helpers:

```go
package enterprisegraph

import "sort"

type NodeKind string

const (
	NodeClass        NodeKind = "class"
	NodeInterface    NodeKind = "interface"
	NodeEnum         NodeKind = "enum"
	NodeMethod       NodeKind = "method"
	NodeField        NodeKind = "field"
	NodeTrigger      NodeKind = "trigger"
	NodeTestMethod   NodeKind = "test_method"
	NodeMetadataFile NodeKind = "metadata_file"
	NodeSObject      NodeKind = "sobject"
	NodeEndpoint     NodeKind = "external_endpoint"
	NodePlatformEvent NodeKind = "platform_event"
)

type EdgeKind string

const (
	EdgeCalls              EdgeKind = "calls"
	EdgeReferences         EdgeKind = "references"
	EdgeExtends            EdgeKind = "extends"
	EdgeImplements         EdgeKind = "implements"
	EdgeQueries            EdgeKind = "queries"
	EdgeMutates            EdgeKind = "mutates"
	EdgeEnqueues           EdgeKind = "enqueues"
	EdgePublishes          EdgeKind = "publishes"
	EdgeCalloutTo          EdgeKind = "callout_to"
	EdgeTestCovers         EdgeKind = "test_covers"
	EdgeMetadataReferences EdgeKind = "metadata_references"
	EdgeExposesAPI         EdgeKind = "exposes_api"
)

type Node struct {
	ID       string            `json:"id"`
	Kind     NodeKind          `json:"kind"`
	Name     string            `json:"name"`
	File     string            `json:"file,omitempty"`
	Line     int               `json:"line,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Edge struct {
	From     string            `json:"from"`
	To       string            `json:"to"`
	Kind     EdgeKind          `json:"kind"`
	Evidence string            `json:"evidence,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
	nodeSeen map[string]struct{}
	edgeSeen map[string]struct{}
}

func (g *Graph) AddNode(node Node) {
	if g.nodeSeen == nil {
		g.nodeSeen = make(map[string]struct{})
	}
	if _, ok := g.nodeSeen[node.ID]; ok {
		return
	}
	g.nodeSeen[node.ID] = struct{}{}
	g.Nodes = append(g.Nodes, node)
}

func (g *Graph) AddEdge(edge Edge) {
	if edge.From == "" || edge.To == "" || edge.Kind == "" {
		return
	}
	if g.edgeSeen == nil {
		g.edgeSeen = make(map[string]struct{})
	}
	key := edge.From + "\x00" + string(edge.Kind) + "\x00" + edge.To
	if _, ok := g.edgeSeen[key]; ok {
		return
	}
	g.edgeSeen[key] = struct{}{}
	g.Edges = append(g.Edges, edge)
}

func (g *Graph) Sort() {
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].ID < g.Nodes[j].ID })
	sort.Slice(g.Edges, func(i, j int) bool {
		a := g.Edges[i].From + string(g.Edges[i].Kind) + g.Edges[i].To
		b := g.Edges[j].From + string(g.Edges[j].Kind) + g.Edges[j].To
		return a < b
	})
}
```

- [ ] **Step 3: Write graph build tests**

Add `internal/enterprisegraph/build_test.go`:

```go
package enterprisegraph

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/enterprise"
)

func TestBuildGraphFromEnterpriseComposed(t *testing.T) {
	ctx, err := enterprise.LoadContext(filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed"))
	if err != nil {
		t.Fatalf("LoadContext: %v", err)
	}
	graph, err := Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(graph.Nodes) == 0 {
		t.Fatalf("expected graph nodes")
	}
	if !hasNodeKind(graph, NodeClass) {
		t.Fatalf("expected class nodes: %#v", graph.Nodes)
	}
	if !hasNodeKind(graph, NodeTrigger) && len(ctx.Index.Triggers) > 0 {
		t.Fatalf("expected trigger nodes")
	}
}

func hasNodeKind(g Graph, kind NodeKind) bool {
	for _, node := range g.Nodes {
		if node.Kind == kind {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Implement graph builder**

Use `typesys.Index` as the source of truth. For member call edges, start with conservative text scans against known type/member names. Matches inside strings or comments may over-select; mark evidence as `identifier scan` and keep confidence handling in assessment/cruft.

Required exported function:

```go
func Build(ctx enterprise.Context) (Graph, error)
```

Implementation requirements:

- Add nodes for project Apex types and members.
- Add nodes for triggers.
- Add nodes for schema objects.
- Add `extends` and `implements` edges from type symbols.
- Add `references` edges from source identifier scans against known type names.
- Add `queries` edges when source contains `FROM <ObjectName>` for known object names.
- Add `mutates` edges when source contains `insert`, `update`, `upsert`, `delete`, `undelete`, or `merge` near a known object variable is not required in Phase 2. For the first pass, emit type-level `mutates` evidence when DML keywords appear in the file.
- Add metadata nodes and `metadata_references` edges in `metadata_refs.go`.
- Call `graph.Sort()` before returning.

- [ ] **Step 5: Write impact tests**

Add `internal/enterprisegraph/impact_test.go`:

```go
package enterprisegraph

import "testing"

func TestImpactWalksReverseEdges(t *testing.T) {
	var g Graph
	g.AddNode(Node{ID: "class:PricingService", Kind: NodeClass, Name: "PricingService"})
	g.AddNode(Node{ID: "class:AccountService", Kind: NodeClass, Name: "AccountService"})
	g.AddNode(Node{ID: "class:AccountServiceTest", Kind: NodeClass, Name: "AccountServiceTest", Metadata: map[string]string{"test": "true"}})
	g.AddEdge(Edge{From: "class:AccountService", To: "class:PricingService", Kind: EdgeReferences})
	g.AddEdge(Edge{From: "class:AccountServiceTest", To: "class:AccountService", Kind: EdgeReferences})

	impact := Impact(g, []string{"class:PricingService"})
	if !contains(impact.ImpactedNodes, "class:AccountService") {
		t.Fatalf("impact = %#v", impact)
	}
	if !contains(impact.RecommendedTests, "AccountServiceTest") {
		t.Fatalf("recommended tests = %#v", impact.RecommendedTests)
	}
}
```

- [ ] **Step 6: Implement impact**

Add `internal/enterprisegraph/impact.go` with:

```go
type ImpactResult struct {
	Roots            []string `json:"roots"`
	ImpactedNodes    []string `json:"impacted_nodes"`
	RecommendedTests []string `json:"recommended_tests"`
	RiskNotes        []string `json:"risk_notes,omitempty"`
}

func Impact(g Graph, roots []string) ImpactResult
```

Walk reverse edges. Treat a node as a test when `node.Metadata["test"] == "true"` or its name ends with `Test`.

- [ ] **Step 7: Write metadata reference tests**

Use small temporary XML files in the test. Do not depend on a full Salesforce XML parser.

Expected behavior:

- `LegacyWorkOrderReview.page` referencing `LegacyWorkOrderReviewExtension` creates a metadata reference.
- Aura `.cmp` and LWC `.js` text that names an Apex class creates a metadata reference.
- Flow XML that names an invocable class creates a metadata reference.

- [ ] **Step 8: Implement metadata reference scanner**

Read files listed on `project.Project`. Use string/regexp scans, not XML DOM parsing, for Phase 2:

- Visualforce pages/components: `controller="ClassName"`, `extensions="ClassName"`, `<apex:component controller="ClassName">`.
- Aura files: references to known class names and `@AuraEnabled` targets by name.
- LWC JavaScript files: `@salesforce/apex/ClassName.methodName`.
- Flow XML: `<apexClass>ClassName</apexClass>` and text references to known invocable class names.
- Labels/custom metadata: references reduce cruft confidence later.

- [ ] **Step 9: Write fflib inventory tests**

Create source snippets that include:

```apex
public class AccountDomain extends fflib_SObjectDomain {}
public class AccountSelector {
  private static final String MOCK = 'AccountSelector.selectById';
}
```

Expected: inventory reports domain pattern and selector naming.

- [ ] **Step 10: Implement fflib inventory**

Add:

```go
type FFLibInventory struct {
	DomainClasses []string `json:"domain_classes,omitempty"`
	SelectorClasses []string `json:"selector_classes,omitempty"`
	ServiceClasses []string `json:"service_classes,omitempty"`
	UnitOfWorkUsers []string `json:"unit_of_work_users,omitempty"`
	FactoryUsers []string `json:"factory_users,omitempty"`
}

func DetectFFLib(ctx enterprise.Context) FFLibInventory
```

Use class names, superclasses, interfaces, and source text. Keep this heuristic.

- [ ] **Step 11: Write CLI tests**

Add `internal/gladecli/enterprise_graph_command_test.go`:

```go
package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInspectGraphJSON(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "graph", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, stderr.String())
	}
	var got struct {
		Nodes []map[string]any `json:"nodes"`
		Edges []map[string]any `json:"edges"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not graph JSON: %v\n%s", err, stdout.String())
	}
	if len(got.Nodes) == 0 {
		t.Fatalf("expected nodes")
	}
}

func TestRunInspectGraphHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "graph", "--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "glade inspect graph") {
		t.Fatalf("help = %q", stdout.String())
	}
}
```

- [ ] **Step 12: Implement `glade inspect graph`**

Add `internal/gladecli/enterprise_graph_command.go`:

```go
package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/glade-sh/glade/internal/enterprise"
	"github.com/glade-sh/glade/internal/enterprisegraph"
	"github.com/glade-sh/glade/internal/flagparse"
)

func runInspectGraph(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) > 0 && isHelpArg(args[0]) {
		_, _ = fmt.Fprintln(w, "usage: glade inspect graph [--project <root>] [--json]")
		return nil
	}
	parsed, err := flagparse.New("glade inspect graph").
		String("project", "p").
		Bool("json", "j").
		Parse(args)
	if err != nil {
		return err
	}
	root := "."
	if parsed.String("project") != "" {
		root = parsed.String("project")
	}
	ctxData, err := enterprise.LoadContext(root)
	if err != nil {
		return err
	}
	graph, err := enterprisegraph.Build(ctxData)
	if err != nil {
		return err
	}
	if !parsed.Bool("json") {
		return errors.New("inspect graph currently requires --json")
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(graph)
}
```

Modify `runInspect` in `internal/gladecli/cli.go` so `glade inspect graph` routes before existing symbol output.

- [ ] **Step 13: Update help registry**

Add `graph` to the `inspect` help in `internal/cliui/help.go`. Include:

```text
glade inspect graph --project . --json
```

- [ ] **Step 14: Run Phase 2 tests**

```bash
go test ./internal/enterprise ./internal/enterprisegraph ./internal/gladecli ./internal/cliui -count=1
go run ./cmd/glade inspect graph --project testdata/local-tests/enterprise-composed --json >/tmp/glade-enterprise-graph.json
git diff --check
```

Expected: tests pass; graph JSON is nonempty.

- [ ] **Step 15: Commit Phase 2**

```bash
git add internal/enterprisegraph internal/gladecli internal/cliui
git commit -m "feat: add enterprise graph inspection"
```

## Phase 3: Assessment Report And `glade report assess`

**Files:**
- Create: `internal/enterpriseassess/assess.go`
- Create: `internal/enterpriseassess/assess_test.go`
- Create: `internal/enterpriseassess/risk.go`
- Create: `internal/enterpriseassess/risk_test.go`
- Create: `internal/enterpriseassess/review_patterns.go`
- Create: `internal/enterpriseassess/review_patterns_test.go`
- Create or modify: `internal/gladecli/enterprise_report_command.go`
- Create: `internal/gladecli/enterprise_report_command_test.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/cliui/help.go`

- [ ] **Step 1: Write risk tests**

Add `internal/enterpriseassess/risk_test.go`:

```go
package enterpriseassess

import (
	"testing"

	"github.com/glade-sh/glade/internal/enterprisegraph"
)

func TestScoreNodeExplainsInputs(t *testing.T) {
	node := enterprisegraph.Node{
		ID: "class:AccountTriggerHandler", Kind: enterprisegraph.NodeClass, Name: "AccountTriggerHandler",
		Metadata: map[string]string{"loc": "350", "methods": "22", "visibility": "public", "test": "false"},
	}
	score := ScoreNode(node, GraphStats{FanOut: 12, FanIn: 3, SOQL: 4, DML: 2, TriggerPath: true})
	if score.Value == 0 {
		t.Fatalf("expected risk score")
	}
	if len(score.Evidence) == 0 {
		t.Fatalf("expected evidence")
	}
	if score.Severity != "high" {
		t.Fatalf("severity = %q score=%#v", score.Severity, score)
	}
}
```

- [ ] **Step 2: Implement transparent risk scoring**

Add:

```go
type GraphStats struct {
	FanIn int
	FanOut int
	SOQL int
	DML int
	Async int
	Callout int
	TriggerPath bool
	DynamicReference bool
}

type RiskScore struct {
	Value int
	Severity enterprise.Severity
	Evidence []enterprise.Evidence
}

func ScoreNode(node enterprisegraph.Node, stats GraphStats) RiskScore
```

Scoring must be explicit:

```text
+20 trigger path
+15 fan-out >= 10
+10 fan-in >= 10
+15 DML > 0
+10 SOQL >= 3
+10 public/global visibility
+10 no test indicator
+10 dynamic reference indicator
```

Severity mapping:

```text
>=60 high
>=35 medium
>=15 low
else info
```

Do not emit `critical` in Phase 3 unless a parse/sema error prevents report generation.

- [ ] **Step 3: Write assessment tests**

Add `internal/enterpriseassess/assess_test.go`:

```go
package enterpriseassess

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/enterprise"
	"github.com/glade-sh/glade/internal/enterprisegraph"
)

func TestAssessProducesInventoryAndLimitations(t *testing.T) {
	ctx, err := enterprise.LoadContext(filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed"))
	if err != nil {
		t.Fatalf("LoadContext: %v", err)
	}
	graph, err := enterprisegraph.Build(ctx)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	report := Assess(ctx, graph, Options{IncludeMetadata: true, IncludeTests: true})
	if report.SchemaVersion != enterprise.SchemaVersion {
		t.Fatalf("schema = %q", report.SchemaVersion)
	}
	if len(report.Sections) == 0 {
		t.Fatalf("expected sections")
	}
	if len(report.Limitations) == 0 {
		t.Fatalf("expected honest limitations")
	}
}
```

- [ ] **Step 4: Implement assessment orchestration**

Add:

```go
type Options struct {
	IncludeMetadata bool
	IncludeTests bool
	Strict bool
}

func Assess(ctx enterprise.Context, graph enterprisegraph.Graph, opts Options) enterprise.Report
```

Report sections:

```text
inventory
top_risks
trigger_map
soql_dml_map
async_callout_surface
fflib_inventory
test_health
limitations
```

Required limitations:

```text
Static graph uses conservative identifier scans and may over-select references.
Dynamic Apex, string-based factories, and custom metadata routing reduce confidence.
Report does not claim full Salesforce runtime parity.
Support-map coverage comes from checked Glade docs and plugin-generated artifacts, not live org introspection.
```

- [ ] **Step 5: Write enterprise review pattern tests**

Use snippets from `docs/research/enterprise-code-review-patterns.md`. Add tests for:

- Empty catch where caught variable is unused.
- Duplicate `mockId('...')`.
- `System.debug([SELECT ...])`.
- API version drift from project default to `.cls-meta.xml`.

Expected IDs:

```text
ENT-REVIEW-EMPTY-CATCH
ENT-REVIEW-DUPLICATE-MOCK-ID
ENT-REVIEW-DEBUG-SOQL
ENT-REVIEW-API-VERSION-DRIFT
```

- [ ] **Step 6: Implement first review patterns**

Use source text and regexp in Phase 3. Do not block on body-level AST.

Rules:

- Empty catch: find `catch (...) {}` or a catch body that does not reference the caught variable and contains no `throw`.
- Duplicate mock ID: find repeated `mockId('value')` or `mockId("value")`.
- Debug SOQL: find `System.debug([SELECT`.
- API version drift: compare `project.SourceAPIVersion` to `<apiVersion>` in adjacent `*.cls-meta.xml` files when present.

Every finding must include file evidence and confidence. Use `medium` if line precision is approximate.

- [ ] **Step 7: Write CLI tests**

Add `internal/gladecli/enterprise_report_command_test.go`:

```go
package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReportAssessJSON(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"report", "assess", "--project", root, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if got["schema_version"] != "glade.enterprise.report/v0" {
		t.Fatalf("report = %#v", got)
	}
}

func TestRunReportAssessWritesHTML(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	out := filepath.Join(t.TempDir(), "assessment.html")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"report", "assess", "--project", root, "--format", "html", "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(data), "Glade Enterprise Report") {
		t.Fatalf("html = %s", string(data))
	}
}
```

- [ ] **Step 8: Implement `glade report assess`**

In `enterprise_report_command.go`, implement shared format handling:

```go
type enterpriseReportOptions struct {
	Root string
	Format string
	Out string
	IncludeMetadata bool
	IncludeTests bool
	Strict bool
	Since string
}
```

Accepted formats:

```text
json
html
md
```

Output behavior:

- If `--out` is empty, write to stdout.
- If `--out` is set, create parent directories and write the file.
- For HTML and Markdown, stdout should print `report: <path>` when `--out` is used.

Route from existing `runReport` without breaking current report behavior. If current `runReport` only handles existing saved reports, add a subcommand branch:

```go
switch args[0] {
case "assess":
	return runEnterpriseAssessReport(ctx, args[1:], w)
case "cruft":
	return runEnterpriseCruftReport(ctx, args[1:], w)
case "refactor-proof":
	return runEnterpriseRefactorProofReport(ctx, args[1:], w, progressW)
default:
	return runExistingReportCommand(args, w)
}
```

If there is no `progressW` in the current `runReport` signature, do not add progress in Phase 3.

- [ ] **Step 9: Update help**

Update `internal/cliui/help.go`:

```text
glade report assess --project . --format html --out reports/glade-assessment.html
```

- [ ] **Step 10: Run Phase 3 tests**

```bash
go test ./internal/enterprise ./internal/enterprisegraph ./internal/enterpriseassess ./internal/gladecli ./internal/cliui -count=1
go run ./cmd/glade report assess --project testdata/local-tests/enterprise-composed --format json >/tmp/glade-assessment.json
go run ./cmd/glade report assess --project testdata/local-tests/enterprise-composed --format html --out /tmp/glade-assessment.html
git diff --check
```

Expected: tests pass; JSON and HTML reports are nonempty.

- [ ] **Step 11: Commit Phase 3**

```bash
git add internal/enterpriseassess internal/gladecli internal/cliui
git commit -m "feat: add enterprise assessment report"
```

## Phase 4: Cruft Scanner And `glade report cruft`

**Files:**
- Create: `internal/enterprisecruft/scan.go`
- Create: `internal/enterprisecruft/scan_test.go`
- Create: `internal/enterprisecruft/classify.go`
- Create: `internal/enterprisecruft/classify_test.go`
- Create: `internal/enterprisecruft/dynamic_refs.go`
- Create: `internal/enterprisecruft/dynamic_refs_test.go`
- Modify: `internal/gladecli/enterprise_report_command.go`
- Modify: `internal/gladecli/enterprise_report_command_test.go`

- [ ] **Step 1: Write classification tests**

Add `internal/enterprisecruft/classify_test.go`:

```go
package enterprisecruft

import "testing"

func TestClassifyProtectsGlobalSymbols(t *testing.T) {
	got := Classify(SymbolFacts{
		Name: "PackageApi",
		Visibility: "global",
		InboundRefs: 0,
		MetadataRefs: 0,
	})
	if got.Bucket != BucketPackageContractDoNotDelete {
		t.Fatalf("bucket = %q", got.Bucket)
	}
	if got.Confidence != "high" {
		t.Fatalf("confidence = %q", got.Confidence)
	}
}

func TestClassifyPrivateNoRefsAsSafeDeleteCandidate(t *testing.T) {
	got := Classify(SymbolFacts{
		Name: "LegacyDiscountService.oldPath",
		Visibility: "private",
		InboundRefs: 0,
		MetadataRefs: 0,
	})
	if got.Bucket != BucketSafeDeleteCandidate {
		t.Fatalf("bucket = %q", got.Bucket)
	}
	if got.Confidence != "high" {
		t.Fatalf("confidence = %q", got.Confidence)
	}
}

func TestClassifyDynamicReferenceRiskReducesConfidence(t *testing.T) {
	got := Classify(SymbolFacts{
		Name: "FactoryTarget",
		Visibility: "private",
		InboundRefs: 0,
		DynamicRisk: true,
	})
	if got.Bucket != BucketReviewDynamicReferenceRisk {
		t.Fatalf("bucket = %q", got.Bucket)
	}
}
```

- [ ] **Step 2: Implement classification**

Add buckets:

```go
type Bucket string

const (
	BucketSafeDeleteCandidate Bucket = "safe_delete_candidate"
	BucketSafeDeprecateCandidate Bucket = "safe_deprecate_candidate"
	BucketReviewDynamicReferenceRisk Bucket = "review_dynamic_reference_risk"
	BucketPackageContractDoNotDelete Bucket = "package_contract_do_not_delete"
	BucketRuntimeCharacterizationNeeded Bucket = "runtime_characterization_needed"
	BucketTestOnlyCleanup Bucket = "test_only_cleanup"
	BucketUnknown Bucket = "unknown"
)
```

Rules:

- Global symbols always `package_contract_do_not_delete`.
- Public symbols with zero inbound refs default to `safe_deprecate_candidate`, not delete.
- Private symbols with zero inbound refs and no dynamic/metadata refs become `safe_delete_candidate`.
- Any dynamic risk becomes `review_dynamic_reference_risk`.
- Test-only helpers with one inbound test ref become `test_only_cleanup`.
- Parsed failures or missing graph context become `unknown`.

- [ ] **Step 3: Write dynamic reference tests**

Test detection for:

```text
Type.forName
JSON.deserialize
Application.Service.newInstance
Custom Metadata type names in string literals
@AuraEnabled
@InvocableMethod
```

- [ ] **Step 4: Implement dynamic reference scanner**

Use source text and symbol modifiers. Return per-symbol risk notes. Do not try to prove dynamic dispatch target resolution in Phase 4.

- [ ] **Step 5: Write scanner tests**

Use an in-memory graph fixture or `testdata/enterprise/cruft-basic`. The scanner must emit:

- A private unused method candidate.
- A public deprecate candidate.
- A global do-not-delete warning.
- A dynamic-reference review candidate.

- [ ] **Step 6: Implement scanner**

Required function:

```go
func Scan(ctx enterprise.Context, graph enterprisegraph.Graph) enterprise.Report
```

Every finding title must include the bucket action:

```text
Safe-delete candidate: private method has no inbound references
Deprecate candidate: public class has no inbound references
Do not delete: global symbol is package-facing
Review dynamic-reference risk: string or factory dispatch detected nearby
```

Every finding recommendation must say what to do next, not just what was found.

- [ ] **Step 7: Implement `glade report cruft`**

Route:

```bash
glade report cruft --project . --format json
glade report cruft --project . --format html --out reports/glade-cruft.html
```

Do not add deletion patches. This phase only reports.

- [ ] **Step 8: Run Phase 4 tests**

```bash
go test ./internal/enterprise ./internal/enterprisegraph ./internal/enterprisecruft ./internal/gladecli -count=1
go run ./cmd/glade report cruft --project testdata/local-tests/enterprise-composed --format json >/tmp/glade-cruft.json
git diff --check
```

Expected: pass. Report must not mark public or global symbols as safe-delete candidates.

- [ ] **Step 9: Commit Phase 4**

```bash
git add internal/enterprisecruft internal/gladecli
git commit -m "feat: add enterprise cruft report"
```

## Phase 5: Refactor Proof And `glade report refactor-proof`

**Files:**
- Create: `internal/refactorproof/prove.go`
- Create: `internal/refactorproof/prove_test.go`
- Create: `internal/refactorproof/gitdiff.go`
- Create: `internal/refactorproof/gitdiff_test.go`
- Create: `internal/refactorproof/tests.go`
- Create: `internal/refactorproof/tests_test.go`
- Create: `internal/refactorproof/api_surface.go`
- Create: `internal/refactorproof/api_surface_test.go`
- Modify: `internal/gladecli/enterprise_report_command.go`
- Modify: `internal/gladecli/enterprise_report_command_test.go`

- [ ] **Step 1: Write proof status tests**

Add `internal/refactorproof/prove_test.go`:

```go
package refactorproof

import (
	"testing"

	"github.com/glade-sh/glade/internal/enterprise"
)

func TestProofStatusWarnsOnMissingTests(t *testing.T) {
	result := Result{
		Stages: []StageResult{
			{Name: StageGitDiff, Status: enterprise.StatusPass},
			{Name: StageParseCheck, Status: enterprise.StatusPass},
			{Name: StageAffectedTests, Status: enterprise.StatusNotRun, Message: "No affected tests selected."},
		},
	}
	if got := result.Status(); got != enterprise.StatusWarn {
		t.Fatalf("status = %q", got)
	}
}

func TestProofStatusFailsOnSemaError(t *testing.T) {
	result := Result{Stages: []StageResult{{Name: StageSemanticCheck, Status: enterprise.StatusFail}}}
	if got := result.Status(); got != enterprise.StatusFail {
		t.Fatalf("status = %q", got)
	}
}
```

- [ ] **Step 2: Implement proof result types**

Add:

```go
type Stage string

const (
	StageGitDiff Stage = "git_diff"
	StageParseCheck Stage = "parse_check"
	StageSemanticCheck Stage = "semantic_check"
	StageGraphImpact Stage = "graph_impact"
	StageAffectedTests Stage = "affected_tests"
	StageRuntimeTrace Stage = "runtime_trace"
	StageAPISurfaceDelta Stage = "api_surface_delta"
)

type StageResult struct {
	Name Stage `json:"name"`
	Status enterprise.Status `json:"status"`
	Message string `json:"message,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

type Options struct {
	Root string
	Since string
	RunTests bool
	TracePath string
	FailOnAPIBreak bool
}

type Result struct {
	Stages []StageResult `json:"stages"`
	Report enterprise.Report `json:"report"`
}
```

Status rules:

- Any `fail` stage makes result `fail`.
- Any `warn`, `not_run`, or `unsupported` stage makes result `warn` unless a fail exists.
- All pass makes result `pass`.

- [ ] **Step 3: Write git diff wrapper tests**

Use a temporary git repo. Create one Apex class, commit it, modify it, then call wrapper. Expected: changed Apex file is returned.

If git is unavailable, skip test with a clear `t.Skip`.

- [ ] **Step 4: Implement git diff wrapper**

Wrap `watch.GitChangesSince(root, ref)`. Convert changes into:

```go
type ChangedFile struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Operation string `json:"operation"`
	Symbol string `json:"symbol,omitempty"`
}
```

- [ ] **Step 5: Write API surface delta tests**

Create before/after symbol sets:

- Removing a `global` method should create a high-severity API break finding.
- Removing a private method should not create an API break.
- Changing public/global method signature should warn.

- [ ] **Step 6: Implement API surface checks**

Use `typesys.Index` before and after when available. For Phase 5, compare current index to baseline by checking deleted/modified files is enough:

- If a changed file contains `global` or `public` declarations, emit warning finding.
- If a deleted file was global/public in baseline, emit fail finding.

Do not overclaim signature diff precision unless both before and after indexes were built.

- [ ] **Step 7: Write affected tests tests**

Use `internal/watch` changed-since machinery where possible. If direct exported affected-test APIs are not enough, add a small exported helper to `internal/watch` instead of duplicating the graph. The helper should be:

```go
func AffectedTests(index typesys.Index, changes []Change) []string
```

Add tests under `internal/watch` for the helper if exported.

- [ ] **Step 8: Implement affected test selection**

Preferred path:

- Add exported `AffectedTests` wrapper to `internal/watch` around `BuildReferenceGraph(index)` and `affectedTests`.
- Use that from `internal/refactorproof/tests.go`.

Do not duplicate `RefGraph` internals in `refactorproof`.

- [ ] **Step 9: Implement proof orchestration**

Required:

```go
func Prove(ctx context.Context, opts Options) (Result, error)
```

Pipeline:

1. Load enterprise context for current project.
2. Run sema and fail/warn based on diagnostics.
3. Get git changes since `opts.Since`.
4. Build enterprise graph and impact.
5. Select affected tests.
6. If `opts.RunTests`, run selected tests with `apextest.RunCasesContext`; otherwise mark affected tests `not_run` with exact count.
7. Summarize traces from test results when available.
8. Add API surface warnings.
9. Produce enterprise report with sections and findings.

Default CLI behavior should run tests only when `--run-tests` is set in Phase 5. This keeps the first report fast and avoids surprising long runs.

- [ ] **Step 10: Implement CLI route**

Command:

```bash
glade report refactor-proof --project . --since origin/main --format json
glade report refactor-proof --project . --since origin/main --run-tests --format html --out reports/glade-refactor-proof.html
```

Flags:

```text
--project <root>
--since <ref>
--run-tests
--format json|html|md
--out <path>
--fail-on-api-break
```

Exit code:

- 0 for pass or warn by default.
- 1 for fail.
- 1 for API break when `--fail-on-api-break` is set.

- [ ] **Step 11: Run Phase 5 tests**

```bash
go test ./internal/watch ./internal/refactorproof ./internal/gladecli -count=1
go run ./cmd/glade report refactor-proof --project . --since HEAD --format json >/tmp/glade-refactor-proof.json
git diff --check
```

Expected: tests pass. Refactor proof emits a report even when no changed files exist.

- [ ] **Step 12: Commit Phase 5**

```bash
git add internal/refactorproof internal/watch internal/gladecli
git commit -m "feat: add refactor proof report"
```

## Phase 6: Trace Normalization And Service Config Skeleton

**Files:**
- Create: `internal/enterprise/services.go`
- Create: `internal/enterprise/services_test.go`
- Modify: `internal/gladecli/test_command.go`
- Modify: `internal/gladecli/cli_test.go`
- Modify: `docs/CONFIG.md`

This phase is intentionally smaller than the imported plan. The VM already emits many useful trace events. Start by exposing trace output for tests and validating a small service config. Do not implement full callout fixture injection here unless a runtime hook already exists.

- [ ] **Step 1: Write service config parser tests**

Use the existing custom YAML subset style. Since current `go.mod` has no YAML dependency, do not add `gopkg.in/yaml.v3` just for this packet.

Accept this subset in `.glade/services.yml`:

```yaml
version: 0
mode: strict
calloutFixtures: [fixtures/callouts/pricing-success.json]
asyncDrain: true
asyncMaxDepth: 5
platformEventsOut: reports/platform-events.jsonl
```

Tests:

- Valid file parses.
- Missing fixture path fails with file path in error.
- `asyncMaxDepth: 0` fails when `asyncDrain: true`.
- Unknown key fails.

- [ ] **Step 2: Implement service config model and validator**

Add:

```go
type ServiceConfig struct {
	Version int `json:"version"`
	Mode string `json:"mode"`
	CalloutFixtures []string `json:"calloutFixtures,omitempty"`
	AsyncDrain bool `json:"asyncDrain,omitempty"`
	AsyncMaxDepth int `json:"asyncMaxDepth,omitempty"`
	PlatformEventsOut string `json:"platformEventsOut,omitempty"`
}

func LoadServiceConfig(path string) (ServiceConfig, error)
func ValidateServiceConfig(path string) error
```

This is a validation skeleton. It does not change VM callout behavior yet.

- [ ] **Step 3: Add `glade test --trace <path>` test**

In `internal/gladecli/cli_test.go`, add a test that runs:

```bash
glade test --project testdata/local-tests/basic --trace <tmp>/trace.json
```

Expected:

- Exit code 0.
- Trace file exists.
- JSON decodes as `trace.Document`.

- [ ] **Step 4: Implement `glade test --trace <path>`**

Current test runner attaches traces when `--trace-blockers` or slow-test profiling asks for them. Add a separate `tracePath` flag that enables trace collection for all selected tests and writes a single `trace.Document`.

Implementation notes:

- Add `tracePath := ""` in `runTest`.
- Parse `String("trace", "")`.
- When set, enable test tracing in `apextest.Options` with a new field `TraceAll bool`.
- In `apextest.attachTraceProfile`, attach trace when `opts.TraceAll` is true.
- After the run, flatten all case traces and write `trace.NewDocument(events)` to the path.

Do not change default test trace behavior.

- [ ] **Step 5: Add `glade test --services <path>` validation only**

Add CLI behavior:

- If `--services` is present, validate the file before running tests.
- If validation fails, exit 1.
- If validation passes, add a warning line to stderr: `services: config validated; runtime virtualization hooks are not enabled yet`.

This avoids a false promise. Full runtime hook integration is a later packet.

- [ ] **Step 6: Run Phase 6 tests**

```bash
go test ./internal/enterprise ./internal/apextest ./internal/gladecli -count=1
go run ./cmd/glade test --project testdata/local-tests/basic --trace /tmp/glade-basic-trace.json --json >/tmp/glade-basic-test.json
git diff --check
```

Expected: trace file is nonempty Chrome trace-event JSON.

- [ ] **Step 7: Commit Phase 6**

```bash
git add internal/enterprise internal/apextest internal/gladecli docs/CONFIG.md
git commit -m "feat: expose test trace output and service config validation"
```

## Phase 7: Docs, Demo, And Alias Decision

**Files:**
- Create: `docs/ENTERPRISE_WORKFLOWS.md`
- Modify: `docs/RICH_LOCAL_WORKFLOWS.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/README.md`
- Modify: `site/docs-src/guide/rich-local-workflows.md`
- Modify: `site/docs-src/guide/cli-reference.md`
- Create: `scripts/demo-enterprise-workbench.sh`
- Create or expand: `testdata/enterprise/mri-basic`
- Create or expand: `testdata/enterprise/cruft-basic`
- Create or expand: `testdata/enterprise/refactor-proof`

- [ ] **Step 1: Write docs page**

`docs/ENTERPRISE_WORKFLOWS.md` must include:

```markdown
# Enterprise Workflows

Glade can inspect a large Apex project, report architecture risk, find conservative cruft candidates, and collect proof for branch changes without waiting on an org.

## Assessment
## Cruft and dead code
## Refactor proof
## Runtime traces
## Service config validation
## Known limitations
```

Known limitations must include:

```text
Static graph references are conservative.
Dynamic Apex and metadata-driven dispatch reduce confidence.
Global/public managed-package APIs are never safe-delete candidates.
Service config validation is available before full runtime fixture injection.
Compatibility/support-map generation remains plugin-owned.
```

- [ ] **Step 2: Add demo script**

Add `scripts/demo-enterprise-workbench.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-testdata/local-tests/enterprise-composed}"
OUT="${2:-reports/enterprise-demo}"

mkdir -p "$OUT"

go run ./cmd/glade inspect graph --project "$ROOT" --json > "$OUT/graph.json"
go run ./cmd/glade report assess --project "$ROOT" --format html --out "$OUT/assessment.html"
go run ./cmd/glade report cruft --project "$ROOT" --format html --out "$OUT/cruft.html"
go run ./cmd/glade report refactor-proof --project "$ROOT" --since HEAD --format html --out "$OUT/refactor-proof.html"
go run ./cmd/glade test --project "$ROOT" --trace "$OUT/trace.json" --json > "$OUT/test.json"

printf 'enterprise demo reports: %s\n' "$OUT"
```

- [ ] **Step 3: Add script test by execution**

```bash
chmod +x scripts/demo-enterprise-workbench.sh
scripts/demo-enterprise-workbench.sh testdata/local-tests/enterprise-composed /tmp/glade-enterprise-demo
test -s /tmp/glade-enterprise-demo/graph.json
test -s /tmp/glade-enterprise-demo/assessment.html
test -s /tmp/glade-enterprise-demo/cruft.html
test -s /tmp/glade-enterprise-demo/refactor-proof.html
test -s /tmp/glade-enterprise-demo/trace.json
```

- [ ] **Step 4: Decide aliases**

After the report commands pass, choose one:

Option A, keep existing roots only:

```text
glade inspect graph
glade report assess
glade report cruft
glade report refactor-proof
```

Option B, add thin aliases:

```text
glade assess -> glade report assess
glade cruft scan -> glade report cruft
glade refactor prove -> glade report refactor-proof
```

Recommendation: choose Option A for the first merge. Add aliases only after the docs and product copy prove users need the shorter commands.

- [ ] **Step 5: Run full phase gate**

```bash
go test ./internal/enterprise ./internal/enterprisegraph ./internal/enterpriseassess ./internal/enterprisecruft ./internal/refactorproof ./internal/gladecli ./internal/cliui ./internal/watch ./internal/apextest -count=1
scripts/demo-enterprise-workbench.sh testdata/local-tests/enterprise-composed /tmp/glade-enterprise-demo
go run ./cmd/glade report assess --project testdata/local-tests/enterprise-composed --format json >/tmp/glade-assessment.json
go run ./cmd/glade report cruft --project testdata/local-tests/enterprise-composed --format json >/tmp/glade-cruft.json
go run ./cmd/glade report refactor-proof --project . --since HEAD --format json >/tmp/glade-refactor-proof.json
git diff --check
```

Expected: pass.

- [ ] **Step 6: Commit Phase 7**

```bash
git add docs site scripts testdata
git commit -m "docs: add enterprise workflow demo"
```

## Final Verification Gate

Run these from the implementation worktree:

```bash
go test ./internal/enterprise ./internal/enterprisegraph ./internal/enterpriseassess ./internal/enterprisecruft ./internal/refactorproof ./internal/gladecli ./internal/cliui ./internal/watch ./internal/apextest ./internal/testreport ./internal/trace -count=1
go run ./cmd/glade inspect graph --project testdata/local-tests/enterprise-composed --json >/tmp/glade-enterprise-graph.json
go run ./cmd/glade report assess --project testdata/local-tests/enterprise-composed --format html --out /tmp/glade-assessment.html
go run ./cmd/glade report cruft --project testdata/local-tests/enterprise-composed --format html --out /tmp/glade-cruft.html
go run ./cmd/glade report refactor-proof --project . --since HEAD --format html --out /tmp/glade-refactor-proof.html
go run ./cmd/glade test --project testdata/local-tests/basic --trace /tmp/glade-basic-trace.json --json >/tmp/glade-basic-test.json
scripts/demo-enterprise-workbench.sh testdata/local-tests/enterprise-composed /tmp/glade-enterprise-demo
git diff --check
```

If command roots or product surfaces moved, run the broader gate:

```bash
go test ./...
scripts/smoke.sh
```

Expected:

- All focused tests pass.
- Graph, assessment, cruft, refactor-proof, and trace artifacts exist and are nonempty.
- Reports show severity, confidence, evidence, recommendations, and limitations.
- No report claims full Salesforce parity.
- No global/public symbol is marked safe to delete.
- Plugin-owned support/compat generators remain outside base Glade.

## Out Of Scope

- Automated refactoring edits.
- Full fflib migration.
- Full service virtualization and callout fixture injection.
- Full platform event subscriber parity.
- Exact governor-limit parity.
- Live org calls.
- Surface ledger generation.
- Salesforce docs inventory mining.
- Broad project-corpus readiness scoring.
- New web app.

## Execution Notes For Subagents

Start each subagent with:

```bash
git status --short
go test ./internal/gladecli ./internal/cliui -count=1
```

Keep ownership narrow:

- Schema worker owns `internal/enterprise`.
- Graph worker owns `internal/enterprisegraph`.
- Assessment worker owns `internal/enterpriseassess`.
- Cruft worker owns `internal/enterprisecruft`.
- Proof worker owns `internal/refactorproof` and a small exported helper in `internal/watch` if needed.
- CLI integration worker owns `internal/gladecli` and `internal/cliui`.
- Docs worker owns `docs`, `site`, `scripts`, and `testdata`.

The CLI integration worker merges phases. No other worker should make broad edits to `internal/gladecli/cli.go` or `internal/cliui/help.go`.

## Success Shape

The first good demo is not a magic refactoring tool.

It is this:

```bash
glade inspect graph --project testdata/local-tests/enterprise-composed --json
glade report assess --project testdata/local-tests/enterprise-composed --format html --out reports/assessment.html
glade report cruft --project testdata/local-tests/enterprise-composed --format html --out reports/cruft.html
glade report refactor-proof --project . --since HEAD --format html --out reports/refactor-proof.html
glade test --project testdata/local-tests/basic --trace reports/trace.json --json
```

The reports name what Glade saw. They count the evidence. They state what they did not prove.

Not bad.

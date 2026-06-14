# Transaction Performance Analyzer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `@glade/performance` into a real transaction performance analyzer that correlates static Apex, metadata, local trace measurements, and org-shape facts into ranked root-cause findings.

**Architecture:** Keep advisory analysis in the first-party `glade-tools` performance plugin. Improve base `glade` only where the analyzer needs better trace facts from real local execution. The analyzer should produce one evidence graph, merge static and measured evidence onto stable nodes, then rank findings from combined proof rather than from raw pattern matches.

**Tech Stack:** Go, `internal/apexast`, `internal/typesys`, `internal/schema`, `internal/profile`, `internal/trace`, `glade-tools/internal/perfscan`, `glade-tools/internal/perftool`, local SFDX metadata XML, Glade Chrome trace JSON.

---

## Boundary

The product boundary stays firm.

Base `glade` owns execution, trace capture, debug-log parsing, source ranges, and local runtime counters. It should not grow a public `inspect performance` or report scanner.

`glade-tools` owns `@glade/performance`. The plugin may depend on `glade`. It may read Glade trace JSON, local metadata, and later org-derived snapshots. It should produce JSON, Markdown, and eventually SARIF.

## Current State

Base `glade` already has:

- `internal/profile/profile.go`: trace summarization into hot events, measured spans, SOQL, DML, describe, automation, CPU, heap, and wall time.
- `glade exec --trace` and `glade test --trace`: Chrome trace output.
- `glade debug profile`: profile reports from debug-log-shaped input.
- VM counters for SOQL rows, DML rows, deterministic CPU, and heap.

`glade-tools` already has:

- `internal/perfscan/model.go`: report, finding, entry point, measurement, and evidence model.
- `internal/perfscan/apex_scan.go`: static Apex scans for loop SOQL/DML, describe, SOQL selectivity, overfetch, async risks, and some UI entry points.
- `internal/perfscan/metadata_scan.go`: Flow, Workflow, Visualforce, Aura, and LWC entry-point inventory and simple automation findings.
- `internal/perfscan/trace_scan.go`: measured hot-span and high-SOQL-row findings from trace data.
- `internal/perftool/cli.go`: `glade performance scan --project . --trace <path> --json --top <n>`.

The missing part is a durable transaction graph and correlation layer. Static and measured facts still sit near each other, like tools on the bench but not yet fitted to one handle.

## Target Behavior

By the end of this plan:

- A finding has an entry point, call path, operation path, namespace path when known, multiplicity, evidence, resource risk, confidence, score, fix, and acceptance check.
- Static-only scans produce useful risk leads.
- Trace-backed scans promote measured bottlenecks above speculative findings.
- Metadata-backed scans explain why one clean DML line wakes Flow, Workflow, trigger, rollup, or sharing work.
- JSON output is stable enough for CI and downstream tools.
- Markdown output is useful to a senior Apex developer without reading raw JSON.

## Approach Decision

Recommended approach: build the evidence graph first, then port existing scans onto it, then add ranking and trace correlation. This keeps each detector from inventing its own half-model.

Rejected approach: add many more regex-style detectors first. It would make the report louder before it gets smarter.

Rejected approach: move the analyzer into base `glade`. That breaks the current product boundary and puts advisory scanner work back in the public tool.

## File Structure

`/Users/matt/Dev/glade-tools/internal/perfscan/graph.go`
: Create. Defines graph nodes, operation facts, edges, evidence attachments, and path helpers.

`/Users/matt/Dev/glade-tools/internal/perfscan/graph_test.go`
: Create. Tests graph construction, stable IDs, path formatting, and evidence merging.

`/Users/matt/Dev/glade-tools/internal/perfscan/test_helpers_test.go`
: Create. Shared helper code for later tests in this plan.

`/Users/matt/Dev/glade-tools/internal/perfscan/source_graph.go`
: Create. Builds the static source graph from parsed Apex files and `typesys.Index`.

`/Users/matt/Dev/glade-tools/internal/perfscan/source_graph_test.go`
: Create. Tests entry-point discovery, call edges, loop multiplicity, SOQL, DML, describe, and static initializer facts.

`/Users/matt/Dev/glade-tools/internal/perfscan/metadata_graph.go`
: Create. Converts Flow, Workflow, object, field, rollup, and sharing metadata into graph facts.

`/Users/matt/Dev/glade-tools/internal/perfscan/metadata_graph_test.go`
: Create. Tests DML blast-radius facts from metadata fixtures.

`/Users/matt/Dev/glade-tools/internal/perfscan/trace_correlation.go`
: Create. Maps `profile.Report` entries onto graph nodes and creates combined evidence.

`/Users/matt/Dev/glade-tools/internal/perfscan/trace_correlation_test.go`
: Create. Tests measured span, SOQL row, DML row, describe, and source-range correlation.

`/Users/matt/Dev/glade-tools/internal/perfscan/ranking.go`
: Create. Scores findings from static risk, measured cost, multiplicity, metadata fanout, confidence, and suppression rules.

`/Users/matt/Dev/glade-tools/internal/perfscan/ranking_test.go`
: Create. Tests deterministic ranking and confidence promotion.

`/Users/matt/Dev/glade-tools/internal/perfscan/detectors.go`
: Create. Hosts detector functions that consume graph facts and emit `Finding` values.

`/Users/matt/Dev/glade-tools/internal/perfscan/detectors_test.go`
: Create. Tests P0 detectors against small source and metadata fixtures.

`/Users/matt/Dev/glade-tools/internal/perfscan/model.go`
: Modify. Extend `Finding`, `Evidence`, `Measurement`, and schema version for transaction-path output.

`/Users/matt/Dev/glade-tools/internal/perfscan/analyze.go`
: Modify. Build graph, attach metadata, correlate trace, run detectors, rank, then finalize.

`/Users/matt/Dev/glade-tools/internal/perfscan/report.go`
: Modify. Render top findings with entry point, path, evidence, resource risk, and fix acceptance checks.

`/Users/matt/Dev/glade-tools/internal/perftool/cli.go`
: Modify. Add flags for `--format json|markdown|sarif`, `--min-confidence`, `--fail-on high|measured|none`, and `--org-facts <path>`.

`/Users/matt/Dev/glade/internal/trace/trace.go`
: Modify only if needed. Add stable trace argument names for entry point, class, method, operation ID, object, query hash, rows, source line, and namespace.

`/Users/matt/Dev/glade/internal/vm/*`
: Modify only where trace events lack facts needed for correlation. Keep behavior unchanged.

## Task 0: Add Perfscan Test Helpers

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/test_helpers_test.go`

- [ ] **Step 1: Create shared helper file**

Create `test_helpers_test.go`:

```go
package perfscan

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/trace"
)

func testPerfProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"63.0"}`)
	for rel, body := range files {
		writeTestFile(t, filepath.Join(root, filepath.FromSlash(rel)), body)
	}
	return root
}

func testPerfProjectWithMetadata(t *testing.T) string {
	t.Helper()
	return testPerfProject(t, map[string]string{
		"force-app/main/default/triggers/AccountTrigger.trigger": "trigger AccountTrigger on Account (after update) { update Trigger.new; }",
		"force-app/main/default/flows/Account_After_Save.flow-meta.xml": "<Flow><start><object>Account</object><triggerType>RecordAfterSave</triggerType></start><recordUpdates><name>Update_Account</name></recordUpdates></Flow>",
		"force-app/main/default/workflows/Account.workflow-meta.xml": "<Workflow><rules><fullName>Active_Rule</fullName><active>true</active></rules></Workflow>",
		"force-app/main/default/objects/Account/Account.object-meta.xml": "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><label>Account</label></CustomObject>",
	})
}

func analyzeTestProject(t *testing.T, root string, options Options) Report {
	t.Helper()
	options.ProjectRoot = root
	report, err := AnalyzeProject(options)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func requireFinding(t *testing.T, report Report, id string) Finding {
	t.Helper()
	for _, finding := range report.Findings {
		if finding.ID == id {
			return finding
		}
	}
	t.Fatalf("missing finding %s in %#v", id, report.Findings)
	return Finding{}
}

func requireEvidence(t *testing.T, finding Finding, kind, messagePart string) {
	t.Helper()
	for _, evidence := range finding.Evidence {
		if evidence.Kind == kind && strings.Contains(evidence.Message, messagePart) {
			return
		}
	}
	t.Fatalf("missing evidence %s/%s in %#v", kind, messagePart, finding.Evidence)
}

func writeTrace(t *testing.T, events []trace.Event) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trace.json")
	var buf bytes.Buffer
	if err := trace.WriteJSON(&buf, trace.NewDocument(events)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeOrgFactsFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "org-facts.json")
	writeTestFile(t, path, `{
  "schemaVersion": 1,
  "objects": {
    "Account": {
      "estimatedRows": 1200000,
      "sharingModel": "Private",
      "fields": {
        "External_Id__c": {"indexed": true, "unique": true},
        "Formula_Key__c": {"formula": true}
      }
    },
    "Contact": {
      "estimatedRows": 900000,
      "parentSkew": [{"field": "AccountId", "parentId": "001xx000003DHP0", "count": 24000}]
    }
  }
}`)
	return path
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func marshalReport(t *testing.T, report Report) []byte {
	t.Helper()
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}
```

- [ ] **Step 2: Run helper compile check**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestNameThatDoesNotExist -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/tools/internal/perfscan
```

## Task 1: Lock The Report Schema

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/model.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/model_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/report.go`

- [ ] **Step 1: Write failing schema tests**

Add this test to `model_test.go`:

```go
func TestFindingCarriesTransactionEvidence(t *testing.T) {
	report := Report{SchemaVersion: SchemaVersion, Project: "/tmp/project"}
	report.AddFinding(Finding{
		ID:         "perf.soql.loop.interprocedural",
		Category:   CategorySOQL,
		Severity:   SeverityHigh,
		Confidence: ConfidenceCombined,
		Score:      96,
		EntryPoint: EntryPoint{Kind: EntryTrigger, Name: "AccountTrigger"},
		Path: []PathStep{
			{Kind: "trigger", Name: "AccountTrigger"},
			{Kind: "method", Name: "PricingService.reprice"},
			{Kind: "soql", Name: "SELECT Id FROM Product2"},
		},
		Evidence: []Evidence{
			{Kind: "static", Message: "loop multiplicity", Value: "per-record"},
			{Kind: "trace", Message: "duration ms", Value: "421"},
			{Kind: "metadata", Message: "record-triggered flows", Value: "2"},
		},
		ResourceRisk: ResourceRisk{CPU: true, DBRows: true, SharedLimit: true},
		Acceptance:  "For 200 trigger records, query count stays O(1) and selected fields match the read path.",
	})
	report.Finalize()

	if report.SchemaVersion < 2 {
		t.Fatalf("schema version = %d, want at least 2", report.SchemaVersion)
	}
	if report.Findings[0].ResourceRisk.SharedLimit != true {
		t.Fatalf("missing shared limit risk: %#v", report.Findings[0])
	}
	if report.Findings[0].Acceptance == "" {
		t.Fatalf("missing acceptance check: %#v", report.Findings[0])
	}
}
```

- [ ] **Step 2: Run the test and confirm failure**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestFindingCarriesTransactionEvidence -count=1
```

Expected:

```text
undefined: ResourceRisk
```

- [ ] **Step 3: Add schema fields**

Add these types and fields in `model.go`:

```go
const SchemaVersion = 2

type ResourceRisk struct {
	CPU         bool `json:"cpu,omitempty"`
	Heap        bool `json:"heap,omitempty"`
	DBTime      bool `json:"dbTime,omitempty"`
	DBRows      bool `json:"dbRows,omitempty"`
	Locks       bool `json:"locks,omitempty"`
	SharedLimit bool `json:"sharedLimit,omitempty"`
}

type Finding struct {
	ID           string       `json:"id"`
	Category     Category     `json:"category"`
	Severity     Severity     `json:"severity"`
	Confidence   Confidence   `json:"confidence"`
	Score        int          `json:"score"`
	EntryPoint   EntryPoint   `json:"entryPoint,omitempty"`
	Message      string       `json:"message"`
	Location     Location     `json:"location,omitempty"`
	Path         []PathStep   `json:"path,omitempty"`
	NamespacePath []string    `json:"namespacePath,omitempty"`
	Multiplicity string       `json:"multiplicity,omitempty"`
	Evidence     []Evidence   `json:"evidence,omitempty"`
	ResourceRisk ResourceRisk `json:"resourceRisk,omitempty"`
	Fix          string       `json:"fix,omitempty"`
	Acceptance   string       `json:"acceptance,omitempty"`
}
```

- [ ] **Step 4: Update Markdown rendering**

In `report.go`, render these blocks when present:

```text
Path: trigger AccountTrigger -> method PricingService.reprice -> soql SELECT Id FROM Product2
Multiplicity: per-record
Resource risk: CPU, DB rows, shared limits
Acceptance: For 200 trigger records, query count stays O(1) and selected fields match the read path.
```

- [ ] **Step 5: Verify**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run 'TestFindingCarriesTransactionEvidence|TestMarkdownReportIncludesEvidenceAndFix' -count=1
```

Expected: both tests pass.

## Task 2: Build The Evidence Graph

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/graph.go`
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/graph_test.go`

- [ ] **Step 1: Write failing graph tests**

Create `graph_test.go`:

```go
package perfscan

import "testing"

func TestGraphBuildsStableTransactionPath(t *testing.T) {
	g := NewGraph()
	trigger := g.AddNode(Node{Kind: NodeEntryPoint, Name: "AccountTrigger", File: "Account.trigger", Line: 1})
	method := g.AddNode(Node{Kind: NodeMethod, Name: "PricingService.reprice", File: "PricingService.cls", Line: 8})
	query := g.AddNode(Node{Kind: NodeSOQL, Name: "SELECT Id FROM Product2", File: "PricingService.cls", Line: 10})
	g.AddEdge(trigger, method, EdgeCalls)
	g.AddEdge(method, query, EdgeExecutes)
	g.AddEvidence(query, Evidence{Kind: "static", Message: "query in per-record path"})

	path := g.Path(trigger, query)
	if len(path) != 3 {
		t.Fatalf("path = %#v", path)
	}
	if path[2].Kind != "soql" || path[2].Name != "SELECT Id FROM Product2" {
		t.Fatalf("path tail = %#v", path[2])
	}
	if len(g.Evidence(query)) != 1 {
		t.Fatalf("evidence = %#v", g.Evidence(query))
	}
}
```

- [ ] **Step 2: Run the test and confirm failure**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestGraphBuildsStableTransactionPath -count=1
```

Expected:

```text
undefined: NewGraph
```

- [ ] **Step 3: Implement graph primitives**

Create `graph.go` with:

```go
type NodeKind string

const (
	NodeEntryPoint NodeKind = "entryPoint"
	NodeMethod     NodeKind = "method"
	NodeSOQL       NodeKind = "soql"
	NodeDML        NodeKind = "dml"
	NodeDescribe   NodeKind = "describe"
	NodeStaticInit NodeKind = "staticInit"
	NodeAutomation NodeKind = "automation"
)

type EdgeKind string

const (
	EdgeCalls    EdgeKind = "calls"
	EdgeExecutes EdgeKind = "executes"
	EdgeWakes    EdgeKind = "wakes"
	EdgeMeasures EdgeKind = "measures"
)

type NodeID int

type Node struct {
	Kind      NodeKind
	Name      string
	File      string
	Line      int
	Namespace string
	Operation string
}
```

Implement:

```go
func NewGraph() *Graph
func (g *Graph) AddNode(n Node) NodeID
func (g *Graph) AddEdge(from, to NodeID, kind EdgeKind)
func (g *Graph) AddEvidence(node NodeID, e Evidence)
func (g *Graph) Evidence(node NodeID) []Evidence
func (g *Graph) Path(from, to NodeID) []PathStep
```

Use a stable node key of `kind|namespace|file|line|name|operation` so duplicate scans merge onto one node.

- [ ] **Step 4: Verify**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestGraphBuildsStableTransactionPath -count=1
```

Expected: pass.

## Task 3: Replace The Static Scan Core With Source Graph Facts

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/source_graph.go`
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/source_graph_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/apex_scan.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/analyze.go`

- [ ] **Step 1: Write failing source graph tests**

Create a test with a trigger, service, selector, loop, and query:

```go
func TestSourceGraphPropagatesPerRecordQueryThroughCall(t *testing.T) {
	root := testPerfProject(t, map[string]string{
		"force-app/main/default/triggers/AccountTrigger.trigger": `
trigger AccountTrigger on Account (after update) {
  for (Account account : Trigger.new) {
    PricingService.reprice(account);
  }
}`,
		"force-app/main/default/classes/PricingService.cls": `
public class PricingService {
  public static void reprice(Account account) {
    ProductSelector.byFamily(account.Industry);
  }
}`,
		"force-app/main/default/classes/ProductSelector.cls": `
public class ProductSelector {
  public static List<Product2> byFamily(String family) {
    return [SELECT Id, Name FROM Product2 WHERE Family = :family];
  }
}`,
	})
	report := analyzeTestProject(t, root, Options{})
	finding := requireFinding(t, report, "perf.soql.loop.interprocedural")
	if finding.Multiplicity != "per-record" {
		t.Fatalf("multiplicity = %q", finding.Multiplicity)
	}
	if len(finding.Path) < 4 {
		t.Fatalf("path = %#v", finding.Path)
	}
}
```

- [ ] **Step 2: Run and confirm failure**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestSourceGraphPropagatesPerRecordQueryThroughCall -count=1
```

Expected: missing finding.

- [ ] **Step 3: Build source graph extraction**

Implement `BuildSourceGraph(parsed apexast.Result, index typesys.Index) *Graph`.

It must collect:

- Entry points: triggers, `@AuraEnabled`, `@InvocableMethod`, `Queueable.execute`, `Batchable.start/execute/finish`, `Schedulable.execute`, `@future`.
- Methods: class name, method name, file, line, namespace when known from `typesys.TypeSymbol`.
- Calls: direct static calls like `PricingService.reprice(account)`, instance calls when the receiver type is known from local declarations, and unresolved dynamic calls as evidence on the caller node.
- Operations: SOQL, DML syntax, `Database.*` DML, describe calls, static initializer blocks, static fields with describe/config/query work.
- Multiplicity edges: loop body, trigger record loop, child loop, per-field loop.

Keep the first version conservative. Unknown calls should not fabricate paths. Mark them with evidence:

```go
Evidence{Kind: "static", Message: "unresolved call edge", Value: "receiver.method"}
```

- [ ] **Step 4: Port existing direct-loop findings**

Keep existing `perf.soql.loop` and `perf.dml.loop` IDs for direct loops, but emit them from graph facts. Add new IDs:

- `perf.soql.loop.interprocedural`
- `perf.dml.loop.interprocedural`
- `perf.static.first-touch`

- [ ] **Step 5: Verify**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run 'TestSourceGraph|TestAnalyzeProjectDetectsApexRisks' -count=1
```

Expected: all source graph tests and existing Apex scan tests pass.

## Task 4: Add Static First-Touch And Repeated Metadata Work

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/source_graph.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/detectors.go`
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/detectors_test.go`

- [ ] **Step 1: Write failing detector tests**

Add:

```go
func TestDetectorFindsHeavyStaticFirstTouch(t *testing.T) {
	root := testPerfProject(t, map[string]string{
		"force-app/main/default/classes/Constants.cls": `
public class Constants {
  public static final String LABEL = 'A';
  public static Map<String, Schema.SObjectType> TOKENS = Schema.getGlobalDescribe();
}`,
		"force-app/main/default/classes/Controller.cls": `
public class Controller {
  @AuraEnabled(cacheable=true)
  public static String label() {
    return Constants.LABEL;
  }
}`,
	})
	report := analyzeTestProject(t, root, Options{})
	finding := requireFinding(t, report, "perf.static.first-touch")
	if finding.EntryPoint.Kind != EntryLWC && finding.EntryPoint.Kind != EntryAura {
		t.Fatalf("entry point = %#v", finding.EntryPoint)
	}
}
```

- [ ] **Step 2: Run and confirm failure**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestDetectorFindsHeavyStaticFirstTouch -count=1
```

Expected: missing finding.

- [ ] **Step 3: Implement detector**

Detect:

- `Schema.getGlobalDescribe()` in static field initializer or static block.
- `Schema.describeSObjects()` in static field initializer or static block.
- `__mdt.getAll()` and custom settings `__c.getAll()` in static initializer.
- SOQL in static initializer.
- Cheap static field read that first-touches a class with heavy static work.

Emit:

```go
Finding{
	ID: "perf.static.first-touch",
	Category: CategoryDescribe,
	Severity: SeverityHigh,
	Confidence: ConfidenceStatic,
	Multiplicity: "once-per-transaction",
	ResourceRisk: ResourceRisk{CPU: true, Heap: true, SharedLimit: true},
	Fix: "Split cheap constants from metadata/config loaders and lazy-load describe/config by key.",
	Acceptance: "Referencing a string constant must not execute describe, SOQL, or getAll work.",
}
```

- [ ] **Step 4: Verify**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run 'TestDetectorFindsHeavyStaticFirstTouch|TestAnalyzeProjectDetectsApexRisks' -count=1
```

Expected: pass.

## Task 5: Correlate Trace Measurements Onto Graph Nodes

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/trace_correlation.go`
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/trace_correlation_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/trace_scan.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/analyze.go`

- [ ] **Step 1: Write failing trace correlation tests**

Use a trace with a measured method and measured SOQL at source lines:

```go
func TestTraceCorrelationPromotesStaticFindingToCombined(t *testing.T) {
	root := testPerfProject(t, map[string]string{
		"force-app/main/default/classes/PerfRisk.cls": `
public class PerfRisk {
  public static void run(List<Account> accounts) {
    for (Account account : accounts) {
      List<Contact> contacts = [SELECT Id FROM Contact WHERE AccountId = :account.Id];
    }
  }
}`,
	})
	tracePath := writeTrace(t, []trace.Event{
		trace.Duration("apex.method.PerfRisk.run", "apex.method", 0, 250000, map[string]any{"file": "PerfRisk.cls", "line": 3}),
		trace.Instant("apex.soql", "apex.soql", 260000, map[string]any{"query": "SELECT Id FROM Contact WHERE AccountId = :account.Id", "rows": 1000, "file": "PerfRisk.cls", "line": 5}),
		trace.Duration("apex.soql", "apex.soql", 260000, 120000, map[string]any{"query": "SELECT Id FROM Contact WHERE AccountId = :account.Id", "file": "PerfRisk.cls", "line": 5}),
	})
	report := analyzeTestProject(t, root, Options{TracePath: tracePath})
	finding := requireFinding(t, report, "perf.soql.loop")
	if finding.Confidence != ConfidenceCombined {
		t.Fatalf("confidence = %s", finding.Confidence)
	}
	if finding.Score < 95 {
		t.Fatalf("score = %d", finding.Score)
	}
}
```

- [ ] **Step 2: Run and confirm failure**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestTraceCorrelationPromotesStaticFindingToCombined -count=1
```

Expected: confidence remains static or missing finding.

- [ ] **Step 3: Implement correlation**

Create:

```go
func CorrelateTrace(g *Graph, profile profile.Report)
```

Match order:

1. Exact file and line.
2. File and nearest node line within the same method range.
3. Query text hash for SOQL.
4. DML object when trace args contain object.
5. Entry name for method, trigger, batch, queueable, flow, or visualforce.

When matched, attach:

```go
Evidence{Kind: "trace", Message: "duration ms", Value: strconv.FormatInt(entry.DurationMS, 10)}
Evidence{Kind: "trace", Message: "count", Value: strconv.Itoa(entry.Count)}
Evidence{Kind: "trace", Message: "rows", Value: strconv.Itoa(entry.Rows)}
```

Create measured-only findings only when no static graph node matches, using `perf.measured.hot-span` and `perf.measured.soql-rows`.

- [ ] **Step 4: Verify**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run 'TestTraceCorrelation|TestTraceScanAddsMeasuredFindings' -count=1
```

Expected: pass.

## Task 6: Improve Base Glade Trace Facts

**Files:**
- Modify: `/Users/matt/Dev/glade/internal/trace/trace.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/soql_runtime.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/dml_runtime.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/describe_runtime.go`
- Modify: `/Users/matt/Dev/glade/internal/vm/method.go`
- Modify: `/Users/matt/Dev/glade/internal/gladecli/cli_test.go`
- Modify: `/Users/matt/Dev/glade/internal/profile/profile_test.go`

- [ ] **Step 1: Write failing trace shape tests in Glade**

In `internal/gladecli/cli_test.go`, add:

```go
func TestRunExecTraceIncludesOperationCorrelationFacts(t *testing.T) {
	tracePath := filepath.Join(t.TempDir(), "trace.json")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"exec", "--trace", tracePath,
		"List<Account> rows = [SELECT Id, Name FROM Account]; System.debug(rows.size());",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exec failed stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
	data, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"file"`, `"line"`, `"operationId"`, `"queryHash"`, `"rows"`} {
		if !bytes.Contains(data, []byte(want)) {
			t.Fatalf("trace missing %s: %s", want, string(data))
		}
	}
}
```

- [ ] **Step 2: Run and confirm failure**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/apex-perf-doc-review
go test ./internal/gladecli -run TestRunExecTraceIncludesOperationCorrelationFacts -count=1
```

Expected: missing `operationId` or `queryHash`.

- [ ] **Step 3: Add trace args without behavior changes**

For SOQL trace events, include:

```go
args["operationId"] = stableOperationID(file, line, "soql", queryText)
args["queryHash"] = sha256Hex(normalizedQueryText)
args["object"] = rootObjectName
args["rows"] = rowCount
args["file"] = file
args["line"] = line
```

For DML trace events, include:

```go
args["operationId"] = stableOperationID(file, line, "dml", operation+" "+objectName)
args["operation"] = operation
args["object"] = objectName
args["rows"] = rowCount
args["file"] = file
args["line"] = line
```

For describe and static initializer events, include class, method, file, line, and namespace when known.

- [ ] **Step 4: Verify Glade**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/apex-perf-doc-review
go test -count=1 ./internal/gladecli -run 'TestRunExecTraceIncludesOperationCorrelationFacts|TestRunExecTraceWritesChromeTrace'
go test -count=1 ./internal/profile
```

Expected: pass.

## Task 7: Add Metadata Blast-Radius Facts

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/metadata_graph.go`
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/metadata_graph_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/metadata_scan.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/detectors.go`

- [ ] **Step 1: Write failing metadata tests**

Add fixtures for:

- Account trigger.
- Account record-triggered Flow with record update.
- Account workflow rule active.
- Child object with roll-up summary parent.
- Sharing model file with private OWD when available.

Test:

```go
func TestMetadataGraphExplainsDMLBlastRadius(t *testing.T) {
	root := testPerfProjectWithMetadata(t)
	report := analyzeTestProject(t, root, Options{})
	finding := requireFinding(t, report, "perf.dml.blast-radius")
	if finding.ResourceRisk.CPU != true || finding.ResourceRisk.Locks != true {
		t.Fatalf("resource risk = %#v", finding.ResourceRisk)
	}
	requireEvidence(t, finding, "flow", "record-triggered flows")
	requireEvidence(t, finding, "workflow", "active workflow rules")
}
```

- [ ] **Step 2: Run and confirm failure**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestMetadataGraphExplainsDMLBlastRadius -count=1
```

Expected: missing finding.

- [ ] **Step 3: Implement metadata facts**

For each object, collect:

- Apex triggers by object.
- Record-triggered flows by object.
- Workflow rules by object.
- Roll-up summary fields and parent object.
- Lookup/master-detail fields and parent object.
- Sharing model when metadata exists.
- Custom metadata type row counts from local metadata records.

Attach facts to DML nodes through `EdgeWakes`.

- [ ] **Step 4: Emit DML blast-radius findings**

Emit `perf.dml.blast-radius` when a DML node touches an object with two or more downstream automation facts, or a rollup/sharing fact. Use:

```go
ResourceRisk{CPU: true, DBTime: true, Locks: true, SharedLimit: true}
```

Acceptance:

```text
A no-op field update on the same object is removed or guarded, and DML tests prove only necessary rows are updated.
```

- [ ] **Step 5: Verify**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run 'TestMetadataGraph|TestAnalyzeProjectDetectsMetadataRisks' -count=1
```

Expected: pass.

## Task 8: Add Org Facts Snapshot Input

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/org_facts.go`
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/org_facts_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/analyze.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perftool/cli.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perftool/cli_test.go`

- [ ] **Step 1: Define org facts JSON test**

Add a fixture:

```json
{
  "schemaVersion": 1,
  "objects": {
    "Account": {
      "estimatedRows": 1200000,
      "sharingModel": "Private",
      "fields": {
        "External_Id__c": {"indexed": true, "unique": true},
        "Formula_Key__c": {"formula": true}
      }
    },
    "Contact": {
      "estimatedRows": 900000,
      "parentSkew": [{"field": "AccountId", "parentId": "001xx000003DHP0", "count": 24000}]
    }
  }
}
```

Test:

```go
func TestOrgFactsRaiseSelectivityAndSkewRisk(t *testing.T) {
	root := testPerfProject(t, map[string]string{
		"force-app/main/default/classes/QueryRisk.cls": `
public class QueryRisk {
  public static List<Account> byFormula(String value) {
    return [SELECT Id FROM Account WHERE Formula_Key__c = :value];
  }
}`,
	})
	orgFactsPath := writeOrgFactsFixture(t)
	report := analyzeTestProject(t, root, Options{OrgFactsPath: orgFactsPath})
	requireFinding(t, report, "perf.soql.query-plan-risk")
	requireFinding(t, report, "perf.data-skew.parent")
}
```

- [ ] **Step 2: Run and confirm failure**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestOrgFactsRaiseSelectivityAndSkewRisk -count=1
```

Expected: `Options` lacks `OrgFactsPath` or findings missing.

- [ ] **Step 3: Implement org facts loading**

Add to `Options`:

```go
OrgFactsPath string
```

Add:

```go
func LoadOrgFacts(path string) (OrgFacts, error)
func ApplyOrgFacts(g *Graph, facts OrgFacts)
```

Do not call Salesforce from the plugin in this task. It reads a snapshot only.

- [ ] **Step 4: Wire CLI flag**

Add:

```text
--org-facts <path>  Read org/data-shape facts from JSON.
```

- [ ] **Step 5: Verify**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan ./internal/perftool -run 'OrgFacts|PerformanceScan' -count=1
```

Expected: pass.

## Task 9: Add Ranking And Noise Control

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/ranking.go`
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/ranking_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/model.go`

- [ ] **Step 1: Write failing ranking tests**

Add:

```go
func TestRankerPromotesMeasuredSharedLimitFindings(t *testing.T) {
	findings := []Finding{
		{ID: "perf.soql.loop", Severity: SeverityHigh, Confidence: ConfidenceStatic, Score: 80, ResourceRisk: ResourceRisk{DBRows: true}},
		{ID: "perf.static.first-touch", Severity: SeverityMedium, Confidence: ConfidenceCombined, Score: 70, ResourceRisk: ResourceRisk{CPU: true, Heap: true, SharedLimit: true}, Evidence: []Evidence{{Kind: "trace", Message: "duration ms", Value: "900"}}},
	}
	ranked := RankFindings(findings, RankOptions{TopN: 0})
	if ranked[0].ID != "perf.static.first-touch" {
		t.Fatalf("ranked = %#v", ranked)
	}
}
```

- [ ] **Step 2: Run and confirm failure**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestRankerPromotesMeasuredSharedLimitFindings -count=1
```

Expected: `undefined: RankFindings`.

- [ ] **Step 3: Implement score model**

Use this first scoring table:

```text
base severity: high 60, medium 40, low 20
confidence: measured +20, combined +30, static +0
resource: shared CPU/heap/time +15, DB rows +8, locks +12, automation fanout +8
multiplicity: per-record +15, per-child +12, per-field +10, once-per-transaction +4
trace duration: min(durationMs / 50, 25)
trace rows: min(rows / 100, 15)
noise cap: static-only low confidence max 55
```

Clamp to 100.

- [ ] **Step 4: Add top and confidence filters**

Implement:

```go
type RankOptions struct {
	TopN int
	MinConfidence Confidence
}
```

Order confidence as static < measured < combined.

- [ ] **Step 5: Verify**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run 'TestRanker|TestReportModelSortsFindingsByScore' -count=1
```

Expected: pass.

## Task 10: Ship CLI Output For Real Triage

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/perftool/cli.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perftool/cli_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/report.go`

- [ ] **Step 1: Write failing CLI tests**

Add:

```go
func TestPerformanceScanSupportsFormatAndFailOn(t *testing.T) {
	project := filepath.Join("..", "perfscan", "testdata", "perf-project")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{
		"performance", "scan",
		"--project", project,
		"--format", "json",
		"--fail-on", "high",
		"--top", "5",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"schemaVersion"`) || !strings.Contains(stdout.String(), `"findings"`) {
		t.Fatalf("missing json report: %s", stdout.String())
	}
}
```

- [ ] **Step 2: Run and confirm failure**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perftool -run TestPerformanceScanSupportsFormatAndFailOn -count=1
```

Expected: unknown `--format` or `--fail-on`.

- [ ] **Step 3: Implement flags**

Support:

```text
--format markdown|json|sarif
--top <n>
--min-confidence static|measured|combined
--fail-on none|high|measured
--org-facts <path>
```

Default behavior remains Markdown and exit 0.

- [ ] **Step 4: Add Markdown triage layout**

Each top finding must render:

```text
1. perf.soql.loop.interprocedural [high/combined] score=96
   Entry point: trigger AccountTrigger
   Path: AccountTrigger -> PricingService.reprice -> ProductSelector.byFamily -> SOQL
   Multiplicity: per-record
   Resource risk: CPU, DB rows, shared limits
   Evidence:
     - static: query reachable from trigger loop
     - trace: 421 ms
     - trace: 1000 rows
   Fix: collect keys first, query once, map by key.
   Acceptance: 200 trigger records issue one Product2 query.
```

- [ ] **Step 5: Verify**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perftool ./internal/perfscan -run 'PerformanceScan|MarkdownReport' -count=1
```

Expected: pass.

## Task 11: Add SARIF For CI

**Files:**
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/sarif.go`
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/sarif_test.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/perftool/cli.go`

- [ ] **Step 1: Write failing SARIF test**

Add:

```go
func TestWriteSARIFIncludesRulesAndLocations(t *testing.T) {
	report := Report{SchemaVersion: SchemaVersion, Project: "/tmp/project"}
	report.AddFinding(Finding{
		ID: "perf.soql.loop",
		Severity: SeverityHigh,
		Confidence: ConfidenceStatic,
		Score: 95,
		Message: "SOQL inside a loop can exceed query limits.",
		Location: Location{File: "/tmp/project/force-app/main/default/classes/Risk.cls", Line: 7, Column: 5},
	})
	report.Finalize()
	var out bytes.Buffer
	if err := WriteSARIF(&out, report); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{`"version": "2.1.0"`, `"ruleId": "perf.soql.loop"`, `"Risk.cls"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %s", want, text)
		}
	}
}
```

- [ ] **Step 2: Run and confirm failure**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestWriteSARIFIncludesRulesAndLocations -count=1
```

Expected: `undefined: WriteSARIF`.

- [ ] **Step 3: Implement SARIF writer**

Map:

- high -> error
- medium -> warning
- low -> note

Use repository-relative paths when `report.Project` is an ancestor of `finding.Location.File`.

- [ ] **Step 4: Verify**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan ./internal/perftool -run 'SARIF|PerformanceScanSupportsFormat' -count=1
```

Expected: pass.

## Task 12: End-To-End Fixture And Golden Output

**Files:**
- Modify: `/Users/matt/Dev/glade-tools/internal/perfscan/testdata/perf-project/**`
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/testdata/golden/perf-project-report.json`
- Create: `/Users/matt/Dev/glade-tools/internal/perfscan/e2e_test.go`

- [ ] **Step 1: Extend fixture project**

Add fixture code paths for:

- Trigger loop calling a selector through a service.
- Static constants first-touch.
- DML update on Account with Flow and Workflow metadata.
- SOQL with `IN :List<SObject>`.
- SOQL bind without null proof.
- Parent-child subquery without limit.
- Expensive `System.debug(JSON.serialize(records))`.

- [ ] **Step 2: Write golden test**

Add:

```go
func TestPerformanceScanGoldenReport(t *testing.T) {
	root := filepath.Join("testdata", "perf-project")
	report, err := AnalyzeProject(Options{ProjectRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "perf-project-report.json")
	if *updateGolden {
		if err := os.WriteFile(golden, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	got := string(append(data, '\n'))
	if string(want) != got {
		t.Fatalf("golden mismatch\nwant:\n%s\n\ngot:\n%s", string(want), got)
	}
}
```

Use an existing local golden-update pattern if the repo already has one. If not, add:

```go
var updateGolden = flag.Bool("update", false, "update golden files")
```

- [ ] **Step 3: Generate golden once**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestPerformanceScanGoldenReport -count=1 -args -update
```

Expected: golden file written.

- [ ] **Step 4: Verify golden**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test ./internal/perfscan -run TestPerformanceScanGoldenReport -count=1
```

Expected: pass.

## Task 13: Documentation And Boundary Update

**Files:**
- Modify: `/Users/matt/Dev/glade/docs/LOCAL_TESTING.md`
- Modify: `/Users/matt/Dev/glade/docs/COMPATIBILITY.md`
- Modify: `/Users/matt/Dev/glade-tools/README.md`
- Modify: `/Users/matt/Dev/glade-tools/plugins/performance/plugin.json`

- [ ] **Step 1: Update user-facing docs**

In `docs/LOCAL_TESTING.md`, keep the existing short path and add a measured triage example:

```bash
mkdir -p reports
glade test --project . --class SlowPathTest --trace reports/slow-path.trace.json
glade performance scan --project . --trace reports/slow-path.trace.json --format markdown --top 10
```

State:

```text
Static findings are leads. Trace-backed findings are measured local evidence. Org facts raise or lower confidence when metadata or data shape changes the risk.
```

- [ ] **Step 2: Update plugin README**

Document:

```bash
glade performance scan --project . --format json
glade performance scan --project . --trace reports/slow.trace.json --top 10
glade performance scan --project . --org-facts reports/org-facts.json --fail-on high
glade performance scan --project . --format sarif > reports/glade-performance.sarif
```

- [ ] **Step 3: Verify docs**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/apex-perf-doc-review
go test ./internal/repoguard -count=1
cd /Users/matt/Dev/glade-tools
go test ./internal/perftool -run TestPerformanceScanHelp -count=1
```

Expected: pass.

## Task 14: Release Proof

**Files:**
- Modify only files touched by earlier tasks.

- [ ] **Step 1: Run plugin proof**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go test -count=1 ./internal/perfscan ./internal/perftool
go test -count=1 ./cmd/glade-plugin-performance
```

Expected: pass.

- [ ] **Step 2: Build plugin**

Run:

```bash
cd /Users/matt/Dev/glade-tools
go build -o /tmp/glade-plugin-performance ./cmd/glade-plugin-performance
/tmp/glade-plugin-performance manifest --json
```

Expected: manifest JSON includes `performance` command.

- [ ] **Step 3: Run plugin on fixture**

Run:

```bash
cd /Users/matt/Dev/glade-tools
/tmp/glade-plugin-performance performance scan --project internal/perfscan/testdata/perf-project --format json --top 10 > /tmp/perf-report.json
```

Expected:

```bash
rg '"findings":' /tmp/perf-report.json
rg 'perf.soql.loop|perf.static.first-touch|perf.dml.blast-radius' /tmp/perf-report.json
```

Both `rg` commands find matches.

- [ ] **Step 4: Run Glade proof for trace changes**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/apex-perf-doc-review
go test -count=1 ./internal/trace ./internal/profile ./internal/gladecli -run 'Trace|Profile|RunExecTrace'
```

Expected: pass.

- [ ] **Step 5: Check dirty files**

Run:

```bash
git -C /Users/matt/Dev/glade/.worktrees/apex-perf-doc-review status --short
git -C /Users/matt/Dev/glade-tools status --short
```

Expected: only intentional files from this plan are modified.

## Stop Rules

Stop and ask for direction if:

- A detector needs live Salesforce credentials instead of a local snapshot.
- A base Glade trace change would alter VM behavior or runtime semantics.
- `glade-tools` needs an API not exported by base Glade and adding that API would expose maintenance behavior in public `glade`.
- The plugin cannot correlate trace data without adding stable operation facts to base Glade.

## Definition Of Done

The work is done when:

- `glade performance scan --project . --trace <trace> --org-facts <facts> --format json` emits schema version 2 findings with path, evidence, resource risk, confidence, score, fix, and acceptance.
- At least five P0 findings have graph-backed tests: interprocedural DB-in-loop, static first-touch, repeated describe/config work, SOQL bind risk, and DML blast radius.
- Trace-backed findings outrank static-only findings in tests.
- Metadata-backed DML blast-radius findings include Flow or Workflow evidence.
- Plugin tests pass uncached.
- Any base Glade trace changes pass focused trace/profile tests.

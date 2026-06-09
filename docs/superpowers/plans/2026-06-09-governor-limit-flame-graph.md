# Governor Limit Flame Graph

> **Prerequisite:** Phase 12 (VM-powered test profiling) must be landed first.
> This phase reads the trace events already emitted by the VM and the
> per-test limit data already captured in `testreport.Case`.

**Goal:** Build a hierarchical call tree from VM trace events that shows governor limit consumption attributed to each method in the call stack, so developers can see exactly which methods consume the most SOQL, DML, CPU, and heap.

**Architecture:** The VM emits Chrome trace duration events for every method call, SOQL execution, and DML operation. Each duration event carries method name, class name, file, line, and column in `Args`. Parse the flat trace into a tree using parent-child nesting (a span starts when a method enters and ends when it exits — all events between enter and exit are children). Aggregate limit consumption at each tree node from the child SOQL/DML events and the `apex.limits` trace event.

---

## File Structure

- Create `internal/profile/flame.go`: flame graph builder, tree nodes, aggregation.
- Create `internal/profile/flame_test.go`: unit tests for tree construction and attribution.
- Modify `internal/gladecli/test_command.go`: add `--flame` flag, emit flame graph after test run.
- Modify `internal/gladecli/test_command_test.go`: CLI contract test for flame output.

---

## Precondition Check

Run before starting:

```bash
go test ./internal/vm -run TestTraceIncludesMethodSOQLAndDMLDurations -count=1
```

Expected: PASS. This confirms the VM emits method, SOQL, and DML duration events with file/line/column metadata.

---

## Task 1: Build The Flame Graph Engine

### Step 1: Write the flame graph test

Create `internal/profile/flame_test.go`:

```go
package profile

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/trace"
)

func TestFlameGraphBuildsCallTree(t *testing.T) {
	events := []trace.Event{
		// method "outerMethod" enters at time 0, exits at time 200000
		trace.Duration("apex.method.AccountHandler.handle", "apex.method", 0, 200000, map[string]any{
			"method": "handle", "class": "AccountHandler", "file": "AccountHandler.cls", "line": 12, "column": 5,
		}),
		// SOQL inside outerMethod
		trace.Duration("apex.soql", "apex.soql", 10000, 50000, map[string]any{
			"query": "SELECT Id, Name FROM Account", "rows": 200, "line": 15,
		}),
		// inner method enters at time 70000, exits at 170000 (nested inside outerMethod)
		trace.Duration("apex.method.AccountSelector.selectByIds", "apex.method", 70000, 100000, map[string]any{
			"method": "selectByIds", "class": "AccountSelector", "file": "AccountSelector.cls", "line": 4, "column": 9,
		}),
		// SOQL inside inner method
		trace.Duration("apex.soql", "apex.soql", 80000, 30000, map[string]any{
			"query": "SELECT Id, Name FROM Contact", "rows": 50, "line": 6,
		}),
		// DML after inner method returns
		trace.Duration("apex.dml.update", "apex.dml", 180000, 15000, map[string]any{
			"operation": "update", "rows": 3, "objects": "Account",
		}),
		// limits event
		trace.Instant("apex.limits", "apex.limits", 200001, map[string]any{
			"queries": 2, "queryRows": 250, "dmlStatements": 1, "dmlRows": 3, "cpuTimeMs": 35,
		}),
	}

	root := BuildFlameGraph(events)
	if root == nil {
		t.Fatal("expected root node")
	}

	// Root should have 1 child (outerMethod)
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 root child, got %d", len(root.Children))
	}

	outer := root.Children[0]
	if outer.Name != "AccountHandler.handle" {
		t.Fatalf("expected AccountHandler.handle, got %s", outer.Name)
	}

	// outer should have SOQL count = 2 (its own + child's)
	if outer.SOQLQueries != 2 || outer.SOQLRows != 250 {
		t.Fatalf("outer SOQL: queries=%d rows=%d", outer.SOQLQueries, outer.SOQLRows)
	}
	if outer.DMLStatements != 1 || outer.DMLRows != 3 {
		t.Fatalf("outer DML: statements=%d rows=%d", outer.DMLStatements, outer.DMLRows)
	}

	// outer should have 2 children: the inner method + the DML event
	if len(outer.Children) < 2 {
		t.Fatalf("expected >= 2 children, got %d", len(outer.Children))
	}

	// inner method
	var inner *FlameNode
	for _, c := range outer.Children {
		if c.Name == "AccountSelector.selectByIds" {
			inner = c
			break
		}
	}
	if inner == nil {
		t.Fatal("expected AccountSelector.selectByIds child")
	}
	if inner.SOQLQueries != 1 || inner.SOQLRows != 50 {
		t.Fatalf("inner SOQL: queries=%d rows=%d", inner.SOQLQueries, inner.SOQLRows)
	}
	if inner.File != "AccountSelector.cls" || inner.Line != 4 {
		t.Fatalf("inner location: file=%s line=%d", inner.File, inner.Line)
	}
}

func TestFlameGraphJSONIncludesLimitAttribution(t *testing.T) {
	events := []trace.Event{
		trace.Duration("apex.method.Foo.bar", "apex.method", 0, 1000, map[string]any{
			"method": "bar", "class": "Foo", "file": "Foo.cls", "line": 1,
		}),
		trace.Duration("apex.soql", "apex.soql", 100, 500, map[string]any{
			"query": "SELECT Id FROM Account", "rows": 100,
		}),
		trace.Instant("apex.limits", "apex.limits", 1001, map[string]any{
			"queries": 1, "queryRows": 100, "cpuTimeMs": 12,
		}),
	}

	node := BuildFlameGraph(events).Children[0]
	data, err := json.Marshal(node)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`"soqlQueries":1`,
		`"soqlRows":100`,
		`"cpuTimeMs":12`,
		`"name":"Foo.bar"`,
		`"file":"Foo.cls"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("json missing %q: %s", want, text)
		}
	}
}
```

### Step 2: Run flame test and verify it fails

```bash
go test ./internal/profile -run TestFlameGraph -count=1
```

Expected: FAIL because `BuildFlameGraph` and `FlameNode` do not exist.

### Step 3: Add the flame graph types and builder

Create `internal/profile/flame.go`:

```go
package profile

import (
	"sort"

	"github.com/glade-sh/glade/internal/trace"
)

type FlameNode struct {
	Name          string       `json:"name"`
	Category      string       `json:"category,omitempty"`
	File          string       `json:"file,omitempty"`
	Line          int          `json:"line,omitempty"`
	Column        int          `json:"column,omitempty"`
	DurationUS    int64        `json:"durationUs"`
	SelfDurationUS int64       `json:"selfDurationUs,omitempty"`
	Count         int          `json:"count"`
	SOQLQueries   int          `json:"soqlQueries"`
	SOQLRows      int          `json:"soqlRows"`
	DMLStatements int          `json:"dmlStatements"`
	DMLRows       int          `json:"dmlRows"`
	CPUTimeMS     int          `json:"cpuTimeMs"`
	Children      []*FlameNode `json:"children,omitempty"`
}

type flameBuilder struct {
	events []trace.Event
	nodes  []*FlameNode
	stack  []*FlameNode
	root   *FlameNode
}

func BuildFlameGraph(events []trace.Event) *FlameNode {
	sorted := make([]trace.Event, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp < sorted[j].Timestamp
	})

	fb := &flameBuilder{
		events: sorted,
		root: &FlameNode{
			Name:     "root",
			Category: "transaction",
		},
	}
	fb.stack = []*FlameNode{fb.root}

	for _, e := range sorted {
		fb.process(e)
	}

	// Build stack to close any remaining open spans
	for len(fb.stack) > 1 {
		fb.popNode(fb.events[len(fb.events)-1].Timestamp + fb.events[len(fb.events)-1].Duration)
	}

	fb.aggregate(fb.root)
	return fb.root
}

func (fb *flameBuilder) process(e trace.Event) {
	if e.Phase == trace.PhaseInstant {
		if e.Category == "apex.limits" {
			fb.root.SOQLQueries = intArg(e.Args, "queries")
			fb.root.SOQLRows = intArg(e.Args, "queryRows")
			fb.root.DMLStatements = intArg(e.Args, "dmlStatements")
			fb.root.DMLRows = intArg(e.Args, "dmlRows")
			fb.root.CPUTimeMS = intArg(e.Args, "cpuTimeMs")
		}
		return
	}
	if e.Phase != trace.PhaseComplete {
		return
	}

	// Close any spans that end before this event starts
	fb.closeSpansBefore(e.Timestamp)

	// Determine if this is a method span or an operation span
	if e.Category == "apex.method" {
		fb.pushMethodNode(e)
	} else {
		fb.pushOperationNode(e)
	}
}

func (fb *flameBuilder) closeSpansBefore(ts int64) {
	for len(fb.stack) > 1 {
		top := fb.stack[len(fb.stack)-1]
		topStart := topStartTime(fb.events, top)
		topDur := top.DurationUS
		if topStart+topDur <= ts || topStart+topDur == 0 {
			fb.popNode(topStart + topDur)
		} else {
			break
		}
	}
}

func (fb *flameBuilder) pushMethodNode(e trace.Event) {
	name := methodName(e)
	node := &FlameNode{
		Name:       name,
		Category:   "apex.method",
		File:       stringArg(e.Args, "file"),
		Line:       intArg(e.Args, "line"),
		Column:     intArg(e.Args, "column"),
		DurationUS: e.Duration,
		Count:      1,
	}
	parent := fb.stack[len(fb.stack)-1]
	parent.Children = append(parent.Children, node)
	fb.stack = append(fb.stack, node)
	fb.nodes = append(fb.nodes, node)
}

func (fb *flameBuilder) pushOperationNode(e trace.Event) {
	name := e.Name
	if e.Category == "apex.soql" {
		if q, ok := e.Args["query"].(string); ok {
			name = truncateQuery(q, 80)
		}
	}
	node := &FlameNode{
		Name:       name,
		Category:   e.Category,
		DurationUS: e.Duration,
		Count:      1,
	}
	if e.Category == "apex.soql" {
		node.SOQLQueries = 1
		node.SOQLRows = intArg(e.Args, "rows")
	}
	if e.Category == "apex.dml" || strings.HasPrefix(e.Category, "apex.dml.") {
		node.DMLStatements = 1
		node.DMLRows = intArg(e.Args, "rows")
	}
	parent := fb.stack[len(fb.stack)-1]
	parent.Children = append(parent.Children, node)
}

func (fb *flameBuilder) popNode(endTS int64) {
	if len(fb.stack) <= 1 {
		return
	}
	node := fb.stack[len(fb.stack)-1]
	fb.stack = fb.stack[:len(fb.stack)-1]
	node.DurationUS = endTS - topStartTime(fb.events, node)
}

func (fb *flameBuilder) aggregate(node *FlameNode) {
	for _, child := range node.Children {
		fb.aggregate(child)
		node.SOQLQueries += child.SOQLQueries
		node.SOQLRows += child.SOQLRows
		node.DMLStatements += child.DMLStatements
		node.DMLRows += child.DMLRows
		node.CPUTimeMS += child.CPUTimeMS
		node.DurationUS += child.DurationUS
	}
	// Self duration = total duration - sum of child durations
	childSum := int64(0)
	for _, child := range node.Children {
		childSum += child.DurationUS
	}
	node.SelfDurationUS = node.DurationUS - childSum
	if node.SelfDurationUS < 0 {
		node.SelfDurationUS = 0
	}
}

func topStartTime(events []trace.Event, node *FlameNode) int64 {
	for _, e := range events {
		if matchesNode(e, node) {
			return e.Timestamp
		}
	}
	return 0
}

func matchesNode(e trace.Event, node *FlameNode) bool {
	if e.Category == "apex.method" && node.Category == "apex.method" {
		return methodName(e) == node.Name
	}
	if (e.Category == "apex.soql" || strings.HasPrefix(e.Category, "apex.dml")) &&
		node.Category != "apex.method" {
		return e.Name == node.Name
	}
	return false
}

func methodName(e trace.Event) string {
	if e.Category != "apex.method" {
		return e.Name
	}
	class := stringArg(e.Args, "class")
	method := stringArg(e.Args, "method")
	if class != "" && method != "" {
		return class + "." + method
	}
	return e.Name
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func intArg(args map[string]any, key string) int {
	if args == nil {
		return 0
	}
	switch v := args[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func truncateQuery(query string, maxLen int) string {
	q := strings.TrimSpace(query)
	if len(q) <= maxLen {
		return q
	}
	return q[:maxLen-3] + "..."
}
```

Add `"strings"` to imports.

### Step 4: Run flame tests

```bash
gofmt -w internal/profile/flame.go
go test ./internal/profile -run TestFlameGraph -count=1
```

Expected: PASS.

---

## Task 2: Wire Flame Graph Into `glade test`

### Step 1: Add `--flame` flag

In `internal/gladecli/test_command.go`, add:

```go
flameOut string  // --flame <path> writes flame graph JSON
```

### Step 2: Build and write flame graph after test run

After the test report is built, if `--flame` is set, iterate over cases with trace data and build a flame graph:

```go
if opts.flameOut != "" {
    root := &profile.FlameNode{Name: "All Tests", Category: "test-run"}
    for _, c := range report.Cases {
        if len(c.Trace) == 0 {
            continue
        }
        testRoot := profile.BuildFlameGraph(c.Trace)
        testRoot.Name = c.Name
        testRoot.File = c.ClassName
        root.Children = append(root.Children, testRoot)
    }
    data, err := json.MarshalIndent(root, "", "  ")
    if err != nil {
        return fmt.Errorf("flame: %w", err)
    }
    if err := os.WriteFile(opts.flameOut, data, 0644); err != nil {
        return fmt.Errorf("flame: %w", err)
    }
}
```

### Step 3: Write CLI test

In `internal/gladecli/test_command_test.go`:

```go
func TestTestCommandFlameOutput(t *testing.T) {
	root := writeTestProjectWithPerformanceRisks(t)
	outDir := t.TempDir()
	flamePath := filepath.Join(outDir, "flame.json")
	var stdout, stderr strings.Builder
	code := Run(context.Background(), []string{
		"test", "--project", root, "--json",
		"--flame", flamePath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	data, err := os.ReadFile(flamePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"name"`, `"soqlQueries"`, `"children"`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("flame json missing %q: %s", want, string(data))
		}
	}
}
```

### Step 4: Run CLI tests

```bash
go test ./internal/gladecli -run TestTestCommandFlameOutput -count=1
```

Expected: PASS.

---

## Task 3: Final Validation

```bash
go test ./internal/profile ./internal/gladecli -count=1
```

Expected: PASS.

---

## Design Notes

- **Span nesting uses time containment**: A method span `[ts=0, dur=200000]` contains all events with timestamp in `[0, 200000]`. The builder matches by pushing method nodes onto a stack and popping when the timestamp exceeds the parent's end time. This is the standard approach for Chrome trace conversion.

- **Limit attribution flows up**: Each leaf node (SOQL, DML) carries its own limit counts. Aggregation walks bottom-up, summing child counts into parents. The root node shows total transaction consumption.

- **Self duration** = total duration - sum of child durations. This surfaces how much time a method spent on its own work (not callees). Useful for identifying methods that do heavy CPU work without SOQL/DML children.

- **The `apex.limits` trace event** is always an instant event at the end of execution. It provides the authoritative totals. The flame graph uses it to populate root-level limits for cross-verification with aggregated child values.

- **Output format**: JSON matching the `FlameNode` schema. This is consumable by flame graph visualizers (Speedscope, Chrome trace viewer) after conversion, or can be rendered as a custom HTML report in a follow-up phase.


package apextest

import (
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

// conformanceCounts observes the existing assertions. It adds no assertion and
// does not turn a carried, skipped, or local-only check into a native match.
type conformanceCounts struct {
	mu     sync.Mutex
	rows   map[string]map[string]conformanceCountResult
	serial int
}

type conformanceCountResult struct {
	rows   int
	passed bool
	kind   string
}

func newConformanceCounts() *conformanceCounts {
	return &conformanceCounts{rows: make(map[string]map[string]conformanceCountResult)}
}

func newConformanceCountLog(t *testing.T, family, api string) *conformanceCounts {
	t.Helper()
	counts := newConformanceCounts()
	t.Cleanup(func() { counts.Log(t, family, api, "") })
	return counts
}

func (c *conformanceCounts) put(route, id string, rows int, passed bool, kind string) {
	if route == "isTest" {
		route = "@IsTest"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rows[route] == nil {
		c.rows[route] = make(map[string]conformanceCountResult)
	}
	if previous, ok := c.rows[route][id]; ok {
		passed = passed && previous.passed
		rows = previous.rows
		if previous.kind == "exact" {
			kind = "exact"
		}
	}
	c.rows[route][id] = conformanceCountResult{rows: rows, passed: passed, kind: kind}
}

// Run keeps the existing subtest identity. A native row asserted more than once
// on one route is counted once, and every repeated assertion must pass.
func (c *conformanceCounts) Run(t *testing.T, route, id, name string, fn func(*testing.T)) bool {
	return c.RunKind(t, route, id, name, "exact", fn)
}

func (c *conformanceCounts) RunKind(t *testing.T, route, id, name, kind string, fn func(*testing.T)) bool {
	t.Helper()
	return t.Run(name, func(child *testing.T) {
		child.Cleanup(func() {
			if !child.Skipped() {
				c.put(route, id, 1, !child.Failed(), kind)
			}
		})
		fn(child)
	})
}

// Track observes an existing batch assertion. If that batch fails, its match
// subtotal is zero; individual matches are not inferred from a failed batch.
func (c *conformanceCounts) Track(t *testing.T, route string, rows int) {
	c.TrackKind(t, route, "exact", rows)
}

func (c *conformanceCounts) TrackKind(t *testing.T, route, kind string, rows int) {
	t.Helper()
	id := t.Name()
	t.Cleanup(func() {
		if !t.Skipped() {
			c.put(route, id, rows, !t.Failed(), kind)
		}
	})
}

func (c *conformanceCounts) run(t *testing.T, name, route, kind string, fn func(*testing.T)) bool {
	t.Helper()
	if kind == "" {
		return t.Run(name, fn)
	}
	return c.RunKind(t, route, t.Name()+"/"+name, name, kind, fn)
}

func (c *conformanceCounts) record(route, kind string, rows int, passed bool) {
	if kind == "" {
		return
	}
	c.mu.Lock()
	c.serial++
	id := fmt.Sprintf("batch-%d", c.serial)
	c.mu.Unlock()
	c.put(route, id, rows, passed, kind)
}

func (c *conformanceCounts) Log(t *testing.T, family, api, source string) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	routes := make([]string, 0, len(c.rows))
	for route := range c.rows {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	for _, route := range routes {
		matched, total := 0, 0
		kinds := map[string]int{}
		for _, result := range c.rows[route] {
			if result.kind != "exact" {
				kinds[result.kind] += result.rows
				continue
			}
			total += result.rows
			if result.passed {
				matched += result.rows
			}
		}
		suffix := ""
		if source != "" {
			suffix = "; subset " + source
		}
		t.Logf("%s API %s %s exact %d/%d; category-only %d; carried %d; partial %d; legacy %d; line-framing %d; mismatch %d%s", family, api, route, matched, total, kinds["category"], kinds["carried"], kinds["partial"], kinds["legacy"], kinds["line-framing"], kinds["mismatch"], suffix)
	}
}

// conformanceCompilerText uses the compiler's native diagnostic projection,
// as existing conformance adapters do. It never derives wording or a source
// line from the expected capture, so adapter or compiler differences stay visible.
func conformanceCompilerText(diagnostics []diagnostic.Diagnostic, compileErr error) string {
	for _, d := range diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		message := d.Message
		if d.NativeMessage != "" {
			message = d.NativeMessage
		}
		line := 0
		if d.Range != nil {
			line = d.Range.Start.Line
		}
		if d.NativeLine != nil {
			line = *d.NativeLine
		}
		if line > 0 {
			return fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, message)
		}
		return "COMPILE_ERROR\t" + message
	}
	if compileErr != nil {
		return "COMPILE_ERROR\t" + compileErr.Error()
	}
	return ""
}

// Package cpurisk estimates how close local tests come to Salesforce CPU
// limits. Glade runs Apex far faster than Salesforce and does not emulate
// Salesforce CPU time, so this is an advisory estimate: it counts expensive
// operations and weights them with rough per-operation Salesforce costs. It
// never changes a test outcome.
package cpurisk

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/vm"
)

const (
	// SyncLimitMS and AsyncLimitMS are the Salesforce per-transaction CPU caps.
	SyncLimitMS  = 10000.0
	AsyncLimitMS = 60000.0

	// WatchFraction and HighFraction set the warning levels, as a fraction of
	// the synchronous limit.
	WatchFraction = 0.5
	HighFraction  = 0.8
)

// Weights holds approximate Salesforce milliseconds per counted operation.
// Sources say whether a weight comes from a native capture or is a rough guess.
type Weights struct {
	DMLRowMS        float64 `json:"dmlRowMs"`
	SOQLRowMS       float64 `json:"soqlRowMs"`
	SOQLQueryMS     float64 `json:"soqlQueryMs"`
	DMLStatementMS  float64 `json:"dmlStatementMs"`
	LoopIterationMS float64 `json:"loopIterationMs"`
	InstructionMS   float64 `json:"instructionMs"`
	TriggerRunMS    float64 `json:"triggerRunMs"`
	FlowRunMS       float64 `json:"flowRunMs"`
	CalloutMS       float64 `json:"calloutMs"`
}

// DefaultWeights returns the built-in approximate weights.
//
// DML and SOQL row weights come from the native cpu_followup capture
// (1000 Account rows: insert 707 ms, query 5 ms). Every other weight is a
// rough estimate with no capture behind it.
func DefaultWeights() Weights {
	return Weights{
		DMLRowMS:        0.707,
		SOQLRowMS:       0.005,
		SOQLQueryMS:     1,
		DMLStatementMS:  1,
		LoopIterationMS: 0.002,
		InstructionMS:   0.0005,
		TriggerRunMS:    2,
		FlowRunMS:       2,
		CalloutMS:       1,
	}
}

// WeightSources names where each weight comes from.
func WeightSources() map[string]string {
	captured := "captured: Account insert 707 ms and query 5 ms per 1000 rows, cpu_followup native rows"
	return map[string]string{
		"dmlRowMs": captured, "soqlRowMs": captured,
		"soqlQueryMs": "approximate", "dmlStatementMs": "approximate",
		"loopIterationMs": "approximate", "instructionMs": "approximate",
		"triggerRunMs": "approximate", "flowRunMs": "approximate", "calloutMs": "approximate",
	}
}

// Estimate returns the approximate Salesforce CPU milliseconds for the work.
func (w Weights) Estimate(c vm.WorkCounts) float64 {
	return float64(c.DMLRows)*w.DMLRowMS +
		float64(c.SOQLRows)*w.SOQLRowMS +
		float64(c.SOQLQueries)*w.SOQLQueryMS +
		float64(c.DMLStatements)*w.DMLStatementMS +
		float64(c.LoopIterations)*w.LoopIterationMS +
		float64(c.Instructions)*w.InstructionMS +
		float64(c.TriggerRuns)*w.TriggerRunMS +
		float64(c.FlowRuns)*w.FlowRunMS +
		float64(c.Callouts)*w.CalloutMS
}

// Level classifies an estimate against the synchronous limit.
func Level(estimateMS float64) string {
	switch {
	case estimateMS >= SyncLimitMS:
		return "OVER"
	case estimateMS >= HighFraction*SyncLimitMS:
		return "HIGH"
	case estimateMS >= WatchFraction*SyncLimitMS:
		return "WATCH"
	default:
		return "ok"
	}
}

// TestRisk is one test's counted work and estimate.
type TestRisk struct {
	Class      string        `json:"class"`
	Method     string        `json:"method"`
	Work       vm.WorkCounts `json:"work"`
	EstimateMS float64       `json:"estimateMs"`
	SyncPct    float64       `json:"syncPct"`
	AsyncPct   float64       `json:"asyncPct"`
	Level      string        `json:"level"`
	Methods    []MethodRisk  `json:"methods,omitempty"`
}

// MethodRisk is the work inside one Apex method, callees included.
type MethodRisk struct {
	Method     string        `json:"method"`
	Calls      int           `json:"calls"`
	Work       vm.WorkCounts `json:"work"`
	EstimateMS float64       `json:"estimateMs"`
	SyncPct    float64       `json:"syncPct"`
	Level      string        `json:"level"`
}

// Collector gathers per-test work from concurrent test runs.
type Collector struct {
	mu      sync.Mutex
	weights Weights
	tests   []TestRisk
}

// NewCollector returns a collector that uses the default weights.
func NewCollector() *Collector { return &Collector{weights: DefaultWeights()} }

// Record adds one finished test. It is safe for concurrent use.
func (c *Collector) Record(class, method string, total vm.WorkCounts, methods []vm.MethodWork) {
	risk := TestRisk{Class: class, Method: method, Work: total}
	risk.EstimateMS = c.weights.Estimate(total)
	risk.SyncPct = 100 * risk.EstimateMS / SyncLimitMS
	risk.AsyncPct = 100 * risk.EstimateMS / AsyncLimitMS
	risk.Level = Level(risk.EstimateMS)
	self := class + "." + method
	for _, m := range methods {
		if strings.EqualFold(m.Method, self) {
			continue
		}
		est := c.weights.Estimate(m.Work)
		risk.Methods = append(risk.Methods, MethodRisk{
			Method: m.Method, Calls: m.Calls, Work: m.Work, EstimateMS: est,
			SyncPct: 100 * est / SyncLimitMS, Level: Level(est),
		})
	}
	sort.Slice(risk.Methods, func(i, j int) bool {
		if risk.Methods[i].EstimateMS != risk.Methods[j].EstimateMS {
			return risk.Methods[i].EstimateMS > risk.Methods[j].EstimateMS
		}
		return risk.Methods[i].Method < risk.Methods[j].Method
	})
	c.mu.Lock()
	c.tests = append(c.tests, risk)
	c.mu.Unlock()
}

// Report is the advisory CPU risk report.
type Report struct {
	Advisory     string            `json:"advisory"`
	Weights      Weights           `json:"weights"`
	Sources      map[string]string `json:"weightSources"`
	SyncLimitMS  float64           `json:"syncLimitMs"`
	AsyncLimitMS float64           `json:"asyncLimitMs"`
	Tests        []TestRisk        `json:"tests"`
	Methods      []MethodRisk      `json:"methods"`
	Warnings     []string          `json:"warnings"`
}

const advisoryText = "Estimates are approximate. Glade does not emulate Salesforce CPU time; weights are rough per-operation Salesforce costs."

// Report builds the report, tests and methods sorted by estimate descending.
func (c *Collector) Report() Report {
	c.mu.Lock()
	tests := append([]TestRisk(nil), c.tests...)
	c.mu.Unlock()
	sort.Slice(tests, func(i, j int) bool {
		if tests[i].EstimateMS != tests[j].EstimateMS {
			return tests[i].EstimateMS > tests[j].EstimateMS
		}
		if tests[i].Class != tests[j].Class {
			return tests[i].Class < tests[j].Class
		}
		return tests[i].Method < tests[j].Method
	})
	// A method's row is its worst single test, since limits apply per transaction.
	worst := map[string]MethodRisk{}
	calls := map[string]int{}
	for _, t := range tests {
		for _, m := range t.Methods {
			calls[m.Method] += m.Calls
			if cur, ok := worst[m.Method]; !ok || m.EstimateMS > cur.EstimateMS {
				worst[m.Method] = m
			}
		}
	}
	methods := make([]MethodRisk, 0, len(worst))
	for name, m := range worst {
		m.Calls = calls[name]
		methods = append(methods, m)
	}
	sort.Slice(methods, func(i, j int) bool {
		if methods[i].EstimateMS != methods[j].EstimateMS {
			return methods[i].EstimateMS > methods[j].EstimateMS
		}
		return methods[i].Method < methods[j].Method
	})
	report := Report{
		Advisory: advisoryText, Weights: c.weights, Sources: WeightSources(),
		SyncLimitMS: SyncLimitMS, AsyncLimitMS: AsyncLimitMS,
		Tests: tests, Methods: methods, Warnings: []string{},
	}
	for _, t := range tests {
		switch t.Level {
		case "OVER":
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s.%s estimate %s exceeds the 10 s synchronous limit; it fits the 60 s async limit only if it runs async", t.Class, t.Method, formatMS(t.EstimateMS)))
		case "HIGH", "WATCH":
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s.%s estimate %s is %.0f%% of the 10 s synchronous limit", t.Class, t.Method, formatMS(t.EstimateMS), t.SyncPct))
		}
	}
	return report
}

// WriteJSON writes the report as indented JSON.
func WriteJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func formatMS(ms float64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1f s", ms/1000)
	}
	return fmt.Sprintf("%.0f ms", ms)
}

// WriteText writes the top tests and methods and every warning.
func WriteText(w io.Writer, r Report, top int) error {
	if top <= 0 {
		top = 10
	}
	var b strings.Builder
	b.WriteString("CPU risk (advisory)\n")
	b.WriteString(r.Advisory + "\n")
	b.WriteString("It never changes a test result. Limits: 10 s sync, 60 s async.\n\n")
	b.WriteString("Weights (ms per operation):\n")
	src := r.Sources
	for _, row := range []struct {
		name string
		val  float64
		key  string
	}{
		{"DML row", r.Weights.DMLRowMS, "dmlRowMs"}, {"SOQL row returned", r.Weights.SOQLRowMS, "soqlRowMs"},
		{"SOQL query", r.Weights.SOQLQueryMS, "soqlQueryMs"}, {"DML statement", r.Weights.DMLStatementMS, "dmlStatementMs"},
		{"loop iteration", r.Weights.LoopIterationMS, "loopIterationMs"}, {"instruction", r.Weights.InstructionMS, "instructionMs"},
		{"trigger run", r.Weights.TriggerRunMS, "triggerRunMs"}, {"flow run", r.Weights.FlowRunMS, "flowRunMs"},
		{"callout (mocked)", r.Weights.CalloutMS, "calloutMs"},
	} {
		kind := "approximate"
		if strings.HasPrefix(src[row.key], "captured") {
			kind = "captured"
		}
		fmt.Fprintf(&b, "  %-18s %-8g %s\n", row.name, row.val, kind)
	}
	b.WriteString("\n")
	if len(r.Tests) == 0 {
		b.WriteString("No tests ran.\n")
		_, err := io.WriteString(w, b.String())
		return err
	}
	fmt.Fprintf(&b, "Top tests by estimated CPU (%d of %d):\n", min(top, len(r.Tests)), len(r.Tests))
	fmt.Fprintf(&b, "  %-5s %-10s %-7s %-8s %-8s %-8s %-9s %-5s %-5s %s\n", "level", "estimate", "%sync", "dmlRows", "soqlRows", "loops", "triggers", "flows", "callouts", "test")
	for _, t := range r.Tests[:min(top, len(r.Tests))] {
		fmt.Fprintf(&b, "  %-5s %-10s %-7.1f %-8d %-8d %-8d %-9d %-5d %-5d %s.%s\n", t.Level, formatMS(t.EstimateMS), t.SyncPct,
			t.Work.DMLRows, t.Work.SOQLRows, t.Work.LoopIterations, t.Work.TriggerRuns, t.Work.FlowRuns, t.Work.Callouts, t.Class, t.Method)
	}
	if len(r.Methods) > 0 {
		fmt.Fprintf(&b, "\nTop methods, worst single test, callees included (%d of %d):\n", min(top, len(r.Methods)), len(r.Methods))
		fmt.Fprintf(&b, "  %-5s %-10s %-7s %-6s %-8s %-8s %-8s %s\n", "level", "estimate", "%sync", "calls", "dmlRows", "soqlRows", "loops", "method")
		for _, m := range r.Methods[:min(top, len(r.Methods))] {
			fmt.Fprintf(&b, "  %-5s %-10s %-7.1f %-6d %-8d %-8d %-8d %s\n", m.Level, formatMS(m.EstimateMS), m.SyncPct, m.Calls, m.Work.DMLRows, m.Work.SOQLRows, m.Work.LoopIterations, m.Method)
		}
	}
	b.WriteString("\n")
	if len(r.Warnings) == 0 {
		b.WriteString("No test is near the 10 s synchronous limit (warns at 50% of it).\n")
	} else {
		b.WriteString("Warnings:\n")
		for _, warning := range r.Warnings {
			b.WriteString("  " + warning + "\n")
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

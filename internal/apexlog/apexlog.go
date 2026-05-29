// Package apexlog renders a glade execution result into a Salesforce-style
// debug log. The output mirrors the text format the Apex platform produces
// (the familiar "HH:MM:SS.mmm (nanos)|EVENT|details" stream) so developers get
// a recognizable log and can diff glade's behavior against a real org running
// the same code.
//
// The formatter is structurally faithful rather than byte-identical: it emits
// the high-signal events developers recognize (EXECUTION_STARTED,
// CODE_UNIT_STARTED, USER_DEBUG, SOQL_EXECUTE_*, DML_*, exceptions, and the
// cumulative limit usage block) in true execution order. It consumes the
// existing trace stream plus the ordering-anchored debug events without
// mutating either, so the parity/oracle tooling is unaffected.
package apexlog

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/trace"
	"github.com/glade-sh/glade/internal/vm"
)

// Options controls debug-log rendering.
type Options struct {
	// APIVersion is the api version shown in the header line. Defaults to "64.0".
	APIVersion string
	// CodeUnit is the code-unit label. Defaults to "execute_anonymous_apex".
	CodeUnit string
	// User identity rendered on the USER_INFO line. Neutral local-runner
	// defaults are used when empty.
	UserID    string
	Username  string
	TimeZone  string
	GMTOffset string
}

const (
	defaultAPIVersion = "64.0"
	defaultCodeUnit   = "execute_anonymous_apex"
	headerCategories  = "APEX_CODE,DEBUG;APEX_PROFILING,INFO;CALLOUT,INFO;DB,INFO;NBA,INFO;SYSTEM,DEBUG;VALIDATION,INFO;VISUALFORCE,INFO;WAVE,INFO;WORKFLOW,INFO"

	defaultUserID    = "005000000000000AAA"
	defaultUsername  = "glade@local.run"
	defaultTimeZone  = "Greenwich Mean Time (GMT)"
	defaultGMTOffset = "GMT+00:00"
)

// Format renders the result of an anonymous Apex execution as a
// Salesforce-style debug log. runErr is the error returned by execution (nil on
// success); a non-nil error is rendered as a FATAL_ERROR line.
func Format(result *vm.Result, runErr error, opts Options) string {
	if result == nil {
		result = &vm.Result{}
	}
	if opts.APIVersion == "" {
		opts.APIVersion = defaultAPIVersion
	}
	if opts.CodeUnit == "" {
		opts.CodeUnit = defaultCodeUnit
	}
	if opts.UserID == "" {
		opts.UserID = defaultUserID
	}
	if opts.Username == "" {
		opts.Username = defaultUsername
	}
	if opts.TimeZone == "" {
		opts.TimeZone = defaultTimeZone
	}
	if opts.GMTOffset == "" {
		opts.GMTOffset = defaultGMTOffset
	}

	var b strings.Builder
	c := &clock{}

	b.WriteString(opts.APIVersion + " " + headerCategories + "\n")
	writeLine(&b, c, fmt.Sprintf("USER_INFO|[EXTERNAL]|%s|%s|%s|%s", opts.UserID, opts.Username, opts.TimeZone, opts.GMTOffset))
	writeLine(&b, c, "EXECUTION_STARTED")
	writeLine(&b, c, "CODE_UNIT_STARTED|[EXTERNAL]|"+opts.CodeUnit)

	emitBody(&b, c, result)

	if runErr != nil {
		writeLine(&b, c, "FATAL_ERROR|"+sanitize(runErr.Error()))
	}

	writeLimitBlock(&b, c, result)
	writeLine(&b, c, "CODE_UNIT_FINISHED|"+opts.CodeUnit)
	writeLine(&b, c, "EXECUTION_FINISHED")
	return b.String()
}

// emitBody walks the trace stream and interleaves debug events at their
// recorded trace positions, rendering each into the recognizable SF events.
func emitBody(b *strings.Builder, c *clock, result *vm.Result) {
	debugByPos := groupDebugByPos(result.DebugEvents)

	emitDebugAt := func(pos int) {
		for _, d := range debugByPos[pos] {
			line := d.Line
			if line == 0 {
				line = nearestLine(result.Trace, pos)
			}
			writeLine(b, c, fmt.Sprintf("USER_DEBUG|[%d]|%s|%s", line, levelOrDefault(d.Level), sanitize(d.Message)))
		}
	}

	for i := range result.Trace {
		emitDebugAt(i)
		emitTraceEvent(b, c, result.Trace[i])
	}
	emitDebugAt(len(result.Trace))
}

func emitTraceEvent(b *strings.Builder, c *clock, e trace.Event) {
	name := e.Name
	switch {
	case name == "apex.soql" || name == "apex.soql.stub" || name == "apex.sosl":
		line := argInt(e.Args, "line")
		query := argString(e.Args, "query")
		rows := argInt(e.Args, "rows")
		writeLine(b, c, fmt.Sprintf("SOQL_EXECUTE_BEGIN|[%d]|Aggregations:0|%s", line, query))
		writeLine(b, c, fmt.Sprintf("SOQL_EXECUTE_END|[%d]|Rows:%d", line, rows))
	case strings.HasPrefix(name, "apex.dml."):
		line := argInt(e.Args, "line")
		op := titleOp(argString(e.Args, "operation"))
		rows := argInt(e.Args, "rows")
		typ := firstObject(argStrings(e.Args, "objects"))
		writeLine(b, c, fmt.Sprintf("DML_BEGIN|[%d]|Op:%s|Type:%s|Rows:%d", line, op, typ, rows))
		writeLine(b, c, fmt.Sprintf("DML_END|[%d]", line))
	case strings.HasPrefix(name, "apex.callout."):
		writeLine(b, c, "CALLOUT_REQUEST|[0]|"+argString(e.Args, "endpoint"))
		writeLine(b, c, "CALLOUT_RESPONSE|[0]|"+argString(e.Args, "status"))
	}
}

func groupDebugByPos(events []vm.DebugEvent) map[int][]vm.DebugEvent {
	if len(events) == 0 {
		return nil
	}
	out := make(map[int][]vm.DebugEvent, len(events))
	for _, d := range events {
		out[d.TracePos] = append(out[d.TracePos], d)
	}
	return out
}

func writeLimitBlock(b *strings.Builder, c *clock, result *vm.Result) {
	l := result.Limits
	type govLimit struct {
		label string
		used  int
		cap   int
	}
	limits := []govLimit{
		{"Number of SOQL queries", l.Queries, 100},
		{"Number of query rows", l.QueryRows, 50000},
		{"Number of SOSL queries", 0, 20},
		{"Number of DML statements", l.DMLStatements, 150},
		{"Number of Publish Immediate DML", 0, 150},
		{"Number of DML rows", l.DMLRows, 10000},
		{"Maximum CPU time", l.CPUTimeMS, 10000},
		{"Maximum heap size", l.HeapSize, 6000000},
		{"Number of callouts", l.Callouts, 100},
		{"Number of Email Invocations", l.EmailInvokes, 10},
		{"Number of future calls", l.FutureCalls, 50},
		{"Number of queueable jobs added to the queue", l.QueueableJobs, 50},
		{"Number of Mobile Apex push calls", 0, 10},
	}

	writeLine(b, c, "CUMULATIVE_LIMIT_USAGE")
	writeLine(b, c, "LIMIT_USAGE_FOR_NS|(default)|")
	for _, g := range limits {
		b.WriteString(fmt.Sprintf("  %s: %d out of %d\n", g.label, g.used, g.cap))
	}
	writeLine(b, c, "CUMULATIVE_LIMIT_USAGE_END")
}

// clock synthesizes monotonic timestamps from a sequence counter so the log is
// deterministic (trace timestamps are sequence numbers, not real nanoseconds).
type clock struct {
	step int64
}

func (c *clock) next() string {
	nanos := c.step * 1_000_000
	c.step++
	totalMs := nanos / 1_000_000
	ms := totalMs % 1000
	totalS := totalMs / 1000
	s := totalS % 60
	m := (totalS / 60) % 60
	h := (totalS / 3600) % 24
	return fmt.Sprintf("%02d:%02d:%02d.%03d (%d)", h, m, s, ms, nanos)
}

func writeLine(b *strings.Builder, c *clock, body string) {
	b.WriteString(c.next() + "|" + body + "\n")
}

func levelOrDefault(level string) string {
	if level == "" {
		return "DEBUG"
	}
	return level
}

func nearestLine(events []trace.Event, pos int) int {
	for i := pos - 1; i >= 0 && i < len(events); i-- {
		if v := argInt(events[i].Args, "line"); v > 0 {
			return v
		}
	}
	return 1
}

func titleOp(op string) string {
	switch strings.ToLower(op) {
	case "insert":
		return "Insert"
	case "update":
		return "Update"
	case "delete":
		return "Delete"
	case "upsert":
		return "Upsert"
	case "undelete":
		return "Undelete"
	case "merge":
		return "Merge"
	case "":
		return "Unknown"
	default:
		return strings.ToUpper(op[:1]) + strings.ToLower(op[1:])
	}
}

func firstObject(objects []string) string {
	if len(objects) == 0 {
		return "Unknown"
	}
	return objects[0]
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

func argString(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func argInt(args map[string]any, key string) int {
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
	default:
		return 0
	}
}

func argStrings(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	switch v := args[key].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

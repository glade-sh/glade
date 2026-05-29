# Salesforce-style debug log

Date: 2026-05-29
Status: v1 implemented (exec surface)

## Problem

glade emits a JSON trace (`exec --trace`, `exec --json`). It is precise but
foreign. Apex developers read the platform's text debug log every day — the
`HH:MM:SS.mmm (nanos)|EVENT|details` stream. Two goals:

1. Give developers the familiar log so glade output feels like the platform.
2. Let a developer run the same code on glade and on a real org and compare the
   two logs to confirm glade runs accurately.

## Constraints

- Mimic Salesforce, do not diverge. The log must be recognizable.
- Do not break tests and do not touch the parity/oracle tooling. The oracle
  (`internal/oracle`) normalizes BOTH glade's trace stream AND parsed real
  Salesforce logs into a common model and diffs them. Anything that changes the
  trace event sequence risks shifting those comparisons.
- Trace timestamps are sequence numbers, not nanoseconds.
- `System.debug` did not emit a trace event — it appended to `result.Debug`, a
  flat ordered list with no position relative to SOQL/DML in the trace.

## Approach

Render a `vm.Result` into Salesforce-style text in a new `internal/apexlog`
package. Structurally faithful, not byte-identical: emit the high-signal events
developers recognize, in true execution order.

### Why not add debug to the trace stream

The obvious fix for interleaving is to emit an `apex.debug` trace event at
`System.debug` time. But `internal/oracle/local_runner.go`
`oracleEventsFromTrace` maps EVERY trace event into the comparison model, so a
new event would change local parity sequences and could break oracle diffs.

### Ordering anchor (chosen)

Add an additive `Result.DebugEvents []DebugEvent{Level, Message, Line, TracePos}`
field, populated at `System.debug` time only when tracing is enabled. `TracePos`
is `len(Trace)` at emit time. The formatter interleaves each debug event at its
recorded trace position. The trace stream and `result.Debug` are untouched, so
the oracle is unaffected (confirmed: `go test ./internal/oracle/...` green).

## Event mapping (v1)

| glade trace / data        | Salesforce log line                                  |
| ------------------------- | ---------------------------------------------------- |
| header                    | `64.0 APEX_CODE,DEBUG;...`                            |
| start                     | `EXECUTION_STARTED`, `CODE_UNIT_STARTED|[EXTERNAL]|execute_anonymous_apex` |
| `DebugEvent`              | `USER_DEBUG|[line]|LEVEL|message`                    |
| `apex.soql` / `.stub` / `apex.sosl` | `SOQL_EXECUTE_BEGIN|[line]|Aggregations:0|query` + `SOQL_EXECUTE_END|[line]|Rows:n` |
| `apex.dml.<op>`           | `DML_BEGIN|[line]|Op:Insert|Type:Account|Rows:n` + `DML_END` |
| `apex.callout.*`          | `CALLOUT_REQUEST` / `CALLOUT_RESPONSE`               |
| run error                 | `FATAL_ERROR|message`                                |
| limits                    | `CUMULATIVE_LIMIT_USAGE` + `LIMIT_USAGE_FOR_NS|(default)|` + `... out of cap` |
| end                       | `CODE_UNIT_FINISHED`, `EXECUTION_FINISHED`           |

Timestamps are synthesized monotonic (`step * 1ms`) so the log is deterministic
and diff-friendly. Debug line numbers fall back to the nearest preceding
statement trace line when not captured directly.

YAGNI for v1: METHOD_ENTRY/EXIT, STATEMENT_EXECUTE, HEAP_ALLOCATE, and per-event
profiling noise are omitted. They add volume without helping the "is glade
correct?" comparison.

## Surface (v1)

`glade exec --debug-log <path>` writes the log to a file; `--debug-log -` writes
to stdout. `--json` and `--trace` are unchanged. The flag enables tracing
internally. On a runtime error the log is written first (capturing FATAL_ERROR),
then the error is reported as usual.

## Testing

- `internal/apexlog`: header/framing, debug↔SOQL↔DML interleave order, FATAL,
  limit usage reflecting `Result.Limits`, newline sanitization, debug line
  fallback.
- `internal/gladecli`: `exec --debug-log <file>` and `--debug-log -`.
- Regression guard: `go test ./...` and `./internal/oracle/...` stay green.

## Comparing against a real org

Capture the org log for the same anonymous Apex (Developer Console / `sf apex
run` with a trace flag) and diff against `glade exec --debug-log`. The
`internal/oracle` package already parses real Salesforce logs and the glade
trace into a common model for structured diffing; that remains the deeper
equivalence path. The text log is the familiar, eyeball-friendly surface.

## Follow-ups (not in v1)

- Per-test Salesforce logs from `glade test` (one log per test method).
- Playground: add the Salesforce-style log to the run response / a UI toggle.
- A `glade ... --diff-log <org.log>` convenience that renders glade's log and
  diffs it against a captured org log via the oracle model.
- Richer events (METHOD_ENTRY/EXIT, VARIABLE_ASSIGNMENT) behind a verbosity flag
  if a fuller comparison is needed.

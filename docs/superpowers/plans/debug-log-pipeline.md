# Debug Log Pipeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. This plan is written for a low-context agent. Do not skip steps. Use the checkbox syntax to track progress.

**Goal:** Build a useful offline Salesforce debug-log pipeline for ISV support: parse subscriber-org logs, turn measured log events into profile data, annotate likely source locations, and expose the result through `glade debug` commands.

**Architecture:** Keep raw Salesforce debug-log parsing in `internal/apexlog`, because that package already renders Salesforce-style logs for `glade exec --debug-log`. Put source matching and annotation in `internal/debuglog`, so project/type-index logic stays separate from the low-level log parser. Wire a small `glade debug` command in `internal/gladecli` that reuses the parser, matcher, and existing `internal/profile` report writer.

**Tech Stack:** Go, existing `internal/apexlog`, `internal/trace`, `internal/profile`, `internal/typesys`, `internal/project`, `internal/schema`, `internal/soql`, `internal/gladecli`, and existing DAP package only for future breakpoint export. No Salesforce org login. No REST or Tooling API.

---

## Hard Boundaries

Do these things:

- Parse logs offline from a local `.log` or `.txt` file.
- Preserve original log lines and line numbers.
- Extract measured evidence: SOQL rows, DML rows, cumulative limits, CPU, heap, callouts, async, email, exception type/message, and stack frames.
- Convert parsed logs into a `trace.Document` so `internal/profile` can rank measured log activity.
- Match source with conservative confidence scores. Low confidence is better than false confidence.
- Keep parser and matcher fast enough for large enterprise logs.

Do not do these things in this sprint:

- Do not call Salesforce, Tooling API, REST API, Metadata API, or LMA.
- Do not build a VSCode extension.
- Do not add a new public `glade dap` command.
- Do not add replay or parity diff against the VM.
- Do not claim exact source mapping when the log only gives weak evidence.
- Do not scan the whole project once per log entry. Build indexes once.
- Do not silence failures by changing tests to match weak behavior.

Future work can export DAP breakpoint JSON from annotations. That is not required for the first useful cut.

---

## Current Repo Facts To Use

- `internal/apexlog/apexlog.go` already formats a `vm.Result` into Salesforce-style debug log text.
- `glade exec --debug-log <path>` already writes a local debug log. Use it for smoke fixtures.
- `internal/profile/profile.go` already analyzes `trace.Document` and writes JSON or Markdown reports.
- `internal/trace/trace.go` defines `trace.Document` and `trace.Event`.
- `internal/gladecli/cli.go` already has `runProfile`, `runExec`, `loadIndex`, and `parseProjectFlags`.
- `internal/dap` already has protocol handling and live debug support through existing `--debug` flows. Do not widen DAP until the log pipeline is solid.

---

## Parallel Squad Plan

The main agent coordinates. Use parallel subagents immediately.

If subagents can edit separate worktrees, give each squad its own worktree and merge back after tests. If subagents share one worktree, only one squad may write each file group. In a shared worktree, subagents that are not file owners must return patches or notes instead of editing.

### Squad A: Parser And Trace Conversion

Owns only:

- `internal/apexlog/model.go`
- `internal/apexlog/parse.go`
- `internal/apexlog/trace.go`
- `internal/apexlog/parse_test.go`
- `internal/apexlog/trace_test.go`
- `internal/apexlog/testdata/*`

Goal:

- Parse real Salesforce-style debug logs into typed entries.
- Convert parsed entries into `trace.Document` events.
- Keep parsing single-pass and allocation-conscious.

### Squad B: Source Annotation

Owns only:

- `internal/debuglog/model.go`
- `internal/debuglog/source_index.go`
- `internal/debuglog/match.go`
- `internal/debuglog/output.go`
- `internal/debuglog/*_test.go`
- `internal/debuglog/testdata/*`

Goal:

- Build a source index once from `typesys.Index` and Apex source.
- Annotate log entries with conservative source candidates.
- Render annotated text and JSON.

### Squad C: CLI And Docs

Owns only:

- `internal/gladecli/debug_command.go`
- `internal/gladecli/cli.go`
- `internal/gladecli/cli_test.go`
- `docs/EDITOR.md`
- `docs/LOCAL_TESTING.md`

Goal:

- Add `glade debug parse`, `glade debug profile`, and `glade debug explain`.
- Reuse package APIs from Squads A and B.
- Add focused CLI tests and short docs.

### Main Agent: Integration And Review

Owns:

- Integration order.
- API shape conflicts.
- Final test gates.
- Final report of what is implemented and what remains.

The main agent must review every squad patch before merge. No blind merge.

---

## Preflight

- [ ] **Step 1: Check the worktree**

Run:

```bash
git status --short
```

Expected:

- Note any existing modified files.
- Do not overwrite unrelated user changes.
- If files owned by a squad already have changes, read them before editing.

- [ ] **Step 2: Run focused baseline tests**

Run:

```bash
go test -count=1 ./internal/apexlog ./internal/profile ./internal/gladecli ./internal/dap
```

Expected:

- Existing tests pass before this work starts.
- If they fail, record the failing test names and stop for main-agent triage.

- [ ] **Step 3: Generate one local debug log fixture for smoke work**

Run:

```bash
go run ./cmd/glade exec --debug-log /tmp/glade-debug-sample.log "System.debug(LoggingLevel.INFO, 'start work'); Account a = new Account(Name = 'Acme'); insert a; List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'Acme']; System.debug('rows=' + rows.size());"
```

Expected:

- Exit code `0`.
- `/tmp/glade-debug-sample.log` contains `USER_DEBUG`, `DML_BEGIN`, `SOQL_EXECUTE_BEGIN`, and `CUMULATIVE_LIMIT_USAGE`.

---

## Task 1: Low-Level Apex Log Model And Parser

**Files:**

- Create: `internal/apexlog/model.go`
- Create: `internal/apexlog/parse.go`
- Create: `internal/apexlog/parse_test.go`
- Create: `internal/apexlog/testdata/core.log`
- Create: `internal/apexlog/testdata/exception.log`
- Create: `internal/apexlog/testdata/pipe_query.log`
- Modify: `internal/apexlog/apexlog.go` package comment only when the new parser makes the existing "renders only" wording false

### Model Shape

Create `internal/apexlog/model.go` with these exported types:

```go
package apexlog

type Log struct {
	APIVersion string       `json:"apiVersion,omitempty"`
	Header     string       `json:"header,omitempty"`
	Entries    []Entry      `json:"entries"`
	Limits     []LimitUsage  `json:"limits,omitempty"`
}

type Entry struct {
	Raw       string    `json:"raw"`
	Timestamp string    `json:"timestamp,omitempty"`
	Line      int       `json:"line"`
	Kind      EntryKind `json:"kind"`
	Payload   string    `json:"payload,omitempty"`
	Data      EntryData `json:"data,omitempty"`
}

type EntryKind string

const (
	EntryOther                   EntryKind = "OTHER"
	EntryUserInfo                EntryKind = "USER_INFO"
	EntryExecutionStarted        EntryKind = "EXECUTION_STARTED"
	EntryExecutionFinished       EntryKind = "EXECUTION_FINISHED"
	EntryCodeUnitStarted         EntryKind = "CODE_UNIT_STARTED"
	EntryCodeUnitFinished        EntryKind = "CODE_UNIT_FINISHED"
	EntryUserDebug               EntryKind = "USER_DEBUG"
	EntrySOQLExecuteBegin        EntryKind = "SOQL_EXECUTE_BEGIN"
	EntrySOQLExecuteEnd          EntryKind = "SOQL_EXECUTE_END"
	EntryDMLBegin                EntryKind = "DML_BEGIN"
	EntryDMLEnd                  EntryKind = "DML_END"
	EntryExceptionThrown         EntryKind = "EXCEPTION_THROWN"
	EntryFatalError              EntryKind = "FATAL_ERROR"
	EntryEnteringManagedPackage  EntryKind = "ENTERING_MANAGED_PKG"
	EntryCumulativeLimitUsage    EntryKind = "CUMULATIVE_LIMIT_USAGE"
	EntryCumulativeLimitUsageEnd EntryKind = "CUMULATIVE_LIMIT_USAGE_END"
	EntryLimitUsageForNamespace  EntryKind = "LIMIT_USAGE_FOR_NS"
	EntryCalloutRequest          EntryKind = "CALLOUT_REQUEST"
	EntryCalloutResponse         EntryKind = "CALLOUT_RESPONSE"
)

type EntryData struct {
	SourceLine       int          `json:"sourceLine,omitempty"`
	DebugLevel      string       `json:"debugLevel,omitempty"`
	DebugMessage    string       `json:"debugMessage,omitempty"`
	SOQLQuery       string       `json:"soqlQuery,omitempty"`
	SOQLRows        int          `json:"soqlRows,omitempty"`
	DMLOperation    string       `json:"dmlOperation,omitempty"`
	DMLType         string       `json:"dmlType,omitempty"`
	DMLRows         int          `json:"dmlRows,omitempty"`
	ExceptionType   string       `json:"exceptionType,omitempty"`
	ExceptionText   string       `json:"exceptionText,omitempty"`
	StackFrames     []StackFrame `json:"stackFrames,omitempty"`
	CodeUnit         string       `json:"codeUnit,omitempty"`
	Namespace       string       `json:"namespace,omitempty"`
	CalloutEndpoint string       `json:"calloutEndpoint,omitempty"`
	CalloutStatus   string       `json:"calloutStatus,omitempty"`
}

type StackFrame struct {
	Namespace string `json:"namespace,omitempty"`
	Class     string `json:"class,omitempty"`
	Method    string `json:"method,omitempty"`
	Line      int    `json:"line,omitempty"`
	Raw       string `json:"raw"`
}

type LimitUsage struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Used      int    `json:"used"`
	Limit     int    `json:"limit"`
}
```

### Parser Rules

- Use a streaming reader. Do not use `io.ReadAll` on the whole log.
- Accept both `\n` and `\r\n`.
- Keep every input line's original 1-based line number in `Entry.Line`.
- Treat the first non-event line as header when it begins with an API version, for example `64.0 APEX_CODE,DEBUG;`.
- Split event lines into three parts: timestamp, kind, rest. Use `strings.SplitN(line, "|", 3)`.
- For `USER_DEBUG`, parse `USER_DEBUG|[47]|DEBUG|message`.
- For `SOQL_EXECUTE_BEGIN`, keep all text after `Aggregations:<n>|` as the query. Queries can contain `|`.
- For `SOQL_EXECUTE_END`, parse `Rows:<n>`.
- For `DML_BEGIN`, parse `Op:<op>|Type:<type>|Rows:<n>`.
- For limit blocks, parse lines like `Number of SOQL queries: 1 out of 100`.
- For `EXCEPTION_THROWN`, parse exception type and message, then attach following stack-frame lines until the next timestamped event.
- Unknown event kinds become `EntryOther` and keep `Payload`.

### Tests First

- [ ] **Step 1: Add `core.log` fixture**

Use this exact content in `internal/apexlog/testdata/core.log`:

```text
64.0 APEX_CODE,DEBUG;APEX_PROFILING,INFO;CALLOUT,INFO;DB,INFO;SYSTEM,DEBUG
00:00:00.000 (0)|USER_INFO|[EXTERNAL]|005000000000000AAA|isv@example.com|GMT|GMT+00:00
00:00:00.001 (1000000)|EXECUTION_STARTED
00:00:00.002 (2000000)|CODE_UNIT_STARTED|[EXTERNAL]|ns.TestProcessor.run
00:00:00.003 (3000000)|USER_DEBUG|[12]|INFO|start work
00:00:00.004 (4000000)|DML_BEGIN|[13]|Op:Insert|Type:Account|Rows:1
00:00:00.005 (5000000)|DML_END|[13]
00:00:00.006 (6000000)|SOQL_EXECUTE_BEGIN|[14]|Aggregations:0|SELECT Id, Name FROM Account WHERE Name = 'Acme'
00:00:00.007 (7000000)|SOQL_EXECUTE_END|[14]|Rows:1
00:00:00.008 (8000000)|USER_DEBUG|[15]|DEBUG|rows=1
00:00:00.009 (9000000)|CUMULATIVE_LIMIT_USAGE
00:00:00.010 (10000000)|LIMIT_USAGE_FOR_NS|(default)|
  Number of SOQL queries: 1 out of 100
  Number of query rows: 1 out of 50000
  Number of DML statements: 1 out of 150
  Number of DML rows: 1 out of 10000
  Maximum CPU time: 7 out of 10000
  Maximum heap size: 8120 out of 6000000
00:00:00.011 (11000000)|CUMULATIVE_LIMIT_USAGE_END
00:00:00.012 (12000000)|CODE_UNIT_FINISHED|ns.TestProcessor.run
00:00:00.013 (13000000)|EXECUTION_FINISHED
```

- [ ] **Step 2: Add parser tests**

Create `internal/apexlog/parse_test.go` with tests named:

- `TestParseCoreDebugLog`
- `TestParseSOQLQueryContainingPipe`
- `TestParseExceptionStackFrames`
- `TestParseLimitUsage`

The tests must assert actual parsed values, not just entry counts.

Required assertions:

```go
if log.APIVersion != "64.0" {
	t.Fatalf("api version = %q, want 64.0", log.APIVersion)
}
if got := findEntry(log, EntryUserDebug).Data.DebugMessage; got != "start work" {
	t.Fatalf("debug message = %q, want start work", got)
}
if got := findEntry(log, EntryDMLBegin).Data.DMLType; got != "Account" {
	t.Fatalf("dml type = %q, want Account", got)
}
if got := findEntry(log, EntrySOQLExecuteBegin).Data.SOQLQuery; got != "SELECT Id, Name FROM Account WHERE Name = 'Acme'" {
	t.Fatalf("soql query = %q", got)
}
if got := limitByName(log, "Maximum CPU time").Used; got != 7 {
	t.Fatalf("cpu used = %d, want 7", got)
}
```

Helper functions in the test file should look like this:

```go
func findEntry(log Log, kind EntryKind) Entry {
	for _, entry := range log.Entries {
		if entry.Kind == kind {
			return entry
		}
	}
	return Entry{}
}

func limitByName(log Log, name string) LimitUsage {
	for _, limit := range log.Limits {
		if limit.Name == name {
			return limit
		}
	}
	return LimitUsage{}
}
```

- [ ] **Step 3: Add `pipe_query.log` fixture**

Use this exact content in `internal/apexlog/testdata/pipe_query.log`:

```text
64.0 APEX_CODE,DEBUG;DB,INFO
00:00:00.000 (0)|EXECUTION_STARTED
00:00:00.001 (1000000)|SOQL_EXECUTE_BEGIN|[8]|Aggregations:0|SELECT Id FROM Account WHERE Name = 'A|B'
00:00:00.002 (2000000)|SOQL_EXECUTE_END|[8]|Rows:0
00:00:00.003 (3000000)|EXECUTION_FINISHED
```

The parser test must assert:

```go
if got := findEntry(log, EntrySOQLExecuteBegin).Data.SOQLQuery; got != "SELECT Id FROM Account WHERE Name = 'A|B'" {
	t.Fatalf("pipe query = %q", got)
}
```

- [ ] **Step 4: Add `exception.log` fixture**

Use this exact content in `internal/apexlog/testdata/exception.log`:

```text
64.0 APEX_CODE,DEBUG;SYSTEM,DEBUG
00:00:00.000 (0)|EXECUTION_STARTED
00:00:00.001 (1000000)|CODE_UNIT_STARTED|[EXTERNAL]|ns.TestProcessor.fail
00:00:00.002 (2000000)|EXCEPTION_THROWN|[21]|System.AuraHandledException: Nope
Class.ns.TestProcessor.fail: line 21, column 1
Class.ns.TestController.callProcessor: line 4, column 1
00:00:00.003 (3000000)|FATAL_ERROR|System.AuraHandledException: Nope
00:00:00.004 (4000000)|EXECUTION_FINISHED
```

The parser test must assert:

```go
entry := findEntry(log, EntryExceptionThrown)
if entry.Data.ExceptionType != "System.AuraHandledException" {
	t.Fatalf("exception type = %q", entry.Data.ExceptionType)
}
if len(entry.Data.StackFrames) != 2 {
	t.Fatalf("stack frames = %d, want 2", len(entry.Data.StackFrames))
}
if entry.Data.StackFrames[0].Class != "TestProcessor" || entry.Data.StackFrames[0].Method != "fail" || entry.Data.StackFrames[0].Line != 21 {
	t.Fatalf("first stack frame = %#v", entry.Data.StackFrames[0])
}
```

- [ ] **Step 5: Run tests and verify they fail**

Run:

```bash
go test -count=1 ./internal/apexlog -run 'TestParse'
```

Expected:

- Fail because `Parse` does not exist.

- [ ] **Step 6: Implement `Parse`**

Create:

```go
func Parse(r io.Reader) (Log, error)
```

Implementation notes:

- Use `bufio.Reader.ReadString('\n')` in a loop.
- Trim only trailing newline characters from `Raw`; do not trim message content.
- Use small helper functions:
  - `parseEventLine(raw string, line int) (Entry, bool)`
  - `parseSourceLine(token string) int`
  - `parseKeyValuePayload(payload string) map[string]string`
  - `parseLimitLine(line string, namespace string) (LimitUsage, bool)`
  - `parseStackFrame(line string) (StackFrame, bool)`
- Keep helpers unexported.

- [ ] **Step 7: Run parser tests**

Run:

```bash
go test -count=1 ./internal/apexlog -run 'TestParse'
```

Expected:

- All parser tests pass.

---

## Task 2: Convert Parsed Logs To Trace And Profile Data

**Files:**

- Create: `internal/apexlog/trace.go`
- Create: `internal/apexlog/trace_test.go`
- Modify: `internal/profile/profile_test.go` only when `profile.Analyze` mishandles a valid `trace.Document` produced by `apexlog.TraceDocument`

### Required API

Create:

```go
func TraceDocument(log Log) trace.Document
```

Conversion rules:

- `USER_DEBUG` -> `trace.Instant("apex.debug", "apex.debug", ts, args)`
- `SOQL_EXECUTE_END` -> `trace.Instant("apex.soql", "apex.soql", ts, args)` with `rows` and `line`
- `DML_BEGIN` -> `trace.Instant("apex.dml."+lowerOp, "apex.dml", ts, args)` with `operation`, `objects`, `rows`, and `line`
- `CALLOUT_REQUEST` -> `trace.Instant("apex.callout.http", "apex.callout", ts, args)`
- `LIMIT_USAGE_FOR_NS` plus parsed limit rows -> one `trace.Instant("apex.limits", "apex.limits", ts, args)`
- `EXCEPTION_THROWN` and `FATAL_ERROR` -> `trace.Instant("apex.exception", "apex.exception", ts, args)`

Use a monotonic integer timestamp if the Salesforce timestamp cannot be parsed. Do not fail profile conversion because a timestamp is unusual.

### Tests First

- [ ] **Step 1: Add trace conversion test**

Create `TestTraceDocumentConvertsMeasuredEvents`.

Required assertions:

- The document format is `trace.FormatChromeTraceEvent`.
- It has `apex.soql`, `apex.dml.insert`, and `apex.limits` events.
- The SOQL event has `rows == 1`.
- The DML event has `rows == 1` and `objects` contains `Account`.
- The limits event has `cpuTimeMs == 7` and `heapSize == 8120`.

- [ ] **Step 2: Run test and verify it fails**

Run:

```bash
go test -count=1 ./internal/apexlog -run 'TestTraceDocument'
```

Expected:

- Fail because `TraceDocument` does not exist.

- [ ] **Step 3: Implement `TraceDocument`**

Keep the conversion deterministic. Do not add wall-clock time.

- [ ] **Step 4: Verify profile integration**

Add a test that calls:

```go
report := profile.Analyze(apexlog.TraceDocument(log))
```

Assert:

- `report.Limits.SOQLQueries == 1`
- `report.Limits.SOQLRows == 1`
- `report.Limits.DML == 1`
- `report.Limits.DMLRows == 1`
- `report.Limits.CPUTimeMS == 7`
- `report.Limits.HeapSize == 8120`

Run:

```bash
go test -count=1 ./internal/apexlog ./internal/profile
```

Expected:

- Pass.

---

## Task 3: Source Index And Conservative Matching

**Files:**

- Create: `internal/debuglog/model.go`
- Create: `internal/debuglog/source_index.go`
- Create: `internal/debuglog/match.go`
- Create: `internal/debuglog/match_test.go`
- Create: `internal/debuglog/testdata/project/sfdx-project.json`
- Create: `internal/debuglog/testdata/project/force-app/main/default/classes/TestProcessor.cls`
- Create: `internal/debuglog/testdata/project/force-app/main/default/classes/TestController.cls`
- Create: `internal/debuglog/testdata/subscriber.log`

### Annotation Types

Create `internal/debuglog/model.go`:

```go
package debuglog

import "github.com/glade-sh/glade/internal/apexlog"

type AnnotatedLog struct {
	Log     apexlog.Log      `json:"log"`
	Entries []AnnotatedEntry `json:"entries"`
}

type AnnotatedEntry struct {
	Entry      apexlog.Entry    `json:"entry"`
	Best       SourceCandidate  `json:"best,omitempty"`
	Candidates []SourceCandidate `json:"candidates,omitempty"`
}

type SourceCandidate struct {
	File       string  `json:"file,omitempty"`
	Line       int     `json:"line,omitempty"`
	Symbol     string  `json:"symbol,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	Confidence float64 `json:"confidence"`
}
```

### Source Index Rules

Build the source index once. Do not loop through every Apex file for every log entry.

The source index must capture:

- Class name.
- Optional namespace from type name if present.
- Method names and source ranges.
- `System.debug(...)` calls with literal message fragments where available.
- Inline SOQL query text.
- DML operation hints and nearby object type hints.

Implement the first pass with these rules:

- Use `typesys.Index` for declared types and methods.
- Use `apexast.NewParser().ParseSourceAST(path, source)` to walk AST nodes.
- Use text extraction from AST node ranges for SOQL and invocation literals.
- Use normalized strings for matching:
  - lower-case outside quoted string content where practical
  - collapse whitespace
  - remove namespace prefix from class names for comparison, but keep namespace in output

### Confidence Rules

Use these initial scores:

- Stack frame exact class and method: `0.95`
- Code unit exact class and method: `0.85`
- USER_DEBUG source line inside current code unit: `0.75`
- USER_DEBUG exact literal message match: `0.90`
- SOQL normalized exact query: `0.85`
- SOQL same `FROM` object only: `0.45`
- DML same operation and object type: `0.65`
- DML same operation only: `0.35`

Never output a confidence above `0.50` for a match that only uses a line number without class, method, query, message, or DML evidence.

Keep at most 5 candidates per entry by default.

### Tests First

- [ ] **Step 1: Add fixture project**

Create `internal/debuglog/testdata/project/sfdx-project.json`:

```json
{
  "packageDirectories": [
    {
      "path": "force-app",
      "default": true
    }
  ],
  "sourceApiVersion": "64.0",
  "namespace": "ns"
}
```

Create `TestProcessor.cls`:

```apex
public with sharing class TestProcessor {
    public static void run() {
        System.debug(LoggingLevel.INFO, 'start work');
        Account a = new Account(Name = 'Acme');
        insert a;
        List<Account> rows = [SELECT Id, Name FROM Account WHERE Name = 'Acme'];
        System.debug('rows=' + rows.size());
    }

    public static void fail() {
        throw new AuraHandledException('Nope');
    }
}
```

Create `TestController.cls`:

```apex
public with sharing class TestController {
    public static void callProcessor() {
        TestProcessor.run();
    }
}
```

- [ ] **Step 2: Add `subscriber.log` fixture**

Use this exact content in `internal/debuglog/testdata/subscriber.log`:

```text
64.0 APEX_CODE,DEBUG;APEX_PROFILING,INFO;CALLOUT,INFO;DB,INFO;SYSTEM,DEBUG
00:00:00.000 (0)|USER_INFO|[EXTERNAL]|005000000000000AAA|isv@example.com|GMT|GMT+00:00
00:00:00.001 (1000000)|EXECUTION_STARTED
00:00:00.002 (2000000)|CODE_UNIT_STARTED|[EXTERNAL]|ns.TestProcessor.run
00:00:00.003 (3000000)|USER_DEBUG|[3]|INFO|start work
00:00:00.004 (4000000)|DML_BEGIN|[5]|Op:Insert|Type:Account|Rows:1
00:00:00.005 (5000000)|DML_END|[5]
00:00:00.006 (6000000)|SOQL_EXECUTE_BEGIN|[6]|Aggregations:0|SELECT Id, Name FROM Account WHERE Name = 'Acme'
00:00:00.007 (7000000)|SOQL_EXECUTE_END|[6]|Rows:1
00:00:00.008 (8000000)|USER_DEBUG|[7]|DEBUG|rows=1
00:00:00.009 (9000000)|CODE_UNIT_FINISHED|ns.TestProcessor.run
00:00:00.010 (10000000)|EXECUTION_FINISHED
```

- [ ] **Step 3: Add matcher tests**

Create tests:

- `TestAnnotateMatchesDebugSOQLAndDML`
- `TestAnnotateUsesStackFrameForException`
- `TestAnnotateKeepsWeakMatchesWeak`

Required assertions:

```go
if best.Confidence < 0.85 {
	t.Fatalf("confidence = %.2f, want at least 0.85", best.Confidence)
}
if !strings.HasSuffix(best.File, "TestProcessor.cls") {
	t.Fatalf("best file = %q, want TestProcessor.cls", best.File)
}
if best.Line <= 0 {
	t.Fatalf("best line = %d, want positive line", best.Line)
}
if weak.Confidence > 0.50 {
	t.Fatalf("weak confidence = %.2f, want <= 0.50", weak.Confidence)
}
```

- [ ] **Step 4: Run tests and verify they fail**

Run:

```bash
go test -count=1 ./internal/debuglog -run 'TestAnnotate'
```

Expected:

- Fail because package does not exist.

- [ ] **Step 5: Implement source indexing and matching**

Create:

```go
func Annotate(log apexlog.Log, index typesys.Index, maxCandidates int) (AnnotatedLog, error)
```

Use a helper:

```go
func BuildSourceIndex(index typesys.Index) SourceIndex
```

If `maxCandidates <= 0`, use `5`.

Sort candidates by:

1. Higher confidence.
2. Exact file path match before basename-only match.
3. Lower line number.
4. Lexicographic symbol.

- [ ] **Step 6: Run matcher tests**

Run:

```bash
go test -count=1 ./internal/debuglog
```

Expected:

- Pass.

---

## Task 4: Annotated Output

**Files:**

- Create: `internal/debuglog/output.go`
- Create: `internal/debuglog/output_test.go`

### Required API

Create:

```go
func WriteText(w io.Writer, annotated AnnotatedLog, minConfidence float64) error
func WriteJSON(w io.Writer, annotated AnnotatedLog) error
```

Text output shape:

```text
00:00:00.003 (3000000)|USER_DEBUG|[12]|INFO|start work
  => force-app/main/default/classes/TestProcessor.cls:3 TestProcessor.run confidence=0.90 reason=debug literal
```

Rules:

- Always print the raw log line.
- Print annotation only when `Best.Confidence >= minConfidence`.
- Default CLI threshold will be `0.50`.
- Do not print low-confidence guesses in text by default.
- JSON includes all candidates.

### Tests First

- [ ] **Step 1: Add output tests**

Create:

- `TestWriteTextIncludesHighConfidenceAnnotation`
- `TestWriteTextSuppressesLowConfidenceAnnotation`
- `TestWriteJSONIncludesCandidates`

- [ ] **Step 2: Run tests and verify they fail**

Run:

```bash
go test -count=1 ./internal/debuglog -run 'TestWrite'
```

Expected:

- Fail because output functions do not exist.

- [ ] **Step 3: Implement output functions**

Keep output deterministic.

- [ ] **Step 4: Run package tests**

Run:

```bash
go test -count=1 ./internal/debuglog
```

Expected:

- Pass.

---

## Task 5: `glade debug` CLI

**Files:**

- Create: `internal/gladecli/debug_command.go`
- Modify: `internal/gladecli/cli.go`
- Modify: `internal/gladecli/cli_test.go`

### Commands To Add

Add a top-level `debug` case to `Run()`:

```text
glade debug parse --log <path> [--json]
glade debug profile --log <path> [--json]
glade debug explain --log <path> [--project <dir>] [--min-confidence <float>] [--json]
```

Behavior:

- `parse`: parse a debug log and write JSON. `--json` is accepted and has no special effect.
- `profile`: parse a debug log, convert to `trace.Document`, analyze with `profile.Analyze`, and write Markdown by default or JSON with `--json`.
- `explain`: parse a debug log, load a project index with `loadIndex(projectRoot)`, annotate entries, and write text by default or JSON with `--json`.

Flag rules:

- `--log` is required for all subcommands.
- `--project` defaults to `.` for `explain`.
- `--min-confidence` defaults to `0.50`.
- Unknown flags return an error.
- Missing required values return an error.

### Tests First

- [ ] **Step 1: Add CLI tests**

Add tests to `internal/gladecli/cli_test.go`:

- `TestRunDebugParseJSON`
- `TestRunDebugProfileMarkdown`
- `TestRunDebugProfileJSON`
- `TestRunDebugExplainText`
- `TestRunDebugExplainJSON`
- `TestRunDebugRequiresLog`

Use `internal/apexlog/testdata/core.log` and `internal/debuglog/testdata/project`.

Required assertions:

```go
if !strings.Contains(stdout.String(), `"entries"`) {
	t.Fatalf("parse stdout missing entries: %s", stdout.String())
}
if !strings.Contains(stdout.String(), "SOQL: 1 queries / 1 rows") {
	t.Fatalf("profile stdout missing SOQL summary: %s", stdout.String())
}
if !strings.Contains(stdout.String(), "TestProcessor.cls") {
	t.Fatalf("explain stdout missing source file: %s", stdout.String())
}
if code == 0 {
	t.Fatalf("missing --log should fail")
}
```

- [ ] **Step 2: Run CLI tests and verify they fail**

Run:

```bash
go test -count=1 ./internal/gladecli -run 'TestRunDebug'
```

Expected:

- Fail because command is not wired.

- [ ] **Step 3: Implement `internal/gladecli/debug_command.go`**

Create:

```go
func runDebug(ctx context.Context, args []string, w io.Writer) error
```

Use helpers:

```go
func parseDebugLogFile(path string) (apexlog.Log, error)
func parseDebugFloat(raw string, flag string) (float64, error)
```

Do not duplicate JSON encoder logic in each subcommand. Use one helper:

```go
func writeIndentedJSON(w io.Writer, value any) error
```

If such helper already exists, reuse it instead of adding another.

- [ ] **Step 4: Wire `cli.go`**

Add:

```go
case "debug":
	if err := runDebug(ctx, args[1:], stdout); err != nil {
		fmt.Fprintf(stderr, "glade: %v\n", err)
		return 1
	}
	return 0
```

Add a short help line:

```text
  debug       parse, profile, and explain Salesforce debug logs
```

- [ ] **Step 5: Run CLI tests**

Run:

```bash
go test -count=1 ./internal/gladecli -run 'TestRunDebug'
```

Expected:

- Pass.

---

## Task 6: Docs And Smoke Checks

**Files:**

- Modify: `docs/EDITOR.md`
- Modify: `docs/LOCAL_TESTING.md`

### Docs Scope

Add a short section. Do not write a marketing page.

Include these examples:

```bash
glade debug parse --log subscriber.log --json
glade debug profile --log subscriber.log
glade debug explain --log subscriber.log --project force-app
glade debug explain --log subscriber.log --project force-app --json
```

Explain:

- This is offline.
- Logs can come from subscriber-org support collection.
- Matching is confidence-ranked.
- Profile output is measured from the log, not static lint.
- DAP breakpoint export is future work.

### Smoke Commands

- [ ] **Step 1: Run parser smoke**

Run:

```bash
go run ./cmd/glade debug parse --log internal/apexlog/testdata/core.log --json
```

Expected:

- JSON contains `"entries"` and `"SOQL_EXECUTE_BEGIN"`.

- [ ] **Step 2: Run profile smoke**

Run:

```bash
go run ./cmd/glade debug profile --log internal/apexlog/testdata/core.log
```

Expected:

- Markdown contains `SOQL: 1 queries / 1 rows`.
- Markdown contains `DML: 1 statements / 1 rows`.
- Markdown contains `CPU: 7 ms`.

- [ ] **Step 3: Run explain smoke**

Run:

```bash
go run ./cmd/glade debug explain --log internal/debuglog/testdata/subscriber.log --project internal/debuglog/testdata/project
```

Expected:

- Text contains `TestProcessor.cls`.
- Text contains at least one `confidence=`.

---

## Task 7: Optional Breakpoint Export Only After Required Gates Pass

Do not start this task until Tasks 1 through 6 are complete and all required tests pass.

**Files:**

- Create: `internal/debuglog/breakpoints.go`
- Create: `internal/debuglog/breakpoints_test.go`
- Modify: `internal/gladecli/debug_command.go`
- Modify: `internal/gladecli/cli_test.go`

### Scope

Add only JSON export for breakpoints. Do not build VSCode. Do not add `glade dap`.

Command:

```text
glade debug breakpoints --log <path> --project <dir> [--min-confidence <float>]
```

Output shape:

```json
{
  "breakpoints": [
    {
      "source": {"path": "force-app/main/default/classes/TestProcessor.cls"},
      "line": 3,
      "confidence": 0.9,
      "reason": "debug literal"
    }
  ]
}
```

Rules:

- Use annotation output from `debuglog.Annotate`.
- Include only entries above `min-confidence`.
- De-duplicate by file and line.
- Sort by file then line.

Required tests:

- `TestBreakpointsFromAnnotatedLogDedupesAndSorts`
- `TestRunDebugBreakpointsJSON`

Verification:

```bash
go test -count=1 ./internal/debuglog ./internal/gladecli -run 'TestBreakpoints|TestRunDebugBreakpoints'
```

---

## Final Verification

Run all of these before calling the work done:

```bash
go test -count=1 ./internal/apexlog ./internal/debuglog ./internal/profile ./internal/gladecli ./internal/dap
go test -count=1 ./internal/gladecli -run 'TestRunDebug'
go run ./cmd/glade debug parse --log internal/apexlog/testdata/core.log --json
go run ./cmd/glade debug profile --log internal/apexlog/testdata/core.log
go run ./cmd/glade debug explain --log internal/debuglog/testdata/subscriber.log --project internal/debuglog/testdata/project
git diff --check
```

Expected:

- All tests pass.
- Smoke commands exit `0`.
- `debug profile` output ranks measured SOQL, DML, limits, CPU, and heap from the log.
- `debug explain` output shows confidence-ranked source annotations.
- `git diff --check` prints no whitespace errors.

---

## Done Definition

The sprint is done only when all are true:

- `glade debug parse` works on a Salesforce-style log.
- `glade debug profile` produces measured profile output from that log.
- `glade debug explain` annotates source with conservative confidence.
- Parser handles pipes in SOQL, exception stack frames, and limit blocks.
- Matching uses a prebuilt source index, not repeated whole-project scans.
- Docs mention the feature and its limits.
- Required final verification commands pass.

If anything remains blocked, write the exact blocker with:

- file
- command
- observed output
- what was tried
- next concrete step

Do not say "mostly done." Either the gates pass or there is a named blocker.

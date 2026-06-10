# Plan: Debug Log → Closed-Loop TDD (Follow-On)

> **Dependency:** `docs/superpowers/plans/debug-log-pipeline.md` shipped.
> `glade debug parse|profile|explain` are operational. This plan builds on
> that foundation.

**Goal:** Close the ISV debugging loop: subscriber log → local reproducer
test → interactive DAP debugging → fix → verify. Enable AI agents to
autonomously reproduce and fix managed-package bugs without a Salesforce org.

**Two deliverables:**
1. `glade debug repro` — synthesize a failing glade test from a subscriber log
2. `glade dap` — start a DAP debug server for interactive breakpoint debugging

---

## Deliverable A: `glade debug repro` (Test Synthesis)

### A.1 — Overview

`glade debug repro` reads a subscriber debug log and generates a
glade-compatible Apex test class that exercises the same code path recorded in
the log. Data setup is inferred from SOQL WHERE clauses. Assertions are
inferred from DML results and exception entries. The output compiles and, when
run with `glade test`, reproduces the subscriber's failure mode.

### A.2 — Status Quo

The following API surface is already available:

| Function | Package | What it provides |
|---|---|---|
| `apexlog.Parse(r)` | `apexlog` | Parsed `Log` with `Entry{Kind, Data{SOQLQuery, DMLOperation, ...}}` |
| `debuglog.Annotate(log, index, max)` | `debuglog` | `AnnotatedLog` with `SourceCandidate{File, Line, Symbol, Confidence}` per entry |
| `loadIndex(root)` | `gladecli` | `typesys.Index` from a project directory |
| `apexlog.TraceDocument(log)` | `apexlog` | `trace.Document` for profile analysis |
| `profile.Analyze(doc)` | `profile` | Structured profile report with limits, SOQL count, DML count |

The source index (`debuglog.SourceIndex`) already maps methods to file:line
ranges, captures `System.debug()` literal arguments, SOQL query text and
normalized forms, and DML operation/object-type pairs.

### A.3 — Design

**New file: `internal/debuglog/repro.go`**

```go
func SynthesizeTest(annotated AnnotatedLog, minConfidence float64) (string, error)
```

**Algorithm:**

1. **Extract entry-point method.**
   Walk `annotated.Entries` for `EntryCodeUnitStarted` entries with the managed
   namespace. The outermost code unit is the top-level call. Extract the class
   and method name. If the log has no code-unit markers, use the top stack
   frame from `EntryExceptionThrown` as the entry point.

2. **Extract test class name.**
   Derive from the entry-point class name + method, e.g. `ns.TestProcessor.run`
   → `TestProcessorRunReproTest`.

3. **Synthesize data setup (`@TestSetup` or inline).**
   For each `EntrySOQLExecuteBegin` entry in the log:
   - Parse the SOQL query to extract FROM object and WHERE conditions.
   - For equality filters like `WHERE Industry = 'Tech'`, generate an SObject
     constructor setting that field to the literal value.
   - For the row count from `EntrySOQLExecuteEnd`, create exactly that many
     records.
   - Wrap in a `List<ObjectName>` for `insert`.

   Tricky cases:
   - Compound WHERE (AND/OR): generate records satisfying all equality
     predicates. Skip range predicates initially (future enhancement).
   - Parent-child SOQL: generate parent records first, then children with
     relationship fields.
   - No WHERE clause: generate 1-3 records with default/mock values for
     required fields.

4. **Synthesize the test method.**
   - Call `System.runAs()` with a test user if the log shows `USER_INFO` with
     a non-default user.
   - Import the entry-point class (if different from test class).
   - Construct any required arguments from setup data.
   - Call the entry-point method, wrapped in `Test.startTest()` /
     `Test.stopTest()`.

5. **Synthesize assertions.**
   - **Success case:** If the log shows no `EntryExceptionThrown`, assert
     `System.assert(true)` as a baseline and emit the method call result.
   - **Exception case:** If the log shows `EntryExceptionThrown`, wrap the
     call in `try { } catch (ExceptionType e) { System.assert(e.getMessage()
     .contains('...')); }`. After the catch, add a comment: `// TODO: Remove
     Try/Catch When Fixed`.
   - **DML case:** If the log shows DML operations, assert that the number of
     records matches.

6. **Annotate the generated test.**
   Each generated line includes a comment referencing the subscriber log entry
   that motivated it, e.g.:
   ```apex
   // subscriber.log:6 SOQL_EXECUTE_BEGIN → ns.TestProcessor.run
   ```

7. **Write to stdout.**
   The CLI writes the test source directly to stdout so the developer can pipe
   it to a `.cls` file or into the AI's context.

### A.4 — Output Format

```apex
/**
 * Test synthesized from subscriber debug log.
 * Log: subscriber.log
 * API Version: 64.0
 * Synthesized at: 2026-06-09T09:30:00-07:00
 */
@IsTest
private class TestProcessorRunReproTest {
    @TestSetup
    static void setup() {
        // subscriber.log:6 SOQL_EXECUTE_BEGIN WHERE Name = 'Acme'
        Account a1 = new Account(Name = 'Acme');
        Account a2 = new Account(Name = 'TestCo');
        insert new List<Account>{ a1, a2 };
    }

    @IsTest
    static void reproEntryPoint() {
        Test.startTest();
        try {
            ns.TestProcessor.run();
            // subscriber.log shows no exception — this test passes as-is.
            System.assert(true);
        } catch (Exception e) {
            // subscriber.log:6 EXCEPTION_THROWN NullPointerException
            // TODO: Remove try/catch when fixed.
            System.assert(e.getMessage().contains('de-reference'));
        }
        Test.stopTest();

        // subscriber.log:7 DML_BEGIN insert Account rows=1
        List<Account> accounts = [SELECT Id FROM Account];
        System.assertEquals(2, accounts.size(), 'expected 2 accounts');
    }
}
```

### A.5 — Files

| File | Purpose |
|---|---|
| `internal/debuglog/repro.go` | `SynthesizeTest()` implementation |
| `internal/debuglog/repro_test.go` | Tests against fixture logs |
| `internal/gladecli/debug_command.go` | Add `runDebugRepro` + wire `repro` subcommand |
| `internal/gladecli/cli.go` | Update help text |
| `internal/gladecli/cli_test.go` | Add `TestRunDebugRepro` |
| `docs/EDITOR.md` | Add repro documentation |
| `docs/LOCAL_TESTING.md` | Add repro documentation |

### A.6 — Milestones

| # | Milestone | Estimated effort |
|---|---|---|
| A.1 | `debuglog/repro.go` — SOQL data setup synthesis | 3h |
| A.2 | `debuglog/repro.go` — entry-point extraction + test method synthesis | 2h |
| A.3 | `debuglog/repro.go` — exception/dml assertion synthesis | 2h |
| A.4 | `debuglog/repro_test.go` — tests against fixture project | 2h |
| A.5 | CLI wiring + help text + docs | 1h |
| A.6 | Smoke verification + edge-case handling | 1h |
| **Total A** | | **11h** |

### A.7 — Risks

| Risk | Severity | Mitigation |
|---|---|---|
| SOQL WHERE parsing is incomplete (nested conditions, subqueries) | Medium | Generate records for equality predicates only initially. Document limitation. Add compound-where support in a follow-up. |
| Entry-point inference fails for logs without CODE_UNIT markers | Low | Fall back to exception stack-frame top. If neither exists, emit a synthetic entry point with `// TODO: fill in entry-point class and method`. |
| Generated test references classes outside the test's compilation scope | Low | Verify `internal` vs `public` class visibility. Emit a comment warning if the class isn't `public`. |
| Variable names in setup collide with test method locals | Low | Prefix setup variables with `setup_`. |

### A.8 — Validation

```bash
# Synthesize test from subscriber log
go run ./cmd/glade debug repro \
  --log internal/debuglog/testdata/subscriber.log \
  --project internal/debuglog/testdata/project \
  > /tmp/ReproTest.cls

# Run the synthesized test (expected: fail due to try/catch)
go run ./cmd/glade test --project internal/debuglog/testdata/project \
  --filter ReproTest

# Synthesize from a success log (expected: pass)
go run ./cmd/glade debug repro \
  --log internal/apexlog/testdata/core.log \
  --project internal/debuglog/testdata/project \
  > /tmp/SuccessReproTest.cls

# Run the success test
go run ./cmd/glade test --project internal/debuglog/testdata/project \
  --filter SuccessReproTest
```

---

## Deliverable B: `glade dap` (Interactive Debugger)

### B.1 — Overview

`glade dap` starts a Debug Adapter Protocol server over stdin/stdout. A
VSCode extension (or any DAP-compatible client) connects to it, sets
breakpoints, inspects variables, and steps through code — all running
locally on glade's VM, no Salesforce org required.

### B.2 — Status Quo

The entire DAP protocol engine is already implemented and tested:

| Component | File | Status |
|---|---|---|
| Protocol framing (Content-Length headers, JSON-RPC) | `dap/protocol.go` | Done. `Serve()`, `ReadRequest()`, `Write()` |
| Command dispatch (initialize, setBreakpoints, continue, next, stepIn, stepOut, pause, threads, stackTrace, scopes, variables, evaluate) | `dap/handler.go` | Done. `NewHandler(snapshot).Handle(request)` |
| Breakpoint resolution with filepath matching | `dap/handler.go`, `vm/debug.go` | Done. `debugFileMatches()` |
| Live session (async continue/step/pause) | `dap/live.go` | Done. `StartLiveSession()`, `Continue()`, `StepIn()` |
| Variable inspection with reference chain | `dap/model.go` | Done. `variablesFromMap()`, `variableChildren()` |
| Post-execution snapshot debugging | `glade exec --debug` | Done. `serveDAPSnapshot()` in `cli.go` |

What's missing: a standalone `glade dap` command that starts a server and
waits for `launch`/`attach` with configurable project root and source
directory.

### B.3 — Design

**`glade dap` command:**

```
glade dap [--project <dir>]
```

1. Create a `dap.Handler` with an empty `Snapshot`.
2. Call `dap.Serve(os.Stdin, os.Stdout, handler)`.
3. The handler waits for `initialize` → responds with capabilities.
4. On `launch` request (with `program` and `project` arguments):
   - Load the type index via `loadIndex(projectRoot)`.
   - Compile the target class or anonymous Apex via `vm.Compile(...)`.
   - Call `handler.StartLiveSession(machine, program)`.
   - Begin accepting step/continue/pause commands.
5. On `setBreakpoints`, map DAP breakpoints to `vm.DebugBreakpoint` entries
   and update the handler.
6. On `disconnect`, terminate.

### B.4 — VSCode Extension

A minimal companion VSCode extension at `contrib/vscode-glade/`:

```
contrib/vscode-glade/
  package.json
  src/extension.ts
  src/adapter.ts
  tsconfig.json
```

`package.json`:
```json
{
  "contributes": {
    "debuggers": [{
      "type": "glade",
      "label": "Glade Apex Debugger",
      "languages": ["apex"],
      "configurationAttributes": {
        "launch": {
          "properties": {
            "program": {
              "type": "string",
              "description": "Test class or method to debug (e.g. MyClass.myMethod)"
            },
            "project": {
              "type": "string",
              "description": "SFDX project root (default: workspace root)"
            }
          }
        }
      }
    }]
  }
}
```

`src/extension.ts`:
- Registers `DebugAdapterDescriptorFactory` for type `"glade"`.
- Spawns `glade dap --project <workspace-root>` as child process.
- Pipes stdin/stdout.

`src/adapter.ts`:
- Sends `initialize` → receives capabilities.
- On `launch` request, sends `launch` to glade with `program` and `project`.

### B.5 — Files

| File | Purpose |
|---|---|
| `internal/gladecli/dap_command.go` | `runDAP()` function |
| `internal/gladecli/cli.go` | Add `dap` case to `Run()` switch |
| `internal/gladecli/cli_test.go` | Add `TestRunDAP` (protocol smoke) |
| `contrib/vscode-glade/package.json` | Extension manifest |
| `contrib/vscode-glade/src/extension.ts` | Activation, factory registration |
| `contrib/vscode-glade/src/adapter.ts` | DAP client-side adapter |
| `contrib/vscode-glade/tsconfig.json` | TypeScript config |
| `docs/EDITOR.md` | Add dap documentation |

### B.6 — Milestones

| # | Milestone | Estimated effort |
|---|---|---|
| B.1 | `internal/gladecli/dap_command.go` — `runDAP()` with stdio loop | 0.5h |
| B.2 | `cli.go` — wire `dap` case | 0.25h |
| B.3 | `cli_test.go` — protocol smoke test (initialize → initialized) | 1h |
| B.4 | `contrib/vscode-glade/` — extension skeleton + manifest | 2h |
| B.5 | `contrib/vscode-glade/src/adapter.ts` — launch + breakpoint flow | 2h |
| B.6 | Docs + manual smoke test | 1h |
| **Total B** | | **6.75h** |

### B.7 — Risks

| Risk | Severity | Mitigation |
|---|---|---|
| VSCode extension requires TypeScript/Node toolchain unfamiliar to the Go-centric repo | Low | Ship `contrib/` as opt-in. No build integration with `go build`. Include `npm run compile` in extension package.json. |
| `launch` request needs live-compilation that the DAP handler doesn't expose | Low | `dap/live.go` already has `StartLiveSession(machine, program)`. Wire it to the `launch` request handler in `dap_command.go`. |
| Breakpoint file-path matching across VSCode workspace paths and glade's type index paths | Medium | Normalize both paths via `filepath.Clean` before comparison. Expose `--source-map` flag for path prefix remapping. |

### B.8 — Validation

```bash
# Start DAP server
go run ./cmd/glade dap

# In another terminal, smoke-test protocol handshake
echo '{"seq":1,"type":"request","command":"initialize","arguments":{"clientID":"test"}}' | \
  go run ./cmd/glade dap 2>&1 | head -5
# Expected: Content-Length header + initialized event response

# Full flow with launch + breakpoint + continue
go test -count=1 ./internal/gladecli -run 'TestRunDAP'
go test -count=1 ./internal/dap
```

---

## Combined Milestone Sequence

```
Week 1
  Day 1-2: A.1-A.3 (repro core synthesis)  ← highest complexity
  Day 2-3: A.4 (repro tests + hardening)
  Day 3:   A.5-A.6 (CLI wiring + docs + smoke)

Week 2
  Day 1:   B.1-B.2 (dap CLI wiring)        ← trivial
  Day 1-2: B.4-B.5 (VSCode extension)
  Day 2:   B.3 (dap smoke test)
  Day 3:   B.6 (docs + manual smoke)
```

## Resource Requirements

- Single developer, familiar with Go and the glade codebase
- Access to VSCode for extension testing
- No external services or Salesforce orgs needed

## Integration Test

The end-to-end flow after both deliverables:

```bash
# 1. Parse subscriber log
glade debug parse --log subscriber.log --json > parsed.json

# 2. Profile measured activity from log
glade debug profile --log subscriber.log

# 3. Annotate with source references
glade debug explain --log subscriber.log --project . --json > annotated.json

# 4. Synthesize reproducer test
glade debug repro --log subscriber.log --project . > ReproTest.cls

# 5. Run test (expected to fail — reproduces the bug)
glade test --filter ReproTest
# → FAIL: System.NullPointerException

# 6. Debug interactively
glade dap --project .
# → VSCode attaches, sets breakpoints from annotated.json, steps through

# 7. Fix the code

# 8. Update test to assert fixed behavior (remove try/catch)

# 9. Run test (expected to pass)
glade test --filter ReproTest
# → PASS

# 10. AI agent loop: subscriber log → repro → fix → verify → confidence
```

## Done Definition

- `glade debug repro` generates a compilable, runnable test from a subscriber log
- The generated test reproduces the failure mode observed in the log (passes
  when the bug is still unfixed, fails otherwise via explicit assertions)
- `glade dap` accepts `initialize`, `launch`, `setBreakpoints`, `continue`,
  `next`, `stepIn`, `stepOut`, `threads`, `stackTrace`, `scopes`, `variables`
- `go test ./internal/dap ./internal/debuglog ./internal/gladecli` passes
- `go test ./internal/gladecli -run 'TestRunDebugRepro'` passes
- `go test ./internal/gladecli -run 'TestRunDAP'` passes
- Smoke commands from sections A.8 and B.8 succeed

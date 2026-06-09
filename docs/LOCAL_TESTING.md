# Local Apex Testing

Glade runs Apex tests from local Salesforce source without a Salesforce org.
There is no org login, scratch org, source push, or metadata deploy in the
normal loop. Glade reads the SFDX project on disk, loads supported metadata, and
runs tests in the local VM.

## Project Setup

Start from a Salesforce-shaped project with `sfdx-project.json` at the root:

```text
my-project/
  sfdx-project.json
  force-app/main/default/classes/
  force-app/main/default/triggers/
  force-app/main/default/objects/
```

Then run from the project root:

```bash
glade doctor
glade check --project .
glade test --project .
```

`glade check` parses and type-checks the project. `glade test` discovers
`@IsTest` classes and runs them locally. Add `--json` for machine-readable
output and `--junit reports/glade-junit.xml` for CI reports.

`glade doctor` must report `parser: ok (tree-sitter)`. If the parser is
unavailable, install a C compiler and rebuild or reinstall a parser-capable
release artifact.

## Run Tests Without An Org

Run every local Apex test:

```bash
glade test --project . --json
```

Run one test class or one method:

```bash
glade test --project . --filter AccountServiceTest --json
glade test --project . --filter AccountServiceTest.testCreatesAccount --json
```

## Run a Performance Risk Scan

Generate an advisory performance report from source and metadata first:

```bash
glade inspect performance --project . --json > reports/glade-performance.json
```

Add trace data from a representative local run to rank the highest-cost paths first:

```bash
glade inspect performance --project . --trace reports/slow-test-trace.json > reports/glade-performance.md
```

Use `--parallelism` to choose the worker count. Method-level parallel execution
is on by default; use `--no-parallel-methods` only when a project has tests that
share mutable local state.

```bash
glade test --project . --parallelism 8 --json
```

## Run Tests Affected By Local Changes

Instead of running the whole suite on every edit, `glade` can run only the tests
affected by your changes. This is the fast inner loop: change a class, run the
handful of tests that exercise it, get a verdict in a fraction of the time.

### How selection works (no extra steps)

Selection is **fully static and automatic**. There is no profiling pass, no
instrumentation run, and nothing to record or keep in sync. `glade` reads the
type index it already builds for compilation and derives a **reference graph**:
an edge means "type A uses type B" (B is named in A's source, or is A's
superclass or an interface A implements).

When a file changes, `glade` walks that graph **backwards** from the changed
type to find every test that reaches it — directly or transitively through
helper classes, base classes, and interfaces. So a change to a deep utility
class that no test names by hand still selects the tests that depend on it
through the call chain.

This mirrors how the platform treats your org: compiled classes and their
relationships are static metadata, so the dependency graph is a free byproduct
of indexing. Only the data records change between tests, never the graph.

Every watch event reports a `selection` with one of three modes:

| Mode     | Meaning                                                                 |
| -------- | ----------------------------------------------------------------------- |
| `direct` | A precise set of test classes reaches the changed types. Only they run. |
| `all`    | Conservative fallback — the full suite runs (see below).                |
| `none`   | The change cannot affect any test (e.g. a change with no tests present).|

To never skip a test that could catch a regression, selection falls back to
`all` whenever it cannot prove a precise set:

- a changed production class that no test reaches (it may be called dynamically);
- a changed class not found in the type index;
- a changed **trigger** (it can fire for any DML in any test);
- changed **object or field metadata** (schema affects the whole org).

### Run once against a git ref

Use `--changed-since <git-ref>` to select tests from everything changed since a
ref. The ref is passed to git, so common choices are `origin/main`, `main`, or
the merge-base branch CI uses.

```bash
git fetch origin main
glade test --project . --changed-since origin/main --json
```

Only the selected classes execute. If the change set hits a trigger or schema,
the full suite runs automatically.

### Watch mode for editor-style feedback

Watch mode re-runs on save and streams newline-delimited JSON (NDJSON) events.
The first run covers the full set; each later run selects only affected tests.

```bash
glade test --project . --watch          # run continuously
glade test --project . --watch-once     # run one cycle, then exit (good for CI hooks)
```

A typical event stream after editing one helper class (`InvoiceCalculator.cls`)
that two test classes reach:

```json
{"event":"watch.started","time":"...","config":{...}}
{"event":"watch.changes","time":"...","changes":[{"path":".../InvoiceCalculator.cls","op":"modified","kind":"apex_class","name":"InvoiceCalculator"}]}
{"event":"watch.tests_selected","time":"...","selection":{"mode":"direct","testClasses":["InvoiceServiceTest","InvoiceSummaryTest"],"reason":"changed types reach affected tests"}}
{"event":"watch.run_started","time":"...","runId":2,"testClasses":["InvoiceServiceTest","InvoiceSummaryTest"]}
{"event":"watch.run_finished","time":"...","runId":2,"summary":{"total":9,"passed":9,"failed":0,"passedAll":true}}
```

Editing a trigger instead falls back to the full suite:

```json
{"event":"watch.tests_selected","time":"...","selection":{"mode":"all","reason":"changed trigger may affect any test"}}
```

### Keep the project warm with the daemon

For repeated loops, `--daemon` holds a warm, incrementally-updated index and
reference graph in a background service, so selection on each change is near
instant — only the edited file is re-scanned, never the whole project.

```bash
glade test --project . --daemon --changed-since origin/main --json
glade test --project . --daemon --watch
```

## Compatibility Local-Test Runs

`glade-tools local-tests` uses the same local runtime but reports readiness
outcomes for large-project parity work. Per-test outcome strings include
`pass`, `fail`, `unsupported`, `load_error`, `compile_error`, `internal_error`,
`assert_fail`, `runtime_gap`, `compile_gap`, and `timeout`. Summary keys use
camelCase names such as `loadError`, `compileError`, `internalError`,
`assertFail`, and `runtimeGap`.

Use it when you want triage output, blocker grouping, sharding, or the
compatibility JSON shape:

```bash
glade-tools local-tests --project . --parallel auto --json
```

Focused runs use explicit class and method flags:

```bash
glade-tools local-tests --project . --class AccountServiceTest --json
glade-tools local-tests --project . --class AccountServiceTest --method testCreatesAccount --json
```

Affected-test selection is available here too:

```bash
glade-tools local-tests --project . --changed-since origin/main --parallel auto --json
```

For this command, `--parallel <n|auto>` controls class workers. The
day-to-day `glade test` command uses `--parallelism <n>` and has
`--parallel-methods` enabled by default.

## Anonymous Apex

Run a short execute-anonymous snippet against local source:

```bash
glade exec --project . "System.debug('hello from glade');"
```

Capture a trace when you need to inspect runtime behavior:

```bash
glade exec --project . --trace reports/trace.json "System.debug(1);"
glade profile analyze reports/trace.json
```

## Playground

Use the browser playground for quick experiments with local files, anonymous
Apex, logs, limits, traces, and local org diffs:

```bash
glade playground --project . --db .glade/playground/org.sqlite --open
```

The playground runs on localhost and uses the same compiler, VM, and storage
layers as `glade test` and `glade exec`.

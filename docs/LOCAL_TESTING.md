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

`glade doctor` must report `Ready.`. If the parser is unavailable, install a C
compiler and rebuild or reinstall a parser-capable release artifact.

## Run Tests Without An Org

Run every local Apex test:

```bash
glade test --project . --json
```

Run one test class or one method:

```bash
glade test --project . --class AccountServiceTest --json
glade test --project . --class AccountServiceTest --method testCreatesAccount --json
```

## Run a Performance Risk Scan

Generate an advisory performance report from source and metadata first:

```bash
# Requires a live plugin registry, custom registry, direct archive, or linked plugin.
glade plugins install @glade/performance
glade performance scan --project . --json > reports/glade-performance.json
```

Add trace data from a representative local run to rank the highest-cost paths first:

```bash
glade performance scan --project . --trace reports/slow-test-trace.json > reports/glade-performance.md
```

The static scan records entry points and high-confidence code shape. It does
not treat a Visualforce page, Lightning wire, batch class, trigger, or SOQL
query without a `WHERE` clause as a bottleneck by itself. Use trace input when
you need measured elapsed spans and SOQL row counts.

Plugin install, archive, and author details are in [PLUGINS.md](PLUGINS.md).
The short alias `performance` resolves to `@glade/performance`.

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

Use `glade test changed --since <git-ref>` to select tests from everything
changed since a ref. The ref is passed to git, so common choices are
`origin/main`, `main`, or the merge-base branch CI uses.

```bash
git fetch origin main
glade test changed --project . --since origin/main --json --no-progress
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

### Startup cache and warm runs

Large projects pay a startup cost to build local org state and compile project
helpers before the first test runs. `glade test` can persist that harness in
`.glade/test/startup.gob` and reload it on later runs when fingerprint checks
pass.

**Read [TEST_STARTUP_CACHE.md](TEST_STARTUP_CACHE.md) for the full rules** —
what is cached, when it is written, how freshness works, and when a bad cache
can make results untrustworthy.

Quick reference:

| Goal | Command |
| ---- | ------- |
| Run tests affected by changed files | `glade test changed --project . --since HEAD` |
| Rerun the last failed tests | `glade test failed --project .` |
| Pick the next loop command | `glade test --project . --wizard` |
| Delete on-disk cache | `glade test clear-cache --project .` |
| One run without disk cache | `glade test --project . --no-cache` |
| Fastest repeated CLI loops | `glade test serve --project .` (see below) |

Clear the cache after `git pull`, branch switches, Glade upgrades, or whenever
tests behave as if old project state is still loaded. Use `--no-cache` while
debugging harness or org-inference issues.

For the fastest repeated loops, start a persistent test server in one terminal:

```bash
glade test serve --project .
```

Then run focused tests from another terminal. `glade test` auto-connects when
`.glade/test/serve.sock` is reachable:

```bash
glade test daemon status --project .
glade test --project . --class AccountServiceTest
glade test daemon stop --project .
```

Use `--connect` to require the server, or `--no-serve` to force a local cold or
disk-cache warm build.

### Keep the index warm inside one watch loop

For a single long-lived watch session, `--daemon` holds a warm, incrementally
updated index and reference graph in-process. Selection on each change is near
instant — only the edited file is re-scanned, never the whole project.

```bash
glade test --project . --daemon --watch
```

Use `glade test serve` when separate CLI invocations should stay warm. Use
`--daemon` when one `glade test --watch` process should avoid reloading the
project on every save. For a one-shot changed-file run, use
`glade test changed --project . --since origin/main`.

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

## Offline Debug Log Analysis

`glade debug` runs on saved Salesforce logs without org access:

- `parse` prints structured JSON log entries.
- `profile` builds measured runtime output.
- `explain` adds conservative source candidate matches.
- `repro` writes a best-effort Apex test class from the subscriber log.

```bash
glade debug parse --log internal/debuglog/testdata/subscriber.log --json
glade debug profile --log internal/debuglog/testdata/subscriber.log
glade debug explain --log internal/debuglog/testdata/subscriber.log --project internal/debuglog/testdata/project
glade debug explain --log internal/debuglog/testdata/subscriber.log --project internal/debuglog/testdata/project --json
glade debug repro --log internal/debuglog/testdata/subscriber.log --project internal/debuglog/testdata/project > ReproTest.cls
```

`repro` infers setup records from SOQL equality filters, entry-point calls from
code-unit or stack-frame entries, and baseline assertions from DML and exception
events. Treat the generated class as the first local reproducer, then tighten
the assertions after the bug is understood.

## Playground

Use the browser playground for quick experiments with local files, anonymous
Apex, logs, limits, traces, and local org diffs:

```bash
glade playground --project . --db .glade/playground/org.sqlite --open
```

The playground runs on localhost and uses the same compiler, VM, and storage
layers as `glade test` and `glade exec`.

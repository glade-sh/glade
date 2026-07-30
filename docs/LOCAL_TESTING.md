# Local Apex Testing

Glade runs Apex tests from local Salesforce source without a Salesforce org.
There is no org login, scratch org, source push, or metadata deploy in the
normal loop. Glade reads the SFDX project on disk, loads supported metadata, and
runs tests in the local VM.

## Project Setup

Start from a Salesforce-shaped project with `sfdx-project.json` at the root:

```text
macrodata-apex/
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
output and `--junit reports/glade-junit.xml` for CI reports. Create the
`reports/` directory first on a fresh runner.

`glade doctor` must report `Ready.`. If the parser is unavailable, install a C
compiler and rebuild or reinstall a parser-capable release artifact.

## Run LWCs Locally

Install the local LWC toolchain first:

```bash
glade toolchain install
```

Start the LWC dev shell from a Salesforce-shaped project:

```bash
glade dev lwc --project . --open
```

The command opens the Workbench Console at the printed base URL; `/lwc` is the
stable link. Component Lab filters exposed LWCs and edits preview properties,
page context, and form factor. Page Workbench groups component, record, app,
home, tab, action, utility, Flow, and configured community routes discovered
from LWC bundle metadata, FlexiPages, custom tabs, quick actions, and context
presets. `/lwc/builder` remains the draft page composer.
Use a named context when a page needs record, app, tab, form-factor, or state
values:

```bash
glade dev lwc --project . --context accountRecord --open
```

Use direct flags for one route:

```bash
glade dev lwc --project . --target record-page --object Account --record 001000000000001AAA --page Account_Record_Page --open
```

Use `--target url-addressable`, `--target record-action`, `--target
global-action`, `--target utility-bar`, `--target flow-screen`, or
`--target flow-action` with the matching `--component`, `--action`, `--object`,
`--record`, `--page`, `--flow`, and `--flow-input` flags when you need those
shell contexts without a named preset. Community routes use named contexts in
`glade.lwc.json` so the site, base path, IDs, guest mode, and PageReference
travel together.

Put reusable contexts in `glade.lwc.json` at the project root, or pass an
explicit context file:

```bash
glade dev lwc --project . --context-file config/lwc-contexts.json --context accountRecord --open
```

Use `--port 8080` for the common localhost shortcut. Use `--addr` when scripts
need a full bind address. The selected context is written to the ready file as
`selectedContext` and `selectedUrl` when you use `--ready-file`.

The startup output still prints raw routes for scripts and browser bookmarks:

```text
LWC dev shell: http://127.0.0.1:8080
Selected context accountRecord: http://127.0.0.1:8080/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page
Routes: 12 available
  /lwc
  /lwc/preview/component/c/contextProbe
  /lwc/preview/cmp/c/actionProbe?c__name=value
  /lwc/preview/record/Account/<recordId>?page=Account_Record_Page
  /lwc/preview/app/Sales_Dashboard
  /lwc/preview/home/Custom_Home
  /lwc/preview/tab/Lwc_Probe
  /lwc/preview/utility/Support_Utility
  ... 4 routes omitted. Open /lwc for home or /lwc/builder for the page builder; use --ready-file or /lightning/local/context.json for the complete list.
Watching ... for lwc, flexipage, tab, Visualforce, Apex, and static resource changes.
```

Route shapes:

```text
/lwc
/lwc/preview/component/<namespace>/<component>
/lwc/preview/cmp/<namespace>/<component>?c__name=value
/lwc/preview/record/<Object>/<recordId>?page=<FlexiPage>
/lwc/preview/app/<Page>
/lwc/preview/home/<Page>
/lwc/preview/tab/<Tab>
/lwc/preview/utility/<UtilityBar>
/lwc/preview/flow/<FlowApiName>
/lwc/preview/action/<Object>/<recordId>/<ActionName>
/lwc/preview/action/global/<ActionName>
/lwc/preview/community/<site>/<page>
/lwc/preview/community/<site>/cmp/<namespace>/<component>
```

The LWC local shell is a preview feature. It gives a useful local Lightning
workbench and preview routes, not full hosted Lightning Experience parity.

Visualforce-backed tabs redirect to `/apex/<Page>`. That redirect and shared
Lightning Out runtime are the Visualforce boundary for this LWC loop.

The shell supports `CurrentPageReference`, basic `NavigationMixin` URL and
navigation behavior for local targets, Apex wire and imperative controller
imports, `getRecord`, `getRecords`, local DML-backed create/update/delete
record helpers, `getRecordCreateDefaults`, record-input helper functions,
`lightning/uiObjectInfoApi` object info and picklist wires,
`lightning/uiRelatedListApi` child rows, schema tokens, custom labels, static
resources and subpaths, content assets, user values, checked i18n values, form
factor, custom permission defaults, Experience Cloud context, local message
service, resource loading, toast events, workspace API approximations,
alert/confirm/prompt/config/page-reference helper shims, Flow screen and
refresh events, practical common and expanded checked base components, and
packaged local SLDS 2 styling with classic SLDS assets available. Create
defaults include object info, record defaults, and project layout field
sections when available, with a generated full layout from createable fields as
the local fallback.
`lightning/uiLayoutApi` `getLayout` returns the same local Record Layout shape.
Local org data comes from project schema plus Glade storage fixtures in
`data/*.json`.

When an LWC needs real records or Apex controller data, put the rows the
component needs into a Glade storage fixture under `data/` and run the shell
from the project:

```bash
glade dev lwc --project . --port 8080
```

The active builder context flows into `recordId`, `objectApiName`, LDS wire
adapters, record forms, output fields, navigation state, and controller
arguments that the component passes through.

The shell exposes current local state for tooling:

```text
/lightning/local/context.json
```

That JSON includes active route, PageReference, route context, mounted
components, routes, diagnostics, and supported services.

Read [LWC_LOCAL_SHELL.md](LWC_LOCAL_SHELL.md) for route details and current
limits. Read [LWC_SUPPORT.md](LWC_SUPPORT.md) for the support table.

## Symbol Cache

Code intelligence cache files live under `.glade/symbols`. They are separate
from the test and DAP startup caches. `glade schema import describe --project-cache .`
can populate schema symbols from captured describe JSON for offline definition,
reference, and refactor queries.

```bash
glade inspect definition --project . --symbol Account.Name
glade inspect references --project . --symbol RefinementService.total --json
glade refactor rename --project . --symbol RefinementService --to FileRefinementService --dry-run --json
glade schema import describe --input reports/org-describe.json --output schema/local.schema.json --project-cache .
```

## Run Visualforce Pages Locally

Visualforce local rendering is a preview feature. It gives useful local
`/apex/<PageName>` previews, not full hosted Visualforce parity.

Start the Visualforce dev server from a Salesforce-shaped project:

```bash
glade dev vf --project . --addr 127.0.0.1:8080
```

Startup output lists the local page routes and watched source families:

```text
Visualforce dev server: http://127.0.0.1:8080
Pages:
  /apex/Core
  /apex/CardHost
Watching ... for .page, .component, .cls, aura, lwc, and static resource changes.
```

Open the listed `/apex/<PageName>` route in a browser. Render diagnostics use a
local HTML overlay with the page file and Visualforce expression when Glade can
pin the source. The same server exposes the current standard-component support
rows as local JSON:

```bash
curl http://127.0.0.1:8080/services/data/v61.0/glade/visualforce/support
```

Use `--port 8080` for the common localhost shortcut. Use an ephemeral address
and a ready file for scripts:

```bash
glade dev vf --project . --addr 127.0.0.1:0 --ready-file /tmp/glade-vf-ready.json
```

The local renderer covers common standard components, custom components,
controller actions, page messages, expression and form binding, static
resources, uploads, remoting envelopes, Lightning Out/LWC hosts, AJAX refresh
paths, and local PDF fallback output. It does not promise Salesforce-hosted
chrome, every component edge, exact lifecycle timing, Apex
`PageReference.getContent*` parity, or byte-for-byte PDF output.

Form posts carry signed Visualforce view state and a CSRF token. Controller and
extension state comes back across posts, while Apex fields marked `transient`
stay out of the saved state. Lightning Out pages validate `ltng:outApp`
dependencies before creating local LWC modules, so a missing Aura dependency or
component name reports a local rendering diagnostic instead of a browser fetch
failure.

## Run Tests Without An Org

Run every local Apex test:

```bash
glade test --project . --json
```

Run one test class or one method:

```bash
glade test --project . --class RefinementServiceTest --json
glade test --project . --class RefinementServiceTest --method testRefinesFileRow --json
```

## Capture Local Run Telemetry

`glade check` and local `glade test` runs can write CPU and heap profiles. A
local one-shot run can also write opt-in performance counters. Keep each
artifact path distinct:

```bash
mkdir -p reports
glade check --project . --cpu-profile reports/check.cpu.pprof --mem-profile reports/check.mem.pprof --perf-json reports/check.perf.json
glade test --project . --no-serve --no-cache --cpu-profile reports/test.cpu.pprof --mem-profile reports/test.mem.pprof --perf-json reports/test.perf.json
```

These files are local telemetry artifacts. They help investigate local runtime
costs; they do not replace Salesforce validation. CPU and heap profiles capture
the lifetime of the local command and may also profile daemon or watch mode;
they are written when that command exits and cannot be used with `--connect`.
`--perf-json` is for a local one-shot run and cannot be combined with daemon,
watch, watch-once, connect, wizard, last-failed, or shard-plan-only modes.

Check performance JSON reports the project and semantic cache provenance.
`semanticCache.source` records whether the exact result came from memory, disk,
or build; related fields report waits, safe-miss reasons, retained bytes, and
evictions. Check counters also cover physical source reads, reused logical
views, allocations, and garbage collection.

Test performance JSON adds an `execution` object with invocation arguments,
requested and effective parallelism, method-parallel policy, `GOMAXPROCS`, disk
cache policy, and execution mode. Its remaining counters cover cache phases
and slow classes or methods. These versioned local artifacts contain project
paths and, for tests, command arguments, so review them before sharing.

`glade check` and the test semantic gate store exact results under
`.glade/semantic/`. Use `glade check --no-cache` or test `--no-cache` to bypass
semantic cache reads, writes, and memory reuse. Immutable source-generation
validation still runs before any cached result is published or returned.

## Run a Performance Risk Scan

Generate an advisory performance report from source and metadata first:

```bash
# Uses the default public registry; custom registries, archives, and links are also supported.
mkdir -p reports
glade plugins install @glade/performance
glade performance scan --project . --format json > reports/glade-performance.json
```

Add trace data from a representative local run to rank measured paths first:

```bash
mkdir -p reports
glade test --project . --class SlowPathTest --trace reports/slow-path.trace.json
glade performance scan --project . --trace reports/slow-path.trace.json --format markdown --top 10
```

Static findings are leads. Trace-backed findings are measured local evidence.
Org facts raise or lower confidence when metadata or data shape changes the
risk.

For org-shape triage, pass a local snapshot. The plugin never logs in to
Salesforce for this path:

```bash
glade performance scan --project . --org-facts reports/org-facts.json --fail-on high
glade performance scan --project . --format sarif > reports/glade-performance.sarif
```

Plugin install, archive, and author details are in [PLUGINS.md](PLUGINS.md).
The short alias `performance` resolves to `@glade/performance`.

Use `--parallelism` to choose the worker count. Method-level parallel execution
is on by default; use `--no-parallel-methods` only when a project has tests that
share mutable local state. The scheduler adapts within that fixed worker budget:
it uses recorded class durations when available and shifts freed capacity to
methods in the remaining classes. It never exceeds the requested parallelism.

```bash
glade test --project . --parallelism 8 --json
```

## Split and Balance Test Suites

Use an exact class file when a wrapper already owns selection. It contains one
class per line; blank lines and lines beginning with `#` are ignored:

```bash
glade test --project . --class-file test-classes.txt
```

Run one deterministic shard, or write balanced class lists for separate
workers:

```bash
glade test --project . --shard-count 4 --shard-index 0
glade test --project . \
  --shard-count 4 \
  --duration-history .glade/test-durations.json \
  --write-class-shards reports/test-shards
```

An unfiltered, unsharded run maintains `.glade/test-durations.json`. Duration
history balances later shards; the assignment is deterministic for the same
selected classes, shard count, and history. `--write-class-shards` writes
`shard-NNN.txt` files and exits without running tests. Use `--test-timeout 2m`
to override the five-minute per-test default. On a memory-constrained host,
`--gc-aggressive` reduces heap growth at the cost of more frequent garbage
collection.

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
{"event":"watch.tests_selected","time":"...","selection":{"mode":"direct","testClasses":["RefinementServiceTest","RefinementSummaryTest"],"reason":"changed types reach affected tests"}}
{"event":"watch.run_started","time":"...","runId":2,"testClasses":["RefinementServiceTest","RefinementSummaryTest"]}
{"event":"watch.run_finished","time":"...","runId":2,"summary":{"total":9,"passed":9,"failed":0,"passedAll":true}}
```

Editing a trigger instead falls back to the full suite:

```json
{"event":"watch.tests_selected","time":"...","selection":{"mode":"all","reason":"changed trigger may affect any test"}}
```

### Startup cache and warm runs

Large projects pay a startup cost to build local org state and compile project
helpers before the first test runs. Eligible `glade test` modes can persist
that harness in `.glade/test/startup.meta.json` plus a hashed
`startup.payload.<sha256>.gob` and reload it when fingerprint checks pass.
One-shot parallel-method runs with more than one effective worker deliberately
bypass restored-runtime disk caching to protect isolation; `glade test
--wizard` prints the effective policy.

Semantic analysis is independent. `glade check` and `glade test` can reuse an
exact result under `.glade/semantic/` from memory, disk, or build even when the
restored-runtime payload is bypassed. Source, companion metadata, schema,
dependencies, analyzer and platform ABIs, and options must all match.

**Read [TEST_STARTUP_CACHE.md](TEST_STARTUP_CACHE.md) for the full rules** —
what is cached, when it is written, how freshness works, and when a bad cache
can make results untrustworthy.

Quick reference:

| Goal | Command |
| ---- | ------- |
| Run tests affected by changed files | `glade test changed --project . --since HEAD` |
| Rerun the last failed tests | `glade test failed --project .` |
| Pick the next loop command | `glade test --project . --wizard` |
| Delete startup and semantic caches | `glade test clear-cache --project .` |
| One run without startup or semantic cache reuse | `glade test --project . --no-cache` |
| Fastest repeated CLI loops | `glade test serve --project .` (see below) |

Clear the caches after `git pull`, branch switches, Glade upgrades, or whenever
tests behave as if old project state is still loaded. `--no-cache` bypasses the
startup and semantic caches, including semantic memory reuse, while debugging
harness, org-inference, or analysis issues. A separately running test server
keeps its own memory until stopped or restarted.

For the fastest repeated loops, start a persistent test server in one terminal:

```bash
glade test serve --project .
```

Then run focused tests from another terminal. `glade test` auto-connects when
`.glade/test/serve.sock` is reachable:

```bash
glade test daemon status --project .
glade test --project . --class RefinementServiceTest
glade test daemon stop --project .
```

Use `--connect` to require the server, or `--no-serve` to force a local cold or
disk-cache warm build.

### Keep the index warm inside one watch loop

For a single long-lived watch session, `--daemon` holds a warm, incrementally
updated index and reference graph in-process. Eligible Apex-only edits update
incrementally. Metadata, configuration, project-topology, uncertain, and
recovery changes fall back to an authoritative rebuild. Selection remains
conservative, so a change that cannot be proven safe runs the full suite.

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
mkdir -p reports
glade exec --project . --trace reports/trace.json "System.debug(1);"
glade profile analyze reports/trace.json
glade profile analyze reports/trace.json --format pprof > reports/trace.pb.gz
```

## Offline Debug Log Analysis

`glade debug` runs on saved Salesforce logs without org access:

- `parse` prints structured JSON log entries.
- `profile` builds measured runtime output.
- `explain` adds conservative source candidate matches.
- `editor` builds the Apex Log editor map for navigation, folding, hovers, and diagnostics.
- `repro` writes a best-effort Apex test class from the subscriber log.
- `replay` builds dry-run anonymous Apex for VS Code log replay.

```bash
glade debug parse --log logs/apex-debug.log --json
glade debug profile --log logs/apex-debug.log
glade debug explain --log logs/apex-debug.log --project .
glade debug explain --log logs/apex-debug.log --project . --json
glade debug editor --log logs/apex-debug.log --project . --json
glade debug repro --log logs/apex-debug.log --project . > ReproTest.cls
glade debug replay --log logs/apex-debug.log --project . --json
glade debug replay --log logs/apex-debug.log --project . --entry-index 12 --json
```

`repro` infers setup records from SOQL equality filters, entry-point calls from
code-unit or stack-frame entries, and baseline assertions from DML and exception
events. Treat the generated class as the first local reproducer, then tighten
the assertions after the bug is understood.

`editor` uses that same evidence to make `.apexlog` and `.apex.log` files
easier to scan in VS Code. It folds execution units, methods, constructors,
SOQL, DML, limits, and exceptions. It links classes, methods, source-line
events, variables, SOQL objects, SOQL fields, and DML objects when local source
or metadata proves the target. Downloaded `.log` or `.txt` files can use
`Glade: Treat Current File as Apex Log`.

`replay` uses the same evidence to create anonymous Apex that VS Code can run
through `glade dap --dry-run`. `--entry-index` replays from one source-backed
frame when the log has enough entry-point evidence.

For best replay and editor navigation, capture Apex Code at FINEST, Apex
Profiling at FINE, Callout at INFO, Database at FINE, System at DEBUG,
Validation at INFO, Visualforce at INFO, Workflow at INFO, and NBA/Wave/other
feature categories at NONE unless the feature is in the path. The minimum useful
trace is Apex Code: FINE, Apex Profiling: INFO, Database: INFO, System: DEBUG,
Validation: INFO, and Workflow: INFO. Variable navigation and frame replay lose
shape when Apex Code is below FINEST. SOQL and DML links lose shape when
Database is below INFO. Limit folding loses shape when Apex Profiling is below
INFO.

## Playground

Use the browser playground for quick experiments with local files, anonymous
Apex, logs, limits, traces, and local org diffs:

```bash
glade playground --project . --db .glade/playground/org.sqlite --open
```

The playground runs on localhost and uses the same compiler, VM, and storage
layers as `glade test` and `glade exec`.

Static resources and playground workspaces accept only regular, supported files
that resolve beneath their selected roots. Unsafe paths and symlink escapes are
rejected.

# Editor Integration

`glade` exposes editor-facing entry points through normal CLI commands. DAP has
live VM pause/step primitives, LSP uses full-project indexing at startup with
open-buffer overlays, and watch mode uses native file watching with polling
fallback.

## VS Code Extension

Glade ships a VS Code extension in release archives. Install Glade first, then
install the bundled VSIX:

```bash
glade editor doctor vscode
glade editor install vscode --force
```

From a source checkout, build the local VSIX once and run the same install
command from anywhere inside the checkout:

```bash
npm --prefix contrib/vscode-glade install
npm --prefix contrib/vscode-glade run package
glade editor install vscode --force
```

The extension adds a `Glade` Activity Bar for the local Apex loop. It does not
replace org-backed commands, CodeLens, language features, or scratch-org test
runs. Glade surfaces use `Glade: ...` command titles, `glade.*` command ids, a
separate `Glade Apex` Test Explorer controller, and CodeLens labels that include
`Local`.

The sidebar shows:

- Start Here: SFDX root, active local data environment, local DB state, watch
  state, last run state, and top daily actions.
- Local Runs: changed tests, failed tests, and warm watch controls.
- Data Environments: named SQLite-backed local org states.
- Local Org: inspect, seed, reset, and export for the active environment.
- Debug: active Apex breakpoint count and local debug actions.

## Daily Local Apex Loop

Open the Glade Activity Bar and start in **Start Here**.

1. Confirm the SFDX root and active local data environment.
2. Click **Run local proof** before pushing work to a scratch org.
3. Use **Data Environments** to clone, seed, reset, inspect, and export local state.
4. Use org-backed tools for deploy, retrieve, org tests, SOQL Builder, and Code Analyzer.

Glade actions are local. Salesforce actions stay org-backed.

## Native VS Code Surfaces

Glade uses one Activity Bar item and one Status Bar item. The sidebar shows
Start Here, Local Runs, Data Environments, Local Org, and Debug.

Local Apex tests appear in the native VS Code Testing view under `Glade Apex`.
Glade does not add a second Apex Tests sidebar tree. Breakpoints stay in the
normal editor gutter and debug state stays in VS Code Run and Debug.

The Status Bar shows the active local data environment and the latest local
state, such as `Glade: dev`, `Glade: dev 18ms`, or `Glade: billing-case 1 fail`.
Click it to switch data, inspect local data, run local proof, or open output.

The default local data environment is `dev` at `.glade/envs/dev.sqlite`.
`Glade: Execute Local Anonymous Apex`, CodeLens debug, and Test Explorer debug
all pass the active DB to Glade:

```bash
glade exec --project <root> --db <active-db> --log-out reports/exec.log "insert new Account(Name='local');"
glade dap --project <root> --db <active-db>
```

Configure more environments in workspace settings:

```json
{
  "glade.environments": [
    { "name": "dev", "dbPath": ".glade/envs/dev.sqlite" },
    { "name": "feature", "dbPath": ".glade/envs/feature.sqlite" }
  ],
  "glade.activeEnvironment": "dev"
}
```

## Browser Playground

`glade playground` starts a local web UI for quick Apex experiments. It is useful
when you want a DotNetFiddle-style loop: edit class files, write execute
anonymous Apex that calls those classes, run on demand, and inspect cached
output, variables, limits, traces, diagnostics, and org changes.

Pass `--examples` when you want the built-in scratch examples for DML, SOQL,
triggers, relationships, maps, and limit counters:

```bash
glade playground --db .glade/playground/org.sqlite --examples --open
```

Point it at a project when you want to edit that folder directly:

```bash
glade playground --project . --db .glade/playground/org.sqlite --open
```

The foreground project runs as local source for execute-anonymous calls, even
when `sfdx-project.json` declares a namespace. Configured managed package
dependencies still run as dependencies.

Use one or more `--project-ref name=path` flags when you want selectable local
project folders copied into the scratch playground instead of editing the folder
directly. Project references are available only in the managed scratch
workspace, not when `--project` is used. Built-in examples are hidden while
project references are configured. Project references skip dot files and dot
directories, clear the copied SFDX namespace so classes run as local source, and
treat only `seed.json` as playground data.

Keyboard shortcuts in the web UI:

- `Cmd/Ctrl+Enter`: run execute anonymous.
- `Cmd/Ctrl+S`: save the active file.
- `Cmd/Ctrl+K`: open the command palette.
- `Cmd/Ctrl+Shift+R`: reset the local org.

The playground uses the same local compiler, project runtime registration, VM,
and storage layers as CLI execution. It is a browser-first editor loop, not a
hosted service.

## Offline Debug Log Analysis

`glade debug` reads local Salesforce debug logs and adds useful, offline-only
post processing:

- `parse` prints structured log entries.
- `profile` converts measured log lines into the runtime profile format.
- `explain` adds conservative source annotations for log-backed evidence.
- `repro` emits a best-effort Apex test class from the same log evidence.

Try these command lines from a project with matching `.cls` files:

```bash
glade debug parse --log logs/apex-debug.log --json
glade debug profile --log logs/apex-debug.log
glade debug explain --log logs/apex-debug.log --project .
glade debug explain --log logs/apex-debug.log --project . --json
glade debug repro --log logs/apex-debug.log --project . > ReproTest.cls
```

Matching is conservative. `explain` will rank candidates by confidence and keep
the strongest links only. Confidence below `0.50` stays in JSON for tooling, but
text output keeps the default threshold.

`repro` uses the same annotations, SOQL object names, equality filters, DML row
counts, code-unit entries, and exception stack frames. It writes Apex to stdout
so the file can enter the local test loop after review.

## VS Code Tasks

Create `.vscode/tasks.json` in a Salesforce project to run the same checks that
CI and terminal workflows use. These commands run against local source; they do
not require a Salesforce org.

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "glade: check",
      "type": "shell",
      "command": "glade",
      "args": ["check", "--project", "${workspaceFolder}"],
      "group": "build",
      "problemMatcher": []
    },
    {
      "label": "glade: test",
      "type": "shell",
      "command": "glade",
      "args": ["test", "--project", "${workspaceFolder}", "--json"],
      "group": "test",
      "problemMatcher": []
    },
    {
      "label": "glade: test changed since origin/main",
      "type": "shell",
      "command": "glade",
      "args": [
        "test",
        "changed",
        "--project",
        "${workspaceFolder}",
        "--since",
        "origin/main",
        "--json"
      ],
      "group": "test",
      "problemMatcher": []
    },
    {
      "label": "glade: watch tests",
      "type": "shell",
      "command": "glade",
      "args": [
        "test",
        "--project",
        "${workspaceFolder}",
        "--daemon",
        "--watch",
        "--debounce",
        "750ms"
      ],
      "isBackground": true,
      "problemMatcher": []
    },
    {
      "label": "glade: test serve",
      "type": "shell",
      "command": "glade",
      "args": ["test", "serve", "--project", "${workspaceFolder}"],
      "isBackground": true,
      "problemMatcher": []
    },
    {
      "label": "glade: diagnostics once",
      "type": "shell",
      "command": "glade",
      "args": [
        "lsp",
        "--project",
        "${workspaceFolder}",
        "--diagnostics-once"
      ],
      "problemMatcher": []
    }
  ]
}
```

## VS Code Debug Launch Examples

`glade dap`, `glade exec --debug`, and `glade test --debug` speak Debug Adapter
Protocol over stdio. `glade dap --project .` starts a standalone adapter that
accepts `initialize`, `launch`, breakpoints, stepping, variable inspection, and
disconnect. Use the contrib VS Code extension or a stdio-capable DAP client.

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "glade: debug all tests",
      "type": "glade",
      "request": "launch",
      "program": "MathUtilTest.adds",
      "project": "${workspaceFolder}",
      "cwd": "${workspaceFolder}"
    },
    {
      "name": "glade: debug one test class",
      "type": "glade",
      "request": "launch",
      "program": "${input:testClass}",
      "project": "${workspaceFolder}",
      "cwd": "${workspaceFolder}"
    },
    {
      "name": "glade: debug anonymous Apex",
      "type": "glade",
      "request": "launch",
      "program": "${input:anonymousApex}",
      "project": "${workspaceFolder}",
      "cwd": "${workspaceFolder}"
    }
  ],
  "inputs": [
    {
      "id": "testClass",
      "type": "promptString",
      "description": "Apex test class filter"
    },
    {
      "id": "anonymousApex",
      "type": "promptString",
      "description": "Anonymous Apex to run",
      "default": "System.debug('hello from glade');"
    }
  ]
}
```

## Warm Test Service

The internal `testdaemon` package keeps project load, schema, type index, and
the affected-test **reference graph** warm for repeated editor and watch loops.

`glade test serve` runs a persistent unix-socket server that warms the runtime
once and serves later `glade test` invocations over `.glade/test/serve.sock`.
Client runs auto-connect when the socket is reachable. Use `--connect` to
require the server or `--no-serve` to force a local build.

`glade test --daemon` keeps the same warm index in-process for a single long
`--watch` or changed-test session:

- `RunFilter(filter)` runs a focused test selection against the warm index.
- `RunChangedSince(ref)` uses git file changes and affected-test selection, with
  the conservative full-run fallback when impact is broad (triggers, schema, or
  a changed class no test reaches).
- `Reload()` refreshes the full project state and rebuilds the reference graph
  when incremental impact is not safe to infer.

On each change the daemon refreshes the graph incrementally: a modified file is
re-scanned on its own, while an added or deleted file triggers a full rebuild so
cross-file edges are never missed. Selection then walks the graph's reverse
edges to find every test that transitively reaches the changed types. See
`docs/LOCAL_TESTING.md` for the user-facing workflow and event examples.

Use `glade test serve` when separate CLI invocations should stay warm. Use
`glade test --daemon --watch` when one long process should avoid reloading the
project.

For the common local workflow, use `glade test changed` to select tests affected
by changed Apex and metadata dependencies:

```bash
git fetch origin main
glade test serve --project .
glade test changed --project . --since origin/main --json --no-progress
```

## Test Startup Cache

`glade test` persists warmed org and compiled runtime state in
`.glade/test/startup.gob`. That cache keeps large projects from rebuilding local
org inference and helper compilation on every CLI invocation.

See **[TEST_STARTUP_CACHE.md](TEST_STARTUP_CACHE.md)** for when the cache is
created, how freshness checks work, limitations that can leave a stale cache
in place, and how `glade test serve` interacts with the on-disk file.

```bash
glade test clear-cache --project .
glade test --project . --no-cache --class AccountServiceTest
```

Clear the cache after branch switches or Glade upgrades. Restart
`glade test serve` after `clear-cache` if the server was already running.

## DAP Startup Cache

`glade dap` persists startup state in `.glade/dap/startup.json`. That cache keeps
the warmed project index and org state between launches so large projects can start
faster.

The cache is auto-invalidated when relevant source files, config files, or package
root directories change. To force a full rebuild, remove the cache directory:

```bash
rm -rf .glade/dap
```

The current DAP server supports initialize, launch, attach, breakpoints,
continue, pause, next, step-in, step-out, stack trace, scopes, variables,
evaluate, watch expressions, and disconnect. Live VM pause hooks stop before
statements at breakpoints and step through method calls.

## LSP Wiring

`glade lsp --project <root>` runs an LSP server over stdio. Configure editor
clients to start that command from the workspace root and treat `*.cls` and
`*.trigger` files as Apex. The current server provides initialize/shutdown,
incremental text document sync, open-buffer parse diagnostics, project
diagnostics shaped like `glade check`, test-result diagnostics from stack frames,
document and workspace symbols, semantic tokens, definition, references, rename,
hover, and completion for Apex symbols, schema fields, and keywords from the
project index.

```json
{
  "command": "glade",
  "args": ["lsp", "--project", "${workspaceFolder}"],
  "filetypes": ["apex", "cls", "trigger"],
  "rootPatterns": ["glade.yml", "sfdx-project.json"]
}
```

For a one-shot diagnostics check without starting a long-lived language client,
run:

```bash
glade lsp --project . --diagnostics-once
```

## Watch And Reports

Watch mode emits newline-delimited JSON events for editor and test UI consumers.
Every watch event includes `schemaVersion: 1`, `event`, and `time`. Run events
always include `runId`; `watch.run_started` always includes `testClasses` as an
array, empty for the initial all-test run.

```bash
glade test --project . --watch --debounce 750ms --watch-backend auto
```

For CI or editor tasks that need a single machine-readable run, use:

```bash
glade test --project . --json --no-progress
glade test changed --project . --since origin/main --json --no-progress
glade test --project . --junit reports/glade-junit.xml
glade check --project . --json --no-progress
```

Trace analysis stays native to `glade`:

```bash
glade exec --trace reports/trace.json 'System.debug(1);'
glade profile analyze reports/trace.json
glade profile analyze reports/trace.json --json
```

The Markdown and JSON reports include hot events, category counts, runtime
sections, and governor/resource summaries.

## Salesforce-style debug log

For a log that reads like the platform's Developer Console output — and to
compare glade against a real org running the same code — emit a
Salesforce-style debug log:

```bash
glade exec --debug-log reports/apex.log 'System.debug('"'"'hi'"'"'); Integer x = 1;'
glade exec --debug-log raw 'System.debug('"'"'hi'"'"');'   # write raw log to stdout
```

The log uses the familiar `HH:MM:SS.mmm (nanos)|EVENT|details` format with
`EXECUTION_STARTED`, `CODE_UNIT_STARTED`, `USER_DEBUG`, `SOQL_EXECUTE_BEGIN/END`,
`DML_BEGIN/END`, `FATAL_ERROR`, and a `CUMULATIVE_LIMIT_USAGE` block. It is
structurally faithful rather than byte-identical: high-signal events in true
execution order. Capture the same anonymous Apex's org log and diff the two to
confirm glade matches the platform.

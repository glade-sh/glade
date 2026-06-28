# Editor, LSP, and DAP

Glade includes a VS Code extension for local Apex work. It uses the same parser,
semantic checks, VM, storage layer, test runner, LSP, and DAP features as the
CLI.

## VS Code Extension

Install the bundled extension from a Glade release:

```bash
glade editor doctor vscode
glade editor install vscode --force
```

The VSIX lives in the release archive at
`share/glade/editor/vscode-glade.vsix`. For extension development, package from
the source tree, then run the same install command from anywhere inside the
checkout:

```bash
npm --prefix contrib/vscode-glade install
npm --prefix contrib/vscode-glade run package
glade editor install vscode --force
```

Open an SFDX project. The extension adds one `Glade` Activity Bar and a
`Glade: Open Home` command. The sidebar includes Start Here, Local Runs, Data
Environments, Local Org, Exec & SOQL, Debug, and Plugins views.

Glade keeps a separate local workflow with `glade.*` command ids, a `Glade Apex`
Test Explorer controller, and CodeLens labels that include `Local`. It does not
take over org-backed commands, scratch-org tests, CodeLens, replay debugging, or
language-server ownership.

The Command Palette also includes `Glade: Open TUI`, `Glade: Open Tests TUI`,
`Glade: Open Data TUI`, and `Glade: Open Plugins TUI`. Each command opens a
terminal and runs `glade tui` for the current project. The data TUI uses the
active local data environment and can pass a Salesforce CLI target org to the
import-from-org action.

## Daily Local Apex Loop

Open the Glade Activity Bar and start in **Start Here**.
Use **Glade: Open Home** when you want the daily hub. The first tab groups run,
data, debug, Salesforce, and ship actions. The state tab shows project root,
active Glade org, active data environment, Salesforce target, tests, watch
state, and plugin findings.

1. Confirm the SFDX root and active local data environment.
2. Click **Run local proof** before pushing work to a scratch org.
3. Use **Data Environments** to clone, seed, reset, inspect, and export local state.
4. Use org-backed tools for deploy, retrieve, org tests, SOQL Builder, and Code Analyzer.

Glade actions are local. Salesforce actions stay org-backed.

## Native VS Code Features

Glade uses one Activity Bar item and one Status Bar item. The sidebar shows
Start Here, Local Runs, Data Environments, Local Org, Exec & SOQL, Debug, and
Plugins.

Local Apex tests appear in the native VS Code Testing view under `Glade Apex`.
Glade does not add a second Apex Tests sidebar tree. Breakpoints stay in the
normal editor gutter and debug state stays in VS Code Run and Debug.

The Status Bar shows short local state, such as `Glade: dev`,
`Glade: dev 18ms`, `Glade: dev no DB`, or `Glade: plugin 2 findings`.
Details stay in the tooltip: project root, active DB, plugin finding counts,
and last command. Click it to switch data, inspect local data, run local proof,
manage plugins, or open output.

## LWC and Visualforce preview

LWC and Visualforce preview are CLI preview features. They remain available
through `glade dev`, but the VS Code extension does not start, stop, list, or
monitor those servers until the preview workflow is steadier.

Use [Preview LWC locally](/guide/workflows/lwc-preview) and [Preview Visualforce locally](/guide/workflows/visualforce-preview) for task steps. Use [LWC preview](/guide/modules/lwc-preview) and [Visualforce preview](/guide/modules/visualforce-preview) for subsystem boundaries.

```bash
glade toolchain install
glade dev lwc --project . --open
glade dev vf --project . --port 8080
```

LWC routes use `/lwc/preview/...`. Visualforce routes use `/apex/<Page>`.
Visualforce-backed LWC tab routes resolve through the LWC shell and then open
the Visualforce page.

## Plugin actions and findings

The extension reads installed and linked plugins through:

```bash
glade plugins list --json
```

Installed plugins may declare editor actions for Start Here, Local Runs, Local
Org, Debug, or Plugins views. Linked local plugins work the same way
after `glade plugins link --exec <plugin-executable>`.

When a plugin action declares `output: "glade.findings.v1"`, the extension can
parse stdout and publish findings into VS Code Problems. Findings carry
severity, message, optional file, line, column, rule id, and source.

## Local Tests

Run local tests from the Glade Activity Bar, the native VS Code Test Explorer,
or CodeLens. Focused runs call:

```bash
glade test --project PROJECT_ROOT --json --class CLASS_NAME --method METHOD_NAME
```

Changed-test runs use:

```bash
glade test changed --project PROJECT_ROOT --since origin/main --json
```

Change the default ref with `glade.changedSince`.

The warm watch buttons run:

```bash
glade test --project PROJECT_ROOT --daemon --watch
```

## Local Data Environments

The default environment is `dev` at `.glade/envs/dev.sqlite`. Add more named
environments in workspace settings:

```json
{
  "glade.environments": [
    { "name": "dev", "dbPath": ".glade/envs/dev.sqlite" },
    { "name": "feature", "dbPath": ".glade/envs/feature.sqlite" }
  ],
  "glade.activeEnvironment": "feature"
}
```

Execute anonymous Apex persists successful DML to the active environment:

```bash
glade exec --project PROJECT_ROOT --db ACTIVE_DB --log-out reports/exec.log "insert new Account(Name='local');"
```

The Local Org view can inspect, seed, reset, and export the active DB.

## Debug

Glade debug sessions use Debug Adapter Protocol over stdio:

Use [Debug Apex](/guide/workflows/debug-apex) for the task path and [Debug and profile](/guide/modules/debug-profile) for the subsystem boundary.

```bash
glade dap --project PROJECT_ROOT --db ACTIVE_DB
```

Anonymous debug, CodeLens debug, and Test Explorer debug all use the active
local data environment. Breakpoints come from the normal VS Code Apex gutter.
Glade does not draw a second breakpoint UI.

## Language Server

Start the LSP server over stdio:

```bash
glade lsp --project .
```

Run one diagnostics pass without starting a long-lived server:

```bash
glade lsp --project . --diagnostics-once
```

The VS Code extension keeps the Glade LSP off by default. Set
`glade.enableLsp=true` when you want local Glade diagnostics in VS Code.

## CLI code intelligence

Use the same project graph from terminal tasks when you need definition,
reference, or rename checks outside an editor:

```bash
glade inspect definition --project . --symbol RefinementService
glade inspect definition --project . --file force-app/main/default/classes/RefinementService.cls --line 6 --column 13
glade inspect references --project . --symbol RefinementService.total --json
glade inspect references --project . --symbol Account.Name --include-declaration
glade refactor rename --project . --symbol RefinementService --to FileRefinementService --dry-run
glade schema import describe --input reports/org-describe.json --project-cache .
```

`glade refactor rename` defaults to a dry-run plan. `schema import describe
--project-cache` writes captured schema symbols under `.glade/symbols` for
offline definition, reference, and rename queries.

## VS Code Task Example

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "glade: check",
      "type": "shell",
      "command": "glade check --project . --json",
      "problemMatcher": []
    },
    {
      "label": "glade: watch local tests",
      "type": "shell",
      "command": "glade test --project . --daemon --watch",
      "isBackground": true,
      "problemMatcher": []
    },
    {
      "label": "glade: lsp diagnostics",
      "type": "shell",
      "command": "glade lsp --project . --diagnostics-once",
      "problemMatcher": []
    }
  ]
}
```

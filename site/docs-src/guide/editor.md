# Editor, LSP, and DAP

Glade includes a VS Code extension for local Apex work. It uses the same parser,
semantic checks, VM, storage layer, test runner, LSP, and DAP surfaces as the
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

Open an SFDX project. The extension adds one `Glade` Activity Bar with Start
Here, Local Runs, Data Environments, Local Org, Debug, and Plugins views.

Glade keeps a separate local lane with `glade.*` command ids, a `Glade Apex`
Test Explorer controller, and CodeLens labels that include `Local`. It does not
take over org-backed commands, scratch-org tests, CodeLens, replay debugging, or
language-server ownership.

## Daily Local Apex Loop

Open the Glade Activity Bar and start in **Start Here**.

1. Confirm the SFDX root and active local data environment.
2. Click **Run local proof** before pushing work to a scratch org.
3. Use **Data Environments** to clone, seed, reset, inspect, and export local state.
4. Use org-backed tools for deploy, retrieve, org tests, SOQL Builder, and Code Analyzer.

Glade actions are local. Salesforce actions stay org-backed.

## Native VS Code Surfaces

Glade uses one Activity Bar item and one Status Bar item. The sidebar shows
Start Here, Local Runs, Data Environments, Local Org, Debug, and Plugins.

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

```bash
glade toolchain install
glade dev lwc --project . --port 8080
glade dev vf --project . --port 8080
```

LWC routes use `/lwc/preview/...`. Visualforce routes use `/apex/<Page>`.
Visualforce-backed LWC tab routes resolve through the LWC shell and then open
the Visualforce page.

## Plugin Actions And Findings

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
glade test --project <root> --json --class <Class> --method <Method>
```

Changed-test runs use:

```bash
glade test changed --project <root> --since origin/main --json
```

Change the default ref with `glade.changedSince`.

The warm watch buttons run:

```bash
glade test --project <root> --daemon --watch
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
glade exec --project <root> --db <active-db> --log-out reports/exec.log "insert new Account(Name='local');"
```

The Local Org view can inspect, seed, reset, and export the active DB.

## Debug

Glade debug sessions use Debug Adapter Protocol over stdio:

```bash
glade dap --project <root> --db <active-db>
```

Anonymous debug, CodeLens debug, and Test Explorer debug all use the active
local data environment. Breakpoints come from the normal VS Code Apex gutter.
Glade does not draw a second breakpoint surface.

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

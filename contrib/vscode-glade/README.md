# Glade Local Apex for VS Code

Glade Local Apex adds a local Apex lane inside VS Code.
It does not replace org-backed Salesforce commands.

## Install

Install from a Glade release:

```bash
glade editor doctor vscode
glade editor install vscode --force
```

That command installs the bundled VSIX at
`share/glade/editor/vscode-glade.vsix`. During extension development, package
and install the local VSIX from this checkout:

```bash
npm install
npm run package
glade editor install vscode --force
```

The extension requires a global `glade` command on `PATH`.

## Sidebar

The Activity Bar uses the Glade contour mark. Glade Home uses the same dark
shell, green action color, and status palette as the public site.

Open a normal SFDX project. The Glade Activity Bar shows:

Use **Glade: Open Home** for the daily hub. Home is task-first: run, data,
debug, Salesforce, and ship actions sit on the first tab. State is the second
tab: project root, active Glade org, active data environment, Salesforce target,
tests, watch state, and plugin findings.

- Start Here: SFDX root, active local data environment, local DB state, watch
  state, last run state, plugin action count, and a shortcut into Glade Home.
- Local Runs: changed tests, failed tests, and warm watch controls.
- Data Environments: named SQLite local orgs and the active DB path.
- Local Org: inspect, seed, reset, and export commands for the active DB.
- Exec & SOQL: SOQL scratch buffers, saved SOQL entries, describes, and last results.
- Debug: current VS Code Apex breakpoint count and local debug actions.
- Plugins: installed plugins, plugin actions, and plugin artifacts.

## Daily Local Apex Loop

Open the Glade Activity Bar and start in **Start Here**.

1. Confirm the SFDX root and active local data environment.
2. Click **Run local proof** before pushing work to a scratch org.
3. Use **Data Environments** to clone, seed, reset, inspect, and export local state.
4. Use org-backed tools for deploy, retrieve, org tests, SOQL Builder, and Code Analyzer.

Glade actions are local. Salesforce actions stay org-backed.

## Native VS Code Surfaces

Glade uses one Activity Bar item and one Status Bar item. The sidebar shows
Start Here, Local Runs, Data Environments, Local Org, Exec & SOQL, Debug, and
Plugins.

Local Apex tests appear in the native VS Code Testing view under `Glade Apex`.
Glade does not add a second Apex Tests sidebar tree. Breakpoints stay in the
normal editor gutter and debug state stays in VS Code Run and Debug.

The Status Bar shows short local state, such as `Glade: dev`,
`Glade: dev 18ms`, `Glade: dev no DB`, or `Glade: plugin 2 findings`.
Details stay in the tooltip. Click it to switch data, inspect local data, run
local proof, manage plugins, or open output.

## LWC, Visualforce, And Plugins

LWC and Visualforce preview are CLI preview features. They remain available
through `glade dev lwc` and `glade dev vf`, but the VS Code extension does not
start, stop, list, or monitor those servers until the preview workflow is
steadier.

Plugin actions come from installed plugin metadata:

```bash
glade plugins list --json
```

Linked local plugins are included after:

```bash
glade plugins link --exec <plugin-executable>
```

Plugin actions may target Start Here, Local Runs, Local Org, Debug, or Plugins
views. If an action emits `glade.findings.v1`, the extension maps
those findings into VS Code Problems with severity, file, line, column, rule
id, source, and message.

## Local Tests

The Test Explorer runs local Apex tests through:

```bash
glade test --project <root> --json --class <Class> --method <Method>
```

Changed-test runs use the `glade.changedSince` setting, which defaults to
`origin/main`.

CodeLens labels include `Local`, such as `Run Local Test` and
`Debug Local Test`, so Salesforce org-backed CodeLens entries stay distinct.

## Debug

Anonymous Apex and test debug launches use `glade dap` over stdio. Breakpoints
come from the normal VS Code Apex breakpoint gutter.

Debug launches pass the active local data environment:

```bash
glade dap --project <root> --db <root>/.glade/envs/dev.sqlite
```

## Local Data Environments

The default environment is `dev` at `.glade/envs/dev.sqlite`. Configure named
environments with:

```json
{
  "glade.environments": [
    { "name": "dev", "dbPath": ".glade/envs/dev.sqlite" },
    { "name": "feature", "dbPath": ".glade/envs/feature.sqlite" }
  ],
  "glade.activeEnvironment": "dev"
}
```

`Glade: Execute Local Anonymous Apex` runs:

```bash
glade exec --project <root> --db <active-db> --debug-log - <anonymous-apex>
```

Successful DML persists to the active DB. Failed and dry-run executions do not
persist.

## Local And Org Boundaries

Glade uses its own Activity Bar, `glade.*` commands, `Glade Apex` Test Explorer
controller, and local CodeLens labels. It does not contribute org-backed
commands or start the Glade LSP unless `glade.enableLsp` is true.

Useful settings:

- `glade.enableSidebar`
- `glade.enableTestExplorer`
- `glade.enableCodeLens`
- `glade.enableLsp`
- `glade.environments`
- `glade.activeEnvironment`
- `glade.changedSince`

## Develop

```bash
npm install
npm test
npm run package
```

Open this repo in VS Code and run **Launch Glade VS Code Extension**. In the
Extension Development Host, open an SFDX project.

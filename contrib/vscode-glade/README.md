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

Open a normal Salesforce DX project. The Glade Activity Bar opens with a short
Start Here panel. The other sections stay collapsed until you need them.

Use **Glade: Open Home** for the main developer hub. Home uses a rail and
detail panel for Data browser, Local tests, Glade org, Scratch editors, and
Salesforce. Each section keeps its main command and related commands in one
compact row. State uses the same rail shape for project, org, data,
Salesforce, test, and plugin status.

- Start Here: project state, active local data environment, changed tests, and
  a shortcut into Glade Home.
- Tests: changed tests, failed tests, and warm watch controls.
- Data Environments: named SQLite local orgs and the active DB path.
- Data Browser: active local data state and the inspect action.
- Apex & SOQL: anonymous Apex, SOQL scratch buffers, and saved SOQL entries.
- Debug: local debug actions, hidden by default until needed.
- Plugins: installed plugins, plugin actions, and plugin artifacts, hidden by default.

## Local Apex Loop

Open the Glade Activity Bar and start in **Start Here**.

1. Confirm the Salesforce DX root and active local data environment.
2. Start the local Glade org when you need the Salesforce-shaped API:
   `glade org start my-glade-org --project .`.
3. Click **Run changed tests** before pushing work to a scratch org.
4. Use **Data Environments** to switch or create local data. Use **Data Browser** to inspect it.
5. Use org-backed tools for deploy, retrieve, org tests, SOQL Builder, and Code Analyzer.

Glade actions are local. Salesforce actions stay org-backed.

## Native VS Code Surfaces

Glade uses one Activity Bar item and one Status Bar item. The sidebar shows
Start Here, Tests, Data Environments, Data Browser, Apex & SOQL, Debug, and
Plugins. Start Here opens first; Tests, Data Environments, Data Browser, and
Apex & SOQL default to collapsed. Debug and Plugins stay hidden until needed.

Local Apex tests appear in the native VS Code Testing view under `Glade Apex`.
Glade does not add a second Apex Tests sidebar tree. Breakpoints stay in the
normal editor gutter and debug state stays in VS Code Run and Debug.

The Status Bar shows short local state, such as `Glade: dev`,
`Glade: dev 18ms`, `Glade: dev no DB`, or `Glade: plugin 2 reports`.
Details stay in the tooltip. Click it to switch data, inspect local data, run
changed tests, manage plugins, or open output.

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

Plugin actions may target Tests, Data Browser, Debug, or Plugins
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

Open an `.apexlog` or `.apex.log` file and run `Glade: Replay Apex Debug Log`
to build a dry-run replay from the log evidence. Downloaded `.log` and `.txt`
files can be switched with `Glade: Treat Current File as Apex Log` when the
extension detects Salesforce debug-log events. Glade infers setup rows from
SOQL filters, calls the inferred entry point, and starts normal VS Code
debugging against the active local data environment. Breakpoints stay in the
usual Apex source files. Replay setup data is not saved back to the DB.

The Apex Log editor uses `glade debug editor --log <path> --project . --json`
behind the scenes. It adds folding for execution units, methods, constructors,
SOQL, DML, limits, and exceptions. It also adds outline symbols, hovers,
Problems diagnostics, semantic colors, source links, and go-to-definition for
class, method, source-line, variable, SOQL object, SOQL field, and DML object
references when local source or metadata proves the target. If no project is
open, grammar highlighting, folding, hovers, and parse diagnostics still work
where the log itself has enough shape.

Use `Glade: Refresh Apex Log Analysis` after editing or replacing a log file.
`Glade: Replay From Log Frame` is available from source-backed log frames and
passes `--entry-index` to the replay command.

For meaningful replay and editor navigation logs, capture this trace when you
can:

- Apex Code: FINEST.
- Apex Profiling: FINE.
- Callout: INFO.
- Database: FINE.
- System: DEBUG.
- Validation: INFO.
- Visualforce: INFO.
- Workflow: INFO.
- NBA, Wave, and other feature categories: NONE unless that feature is on the path.

The minimum useful trace is:

- Apex Profiling: INFO for limits and runtime profile.
- Apex Code: FINE.
- Database: INFO.
- System: DEBUG.
- Validation: INFO.
- Workflow: INFO.

Variable navigation and replay degrade when Apex Code is below FINEST because
`VARIABLE_SCOPE_BEGIN`, `VARIABLE_ASSIGNMENT`, and source-line events may be
missing. Query and DML navigation degrade when Database is below INFO. Limit
folding and profiling degrade when Apex Profiling is below INFO.

Avoid setting every category to FINEST. Oversized logs lose the trail when the
platform truncates them.

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
Extension Development Host, open a Salesforce DX project.

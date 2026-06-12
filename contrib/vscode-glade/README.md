# Glade Local Apex for VS Code

Glade Local Apex adds a local Apex lane beside the Salesforce VS Code extensions.
It does not replace org-backed Salesforce commands.

## Install

Install from a Glade release:

```bash
glade editor doctor vscode
glade editor install vscode --force
```

That command installs the bundled VSIX at
`share/glade/editor/vscode-glade.vsix`. During extension development, package
and install a local VSIX:

```bash
npm install
npm run package
glade editor install vscode --vsix dist/vscode-glade-0.0.1.vsix --force
```

The extension requires a global `glade` command on `PATH`.

## Sidebar

Open a normal SFDX project. The Glade Activity Bar shows:

- Project: SFDX root, package directories, namespace, API version, and detected
  Salesforce extensions.
- Recommended Runs: changed tests, failed tests, and warm watch controls.
- Apex Tests: the native VS Code Test Explorer controller named `Glade Apex`.
- Data Environments: named SQLite local orgs and the active DB path.
- Local Org: inspect, seed, reset, and export commands for the active DB.
- Debug And Logs: current VS Code Apex breakpoint count.

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

## Salesforce Extension Coexistence

Glade uses its own Activity Bar, `glade.*` commands, `Glade Apex` Test Explorer
controller, and local CodeLens labels. It does not contribute `SFDX:*`
commands, replace Salesforce test items, or start the Glade LSP unless
`glade.enableLsp` is true.

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

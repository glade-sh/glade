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
the source tree:

```bash
cd contrib/vscode-glade
npm install
npm run package
glade editor install vscode --vsix dist/vscode-glade-0.0.1.vsix --force
```

Open an SFDX project. The extension adds a `Glade` Activity Bar with Project,
Recommended Runs, Apex Tests, Data Environments, Local Org, and Debug And Logs
views.

Glade sits beside the Salesforce VS Code Extension Pack. It keeps separate
`glade.*` command ids, a `Glade Apex` Test Explorer controller, and CodeLens
labels that include `Local`. It does not take over `SFDX:*` commands,
Salesforce scratch-org tests, Salesforce CodeLens, Apex Replay Debugger, or the
Salesforce Apex language server.

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
glade exec --project <root> --db <active-db> --debug-log - "insert new Account(Name='local');"
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

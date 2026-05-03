# Editor Integration

`oaer` exposes editor-facing entry points through normal CLI commands. The
current baseline is useful for local Apex development, but it is still a
preview: DAP has live VM pause/step primitives, LSP uses full-project indexing
at startup with open-buffer overlays, and watch mode uses native file watching
with polling fallback.

## VS Code Tasks

Create `.vscode/tasks.json` in a Salesforce project to run the same checks that
CI and terminal workflows use.

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "oaer: check",
      "type": "shell",
      "command": "oaer",
      "args": ["check", "--project", "${workspaceFolder}"],
      "group": "build",
      "problemMatcher": []
    },
    {
      "label": "oaer: test",
      "type": "shell",
      "command": "oaer",
      "args": ["test", "--project", "${workspaceFolder}", "--json"],
      "group": "test",
      "problemMatcher": []
    },
    {
      "label": "oaer: watch tests",
      "type": "shell",
      "command": "oaer",
      "args": [
        "test",
        "--project",
        "${workspaceFolder}",
        "--watch",
        "--debounce",
        "750ms"
      ],
      "isBackground": true,
      "problemMatcher": []
    },
    {
      "label": "oaer: diagnostics once",
      "type": "shell",
      "command": "oaer",
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

`oaer exec --debug` and `oaer test --debug` speak Debug Adapter Protocol over
stdio. VS Code needs an extension or DAP client configuration that can launch a
stdio adapter and register an `oaer-apex` debug type. Use this
`.vscode/launch.json` shape as the project-side contract for that adapter.

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "oaer: debug all tests",
      "type": "oaer-apex",
      "request": "launch",
      "program": "oaer",
      "args": ["test", "--project", "${workspaceFolder}", "--debug"],
      "cwd": "${workspaceFolder}"
    },
    {
      "name": "oaer: debug one test class",
      "type": "oaer-apex",
      "request": "launch",
      "program": "oaer",
      "args": [
        "test",
        "--project",
        "${workspaceFolder}",
        "--filter",
        "${input:testClass}",
        "--debug"
      ],
      "cwd": "${workspaceFolder}"
    },
    {
      "name": "oaer: debug anonymous Apex",
      "type": "oaer-apex",
      "request": "launch",
      "program": "oaer",
      "args": ["exec", "--debug", "${input:anonymousApex}"],
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
      "default": "System.debug('hello from oaer');"
    }
  ]
}
```

The current DAP server supports initialize, breakpoints, continue, pause, next,
step-in, step-out, stack trace, scopes, variables, evaluate, watch expressions,
and disconnect. Live VM pause hooks can stop before statements at breakpoints
and step through method calls; full IDE launch orchestration is still tracked as
parity work.

## LSP Wiring

`oaer lsp --project <root>` runs an LSP server over stdio. Configure editor
clients to start that command from the workspace root and treat `*.cls` and
`*.trigger` files as Apex. The current server provides initialize/shutdown,
incremental text document sync, open-buffer parse diagnostics, project
diagnostics shaped like `oaer check`, test-result diagnostics from stack frames,
document and workspace symbols, semantic tokens, definition, references, rename,
hover, and completion for Apex symbols, schema fields, and keywords from the
project index.

```json
{
  "command": "oaer",
  "args": ["lsp", "--project", "${workspaceFolder}"],
  "filetypes": ["apex", "cls", "trigger"],
  "rootPatterns": ["oaer.yml", "sfdx-project.json"]
}
```

For a one-shot diagnostics check without starting a long-lived language client,
run:

```bash
oaer lsp --project . --diagnostics-once
```

## Watch And Reports

Watch mode emits newline-delimited JSON events for editor and test UI consumers.

```bash
oaer test --project . --watch --debounce 750ms --watch-backend auto
```

For CI or editor tasks that need a single machine-readable run, use:

```bash
oaer test --project . --json
oaer test --project . --junit reports/oaer-junit.xml
oaer check --project . --json
```

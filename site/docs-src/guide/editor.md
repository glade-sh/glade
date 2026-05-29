# Editor, LSP, and DAP

Glade includes editor-facing surfaces for diagnostics and debug snapshots. They are built on the same parser, semantic analyzer, project loader, and runtime as the CLI.

## Language server

Start the LSP server over stdio:

```bash
glade lsp --project .
```

Run one diagnostics pass without starting a long-lived server:

```bash
glade lsp --project . --diagnostics-once
```

Use `--diagnostics-once` in editor tasks, CI checks, or smoke scripts when you want LSP-shaped diagnostics without wiring a client.

## VS Code task example

Add a task that checks the current workspace:

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
      "label": "glade: lsp diagnostics",
      "type": "shell",
      "command": "glade lsp --project . --diagnostics-once",
      "problemMatcher": []
    }
  ]
}
```

## Debug snapshots

The DAP surface exposes snapshot sessions for supported execution paths. Use it when an editor integration wants stable runtime state rather than only console output.

A common workflow is:

1. Run an Apex command or test with trace output.
2. Analyze the trace with `glade profile analyze`.
3. Use editor diagnostics and snapshots to narrow the next edit.

```bash
glade exec --project . --trace reports/trace.json "System.debug(1);"
glade profile analyze reports/trace.json --json
```

## Editor loop

- Keep `glade check --project . --json` as the fast correctness pass.
- Use `glade test --filter <name> --watch` for active test work.
- Use `glade lsp --diagnostics-once` when configuring an editor task.

The editor should not need a Salesforce org for this loop. That is the point of the tool.

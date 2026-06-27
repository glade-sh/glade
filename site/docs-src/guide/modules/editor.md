# Editor and workbench

Editor and workbench tools own the Glade VS Code extension and local language
features. They wire local checks, tests, data, debug, and plugin actions into
the editor. They do not own org-backed Salesforce editor commands.

## Use it when

Use editor and workbench tools when you want VS Code install checks, local Apex
diagnostics, Test Explorer integration, workbench views, or editor-driven local
runs.

## Entry commands

```bash
glade editor doctor vscode
glade editor install vscode --force
glade lsp --project . --diagnostics-once
```

## What this module owns

- VS Code extension install, doctor checks, Activity Bar, Status Bar, and
  workbench views.
- Local LSP diagnostics, symbols, hover, completion, rename, semantic tokens,
  Test Explorer, CodeLens, and DAP wiring.
- Editor actions that call base Glade commands or installed plugin actions.

## Requires Salesforce

Salesforce remains the validation gate for hosted platform behavior.

Use org-backed Salesforce tools for deploy, retrieve, org tests, SOQL Builder,
Code Analyzer, replay debugging, and org language-server ownership.

## Related workflows

- [Editor guide](/guide/editor)
- [Debug Apex workflow](/guide/workflows/debug-apex)

## Reference

- [Workbench](/guide/workbench)

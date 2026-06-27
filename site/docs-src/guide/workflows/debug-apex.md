# Debug Apex

Use Glade for local breakpoints and profile reports. Use Salesforce logs when
the behavior depends on hosted services.

## Before you start

Run from the project root. Create a `reports` directory if you plan to save log
or profile output. Open VS Code when you want breakpoints through the debug
adapter.

## Steps

Start the debug adapter:

```bash
glade dap --project .
```

Profile an anonymous Apex log as Markdown:

```bash
glade debug profile --log reports/anonymous-output.txt --format markdown
```

Write the same profile as JSON:

```bash
glade debug profile --log reports/anonymous-output.txt --json
```

Convert the log to pprof:

```bash
glade profile analyze --log reports/anonymous-output.txt --format pprof
```

## Expected output

The debug adapter waits for an editor session. Profile commands print method
timing, call counts, and source locations when the log contains trace data. JSON
and pprof modes are for tools.

## Common wrong turn

Do not profile a hosted-only failure from a local trace and call it complete.
Local traces show supported local execution. Hosted integrations still need a
Salesforce log.

## Deeper reference

- [Debug and profile](/guide/modules/debug-profile)
- [Editor and workbench](/guide/modules/editor)
- [Debug with breakpoints](/help/debug-apex-vscode)
- [Anonymous Apex scratch](/help/anonymous-apex-scratch)

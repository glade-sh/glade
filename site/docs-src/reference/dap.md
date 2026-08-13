# DAP reference

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Reference</p>
  <p>Start the Glade Debug Adapter Protocol process and understand the supported local breakpoint model.</p>
</div>

Verified against the stable Glade release shown in the site header.

## Invocation

Start the debug adapter over stdio for a project and optional local database:

```bash
glade dap --project .
glade dap --project . --db .glade/envs/dev.sqlite
```

The VS Code extension launches the same adapter for test, CodeLens, and
anonymous Apex debug actions.

## Breakpoint and state model

- Breakpoints come from the normal VS Code Apex editor gutter.
- Supported local test and anonymous Apex paths can stop at source breakpoints.
- Stack frames, variables, and debug-console evaluation use local runtime state.
- The active data environment supplies local SObjects and DML state.

## Limits

Glade does not replace Salesforce replay debugging, org logs, hosted service
engines, or exact production governor accounting. A breakpoint in an unsupported
hosted path requires Salesforce.

## Troubleshooting

First run `glade doctor --project .`. Then confirm the selected test or snippet
runs locally without the debugger and that the source path belongs to a configured
package directory. See [Debug Apex](/guide/workflows/debug-apex) for the task
workflow.

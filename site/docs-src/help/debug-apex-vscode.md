---
pageType: guide
canonicalTask: /guide/workflows/debug-apex
---

# Debug Apex in VS Code with breakpoints

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Task guide</p>
  <p>Set a normal Apex gutter breakpoint and debug a local test through Glade DAP.</p>
  <ul>
    <li>Set a breakpoint in a path the selected test executes.</li>
    <li>Start `Debug Local Test`.</li>
    <li>Open Run and Debug while the session is active.</li>
  </ul>
</div>

For the main task path, use [the guide](/guide/workflows/debug-apex). This walkthrough keeps the
illustrated steps and recovery details for this interface.

## Before you start

- The [bundled Glade extension](/guide/editor) is installed in VS Code. Keep your existing theme and profile.
- A local data environment is selected.

The screenshots use a clean VS Code profile with Glade, Catppuccin Mocha,
and the Salesforce Apex extension, with unrelated extensions omitted for
capture. That records the illustrated environment; your normal profile and
theme can be used for this task.

## Steps

### 1. Set a breakpoint

Open an Apex class or test file. Click the editor gutter beside the line you want to stop on.

![VS Code showing an Apex gutter breakpoint](/help/screenshots/debug-apex-vscode-01-breakpoint.png)

### 2. Start the local debug session

Click `Debug Local Test` from CodeLens or use the Glade Debug view action.

Expected: VS Code opens a normal debug toolbar and Glade starts a DAP session.

![VS Code showing the Glade debug toolbar](/help/screenshots/debug-apex-vscode-02-debug-toolbar.png)

### 3. Inspect the debug panes and output

Use Step Over, Variables, Watch, and Call Stack first. Open Debug Console or the Glade output channel only when you need logs.

![VS Code showing Run and Debug during local Apex debug](/help/screenshots/debug-apex-vscode-03-variables.png)

## Common wrong turn

If debugging starts but does not stop, check that the breakpoint is in a supported `.cls` or `.trigger` file and that the test path executes that line.

## Next

- [Use anonymous Apex scratch in VS Code](/help/anonymous-apex-scratch)
- [VS Code Extension, LSP, and DAP](/guide/editor)

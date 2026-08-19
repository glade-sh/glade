# Use Anonymous Apex Scratch in VS Code

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Task guide</p>
  <p>Open a scratch Apex file, run it locally, and read the generated debug log.</p>
  <ul>
    <li>Open a small scratch editor.</li>
    <li>Run the whole buffer.</li>
    <li>Use the active local DB.</li>
  </ul>
</div>

## Before you start

- VS Code is running with a clean VS Code profile.
- Only Glade, Catppuccin Mocha, and the Salesforce Apex extension are installed.
- The active Glade local data environment is the one you want to write to.

## Steps

### 1. Open the scratch buffer

Open an anonymous Apex scratch buffer or a small `.apex` scratch file.

![VS Code showing an anonymous Apex scratch buffer](/help/screenshots/anonymous-apex-scratch-01-buffer.png)

### 2. Run or debug the Apex

Use `Cmd+Enter` on macOS or run `glade exec --debug-log - --project . <apex>`. Select a smaller block when you want to run only part of the buffer.

Expected: Glade runs local anonymous Apex against the active DB and prints a Salesforce-style debug log.

![VS Code showing an anonymous Apex debug log](/help/screenshots/anonymous-apex-scratch-02-run.png)

## Common wrong turn

If the command says no Salesforce DX project is open, open the project root folder, not a single `.cls` file.

## Next

- [Work with local data environments](/help/local-data-environments)
- [VS Code Extension, LSP, and DAP](/guide/editor)

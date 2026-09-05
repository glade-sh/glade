---
pageType: guide
canonicalTask: /guide/workflows/apex-tests
---

# Run one Apex test locally

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Task guide</p>
  <p>Run one local Apex test from the terminal, CodeLens, and VS Code Test Explorer debug flow.</p>
  <ul>
    <li>Run one class from the CLI.</li>
    <li>Run the same test from the editor.</li>
    <li>Inspect local state when a selected test stops.</li>
  </ul>
</div>

For the main task path, use [the guide](/guide/workflows/apex-tests). This walkthrough keeps the
illustrated steps and recovery details for this interface.

## Before you start

- `glade doctor --project .` passes.
- The project has at least one Apex test class.
- The [bundled Glade extension](/guide/editor) is installed and the project is open in VS Code. Keep your existing theme and profile.

The screenshots use a clean VS Code profile with Glade, Catppuccin Mocha,
and the Salesforce Apex extension, with unrelated extensions omitted for
capture. That records the illustrated environment; your normal profile and
theme can be used for this task.

## Steps

### 1. Run the test class in a terminal

```bash
glade test --project . --class <TestClass> --no-progress
```

Expected: the named class contains at least one executed test method. Read
selected and executed counts; an empty selection does not confirm that tests
run. Unsupported outcomes are test errors, not passing results.

![Terminal showing one local Apex test run](/help/screenshots/run-one-apex-test-01-cli.png)

### 2. Run the test from CodeLens

Open a test class file. Click `Run Local Test` above the method or class.

![VS Code showing Glade local test CodeLens](/help/screenshots/run-one-apex-test-02-codelens.png)

### 3. Debug from Test Explorer

Open the Apex code path you want to inspect. Set a breakpoint on the line you want to inspect before starting the debug action. Then open VS Code Testing, expand the test class, select the method, and start the debug action. VS Code switches to Run and Debug when the local test stops.

Expected: Variables shows local values at the stopped breakpoint.

![VS Code Run and Debug showing variables from a local Apex test](/help/screenshots/run-one-apex-test-03-test-explorer.png)

## Common wrong turn

If the test tree is empty, confirm the folder contains `sfdx-project.json` and run `Glade: Refresh`.

## Next

- [Debug Apex in VS Code with breakpoints](/help/debug-apex-vscode)
- [Run Apex Tests Locally](/guide/local-testing)

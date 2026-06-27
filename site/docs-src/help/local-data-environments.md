# Work With Local Data Environments

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Create, switch, seed, inspect, reset, and export local SQLite-backed Glade environments.</p>
  <ul>
    <li>Use a named environment for the local DB.</li>
    <li>Clone an environment for a branch.</li>
    <li>Seed and inspect local data.</li>
  </ul>
</div>

## Before you start

- Glade is initialized in an SFDX project.
- VS Code uses only Glade, Catppuccin Mocha, and the Salesforce Apex extension.
- These environments are local SQLite files. They do not copy Salesforce org data unless you import or seed data yourself.

## Steps

### 1. Inspect the active environment in VS Code

Open the Glade side view. Use Data Environments and Local Org to see the active DB.

![VS Code showing Glade local data environments](/help/screenshots/local-data-environments-01-sidebar.png)

### 2. Seed and inspect from a terminal

```bash
mkdir -p .glade/envs
glade db seed --db .glade/envs/<environment>.sqlite --project . <seed-file>
glade db inspect --db .glade/envs/<environment>.sqlite --project .
```

Expected: Glade reports local rows in the SQLite-backed environment.

![Terminal showing local data seed and inspect output](/help/screenshots/local-data-environments-02-terminal.png)

## Common wrong turn

`Glade: Clone Local Data Environment` copies local SQLite state. It does not contact Salesforce or refresh data from an org.

## Next

- [Run changed tests before a PR](/help/changed-tests-before-pr)
- [VS Code Extension, LSP, and DAP](/guide/editor)

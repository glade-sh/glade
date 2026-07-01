# Work With Local Data Environments

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Guided help</p>
  <p>Create, switch, seed, inspect, reset, and export local SQLite-backed Glade environments.</p>
  <ul>
    <li>Use a named environment for the local DB.</li>
    <li>Open a browser record manager for create, edit, delete, and undelete.</li>
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
glade doctor --project .
glade db seed --project . <seed-file>
glade db inspect --project .
```

Expected: `doctor` shows the project local data environment, and `db inspect` reports local rows in `.glade/envs/dev.sqlite`.

Use a named environment when you want a separate project DB:

```bash
glade db seed --project . --env qa <seed-file>
glade db inspect --project . --env qa
```

![Terminal showing local data seed and inspect output](/help/screenshots/local-data-environments-02-terminal.png)

### 3. Open the browser record manager

```bash
glade db ui --project .
```

Expected: Glade opens `/db` for `.glade/envs/dev.sqlite`. Use the object rail
to pick an object, then use Create, Save, Delete, and Undelete from the record
table and drawer.

Use a named project DB when you need a separate data set:

```bash
glade db ui --project . --env qa
```

For automation or smoke tests, avoid opening a browser and wait on a ready file:

```bash
glade db ui --project . --addr 127.0.0.1:0 --no-open --ready-file /tmp/glade-db-ui-ready.json
```

## Common wrong turn

Do not reuse a SQLite file from another project. Glade binds local DBs to the
project schema and stops early when the object or field shape does not match.

`Glade: Clone Local Data Environment` copies local SQLite state. It does not contact Salesforce or refresh data from an org.

## Next

- [Run changed tests before a PR](/help/changed-tests-before-pr)
- [VS Code Extension, LSP, and DAP](/guide/editor)

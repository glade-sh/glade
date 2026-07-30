# Run your first local Apex check

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Start</p>
  <p>Use this path when you want the shortest install, project setup, doctor, check, and test loop for a Salesforce DX project.</p>
  <ul>
    <li>Install the binary.</li>
    <li>Initialize local project config.</li>
    <li>Run one check and the project's discovered local tests.</li>
  </ul>
</div>

This path installs Glade, checks the project, and runs its discovered local tests.
For VS Code, CI, and report workflows, use the
[Tester field guide](/guide/tester-field-guide).

## 1. Install

```bash
curl -fsSL https://glade.sh/install.sh | sh
glade version
```

If `glade` is not found, add `~/.local/bin` to `PATH` and restart your shell.

Expected: `glade version` prints the installed version.

## 2. Open a Salesforce DX project

```bash
cd path/to/salesforce-dx-project
glade init --project . --yes
glade config validate --project .
```

Expected: `glade.yml` exists, and config validation exits with code `0`.

## 3. Check the local environment

```bash
glade doctor
```

Expected:

```text
Glade doctor

Project      ✓ SFDX project found
Parser       ✓ ok (tree-sitter)
Toolchain    ✓ <glade data dir> (ok (global))
Config       ✓ glade.yml
Runtime      ✓ glade <version> · go<version> · <os>/<arch>

Ready.

Next:
  glade check
  glade test changed --since origin/main
  glade playground --examples --open
```

## 4. Check source

```bash
glade check --project .
```

Expected:

- zero diagnostics and exit code `0`
- or one or more file/line diagnostics and exit code `1`

## 5. Run local tests

```bash
glade test --project .
```

Expected: a selected/passed/failed summary, plus file and method details for any failure.

After the first run, narrow a command to a class that exists in your project:

```bash
glade test --project . --class <YourTestClass>
```

## 6. Run affected tests

```bash
glade test changed --project . --since origin/main
```

Expected: Glade maps changed Apex and metadata to the smallest local test set it can prove.

## 7. Open the playground

```bash
glade playground --examples --open
```

Expected: Glade starts the browser workbench and prints the local URL, example mode, database mode, and stop command.

## 8. Know the limits

Glade is a local runtime, not a Salesforce emulator. Check [what Glade runs locally](/guide/support-map)
before relying on platform service APIs, live auth, exact hosted Visualforce
behavior, or REST and Tooling APIs outside the checked local baseline.

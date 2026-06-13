# Quickstart: Check and Test an SFDX Project

<div class="docs-intro">
  <p class="docs-intro-eyebrow">Start</p>
  <p>Use this path when you want the shortest install, doctor, check, and test loop for an SFDX project.</p>
  <ul>
    <li>Install the binary.</li>
    <li>Initialize local project config.</li>
    <li>Run one check and one focused test.</li>
  </ul>
</div>

This path gets from install to the first local check in a few minutes.
For a small pilot with VS Code, AI, CI, and report workflows, use the
[Tester Field Guide](/guide/tester-field-guide).

## 1. Install

```bash
curl -fsSL https://glade.sh/install.sh | sh
glade version
glade doctor
```

If `glade` is not found, add `~/.local/bin` to `PATH` and restart your shell.

Expected:

```text
parser: ok (tree-sitter)
```

## 2. Open an SFDX Project

```bash
cd path/to/sfdx-project
glade init --project . --yes
glade config validate --project .
```

Expected: `glade.yml` exists, and config validation exits with code `0`.

## 3. Check Source

```bash
glade check --project .
```

Expected:

- zero diagnostics and exit code `0`
- or one or more file/line diagnostics and exit code `1`

## 4. Run One Test

```bash
glade test --project . --class AccountServiceTest
```

Expected: a selected/passed/failed summary, plus file and method details for any failure.

## 5. Run Only Affected Tests

```bash
glade test changed --project . --since origin/main
```

Expected: Glade maps changed Apex and metadata to the smallest local test set it can prove.

## 6. Know The Limits

Glade is not a full Salesforce emulator. Check the [Support map](/guide/support-map)
before relying on platform service APIs, live auth, Visualforce rendering, or
full REST/Tooling API parity.

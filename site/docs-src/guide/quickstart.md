# Quickstart: Check and Test an SFDX Project

This path gets from install to the first local check in a few minutes.

## 1. Install

```bash
curl -fsSL https://glade.sh/install.sh | sh
glade version
glade doctor
```

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

## 3. Check Source

```bash
glade check --project .
```

## 4. Run One Test

```bash
glade test --project . --filter AccountServiceTest
```

## 5. Run Only Affected Tests

```bash
glade test changed --project . --since origin/main
```

## 6. Know The Limits

Glade is not a full Salesforce emulator. Check [What Glade supports](/guide/support-map)
before relying on platform service APIs, live auth, Visualforce rendering, or
full REST/Tooling API parity.

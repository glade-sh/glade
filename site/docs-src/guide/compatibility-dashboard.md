# Compatibility Dashboard

This page is the docs-site summary of the generated compatibility dashboard.
Start with the [Support Map](/guide/support-map) for a broad area view. The
checked-in source report lives at `docs/COMPATIBILITY_DASHBOARD.md` in the
repository and is generated from `internal/capability`.

## Current gate commands

Run the MVP gate:

```bash
glade compat mvp
```

Require readiness in CI or release promotion:

```bash
glade compat mvp --require-ready
```

Regenerate repository reports after capability changes:

```bash
glade compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
glade compat gaps --output docs/KNOWN_GAPS.md
glade compat stdlib --output docs/STDLIB_COVERAGE.md
```

## How to read the matrix

Capabilities are grouped by parser, semantic analysis, VM execution, tests, storage, server, stdlib, tooling, and local-test behavior. Each row carries a status and notes about coverage or known gaps.

- `supported` means covered behavior.
- `partial` means usable behavior with explicit gaps.
- `stub` means a controlled placeholder exists.
- `unsupported` means Glade should report a stable diagnostic.
- `unknown` means the team has not measured it yet.

## Release use

Preview releases can ship before the MVP gate is green. MVP-ready releases require all required capabilities to report `supported` and the generated docs to stay in sync with the matrix.

The dashboard is a measuring stick, not a sales sign. It says what the tool does today.

# AI Contributor Guide

## Voice

Write plainly. Keep observations concrete. Name the file, command, and result.
Use short sentences. Let the facts carry the weight.

## Project Boundary

This repository is the deliverable `glade` framework and tool.

Keep product work here:

- Apex parsing, indexing, and semantic checks.
- The VM, test runner, SOQL, DML, storage, schema, and server runtime.
- Product CLI commands: `version`, `doctor`, `completion`, `config`, `init`,
  `parse`, `inspect`, `schema`, `check`, `exec`, `debug`, `editor`, `dap`,
  `test`, `dev`, `report`, `lsp`, `profile`, `package`, `server`,
  `playground`, and `db`.
- Product docs, release scripts, install docs, editor docs, storage docs, and
  checked support reports.

Keep maintenance work in the sibling `~/Dev/glade-tools` project:

- Compatibility fixtures and fixture runners.
- Capability catalogs, dashboards, stdlib ledgers, and known-gap generators.
- Surface ledger refresh, packet, and post-parity scanners.
- Example-project and large-corpus readiness scans.
- Salesforce docs inventory, catalog reconcile, stub reports, and generated
  maintenance artifacts.

`glade-tools` may depend on this repository. This repository must not depend on
`glade-tools`.

## Operating Principles

Make the smallest maintainable change that solves the request.
Prefer existing patterns over new abstractions.
Avoid broad refactors unless the task requires them.
Ground runtime behavior in public Salesforce behavior, public grammars, owned
fixtures, or black-box tests.
Never add project-specific exceptions to product code.
Never infer field behavior from field names.

## Validation

Match validation to risk.

Use focused tests first:

```bash
go test ./internal/<package>
go test ./internal/gladecli
go test ./internal/repoguard
```

Use broader checks when product surfaces move:

```bash
go test ./...
scripts/smoke.sh
```

For generated support reports, use `glade-tools`. Do not reintroduce a
maintenance command under `glade`.

## Release Notes

Do not check in built binaries, `.DS_Store`, `/bin/`, `/dist/`, `coverage.out`,
compiled Go test binaries, or ad-hoc run dumps.

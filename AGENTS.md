# AI Contributor Guide

This is the shared repository guide for coding agents, independent of provider
or model. For Codex setup and the full validation map, read
[docs/AI_SETUP.md](docs/AI_SETUP.md). Read linked references when the task needs
them; start with [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for runtime work and
[docs/README.md](docs/README.md) for other documentation.

## Voice

Write plainly. Keep observations concrete. Name the file, command, and result.
Use short sentences. Let the facts carry the weight.

## Working Agreement

- Inspect the current branch, commit, and `git status --short` before editing.
  Preserve unrelated changes and worktrees. Stage only files owned by this task.
- Carry authorized work through implementation, relevant checks, and review.
  Resolve routine choices from existing patterns. Ask when missing information
  materially changes scope, correctness, or an external action's authorization.
- Parallelize independent investigation and review when agents are available.
  Give each writer explicit file ownership; use isolated worktrees when edits
  would overlap. Coordinate expensive test runs instead of stacking broad Go
  suites on the same host. Keep shared evidence and campaign state single-writer.
- Match process to the change. Use a short plan for substantial work; ordinary
  edits do not need a separate design document or an approval checkpoint.

## Project Boundary

This repository is the deliverable `glade` framework and tool.

Keep product work here:

- Apex parsing, indexing, and semantic checks.
- The VM, test runner, SOQL, DML, storage, schema, and server runtime.
- Product CLI commands: `version`, `update`, `doctor`, `toolchain`, `config`,
  `init`, `parse`, `inspect`, `schema`, `refactor`, `check`, `exec`, `debug`,
  `editor`, `dap`, `test`, `tui`, `dev`, `report`, `lsp`, `profile`,
  `examples`, `explain`, `support`, `plugins`, `package`, `server`, `org`,
  `playground`, `db`, `completion`, and `help`.
- Product docs, release scripts, install docs, editor docs, storage docs, and
  checked support reports.

Keep maintenance work in first-party plugins sourced from the sibling
`~/Dev/glade-tools` project:

- Compatibility fixtures and fixture runners.
- Capability catalogs, dashboards, stdlib ledgers, and known-gap generators.
- Surface ledger refresh, packet, and post-parity scanners.
- Example-project and large-corpus readiness scans.
- Salesforce docs inventory, catalog reconcile, stub reports, and generated
  maintenance artifacts.

The compat and performance plugins may depend on this repository. This
repository must not depend on `glade-tools` or plugin internals.

## Operating Principles

Make the smallest maintainable change that solves the request.
Prefer existing patterns over new abstractions.
Avoid broad refactors unless the task requires them.
Ground runtime behavior in public Salesforce behavior, public grammars, owned
fixtures, or black-box tests.
Never add project-specific exceptions to product code.
Never infer field behavior from field names.

Trace callers and the shared runtime path before fixing behavior. Reuse
`storage`, `dml`, `soql`, `vm`, `apextest`, `server`, and `testreport` rather than
adding a parallel implementation for one CLI or server path.

Read [docs/CLEAN_ROOM.md](docs/CLEAN_ROOM.md) for compatibility changes. Public
API shape, compilation, local execution, and live Salesforce parity are distinct
evidence. State what was actually checked, including the candidate and API
version when relevant. Use only an explicitly authorized org for live probes.

Read generated-file headers before editing. Change the authoritative input or
generator and regenerate the output. For schema sources, see
[docs/STANDARD_OBJECT_SCHEMA.md](docs/STANDARD_OBJECT_SCHEMA.md).

## Validation

Match validation to risk.

Use the Go version in `go.mod` and a C compiler. Real Apex declaration parsing
requires `CGO_ENABLED=1`; a successful no-CGO build is not a working parser.

Use focused tests first:

```bash
CGO_ENABLED=1 go test ./internal/<package>
CGO_ENABLED=1 go test ./internal/gladecli ./internal/repoguard
```

For behavior changes, add or update the smallest meaningful regression test.
For a bug, reproduce the failure before fixing it. For a behavior-preserving
refactor, establish a passing baseline and rerun it. Documentation-only changes
use the relevant documentation checks.

Root Go tests exclude the nested parser module and Node/browser suites. Use the
[validation map](docs/AI_SETUP.md#validation-by-surface) for the changed surface.
For a local binary and runtime smoke:

```bash
CGO_ENABLED=1 go build -o ./bin/glade ./cmd/glade
scripts/smoke-runtime.sh ./bin/glade
```

For broad Go validation use `CGO_ENABLED=1 scripts/ci-go-test.sh local-release`.
For releases, follow [docs/DISTRIBUTION_WORKFLOW.md](docs/DISTRIBUTION_WORKFLOW.md) and
[docs/RELEASE_POLICY.md](docs/RELEASE_POLICY.md). `scripts/release-check.sh`
combines site, Go, and distribution checks; `scripts/smoke.sh` builds and tests
a distribution and is not a quick runtime smoke.

For generated support reports, use the first-party compat plugin or its
`glade-tools` source wrapper. Do not reintroduce a maintenance command under
base `glade`.

Before claiming completion, review the diff and report commands and outcomes.
Name skipped tests, empty selections, environmental limits, and remaining
uncertainty. A local green gate does not establish corpus closure, live parity,
or release readiness.

## Release Notes

Do not check in built binaries, `.DS_Store`, `/bin/`, `/dist/`, `coverage.out`,
compiled Go test binaries, or ad-hoc run dumps.

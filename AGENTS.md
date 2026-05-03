# AI Contributor Guide

This repo is `oaer`, a clean-room, open source local Apex runtime written in Go.
It parses, type-checks, and executes Salesforce Apex code locally, backed by an
in-memory data runtime and an optional SQLite persistence layer. Keep work tied
to public Salesforce behavior, public grammars, owned fixtures, and black-box
compatibility tests. Do not use proprietary AER internals as an implementation
source.

## Project Overview

`oaer` is a single-binary CLI tool and library that provides:

- **Apex front end**: parsing, symbol indexing, and semantic analysis for Apex
  classes, triggers, and SOQL.
- **Runtime**: an interpreter (VM) for the supported Apex subset, including
  anonymous execution, test discovery and execution, DML, SOQL, triggers,
  governor limits, and common platform APIs.
- **Tooling surfaces**: LSP server, DAP debug snapshots, file watcher with
  affected-test selection, native trace/profile analysis, and a
  Salesforce-shaped local HTTP API server.
- **Compatibility tracking**: a machine-readable capability matrix with an MVP
  readiness gate, JSON fixtures, and generated documentation.

The project is organized as narrow, separately testable `internal/` packages
composed by the CLI in `internal/oaercli`.

## Technology Stack

- **Language**: Go 1.26
- **Module**: `github.com/open-aer/oaer`
- **Key dependencies**:
  - `github.com/antlr4-go/antlr/v4` — ANTLR runtime for the Apex parser.
  - `github.com/octoberswimmer/apexfmt` — public Apex grammar and parser
    (wrapped behind `internal/apexast`).
  - `modernc.org/sqlite` — pure-Go SQLite for persistent org storage.
- **Configuration**: `oaer.yml` (minimal YAML-subset parser in
  `internal/config`; only scalar and inline-list values are supported).
- **Project discovery**: `sfdx-project.json` for SFDX package directory layout.

## Code Organization

| Package | Responsibility |
| --- | --- |
| `cmd/oaer` | Executable entry point. |
| `internal/oaercli` | Command routing, flags, and user-facing CLI behavior. |
| `internal/apexast` | Parser adapter and stable source model over `apexfmt`/ANTLR. |
| `internal/config` | `oaer.yml` discovery and parsing. |
| `internal/diagnostic` | Shared diagnostic model for parser, semantic analysis, runtime, and CLI. |
| `internal/project` | SFDX package directory discovery and source file collection. |
| `internal/schema` | Metadata API custom object, field, picklist, and record type model. |
| `internal/typesys` | First symbol index for declarations, members, triggers, and schema objects. |
| `internal/sema` | Semantic analysis, type-checking, and stable diagnostics. |
| `internal/ir` | Compact executable representation for VM-supported Apex. |
| `internal/vm` | Interpreter for the supported Apex subset. |
| `internal/apextest` | Apex test discovery, compilation, and execution. |
| `internal/testreport` | Console, JSON, and JUnit test reporting. |
| `internal/sobject` | Runtime SObject value and schema describe helpers. |
| `internal/storage` | Org/object/record model, fixtures, deterministic IDs, SQLite persistence, and schema migrations. |
| `internal/soql` | In-memory SOQL parser and executor. |
| `internal/dml` | DML pipeline, validation, rollback snapshots, and trigger hooks. |
| `internal/dap` | Debug Adapter Protocol framing and snapshot sessions. |
| `internal/lsp` | stdio LSP/JSON-RPC server. |
| `internal/watch` | File classification, snapshot diffing, polling watch loop, and debounce. |
| `internal/profile` | Native trace/profile aggregation and JSON/Markdown reporting. |
| `internal/server` | Salesforce-shaped HTTP handler. |
| `internal/compat` | Compatibility fixture schema and parse/check/exec/test/DB execution. |
| `internal/capability` | Machine-readable feature matrix and MVP readiness gate. |

### Runtime Pipeline

1. Load project configuration and Salesforce metadata.
2. Parse Apex source through `internal/apexast`.
3. Build symbols and resolve references through `internal/typesys`.
4. Type-check through `internal/sema`.
5. Lower checked code into `internal/ir`.
6. Execute with `internal/vm`, routing SObject, SOQL, DML, trigger, limit, and
   platform calls into dedicated packages.
7. Surface the same runtime through CLI execution, tests, watch mode, LSP/DAP
   snapshots, profile analysis, compatibility checks, and the local API server.
8. Record diagnostics, traces, profiles, test reports, storage fixtures, server
   responses, and compatibility results in stable machine-readable formats.

## Build and Run

Build the CLI:

```bash
go build -o oaer ./cmd/oaer
```

Run locally:

```bash
./oaer version
./oaer doctor
./oaer parse <paths...> [--json]
./oaer inspect symbols [--project <root>] [--json]
./oaer schema load [--project <root>] [--json]
./oaer check [--project <root>] [--json]
./oaer exec [--json] [--trace <path>] [--limit-mode strict|permissive] '<anonymous apex>'
./oaer test [--project <root>] [--filter <pattern>] [--json|--junit <path>] [--limit-mode <mode>] [--watch|--watch-once] [--debug]
./oaer lsp [--project <root>] [--diagnostics-once]
./oaer profile analyze <trace.json> [--json]
./oaer server [--addr <host:port>] [--db <path>] [--project <root>]
./oaer db seed|reset|export|inspect --db <path> [--project <root>] [--json] [fixture.json]
./oaer compat mvp|matrix|dashboard|gaps|validate|run ...
```

## Testing

Run all tests:

```bash
go test ./...
```

Run with the race detector:

```bash
go test -race ./...
```

Run benchmarks for performance-sensitive areas:

```bash
go test -run '^$' -bench . ./internal/apexast ./internal/typesys ./internal/sema ./internal/soql ./internal/dml ./internal/vm ./internal/apextest ./internal/storage ./internal/server ./internal/lsp ./internal/watch
```

### Testing Conventions

- Every package has unit tests in `*_test.go` files.
- **Hardening tests**: many packages include `hardening_test.go` files that
  verify the package never panics on malformed user input, unsupported
  operations, or null dereferences. When modifying a package, ensure its
  hardening tests still pass.
- Tests that need a project on disk create a temporary directory with
  `t.TempDir()`, write a minimal `sfdx-project.json`, and add Apex classes or
  triggers as needed.
- The `internal/apextest` runner is often exercised through full integration
  tests that write real Apex sources to disk, build an index, and run tests
  against the VM.

### Compatibility and Smoke Tests

```bash
go run ./cmd/oaer compat mvp --json
go run ./cmd/oaer compat dashboard --check docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/oaer compat gaps --check docs/KNOWN_GAPS.md
scripts/smoke.sh
```

`scripts/smoke.sh` builds the binary and exercises the full surface: parse,
check, exec, profile, test, db seed/inspect, LSP diagnostics-once, server
startup, and compat commands.

## Code Style Guidelines

- Follow standard Go formatting (`gofmt`). CI runs `go vet ./...`.
- Prefer explicit error returns over panics. **Panics on user Apex, metadata,
  fixtures, or API requests are treated as bugs.**
- Keep source ranges stable from parse through sema, VM, tests, DAP, LSP, trace,
  and profile output.
- Prefer explicit unsupported-feature diagnostics over silent fallbacks.
- Attach source ranges early and preserve them through diagnostics and runtime
  traces.
- Keep the parser behind an adapter (`internal/apexast`) so grammar
  dependencies can change without rippling through the codebase.
- Use the shared `internal/diagnostic` model rather than ad-hoc error strings.
- Return explicit unsupported-feature diagnostics instead of panicking.
- Keep Salesforce behavior claims tied to compatibility fixtures.

## Working Rules

- When moving a capability from `partial` to `supported`, add compatibility
  coverage first.
- Update generated docs after capability changes:

```bash
go run ./cmd/oaer compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/oaer compat gaps --output docs/KNOWN_GAPS.md
```

- Do not introduce proprietary AER internals as implementation sources.
- Do not check in `.DS_Store`, `/bin/`, `/dist/`, or `coverage.out`.

## MVP Gate and Capability System

The source of truth for feature readiness is `internal/capability`. Each
feature has a status:

- `supported` — implemented and covered by compatibility tests.
- `partial` — works for common cases with documented gaps.
- `stub` — exists so code can load, but returns a controlled placeholder or
  explicit unsupported result.
- `unsupported` — fails with a stable diagnostic before or during execution.
- `unknown` — not evaluated yet.

The MVP gate is `oaer compat mvp`. The project is not considered MVP-ready until
every required capability is `supported`. CI enforces this gate and verifies that
generated docs are in sync.

## Docs To Keep In Sync

- `docs/ARCHITECTURE.md` — current package map and runtime pipeline.
- `docs/COMPATIBILITY.md` — human-readable feature status.
- `docs/COMPATIBILITY_DASHBOARD.md` and `docs/KNOWN_GAPS.md` — generated from
  `internal/capability`.
- `docs/FEATURE_PARITY_TODO.md` — remaining parity work.
- `docs/OAER_IMPLEMENTATION_PLAN.md` and `docs/OPEN_AER_PLAN.md` — roadmap and
  historical context; keep stale package names out of these files.
- `docs/RELEASE_NOTES.md` — ongoing release log.
- `docs/RELEASE_POLICY.md` — release promotion and upgrade policy.
- `docs/EDITOR.md` — VS Code tasks, DAP launch examples, and LSP wiring.
- `docs/INSTALL.md` — installation and CI usage instructions.

## Release and Deployment

- Releases are tagged as `vMAJOR.MINOR.PATCH`.
- The `Release` GitHub Actions workflow (`.github/workflows/release.yml`) builds
  cross-platform archives for macOS (amd64/arm64), Linux (amd64/arm64), and
  Windows (amd64), plus `SHA256SUMS.txt`.
- Build script: `scripts/release-build.sh` (uses `CGO_ENABLED=0` and
  `-trimpath`).
- A release can be promoted as MVP-ready only when `oaer compat mvp --require-ready`
  exits successfully and every `requiredForMVP` capability is `supported`.
- Until the MVP gate is green, releases must be described as preview builds.

## Security Considerations

- `oaer exec` compiles and runs arbitrary Apex expressions. In multi-tenant or
  server contexts, treat user-supplied Apex as untrusted code and run it inside
  appropriate sandboxing.
- The local API server (`oaer server`) exposes a Salesforce-shaped REST surface.
  It does not implement full OAuth or authentication; do not expose it to
  untrusted networks without an authenticating reverse proxy.
- SQLite database files (`--db`) contain org state and record data. Protect them
  with standard filesystem permissions.
- The CLI and server surface must never panic on malformed user input; hardening
  tests exist to enforce this.

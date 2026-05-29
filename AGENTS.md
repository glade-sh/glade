# AI Contributor Guide

## Operating Principles

Keep it simple. Simple is better than complex.
Make the smallest maintainable change that solves the actual request.
Prefer existing patterns over new abstractions.
Avoid broad refactors, speculative helpers, and clever architecture unless clearly justified.
Use judgment. Read enough surrounding code to understand the existing pattern, then avoid unnecessary exploration. Validate based on risk.
Assume the user is a principal engineer.
Optimize for correctness, speed, judgment, and token efficiency.
Correct the user when appropriate.

## Success Criteria

Done means:

- the requested behavior is implemented
- the change is minimal and follows existing patterns (unless a large task was assigned)
- risky behavior was validated, or validation was intentionally skipped with a reason
- remaining risks are stated plainly

## Context Discipline

Protect context aggressively.

As tool output, file reads, and conversation history grow, useful signal gets diluted. Keep active context focused on the current decision.

Before opening files or running broad searches, ask:

1. What exact question am I answering?
2. Which file, symbol, route, or component is most likely relevant?
3. Can I inspect a narrower slice first?
4. Can `rg`, imports, references, or file names locate the answer?

Prefer targeted searches, focused file sections, nearby call sites, diffs, capped logs, and targeted test output.

Avoid dumping full files, full logs, unrelated directories, or broad repo exploration after the relevant code is found.

When context gets large, summarize the current task state and keep only:

- decisions
- relevant file paths
- changed behavior
- unresolved risks

## Subagents

Use subagents only when they save context, save time, or materially improve output quality.

For research, review, and exploration tasks, avoid confirmation bias. Do not pass a preferred conclusion. Ask the subagent to investigate, compare, or verify, and require evidence, tradeoffs, uncertainty, and better alternatives.

Good uses:

- repo exploration
- scoped implementation
- QA or review
- documentation/API checks
- web research
- unfamiliar code research
- copywriting/content variants

Avoid subagents for trivial work the main agent can finish faster.

When using a subagent, assign a narrow task and require:

- findings
- files inspected
- files changed, if any
- validation run, if any
- risks or uncertainty

The main agent owns final judgment and integration.

## Code Changes

Prefer direct edits using available environment tools.

Before adding helpers, maps, files, abstractions, or validation layers, ask:

1. Can this be done inline?
2. Can existing code already do this?
3. Is this solving the exact issue?
4. Is reuse or readability clearly improved?

For bugs, patch the narrow failing path first.
For small behavior changes, make the direct edit first.
Avoid unrelated cleanup.

For complex tasks:

- identify the minimal path through the codebase
- split work into small patches
- validate only the risky parts
- keep a short running summary of decisions, changed files, and remaining risks

## Validation

Match validation to risk.

Skip validation by default for low-risk changes and say so plainly.

Low-risk examples:

- copy changes
- labels
- static content
- CSS or Tailwind spacing
- small JSX structure changes
- minor refactors with no behavior change

Also validate when:

- a previous command failed
- the user asked for validation
- the change affects multiple routes, components, or packages

Prefer the cheapest useful check:

1. targeted test
2. type check affected package
3. lint affected files
4. build only when build behavior matters

Do not run a full test suite or full build unless risk justifies it or the user asks.

## Command Output

Protect context usage. **Any command with unknown or potentially large output must be byte-capped.**

Default pattern:

```bash
COMMAND 2>&1 | head -c 4000
```

For logs or recent failures:

```bash
COMMAND 2>&1 | tail -c 4000
```

Do not rely on line limits as the only cap. A single line can be huge. Avoid using only:

```bash
head -n
tail -n
sed -n '1,20p'
```

Scope before printing content:

- list files with `rg -l` before printing matches
- count matches with `rg -c` before reading them
- search specific paths instead of whole directories
- use `rg -m`, `--max-count`, `--max-filesize`, and small context when useful
- inspect file size before reading unknown generated files, logs, JSONL, or minified JSON

For commands where the exit code matters, capture output first, print a capped amount, then exit with the original status:

```bash
tmp="$(mktemp)"
COMMAND >"$tmp" 2>&1
status=$?
tail -c 5000 "$tmp"
rm -f "$tmp"
exit "$status"
```

Avoid unbounded output from:

```bash
cat path/to/file
rg -n "term" .
find .
ls -R
git diff
npm test
npm run build
select *
```

Use bounded versions instead:

```bash
rg -l "term" . | head -c 2000
rg -n -m 20 "term" src 2>&1 | head -c 2000
git diff -- path/to/file 2>&1 | head -c 6000
find . -type f 2>&1 | head -c 2000
```

If the capped output is insufficient, narrow the command. Do not repeatedly increase the cap unless the task requires more context.

## Communication

Before editing, state the approach only for non-trivial tasks.

During complex work, keep updates very short:

- what was found
- what changed
- what risk remains

After work, summarize:

- what changed
- files touched
- validation run, or why skipped
- remaining risk

Keep summaries short. Do not explain obvious edits.

Oververbosity: low

---

## Project: glade

This repo is `glade`, a clean-room, open source local Apex runtime written in Go.
It parses, type-checks, and executes Salesforce Apex code locally, backed by an
in-memory data runtime and an optional SQLite persistence layer. Keep work tied
to public Salesforce behavior, public grammars, owned fixtures, and black-box
compatibility tests. Do not use proprietary GLADE internals as an implementation
source.

### Overview

`glade` is a single-binary CLI tool and library that provides:

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
composed by the CLI in `internal/gladecli`.

### Technology Stack

- **Language**: Go 1.26
- **Module**: `github.com/glade-sh/glade`
- **Key dependencies**:
  - `github.com/glade-sh/apex-parser` — tree-sitter Apex parser module, vendored
    in-repo at `third_party/glade-apex-parser` and wrapped behind `internal/apexast`.
  - `modernc.org/sqlite` — pure-Go SQLite for persistent org storage.
- **Configuration**: `glade.yml` (minimal YAML-subset parser in
  `internal/config`; only scalar and inline-list values are supported).
- **Project discovery**: `sfdx-project.json` for SFDX package directory layout.

### Code Organization

| Package | Responsibility |
| --- | --- |
| `cmd/glade` | Executable entry point. |
| `internal/gladecli` | Command routing, flags, and user-facing CLI behavior. |
| `internal/apexast` | Parser adapter and stable source model over the local tree-sitter Apex parser module. |
| `internal/config` | `glade.yml` discovery and parsing. |
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

### Build and Run

Build the CLI:

```bash
go build -o glade ./cmd/glade
```

Run locally:

```bash
./glade version
./glade doctor
./glade parse <paths...> [--json]
./glade inspect symbols [--project <root>] [--json]
./glade schema load [--project <root>] [--json]
./glade check [--project <root>] [--json]
./glade exec [--json] [--trace <path>] [--limit-mode strict|permissive] '<anonymous apex>'
./glade test [--project <root>] [--filter <pattern>] [--json|--junit <path>] [--limit-mode <mode>] [--watch|--watch-once] [--debug]
./glade lsp [--project <root>] [--diagnostics-once]
./glade profile analyze <trace.json> [--json]
./glade server [--addr <host:port>] [--db <path>] [--project <root>]
./glade db seed|reset|export|inspect --db <path> [--project <root>] [--json] [fixture.json]
./glade compat mvp|matrix|dashboard|gaps|stdlib|validate|run ...
```

### Testing

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

#### Testing Conventions

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

#### Compatibility and Smoke Tests

```bash
go run ./cmd/glade compat mvp --json
go run ./cmd/glade compat dashboard --check docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade compat gaps --check docs/KNOWN_GAPS.md
go run ./cmd/glade compat stdlib --check docs/STDLIB_COVERAGE.md
scripts/smoke.sh
```

`scripts/smoke.sh` builds the binary and exercises the full surface: parse,
check, exec, profile, test, db seed/inspect, LSP diagnostics-once, server
startup, and compat commands.

### Code Style Guidelines

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
- Make MINIMAL changes to achieve the goal.

### Working Rules

- Current priority, as of 2026-05-06, is full local Apex test execution support:
  make `glade test` run broad Salesforce-shaped projects with org-like metadata
  resolution, test isolation, platform APIs, DML/trigger behavior, declarative
  side effects, and explicit unsupported diagnostics.
- Use `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md` for squad-sized implementation
  phases and `docs/POST_PARITY_TODO.md` for the exhaustive post-parity backlog.
- The server-example route harness is currently green. Do not add
  project-specific runtime routes or stdlib stubs for future example-project
  failures; fix the general parser, sema, VM, SOQL, DML, storage, metadata, or
  server behavior.
- The checked post-parity readiness inventory is currently green for the
  `example-projects` corpus (`filesScanned=50457 findings=0
  testBlockingFindings=0 surfaces=0`). Treat that as a scanner/readiness gate,
  not a blanket full-runtime claim. The checked local-test corpus is also green;
  `src-nmb-nutpl-develop` is the current green example-project runtime sentinel
  at `total=761 pass=761`. The remaining checked example projects still have
  measured compile-gap frontiers, so future unsupported runtime cases should
  remain explicit when they are outside the current support claim.
- Keep the parser behind `internal/apexast`. The parser module is
  `github.com/glade-sh/apex-parser`, vendored in-repo at
  `third_party/glade-apex-parser`; parser details live in `docs/APEX_PARSER.md`.
- When moving a capability from `partial` to `supported`, add compatibility
  coverage first.
- Update generated docs after capability changes:

```bash
go run ./cmd/glade compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade compat gaps --output docs/KNOWN_GAPS.md
go run ./cmd/glade compat stdlib --output docs/STDLIB_COVERAGE.md
```

- Do not introduce proprietary GLADE internals as implementation sources.
- Do not check in `.DS_Store`, `/bin/`, `/dist/`, or `coverage.out`.
- Do not check in compiled Go test binaries (`*.test`) or ad-hoc compat run
  result dumps (e.g. `nu.json`, `nutpl.json`, `nams.json`, `sf-cred.json`); these
  are regenerable artifacts and are gitignored.
- Do not stage or commit the built `glade` binary unless the user explicitly asks
  for a binary update.

### Local-Test Execution Work Order

Use this order for full local Apex test execution unless a later checked-in plan
supersedes it:

1. Add a local-test compatibility gate that reports per-test pass, fail,
   unsupported, load error, compile error, and internal error outcomes.
2. Load read-only metadata needed by tests: legacy objects, custom metadata
   records, labels, resources, endpoint metadata, permissions, layouts, tabs,
   and related presentation metadata.
3. Implement test-facing UI controller contracts without rendering UI:
   Visualforce page metadata, `Page.*`, `PageReference`, `ApexPages`, standard
   controllers, extensions, Aura Apex discovery, and LWC Apex imports.
4. Implement test-visible platform APIs: `System.Callable`,
   `System.StubProvider`, `Test.createStub`, Site/Network context, `Auth.*`,
   `ConnectApi.Organization.getSettings`, Platform Cache basics, and endpoint
   resolution.
5. Add files, email, and captured side effects with transaction rollback.
6. Add Workflow and Flow side effects inside the DML/test transaction.
7. Add corpus baselines and release gates for `legacy-project-test-ready` and
   `declarative-automation-test-ready`.

### MVP Gate and Capability System

The source of truth for feature readiness is `internal/capability`. Each
feature has a status:

- `supported` — implemented and covered by compatibility tests.
- `partial` — works for common cases with documented gaps.
- `stub` — exists so code can load, but returns a controlled placeholder or
  explicit unsupported result.
- `unsupported` — fails with a stable diagnostic before or during execution.
- `unknown` — not evaluated yet.

The MVP gate is `glade compat mvp`. The project is not considered MVP-ready until
every required capability is `supported`. CI enforces this gate and verifies that
generated docs are in sync.

### Docs To Keep In Sync

- `docs/ARCHITECTURE.md` — current package map and runtime pipeline.
- `docs/COMPATIBILITY.md` — human-readable feature status.
- `docs/COMPATIBILITY_DASHBOARD.md`, `docs/KNOWN_GAPS.md`, and
  `docs/STDLIB_COVERAGE.md` — generated from `internal/capability`.
- `docs/FEATURE_PARITY_TODO.md` — remaining parity work.
- `docs/POST_PARITY_TODO.md` — exhaustive backlog for large-project local test
  execution beyond MVP parity.
- `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md` — squad-oriented implementation
  phases for full local Apex test execution.
- `docs/APEX_PARSER.md` — Apex parser module, CGO requirement, and validation.
- `docs/RELEASE_NOTES.md` — ongoing release log.
- `docs/RELEASE_POLICY.md` — release promotion and upgrade policy.
- `docs/EDITOR.md` — VS Code tasks, DAP launch examples, and LSP wiring.
- `docs/INSTALL.md` — installation and CI usage instructions.
- `docs/storage-schema.md` — storage model plus fixture, SQLite, and persistent
  server lifecycle notes.

### Release and Deployment

- Releases are tagged as `vMAJOR.MINOR.PATCH`.
- The `Release` GitHub Actions workflow (`.github/workflows/release.yml`) builds
  cross-platform archives for macOS (amd64/arm64), Linux (amd64/arm64), and
  Windows (amd64), plus `SHA256SUMS.txt`.
- Build script: `scripts/release-build.sh` (uses `CGO_ENABLED=0` and
  `-trimpath`).
- A release can be promoted as MVP-ready only when `glade compat mvp --require-ready`
  exits successfully and every `requiredForMVP` capability is `supported`.
- Until the MVP gate is green, releases must be described as preview builds.

### Security Considerations

- `glade exec` compiles and runs arbitrary Apex expressions. In multi-tenant or
  server contexts, treat user-supplied Apex as untrusted code and run it inside
  appropriate sandboxing.
- The local API server (`glade server`) exposes a Salesforce-shaped REST surface.
  It does not implement full OAuth or authentication; do not expose it to
  untrusted networks without an authenticating reverse proxy.
- SQLite database files (`--db`) contain org state and record data. Protect them
  with standard filesystem permissions.
- The CLI and server surface must never panic on malformed user input; hardening
  tests exist to enforce this.

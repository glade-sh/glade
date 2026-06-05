# Architecture Standards

This guide records the standards for reducing god files and keeping future
changes small enough to review. It follows Go's own project-layout guidance,
Effective Go, Go code review comments, and the Go package-name guidance.

## Package Boundaries

- Keep implementation packages under `internal/` unless the API is intended for
  external users.
- Split a large file inside its current package before creating a new package.
  A same-package split keeps APIs stable and lowers refactor risk.
- Create a new package only when a subsystem has its own data model, tests, and
  callers that do not need the parent package's private state.
- Name packages with short lower-case nouns. Do not use `util`, `common`,
  `helpers`, `types`, or `interfaces`.
- Keep package dependencies acyclic. If two packages need each other's private
  details, they are not ready to split.

## File Boundaries

- A file should have one clear job. Prefer `dispatch.go`, `dml_runtime.go`, and
  `platform_signatures.go` over broad names like `runtime.go`.
- Keep types close to the code that owns their invariants.
- Keep generated files separate from handwritten files.
- Move code mechanically first. Change behavior only in a separate patch with
  tests that prove the behavior.
- When splitting a file, preserve declaration order within each moved group
  unless a small reorder removes a compile cycle or duplicate helper.

## Interfaces And APIs

- Define interfaces at the consumer, not beside the implementation.
- Do not add an interface for a single concrete type unless it makes a test,
  boundary, or package split simpler now.
- Keep exported APIs narrow. Prefer passing concrete project models already used
  by the package.
- Every exported type, function, method, const, or var needs a doc comment.
- Prefer explicit unsupported diagnostics or errors over silent fallbacks.

## Runtime And Compatibility

- Keep Salesforce behavior claims tied to compatibility fixtures, owned tests,
  or public Salesforce documentation.
- Make performance a first-order requirement for runtime and test-runner work,
  but never by weakening Salesforce-shaped behavior, metadata semantics, limits,
  or isolation. A faster wrong framework is still wrong.
- Register future Salesforce surface work in `internal/surface` before widening
  runtime, server, capability, or compatibility behavior. Each descriptor should
  name its owner package and focused test command.
- Use `internal/compat.FixtureBuilder` for new code-created compatibility
  fixtures before dropping down to raw `Fixture` literals.
- Keep project-vs-dependency provenance explicit. VM duplicate symbol work
  should use named provenance helpers instead of open-coded boolean ranking.
- Do not use proprietary GLADE internals as an implementation source.
- User Apex, metadata, fixtures, and API requests must not panic the CLI or
  server. Panics in those paths are bugs.
- Preserve source ranges through parse, semantic analysis, IR, VM, trace, LSP,
  DAP, profile, and test reporting.
- Capability status changes require capability coverage and regenerated docs.

## Performance Standards

- Treat local Apex test throughput as part of runtime correctness. A change that
  makes broad projects slow is not done.
- Use `strings.EqualFold` for case-insensitive equality. Do not allocate with
  `strings.ToLower` or `strings.ToUpper` just to compare names.
- Use canonical lowercase map keys only when the map is built once or reused
  enough to pay for the normalization.
- Use `len(s)` for string byte counts. Do not use `len([]byte(s))`.
- Avoid per-record metadata merges, describe rebuilds, reflection-style scans,
  or repeated fixture parsing in DML, SOQL, SObject, JSON, and test-runner hot
  paths.
- Share only immutable compiled code and immutable schema-derived caches across
  parallel Apex tests. Keep org data, mutable metadata overlays, trigger side
  effects, static state, request state, async state, limits, and rollback state
  per test.
- Prefer one indexed lookup over repeated linear scans when the path runs per
  record, per field, per query row, or per assertion in a large test corpus.
- Measure broad changes with the smallest useful package test first, then with a
  saved corpus sentinel or profile when the change touches runtime throughput.

## Salesforce Surface Boundaries

- Keep generic handlers generic. SObject member access, dynamic `get`, SOQL
  projection, DML validation, lookup paths, JSON encoding, and storage must use
  metadata and schema shape instead of standard-object name shortcuts.
- Put standard-SObject-specific business rules in named same-package files such
  as `sobject_dml_account.go`, `sobject_dml_opportunity.go`, or
  `sobject_dml_content.go`. Keep the generic DML and VM files as narrow routers.
- Keep docs-ledger identity rewrites inside `internal/surfaceledger`. Do not let
  parser cleanup, namespace canonicalization, or docs row suppression become
  runtime behavior.
- Pure HTTP, REST, Tooling, SOAP, Bulk, GraphQL, and Pub/Sub surfaces are later
  phases unless a local Apex test needs their Apex-visible shape or explicit
  unsupported behavior.
- For passive DTOs, add shape when it helps compile and test local Apex. Add
  behavior only when Apex tests can observe it without live Salesforce services.

## Validation

- Record the pre-refactor baseline before broad mechanical moves.
- Run the smallest package test that covers the moved code after each slice.
- If the baseline is red before refactoring, compare new failures against the
  recorded baseline instead of claiming a full green tree.
- Use `gofmt` after Go source moves.
- Run `go test ./...` before finishing. Report any remaining failures plainly.

## Current Refactor Targets

The Tier 2 god-file splits below are largely complete. `vm.go` dropped from
13,079 to ~4,000 lines, `method_dispatch.go` from 7,944 to ~3,150, and
`sema.go`, `soql.go`, `stdlib.go`, `construct_runtime.go`, `dml_runtime.go`,
`apextest/runner.go`, `capability/stub_behavior.go`, and `projectscan/scan.go`
were each carved into responsibility files within their packages.

Remaining handwritten mega-functions to extract (Tier 1), each a self-contained
dispatch switch whose arms can move to named helpers in the same package:

- `internal/vm` `VM.call` (static/global dispatch) and
  `VM.callPlatformObjectMember` (platform receiver switch) — convert switch arms
  into registered handlers via the `platformObjectMemberSurface` registry; see
  `docs/ADDING_A_PLATFORM_API.md`.
- `internal/vm` `VM.callValueMember` and `VM.constructValueWithLiteral`.
- `internal/gladecli/cli.go`: split command families into same-package files
  while keeping `Run` as the command router.

To add a new Salesforce surface, follow `docs/ADDING_A_PLATFORM_API.md` and
register the surface in `internal/surface` first.

# Open AER Alternative Plan

## Goal

Build an open source local Apex runtime with the developer experience people
want from AER: local test execution, anonymous Apex execution, schema-aware
SOQL/DML, triggers, governor limits, profiling, debugging, LSP features, and a
Salesforce-compatible local API server.

This must be a clean-room implementation. Use public docs, public grammars,
open formats, observed CLI behavior, and black-box compatibility tests. Do not
copy proprietary code, decompiled logic, private symbols, private data
structures, or license enforcement details.

## Evidence Reviewed

- `aer-reverse-engineering.md`: identifies AER's broad package boundaries:
  CLI, VM, storage, schema, driver, discovery, trace, report, LSP, and a
  DataWeave dependency.
- `docs/PLAN.md`: historical profiling plan. The profiling/reporting work is
  now native to `oaer`; `apexrr` is not a dependency.
- Captured trace/report examples prove the practical value of Chrome Trace
  Event output and method-level attribution.
- Public AER docs and VS Code marketplace docs: current public feature surface
  includes local tests, anonymous Apex, triggers, schema-aware execution,
  governor limits, DAP debugging, and LSP features.

## Product Scope

The open alternative should ship as `oaer` until compatibility is mature. Later
we can add an `aer`-compatible command alias if the community wants it.

### CLI

- `oaer test <paths...>`: run Apex tests locally.
- `oaer test --json`: structured test result output for editors and CI.
- `oaer test --watch`: file watcher with affected-test selection.
- `oaer test --debug`: DAP server for VS Code and other DAP clients.
- `oaer exec <paths...> '<anonymous apex>'`: run anonymous Apex.
- `oaer exec --debug`: debug anonymous Apex.
- `oaer server`: local Salesforce-compatible REST/SOAP-ish API backed by the
  same data store; `--db <path>` enables persistent SQLite state.
- `oaer lsp`: language server.
- `oaer schema pull|load|export`: manage local schema metadata.
- `oaer db seed|reset|export|inspect`: manage local SObject persistence.
- `oaer exec --trace <path>`: write Chrome Trace Event output.
- `oaer profile analyze <trace.json>`: produce native JSON or Markdown trace
  analysis. pprof-compatible CPU output remains future work.

### Runtime Fidelity Targets

- Apex parser, type checker, and interpreter.
- Classes, interfaces, inheritance, visibility, namespaces, inner classes.
- Static initializers, instance construction, properties, enums.
- Test framework: `@isTest`, `@TestSetup`, `Test.startTest`, `Test.stopTest`,
  async draining, assertions, test isolation.
- SObjects, custom objects, fields, relationships, record types, IDs.
- SOQL: static and dynamic queries, relationship traversal, aggregates,
  subqueries, bind variables, security clauses.
- SOSL after SOQL is stable.
- DML: insert, update, upsert, delete, undelete, merge, partial success, errors.
- Trigger ordering and recursion behavior.
- Async Apex: Queueable, Batchable, Schedulable, Future.
- Governor limits with deterministic counters and optional strict enforcement.
- Platform APIs by priority: `Schema`, `Limits`, `Database`, `System`, `JSON`,
  `Http`, `Messaging`, `EventBus`, `FeatureManagement`, crypto/date/math/string
  libraries.
- Managed package namespaces and package visibility.

### Better Than AER

- No license gate; Apache-2.0 or MPL-2.0, depending on dependency constraints.
- Compatibility dashboard against Salesforce API versions.
- Deterministic replay bundles: schema, fixtures, clock, user context, limits,
  and command in one portable archive.
- First-class per-statement instrumentation for SOQL, DML, describe, heap, CPU,
  callouts, and validation rules.
- Query plan visibility for SOQL through SQLite/Postgres explain plans.
- Mutation/fuzz tests for runtime semantics.
- Plugin API for missing standard-library surfaces.
- Fixture anonymizer built in.
- SARIF and JUnit output in addition to JSON, pprof, and trace files.

## Architecture

```
cmd/oaer
internal/
  apexast/       parser adapter, concrete syntax, source ranges
  typesys/       symbols, overloads, inheritance, namespaces, generics-ish rules
  sema/          semantic analysis and diagnostics
  ir/            lowered executable representation
  vm/            interpreter, stack frames, heap, exceptions, debugger hooks
  schema/        metadata model, custom object/field/relationship registry
  sobject/       runtime SObject values and describe helpers
  soql/          parser, binder, optimizer bridge, executor
  storage/       SObject persistence, transactions, fixture import/export
  dml/           DML pipeline, validation, triggers, workflow-like hooks
  apextest/      Apex test discovery, isolation, async drain, JSON/JUnit results
  testreport/    console, JSON, and JUnit report models
  trace/         Chrome Trace Event output
  profile/       native trace aggregation and JSON/Markdown reports
  dap/           Debug Adapter Protocol server
  lsp/           Language Server Protocol server
  server/        local Salesforce-compatible API
  compat/        black-box compatibility suites
  capability/    machine-readable MVP matrix and generated docs
```

## Parser Strategy

Start with `github.com/octoberswimmer/apexfmt` because it is public and
BSD-licensed, and its generated ANTLR parser already exists in Go. Wrap it
behind `internal/apexast` immediately so we can swap to another grammar if
needed.

If apexfmt's tree is too formatter-shaped for semantic analysis, generate a
parser from the public Apex grammar lineage and keep the same adapter
interface.

## Storage Strategy

Use SQLite first because it gives local persistence, transactions, indexing,
and explain plans without a service dependency. The current baseline has a
concrete SQLite-backed store for object definitions, records, and ID sequences;
a neutral `storage.Store` interface can come later when larger fixtures or
alternate backends need it.

Core tables:

- `objects`, `fields`, `relationships`, `record_types`
- `records`: object name, ID, JSON value, created/updated metadata
- optional shadow tables per SObject for faster SOQL once behavior is stable
- `files`, `content_versions`, `users`, `profiles`, `permissions`
- `async_jobs`, `platform_events`, `callout_mocks`

## Execution Model

1. Parse source and metadata.
2. Build symbol table.
3. Type-check and lower to IR.
4. Execute IR in a VM with:
   - stack frames and lexical scopes
   - SObject heap values
   - exception unwinding
   - transaction context
   - governor limit context
   - trace/debug hooks
5. Route platform calls into `platform/*` packages.
6. Commit or roll back storage changes based on Apex transaction outcome.

Interpret first. Do not build a compiler until fidelity is high. The hard part
is Salesforce semantics, not dispatch speed.

## Compatibility Method

Use a black-box test harness:

- same Apex source
- same schema
- same fixture data
- run in Salesforce org and `oaer`
- compare outputs, thrown exceptions, DML results, SOQL result shape, trigger
  side effects, limit counters, and debug events

Fixtures should be generated from minimal public examples plus project-local
metadata the user owns. Avoid using proprietary AER traces as implementation
golden files; they can validate CLI/profile shape only.

## Build Phases

### Phase 0: Legal and Project Foundation

- Choose license and contributor agreement.
- Document clean-room rules.
- Keep profiling and report generation inside `oaer`; do not depend on a
  separate apexrr tool.
- Add architecture decision records and compatibility test format.

Exit: repo builds, docs are clear, no proprietary implementation dependency.

### Phase 1: Apex Front End

- Integrate parser adapter.
- Build AST/source-range model.
- Implement symbol table for classes, methods, fields, properties, interfaces,
  annotations, triggers.
- Type-check enough Apex for real unit tests.
- Emit diagnostics with file/line ranges.

Exit: parse and semantically index a large Salesforce repo.

### Phase 2: Minimal VM and Test Runner

- Execute expressions, statements, methods, constructors, static init.
- Implement core `System`, primitive/string/list/map/set/date/json behavior.
- Discover and run `@isTest` methods.
- Produce `--json`, JUnit, and readable console output.

Exit: simple Apex unit tests run locally without org access.

### Phase 3: SObjects, SOQL, DML, Triggers

- Load Salesforce metadata from local project and exported schema.
- Implement SObject values and ID generation.
- Implement SOQL parser/binder/executor over SQLite.
- Implement DML transactions and trigger invocation.
- Add validation/errors and partial-success Database methods.

Exit: CRUD-heavy Apex tests and triggers run against local data.

### Phase 4: Salesforce Test Semantics and Limits

- `@TestSetup`, isolation, `runAs`, `startTest/stopTest`.
- Async draining for Queueable, Future, Batchable, Schedulable.
- Governor counters for queries, DML, rows, heap, CPU approximation, callouts.
- Strict and permissive modes.

Exit: meaningful org-parity test execution for common enterprise projects.

### Phase 5: Debugging, Profiling, Watch

- DAP server: breakpoints, step in/over/out, scopes, variables, watch eval,
  call stack.
- Chrome Trace Event output.
- Native trace analysis reports; pprof-compatible CPU/profile output or a
  pprof bridge remains future work.
- Statement-level metrics for SOQL/DML/describe/callout/heap.
- Watch mode with affected-test selection from dependency graph.

Exit: local TDD/debug loop is competitive with AER's public experience.

### Phase 6: LSP

- Reuse parser/type index.
- Implement semantic tokens, completion, hover, definition, references,
  document symbols.
- Add diagnostics from the same semantic engine.
- Add schema-aware completions for SObjects and fields.

Exit: VS Code and other editors can use `oaer lsp` directly.

### Phase 7: Local Salesforce API Server

- REST endpoints for SObject CRUD/query.
- Tooling-ish endpoints needed by tests and local integrations.
- Auth stub/user context.
- Persistent SQLite database and fixture reset.
- Optional callout mock server.

Exit: local apps and integration tests can target `oaer server`.

### Phase 8: Enterprise Fidelity

- More SOQL: aggregates, `TYPEOF`, `FIELDS()`, security clauses, query plans.
- SOSL.
- Sharing/FLS/CRUD enforcement modes.
- Managed package visibility and namespace edge cases.
- Platform events and CDC approximations.
- Flow/validation/workflow support where locally representable.
- DataWeave-compatible layer or clean substitute for Apex DataWeave calls.

Exit: compatibility dashboard drives issue priority.

## Current Next Steps

1. Expand SOQL/DML/trigger compatibility for child subqueries, broader
   aggregates, complex predicates, external-ID upsert, `addError`, validation
   rules, and bulk ordering.
2. Improve SObject describe fidelity, system fields, record type information,
   permission semantics, and large-fixture performance.
3. Add live debugger pause hooks and breakpoint-driven execution control on top
   of the current DAP snapshot sessions.
4. Complete LSP definition/references/semantic tokens and native watcher
   backends without weakening full-project correctness.
5. Grow Salesforce black-box compatibility fixtures and keep
   `oaer compat mvp` as the release gate.

## Risk Register

- Apex semantics are larger than they look; keep the compatibility suite ahead
  of feature claims.
- SOQL and DML fidelity will dominate the project timeline.
- Debugging requires source maps and stable pause points from day one of the VM.
- LSP built before the type system settles will churn; defer full LSP until the
  front end is stable.
- Managed-package behavior needs real package fixtures and will be a long tail.
- Salesforce release drift requires versioned schema/runtime behavior.

## Reuse From This Repo

Keep:

- trace parser/report ideas from `internal/trace` and `internal/profile`
- transform lexer tests as useful Apex lexical edge cases
- project config detection from `internal/config`

Replace or keep replaced:

- regex discovery with AST discovery
- AER subprocess driver with native `oaer` runtime
- `internal/lsp` client for AER with an actual LSP server

Do not reuse:

- license details from the reverse engineering report
- inferred proprietary package/type/function internals as implementation design

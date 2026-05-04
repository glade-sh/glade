# Architecture

`oaer` is organized as a set of narrow packages that can be tested separately
and composed by the CLI.

## Current Packages

- `cmd/oaer`: executable entry point.
- `internal/oaercli`: command routing and user-facing CLI behavior.
- `internal/apexast`: parser adapter and stable source model over the public
  `apexfmt` ANTLR parser.
- `internal/apexdocs`: public Apex documentation inventory extraction, diffing,
  and stable JSON generation for the broad support catalog.
- `internal/config`: `oaer.yml` discovery and parsing.
- `internal/diagnostic`: shared diagnostic model for parser, semantic analysis,
  runtime, and CLI.
- `internal/project`: SFDX package directory discovery and source file
  collection.
- `internal/schema`: Metadata API custom object, field, picklist, and record type
  model.
- `internal/typesys`: first symbol index for declarations, members, triggers,
  and schema objects.
- `internal/sema`: semantic analysis for known-type catalogs, declaration and
  member references, method-body checks, overload matching, visibility,
  namespace/schema aliases, and stable diagnostics for the supported subset.
- `internal/ir`: compact executable representation for VM-supported Apex
  statements and expressions.
- `internal/vm`: interpreter for the supported Apex subset, including
  anonymous execution, class and method dispatch, constructors, instance and
  static fields, inheritance/super dispatch, common control flow, exceptions,
  SObjects, SOQL/DML entry points, governor counters, platform API basics, and
  trace/debug snapshots.
- `internal/apextest` and `internal/testreport`: Apex test discovery,
  project class/trigger compilation, `@TestSetup`, per-test org isolation,
  static reset, `startTest`/`stopTest`, `runAs`, Queueable/Future/Batch/
  Scheduled draining, async job records, and console/JSON/JUnit reporting.
- `internal/sobject`: runtime SObject value and schema describe helpers.
- `internal/storage`: org/object/record model, fixture envelope, deterministic
  IDs, cloneable transaction snapshots, fixture alias/reference resolution,
  deterministic platform users/profiles/permissions, SQLite persistence, and
  schema migrations.
- `internal/soql`: in-memory SOQL parser and executor for the supported query
  subset, including binds, ordering, limits, offsets, `COUNT()`, and simple
  parent relationship projection.
- `internal/dml`: DML insert/update/delete/upsert/undelete pipeline,
  all-or-none result shaping, validation, rollback snapshots, and trigger
  invocation hooks for the supported VM paths.
- `internal/dap`: Debug Adapter Protocol framing, request/response handling,
  and snapshot sessions used by `oaer exec --debug` and `oaer test --debug`.
- `internal/lsp`: stdio LSP/JSON-RPC server backed by the project index for
  initialize/shutdown, diagnostics, symbols, hover, and completion basics.
- `internal/watch`: file classification, snapshot diffing, native/polling watch
  backends, debounce, JSON events, cancellation, incremental Apex re-indexing,
  and dependency-graph affected-test selection.
- `internal/profile`: native trace/profile aggregation and JSON/Markdown
  reporting.
- `internal/server`: Salesforce-shaped HTTP handler for supported SObject CRUD,
  query/queryAll, describe/recent, limits, identity/userinfo stubs, Tooling
  `executeAnonymous`, composite sObject insert, fixture/scoped reset endpoints,
  stable unsupported Apex REST dispatch errors, and optional SQLite-backed
  persistence.
- `internal/compat`: compatibility fixture schema, fixture evidence metadata,
  parse/check/exec/test/DB fixture execution, and catalog evidence reports.
- `internal/capability`: machine-readable feature matrix and MVP readiness
  gate, plus the docs-driven Apex support catalog and product namespace typed
  stub reports.

## Runtime Pipeline

1. Load project configuration and Salesforce metadata.
2. Parse Apex source through `internal/apexast`.
3. Build symbols and resolve references through `internal/typesys`.
4. Type-check through `internal/sema`.
5. Lower checked code into `internal/ir`.
6. Execute with `internal/vm`, routing SObject, SOQL, DML, trigger, limit, and
   platform calls into dedicated packages where behavior has a supported
   baseline.
7. Surface the same runtime through CLI execution, tests, watch mode, LSP/DAP
   snapshots, profile analysis, compatibility checks, and the local API server.
8. Record diagnostics, traces, profiles, test reports, storage fixtures, server
   responses, documentation inventories, capability catalogs, product namespace
   stub reports, fixture evidence, and compatibility results in stable
   machine-readable formats.

## Design Constraints

- Keep the parser behind an adapter so grammar dependencies can change.
- Attach source ranges early and preserve them through diagnostics and runtime
  traces.
- Return explicit unsupported-feature diagnostics instead of panicking.
- Keep Salesforce behavior claims tied to compatibility fixtures.

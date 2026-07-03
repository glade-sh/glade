# Architecture

`glade` is organized as a set of narrow packages that can be tested separately
and composed by the CLI.

## Current Packages

- `cmd/glade`: executable entry point.
- `internal/gladecli`: command routing and user-facing CLI behavior.
- `internal/pluginhost`: plugin manifests, install state, archive and registry
  install, lock restore, and command dispatch to installed executables.
- `internal/apexast`: parser adapter and stable source model over the local
  tree-sitter Apex parser module.
- `internal/config`: `glade.yml` discovery and parsing.
- `internal/diagnostic`: shared diagnostic model for parser, semantic analysis,
  runtime, and CLI.
- `internal/flagparse`, `internal/cliui`, `internal/gladehome`, and
  `internal/runartifact`: shared command parsing, terminal output, user data
  locations, and checked artifact paths.
- `internal/project`: SFDX package directory discovery and source file
  collection.
- `internal/resource`, `internal/orgimport`, and `internal/automation`:
  local project resource loading, Salesforce data import orchestration, and
  editor/automation-facing payload contracts.
- `internal/schema`: Metadata API custom object, field, picklist, and record type
  model.
- `internal/orgdescribe`: captured describe-symbol support for local
  intelligence.
- `internal/typesys`: first symbol index for declarations, members, triggers,
  and schema objects.
- `internal/codeintel`: editor-neutral project intelligence built from the type
  index, metadata schema, dependency artifacts, cached describe symbols, and
  source files. It powers symbol inspection, reference queries, conservative
  refactor planning, enterprise graph edges, changed-test selection, semantic
  query diagnostics, local server symbol endpoints, and future editor clients.
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
- `internal/storage`: org/object/record model, generated standard object schema
  baseline, generated SObject stub field overlay, fixture envelope,
  deterministic IDs, cloneable transaction snapshots, fixture alias/reference
  resolution, deterministic platform users/profiles/permissions, SQLite
  persistence, and schema migrations.
- `internal/soql`: in-memory SOQL parser and executor for the supported query
  subset, including binds, ordering, limits, offsets, `COUNT()`, and simple
  parent relationship projection.
- `internal/sosl`: local SOSL parsing/execution for the supported search
  surface.
- `internal/dml`: DML insert/update/delete/upsert/undelete pipeline,
  all-or-none result shaping, validation, rollback snapshots, and trigger
  invocation hooks for the supported VM paths.
- `internal/dap`: Debug Adapter Protocol framing, request/response handling,
  and snapshot sessions used by `glade exec --debug` and `glade test --debug`.
- `internal/lsp`: stdio LSP/JSON-RPC server backed by the project index for
  initialize/shutdown, diagnostics, symbols, hover, and completion basics.
- `internal/watch`: file classification, snapshot diffing, native/polling watch
  backends, debounce, JSON events, cancellation, incremental Apex re-indexing,
  and reference-graph affected-test selection (reverse-reachability over a static
  type-dependency graph, refreshed incrementally).
- `internal/profile`: native trace/profile aggregation and JSON/Markdown
  reporting.
- `internal/debuglog`, `internal/apexlog`, and `internal/trace`: debug log
  parsing, Apex log surfaces, and native trace event capture.
- `internal/enterprise`, `internal/enterprisegraph`,
  `internal/enterpriseassess`, `internal/enterprisecruft`, and
  `internal/refactorproof`: enterprise report contracts, static project graph,
  assessment findings, conservative cruft classification, and branch proof
  reports. These packages consume the product parser, type index, semantic
  analyzer, trace model, and test-selection graph. They do not regenerate
  support ledgers.
- `internal/server`: Salesforce-shaped HTTP handler for supported SObject CRUD,
  query/queryAll, describe/recent, limits and record counts, identity/userinfo
  stubs, Tooling `executeAnonymous`, local Tooling source/schema metadata reads,
  composite sObject insert, fixture/scoped reset endpoints, stable unsupported
  Apex REST dispatch errors, and optional SQLite-backed persistence.
- `internal/dbmanager`: browser-facing local record-manager API contracts used
  by the DB UI.
- `internal/playground`, `internal/tui`, `internal/visualforce`,
  `internal/aura`, `internal/lwc`, `internal/lwcbrowser`,
  `internal/lwcruntime`, and `internal/lwcshell`: local browser preview,
  terminal UI, Visualforce, Aura/LWC analysis, runtime asset, and shell support.
- `internal/namespaceremap`, `internal/refactor`, and `internal/refactorproof`:
  namespace aliasing and conservative refactor/proof support.
- `internal/startupcache`: local startup-cache metadata.
- Maintenance scanners, compatibility fixtures, capability catalogs, advisory
  performance scans, docs inventory, and surface ledgers ship as plugins.
  Salesforce docs inventory extraction lives in the compat plugin because it
  feeds support ledgers, not runtime execution. Plugins
  may depend on this framework; this framework does not depend on plugins.

## Runtime Pipeline

1. Load project configuration and Salesforce metadata.
2. Parse Apex source through `internal/apexast`.
3. Build declarations through `internal/typesys`.
4. Build cross-source symbol references through `internal/codeintel`.
5. Type-check through `internal/sema`.
6. Lower checked code into `internal/ir`.
7. Execute with `internal/vm`, routing SObject, SOQL, DML, trigger, limit, and
   platform calls into dedicated packages where behavior has a supported
   baseline.
8. Surface the same runtime through CLI execution, tests, watch mode, LSP/DAP
   snapshots, profile analysis, plugin dispatch, and the local API server.
9. Record diagnostics, traces, profiles, test reports, storage fixtures, and
   server responses in stable machine-readable formats.

## Design Constraints

- Keep the parser behind an adapter so grammar dependencies can change.
- Attach source ranges early and preserve them through diagnostics and runtime
  traces.
- Return explicit unsupported-feature diagnostics instead of panicking.
- Keep Salesforce behavior claims tied to compatibility fixtures.

## Adding Salesforce Functionality

When Salesforce ships something new (a class, namespace, method, or surface) or
a gap is found, keep product runtime changes in this repository and use
installed plugins for gap discovery, fixtures, and support ledger updates.

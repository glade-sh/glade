# Compatibility Policy

`glade` tracks compatibility at feature level. Every language, runtime,
metadata, SOQL, DML, library, tooling, and server feature should be classified
with one of these statuses:

- `supported`: implemented and covered by compatibility tests.
- `partial`: works for common cases with documented gaps.
- `stub`: exists so code can load, but returns a controlled placeholder or
  explicit unsupported result.
- `unsupported`: fails with a stable diagnostic before or during execution.
- `unknown`: not evaluated yet.

Silent wrong behavior is a release blocker for any feature marked `supported`.
Unsupported behavior should fail loudly and predictably.

## Public Support Map

`glade` publishes product commands and plugin management. Maintenance scanners,
compatibility fixtures, gap discovery, and generated support artifacts ship
through plugins.

Use the public support map first when deciding whether Glade can run a project
or test path:

- <https://glade.sh/guide/support-map>

The checked repository artifacts are the lower layer:

- [`docs/STDLIB_COVERAGE.md`](STDLIB_COVERAGE.md)

Use the installed tool for product checks:

```bash
glade check --project .
glade test --project . --json
```

Advisory performance triage stays in the first-party performance plugin:

```bash
mkdir -p reports
glade performance scan --project . --trace reports/slow.trace.json --format markdown --top 10
```

`glade` owns the local execution trace and profile facts. The plugin owns
static risk ranking, trace correlation, metadata blast-radius analysis, and
optional org-facts snapshots.

Release installation and artifact verification are documented in
[`docs/INSTALL.md`](INSTALL.md).
Editor tasks, LSP wiring, and VS Code launch examples are documented in
[`docs/EDITOR.md`](EDITOR.md).
Release promotion, upgrade, and API-version compatibility policy are documented
in [`docs/RELEASE_POLICY.md`](RELEASE_POLICY.md), with ongoing notes in
[`docs/RELEASE_NOTES.md`](RELEASE_NOTES.md).

## Initial Matrix

| Area | Status | Notes |
| --- | --- | --- |
| CLI surface | supported | `version`, `doctor`, `completion`, `config`, `init`, `parse`, `inspect`, `schema`, `check`, `exec`, `debug`, `editor`, `dap`, `test`, `dev`, `report`, `lsp`, `profile`, `plugins`, `package`, `server`, `playground`, `support`, and `db` exist. Advisory compatibility and performance scans run through first-party plugins. |
| Project config | supported | `glade.yml` and SFDX project discovery cover the local development contract, including package directories, source API version, namespace, org features, storage, limits, and local server settings. |
| Diagnostics | supported | Shared diagnostic shape covers stable code, message, severity, source span, JSON, SARIF, and unsupported-feature reporting for checked local rows. |
| Compatibility fixtures | supported | The maintenance fixture runner ships as the compat plugin. Checked fixtures cover parse, check, exec, test, DB lifecycle, server black-box behavior, and generated support drift checks. |
| Parser | supported | `glade parse` uses the local tree-sitter Apex parser module through `internal/apexast`, preserves Apex methods named `void`, provides stable malformed-parse diagnostics, and has enterprise-style multi-class, namespace/package-directory, and bounded large-index coverage for the local Apex execution corpus. |
| Project loader | supported | SFDX package directories and Apex/object/field/record type files are discovered for the local project contract. |
| Schema loader | supported | Custom object, custom field, picklist, record type, data category, platform data, and relevant org metadata are loaded. Local schema describe also merges the generated Salesforce standard object baseline for all checked SObject shape, with Person Account and Multi-Currency field overlays gated by org features. |
| Symbol table | supported | Top-level Apex declarations, qualified nested types, members, test annotations, triggers, duplicate names, and schema objects are indexed. |
| Semantic analysis | supported | `glade check` validates declaration/member type references, method and constructor parameter type references, project namespace-qualified and namespace-token schema references, visibility and override checks, interface and abstract implementation checks, trigger SObjects, schema lookup references, test discovery, local declarations, duplicate locals, initializer and assignment type mismatches, return type and non-void path checks, constructor references and chaining, non-instantiable constructor calls, unknown variable reads, project method calls, inherited/interface/super calls, this/super field and return type inference, inherited instance field scope, private/protected visibility through inheritance chains, `@TestVisible` access, overload arity and specificity, numeric widening, simple expression typing, object and collection assignability, IR-backed scoped reads and flow checks, user-object field reads/writes, constructor-call validation, and token-level diagnostic ranges for the supported VM subset. |
| VM | supported | `glade exec` runs the supported anonymous Apex subset with variables, expressions, methods/classes, namespace-qualified class names, constructors, `this(...)`/`super(...)` constructor chaining, interface/enum/abstract instantiation guards, abstract method invocation guards, non-void method fallthrough guards, overloaded method/constructor selection by argument types with numeric, class/interface, null, and ambiguity specificity baselines, instance/static fields, property accessor bodies, source-ordered field initializer expressions and initializer blocks, static reset through field initializers and static blocks, inheritance/super dispatch including superclass-typed and interface-typed references, declaring-class `super` method dispatch, inherited concrete methods ahead of interface fallback methods, inherited static fields and methods through subclass names, interface fallback method lookup, namespace-token SObject/field aliases through construction, access, DML, and SOQL, nested classes with constructors, fields, methods, static members, relative owner-local type names, nested interfaces, nested enum values/methods, explicit object `toString()` dispatch for calls/debug/assert messages, user object identity equality, typed coercion for locals, params, returns, fields, collection members, null/object/enum values, numeric widening, and schema-backed DML storage, private/protected visibility through inheritance chains, `@TestVisible` method access from test classes, and namespace-global class/member access, collections, for/enhanced-for/do-while/switch with switch-local break, ordered catch blocks, pipe-style multi-catch, try/catch/finally including return and throw unwinding, finally-preserved and finally-overridden loop signals, bare rethrow with original stack preservation, catchable null dereference, interface-based catch matching, common exception hierarchy matching, `System.*Exception` name normalization, exception `getMessage`/`getTypeName`/`getLineNumber`/`getStackTraceString`, trace events, assertions, and common platform APIs. Hosted-only platform behavior is split into exact unsupported rows. |
| Test runner | supported | `glade test` discovers `@isTest` and legacy test methods, compiles project helper classes/triggers, runs constructor and instance method bodies, executes `@TestSetup` once per class into a setup data snapshot, resets statics, clones org state per test, restores `startTest`/`stopTest` governor windows, scopes `runAs` user/profile/permission identity, enforces the supported mixed-DML guard, drains Queueable, `@future`, Batchable, and Schedulable jobs, records local `AsyncApexJob`/`CronTrigger` state, preserves statement-level assertion/runtime stack frames, and emits console, JSON, and JUnit reports for the supported VM subset. `glade test serve` keeps the warmed runtime across CLI invocations; `.glade/test/startup.meta.json` plus a hashed payload caches the harness between runs (see `docs/TEST_STARTUP_CACHE.md` for freshness and recovery). |
| SObject/schema runtime | supported | Runtime SObject values preserve projected fields and explicit nulls; schema describe registry, deterministic key prefixes, and a generated standard object field baseline are available for checked standard SObject shape. Apex construction, typed/dynamic field access, `get`/`put` with previous-value return, `isSet`, `clear`, `getPopulatedFieldsAsMap`, common system fields after DML and SOQL projection, `Schema.getGlobalDescribe`, `Schema.describeSObjects`, `SObjectType.getDescribe`, `fields.getMap`, field describe basics with picklist values, record type describe maps/lists with common `RecordTypeInfo` methods, child relationship describe basics, object-level and field-level `addError`, multi-error `hasErrors`/`getErrors` and DML result shaping, DML Id propagation, parent relationship projection, and VM/storage record conversion are wired into the VM. |
| SOQL | supported | Static SOQL, `Database.query`, and `Database.queryWithBinds` execute against the in-memory org with scalar and collection binds, projection, `FIELDS(ALL/STANDARD/CUSTOM)`, `TYPEOF`, parent and child relationship projection, polymorphic relationship target resolution from multi-reference metadata, semi-joins, anti-joins, aggregate functions and grouping, common date literals, soft-deleted row visibility through `ALL ROWS`, boolean filters, indexed equality candidates, ordered/limited/offset results, `FOR UPDATE` local lock markers, `WITH SECURITY_ENFORCED`, `WITH USER_MODE`, `WITH SYSTEM_MODE` projection validation including child subquery permissions, single-SObject assignment, serialized row attributes, and catchable `QueryException` parse errors. |
| DML/transactions/triggers | supported | `internal/dml` and the VM support Apex insert/update/delete/upsert/undelete/merge syntax, `Database.*` allOrNone results, `SaveResult`/`UpsertResult`/`MergeResult` objects with structured single and multi-entry `Database.Error` values, required/unknown field validation, deterministic IDs, common system field stamping, soft delete/undelete query visibility, implicit and explicit external-ID upsert, ID/object mismatch checks, unique fields, lookup reference validation, simple Metadata API validation rules, merge loser soft-delete with child lookup reparenting, upsert insert/update trigger contexts, merge master update hooks, merge duplicate delete hooks, cascade soft delete from relationship metadata, rollback snapshots including failed flow record-create effects, and before/after trigger invocation with operation flags, size, new/old lists, nullable unavailable contexts, newMap/oldMap, after-undelete context, bulk partial-success row alignment, deterministic recursion guard rollback, and object-level and field-level `addError` row failures. |
| Governor limits | supported | The VM tracks SOQL queries/rows including projected child relationship rows and query-locator rows, DML statements/rows including cascade-delete child rows, deterministic live heap approximation across locals and mutated collections, deterministic CPU cost from statements plus SOQL/DML row work, callouts, aggregate async jobs, future calls, queueable jobs, batch jobs, scheduled jobs, email invocations, runAs calls, savepoints, and savepoint rollbacks. Supported `Limits` getters expose current and max counters for the tracked families. Strict/permissive limit modes and configurable local profiles are available through CLI exec/test/server surfaces and compatibility exec/test fixtures. Exact hosted Salesforce accounting remains an explicit unsupported row. |
| Storage/fixtures/persistence | supported | SQLite-backed org storage persists object definitions, records, ID sequences, schema migrations/versioning, fixture seed/export/reset/inspect, object-aware alias and relationship-reference resolution, qualified object aliases, ambiguity checks, reference-target validation, deterministic platform data, local org settings, DB lifecycle and export re-import coverage, server persistence, scoped fixture reset endpoints, transaction-scoped prepared inserts, storage performance pragmas, large-fixture save/load coverage, cloned-org commit boundaries for mutating server requests and Tooling executeAnonymous, and serialized server request handling. |
| DAP | supported | local DAP support covers DAP framing, setBreakpoints, continue/pause/next/stepIn/stepOut/disconnect, stackTrace with trace-provided line/column positions, scopes, variables, evaluate, VM debug pause hooks, live statement breakpoint stops with stack/locals/static snapshots, stack-depth live stepping, Locals/Statics/Trigger scopes, collection/object/SObject children, paused-context watch expressions, `glade exec --debug` / `glade test --debug` snapshot sessions, VS Code launch aliases, debug profile wiring, and local glade exec/test DAP sessions. |
| LSP | supported | `glade lsp` runs a stdio LSP transport for initialize/shutdown, shared `glade check` diagnostics, open-buffer parse overlays, test-result diagnostics, incremental document sync, symbols, semantic tokens, definition, references, rename, hover, Apex/SObject/member/field/keyword completion from the project index, and context-aware SOQL `SELECT` projection completion ranking that puts SObject fields ahead of top-level names. |
| Watch mode | supported | `glade test --watch` and `--watch-once` support native `fsnotify` watching with polling fallback, debounce, versioned newline-delimited JSON events with stable run IDs and test class arrays, incremental Apex-only re-indexing, dependency-graph affected-test selection, cancellable in-flight VM/test reruns, stale run-result suppression, and profile/trace report events. |
| Native profile analysis | supported | `glade profile analyze` reads native Chrome Trace Event output and emits native JSON, Markdown, or pprof-compatible reports with hot events, wall-clock summary timing, categories, runtime sections for statements/methods/SOQL/DML/describe/callout/email/async/trigger/limits, source offsets, statement line/column ranges, SOQL/DML row deltas, platform/resource counter attribution, and wall-clock statement timing summaries. No external apexrr dependency is used. |
| Local API server | supported | `glade server` starts the local Salesforce-shaped REST baseline with data version/root discovery, SObject CRUD, normal REST JSON payload decoding, explicit null updates, query/queryAll REST-shaped record attributes, describe/recent, limits and record counts, OAuth userinfo/id stubs with local user selection, Tooling `executeAnonymous` GET/POST success and failure shapes with `--limit-mode`, supported local-object Tooling queries, local source-backed Tooling metadata reads for Apex/source and metadata component rows, virtual Tooling schema metadata queries for EntityDefinition/EntityParticle/FieldDefinition/RelationshipDomain, Composite sObject insert with `referenceId`, partial success, and all-or-none rollback, Composite Batch and Tree, Bulk API v2 simple query job create/status/whole-result CSV, layout/default-value metadata, metadata job status, optional SQLite persistence through `--db`, Glade fixture/scoped reset endpoints, and Salesforce-shaped error arrays. Full auth, live org-only Tooling objects, live metadata deploy/retrieve, Composite Graph execution, broader Bulk API locator paging, and broader hosted REST namespaces remain explicit unsupported boundaries. |
| Visualforce dev rendering | preview feature | `glade dev vf` serves useful local `/apex/<PageName>` preview routes from SFDX pages and components, hot-reloads `.page`, `.component`, Apex, Aura, LWC, and static resource changes, renders common standard components, custom components, controller actions, page messages, expression/form binding, signed view state with CSRF checks, transient field omission, static resources, uploads, remoting envelopes, Lightning Out/LWC hosts with dependency diagnostics, AJAX refresh paths, local diagnostics overlays, local support JSON, and local PDF fallback output. Salesforce-hosted chrome, every component edge, exact lifecycle timing, Apex `PageReference.getContent*` parity, and byte-for-byte PDF output remain explicit unsupported boundaries. |

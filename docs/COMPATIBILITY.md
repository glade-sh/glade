# Compatibility Policy

`oaer` tracks compatibility at feature level. Every language, runtime,
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

## MVP Gate

The MVP target is full-featured aer-parity for local Apex development. The
current implementation should not be called MVP-ready while required features
are `partial`, `stub`, `unsupported`, or `unknown`.

Use:

```bash
oaer compat mvp
oaer compat mvp --require-ready
oaer compat matrix --json
oaer compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
oaer compat gaps --output docs/KNOWN_GAPS.md
oaer compat stdlib --output docs/STDLIB_COVERAGE.md
```

The source of truth for required MVP capabilities is `internal/capability`.
The generated public dashboard is checked in at
[`docs/COMPATIBILITY_DASHBOARD.md`](COMPATIBILITY_DASHBOARD.md). The generated
known-gaps document is checked in at [`docs/KNOWN_GAPS.md`](KNOWN_GAPS.md). CI
prints the machine-readable MVP gate and verifies that both generated documents
match the capability source. The generated standard-library coverage matrix is
checked in at [`docs/STDLIB_COVERAGE.md`](STDLIB_COVERAGE.md). Use
`oaer compat mvp --require-ready` for release promotion checks that must fail
while the target is not ready.

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
| CLI surface | partial | `version`, `help`, `doctor`, `parse`, `inspect`, `schema`, `check`, `exec`, `test`, `profile analyze`, `server`, `db`, `lsp`, and `compat` exist. Several commands are still partial because their underlying runtime fidelity is partial. |
| Project config | partial | Minimal `oaer.yml` discovery exists. |
| Diagnostics | partial | Shared diagnostic shape exists. |
| Compatibility fixtures | partial | JSON schema model exists; `oaer compat validate/run` executes parse, check, exec, test, and DB lifecycle fixtures, including malformed-parse diagnostics, storage seed/export/reset/inspect coverage, and enterprise-style multi-class parse/index/check coverage; `oaer compat matrix`, `oaer compat mvp`, and `oaer compat stdlib` expose machine-readable capability and standard-library status. Broader Salesforce black-box fixtures remain incomplete. |
| Parser | partial | `oaer parse` handles both example projects, including Apex methods named `void`; declaration walking and source utilities exist. |
| Project loader | partial | SFDX package directories and Apex/object/field/record type files are discovered. |
| Schema loader | partial | Custom object, custom field, picklist, and record type metadata are loaded. |
| Symbol table | partial | Top-level Apex declarations, qualified nested types, members, test annotations, triggers, duplicate names, and schema objects are indexed. |
| Semantic analysis | partial | `oaer check` validates declaration/member type references, method and constructor parameter type references, project namespace-qualified and namespace-token schema references, basic visibility conflicts, interface member visibility, override markers, missing concrete interface/abstract method implementations, trigger SObjects, schema lookup references, test discovery, and a conservative method-body baseline for local declarations, duplicate locals, local initializer and simple assignment type mismatches, simple return type mismatches and missing non-void returns, constructor references, constructor chaining, non-instantiable interface/enum/abstract constructor calls, unknown variable reads in call arguments, project method calls, inherited/interface/super calls, this/super field and return type inference, inherited instance field scope, private/protected method and field visibility through inheritance chains, `@TestVisible` method access from test classes, known-receiver overload arity/argument matching with exact and narrowest numeric specificity, nearest class/interface specificity, null specificity, and ambiguous overload diagnostics, integer-to-Long/Decimal/Double widening, decimal-literal argument typing, simple binary expression typing, class/interface object assignability, generic collection constructor assignability, known method-call return typing for receiver and chained constructor calls, an IR-backed sema pass for scoped local reads, Boolean conditions, declaration/assignment/return type checks, all-path non-void returns, known user-object field reads/writes, known receiver/same-class method calls, and constructor-call validation across statements/control-flow bodies, and token-level ranges for body diagnostics. Full expression typing and full flow analysis are not complete. |
| VM | partial | `oaer exec` runs the supported anonymous Apex subset with variables, expressions, methods/classes, namespace-qualified class names, constructors, `this(...)`/`super(...)` constructor chaining, interface/enum/abstract instantiation guards, abstract method invocation guards, non-void method fallthrough guards, overloaded method/constructor selection by argument types with numeric, class/interface, null, and ambiguity specificity baselines, instance/static fields, property accessor bodies, source-ordered field initializer expressions and initializer blocks, static reset through field initializers and static blocks, inheritance/super dispatch including superclass-typed and interface-typed references, declaring-class `super` method dispatch, inherited concrete methods ahead of interface fallback methods, inherited static fields and methods through subclass names, interface fallback method lookup, namespace-token SObject/field aliases through construction, access, DML, and SOQL, nested classes with constructors, fields, methods, static members, relative owner-local type names, nested interfaces, nested enum values/methods, explicit object `toString()` dispatch for calls/debug/assert messages, user object identity equality, typed coercion for locals, params, returns, fields, collection members, null/object/enum values, numeric widening, and schema-backed DML storage, private/protected visibility through inheritance chains, `@TestVisible` method access from test classes, and namespace-global class/member access, collections, for/enhanced-for/do-while/switch with switch-local break, ordered catch blocks, pipe-style multi-catch, try/catch/finally including return and throw unwinding, finally-preserved and finally-overridden loop signals, bare rethrow with original stack preservation, catchable null dereference, interface-based catch matching, common exception hierarchy matching, `System.*Exception` name normalization, exception `getMessage`/`getTypeName`/`getLineNumber`/`getStackTraceString`, trace events, assertions, and common platform APIs. Broader platform API fidelity remains incomplete. |
| Test runner | supported | `oaer test` discovers `@isTest` and legacy test methods, compiles project helper classes/triggers, runs constructor and instance method bodies, executes `@TestSetup` once per class into a setup data snapshot, resets statics, clones org state per test, restores `startTest`/`stopTest` governor windows, scopes `runAs` user/profile/permission identity, enforces the supported mixed-DML guard, drains Queueable, `@future`, Batchable, and Schedulable jobs, records local `AsyncApexJob`/`CronTrigger` state, preserves statement-level assertion/runtime stack frames, and emits console, JSON, and JUnit reports for the supported VM subset. |
| SObject/schema runtime | partial | Runtime SObject values preserve projected fields and explicit nulls; schema describe registry and deterministic key prefixes are available. Apex construction, typed/dynamic field access, `get`/`put` with previous-value return, `isSet`, `clear`, `getPopulatedFieldsAsMap`, common system fields after DML and SOQL projection, `SObjectType.getDescribe`, `fields.getMap`, field describe basics with picklist values, record type describe maps/lists with common `RecordTypeInfo` methods, child relationship describe basics, object-level and field-level `addError` with optional escapeHtml arguments, multi-error `hasErrors`/`getErrors` and DML result shaping, DML Id propagation, and simple parent relationship projection are wired into the VM. Permissions and broader describe/system fields remain incomplete. |
| SOQL | partial | Static SOQL, `Database.query`, and `Database.queryWithBinds` execute in-memory `SELECT fields FROM Object` queries with binds, dynamic binds beside operators, dotted bind paths for frame variables, bind maps, collection binds, projection, `FIELDS(ALL/STANDARD/CUSTOM)`, `TYPEOF` relationship projection, multi-hop parent relationship fields and filters, polymorphic relationship target resolution from multi-reference metadata, equality/inequality/comparison filters with single-field equality index candidates, `AND`/`OR`, `IN`/`NOT IN`, `LIKE`, `NOT`, parenthesized conditions, common date literals, semi-joins, anti-joins, child relationship subqueries, serialized row `attributes.type`/`url` shape, soft-deleted row visibility through `ALL ROWS`, comma-separated `ORDER BY ASC/DESC` with `NULLS FIRST/LAST`, `LIMIT`, `OFFSET`, `FOR UPDATE` local lock markers and contention errors for already locked local rows, `WITH SECURITY_ENFORCED`, `WITH USER_MODE`, `WITH SYSTEM_MODE` parsing with local projection validation, `COUNT()`, `COUNT(field)`, `COUNT_DISTINCT`, `SUM`, `MIN`, `MAX`, `AVG`, `GROUP BY`, `ROLLUP`, `CUBE`, `HAVING` on aggregate expressions, aggregate aliases, `GROUPING(field)`, `AggregateResult` `exprN` fields, single-SObject assignment, and catchable `QueryException` parse errors. Full permission enforcement and broader polymorphic relationship edge cases are not complete. |
| DML/transactions/triggers | partial | `internal/dml` and the VM support Apex insert/update/delete/upsert/undelete/merge syntax, `Database.*` allOrNone results, `SaveResult`/`UpsertResult`/`MergeResult` objects with structured single and multi-entry `Database.Error` values, required/unknown field validation, deterministic IDs, common system field stamping, soft delete/undelete query visibility, implicit and explicit external-ID upsert, ID/object mismatch checks, unique fields, lookup reference validation, simple Metadata API validation rules, merge loser soft-delete with child lookup reparenting, upsert insert/update trigger contexts, merge master update hooks, merge duplicate delete hooks, cascade soft delete from relationship metadata, rollback snapshots, and before/after trigger invocation with operation flags, size, new/old lists, nullable unavailable contexts, newMap/oldMap, after-undelete context without before-undelete invocation, bulk partial-success row alignment before and after engine validation, deterministic recursion guard rollback, and object-level and field-level `addError` row failures. Complex validation-rule formulas, full merge loser relationship result details, and complete bulk ordering remain incomplete. |
| Governor limits | partial | The VM tracks SOQL queries/rows including projected child relationship rows, DML statements/rows including cascade-delete child rows, deterministic live heap approximation across locals and mutated collections, deterministic CPU cost from statements plus SOQL/DML row work, callouts, aggregate async jobs, future calls, queueable jobs, batch jobs, scheduled jobs, and email invocations. Supported `Limits` getters expose current and max SOQL, DML, heap, CPU, callout, aggregate async, future, queueable, batch, scheduled, and email counters. Strict/permissive limit modes are available through CLI exec/test/server surfaces and compatibility exec/test fixtures. Exact Salesforce accounting and all platform-specific counters are not complete. |
| Storage/fixtures/persistence | partial | SQLite-backed org storage persists object definitions, records, and ID sequences with schema migrations/versioning and rebuilds runtime index sets from index definitions. `oaer db seed/reset/export/inspect` is wired to fixture JSON with alias and relationship-reference resolution, deterministic users/profiles/permissions, schema-version inspection, DB lifecycle compatibility coverage, and server-backed fixture/scoped reset endpoints. Large-fixture performance tuning and richer permission semantics remain incomplete. |
| DAP | partial | `internal/dap` handles DAP framing plus setBreakpoints, continue/pause/next, stackTrace with trace-provided line/column positions, scopes, variables, and evaluate against VM snapshots. `oaer exec --debug` and `oaer test --debug` start DAP snapshot sessions. Live VM suspension, breakpoint-driven execution control, and step-in/out fidelity are not complete. |
| LSP | partial | `oaer lsp` runs a stdio LSP transport for initialize/shutdown, shared `oaer check` diagnostics, open-buffer parse overlays, test-result diagnostics, incremental document sync, symbols, semantic tokens, definition, references, rename, hover, and Apex/SObject/member/field/keyword completion from the project index. Deeper context-aware completion is not complete. |
| Watch mode | partial | `oaer test --watch` supports native `fsnotify` watching with polling fallback, debounce, JSON event stream with backend and run IDs, incremental Apex-only re-indexing, dependency-graph affected-test selection, reruns, and context cancellation. In-flight VM cancellation is not complete. |
| Native profile analysis | partial | `oaer profile analyze` reads native Chrome Trace Event output and emits ranked JSON or Markdown reports with statement/method/SOQL/DML categories, source offsets, statement line/column ranges, and SOQL/DML row deltas. No external apexrr dependency is used. pprof-compatible CPU output and wall-clock attribution are not complete. |
| Local API server | partial | `oaer server` starts a Salesforce-shaped REST baseline with data version, SObject CRUD, normal REST JSON payload decoding, query/queryAll, describe/recent, limits, OAuth userinfo/id stubs, Tooling `executeAnonymous` with `--limit-mode`, composite sObject insert, optional SQLite persistence through `--db`, OAER fixture/scoped reset endpoints, and Salesforce-shaped error arrays. Full auth, Tooling object coverage, Composite Graph, Bulk API, and broader REST resources are not complete. |

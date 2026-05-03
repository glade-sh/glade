# Feature Parity Todo

Status date: 2026-05-02.

This is the remaining work to get `oaer` to credible feature parity with aer,
then beyond it. The current baselines are broad, but `oaer compat mvp` is still
expected to report not ready until every required capability is supported and
covered by compatibility fixtures.

Parity means a local Apex development loop that can run real project tests,
execute anonymous Apex, support schema-aware SObjects/SOQL/DML/triggers,
enforce or report limits, provide usable debug/LSP/watch workflows, and expose
a Salesforce-shaped local API server without silently wrong behavior.

## Suggested Completion Order

1. Runtime fidelity: method-body sema, class/object execution, exceptions,
   properties, statics, namespaces, and no-panic VM behavior.
2. Test fidelity: transaction isolation, `@TestSetup`, static reset,
   start/stop windows, `runAs`, async drain, and assertion stack traces.
3. Data fidelity: SObject, SOQL, DML, triggers, rollback, and result/error
   shape coverage.
4. Limits and platform APIs: counters, strict/permissive enforcement, and
   common standard-library surfaces.
5. Fixtures and persistence: large SQLite-backed fixtures, deterministic
   platform data, seed/reset/export/import, and server state reset.
6. Developer experience: debug pause hooks, LSP completeness, watch
   cancellation, and native trace/profile reports.
7. Local API server: auth/user context, broader REST/Tooling/Composite
   resources, persistence, and error fidelity.
8. Compatibility and release: dashboard, black-box/enterprise fixtures,
   no-panic hardening, benchmarks, release artifacts, install docs, and
   known-gaps docs.
9. Beyond parity: query plans, cost attribution, anonymization, replay bundles,
   SARIF, API-versioned dashboard, plugins, fuzzing, and mutation testing.

## Parity Gate

- [ ] Make every `requiredForMVP` capability in `internal/capability`
  `supported`.
- [ ] Keep `oaer compat mvp` as the release gate for calling the project
  MVP-ready.
- [ ] Require compatibility coverage before changing a feature from `partial`
  to `supported`.
- [ ] Treat panics on user Apex, metadata, fixtures, or API requests as release
  blockers.
- [ ] Treat silent wrong behavior as a release blocker for any supported
  feature.

## 1. Apex Front End

- [x] Build method-body semantic analysis beyond declaration/member type
  references.
- [x] Model local variables, scopes, expressions, statements, method calls, and
  constructor calls in sema.
- [x] Add an IR-backed method-body sema pass for scoped local reads across
  declarations, assignments, conditions, returns, calls, loops, switch, and
  try/catch/finally bodies.
- [x] Extend the IR-backed sema pass with condition Boolean checks and scoped
  declaration, assignment, and return type checks.
- [x] Diagnose non-void method bodies where not all IR control-flow paths return
  or throw.
- [x] Validate known user-object field reads and writes in the IR-backed sema
  pass, including inherited fields.
- [x] Validate known receiver and same-class method calls in the IR-backed sema
  pass for unknown methods and argument type mismatches.
- [x] Validate constructor calls in the IR-backed sema pass for unknown types,
  non-instantiable types, and argument mismatches.
- [x] Diagnose simple local initializer and assignment type mismatches in sema.
- [x] Diagnose simple return type mismatches in sema.
- [x] Reject non-void method fallthrough in sema and the VM.
- [x] Infer simple binary expression types in sema for numeric, string,
  comparison, and boolean operators.
- [x] Resolve overloads with Apex-compatible conversion and specificity rules.
- [x] Add a numeric overload/widening baseline for `Integer` to `Long`,
  `Decimal`, and `Double` in sema and VM coverage.
- [x] Choose exact and narrowest numeric overloads ahead of wider candidates in
  sema return inference and VM dispatch.
- [x] Choose the nearest class/interface overload ahead of broader ancestors and
  `Object` in sema return inference and VM dispatch.
- [x] Diagnose ambiguous overloads instead of selecting by registration order
  when candidates are pairwise incomparable.
- [x] Resolve `null` overload calls to the most specific applicable parameter
  type when one candidate is strictly narrower.
- [x] Infer decimal literal argument types in sema method-call matching.
- [x] Enforce a class/interface object assignability baseline for local
  declarations, assignments, returns, and method calls in sema.
- [x] Infer known method-call return types for receiver and chained constructor
  calls in sema.
- [x] Resolve inherited members, interface members, virtual/override methods,
  and `super` references in sema.
- [x] Include inherited instance fields in method-body sema scopes.
- [x] Infer `this`/`super` field and method return types for assignments and
  returns in the IR-backed sema pass.
- [x] Infer interface method calls and superclass-typed virtual method calls from
  the compile-time receiver type.
- [x] Diagnose invalid `override` markers and missing concrete
  interface/abstract method implementations.
- [x] Diagnose constructor calls that instantiate interfaces, enums, and
  abstract classes.
- [x] Add private/protected method-call visibility diagnostics for supported
  same-class and subclass references.
- [x] Resolve protected method visibility through superclass chains in sema and
  the VM.
- [x] Allow `@TestVisible` method access from test classes in sema and the VM.
- [x] Enforce visibility: `private`, `protected`, `public`, `global`, test
  visibility, and package boundaries.
- [x] Diagnose private/protected field visibility for known user-object field
  reads in method-body sema.
- [x] Require global class and member access across runtime namespace
  boundaries, including namespace-qualified constructors.
- [x] Resolve namespaces for managed-package style references, custom metadata,
  custom objects, custom fields, and package-local symbols.
- [x] Resolve namespace-token schema aliases like `pkg__Thing__c` to local
  `Thing__c` metadata in sema when the project namespace is `pkg`.
- [x] Resolve namespace-token custom object and field aliases through VM
  SObject construction, field access, DML validation, and SOQL projection/where
  clauses.
- [x] Preserve stable source ranges through parser, sema, VM, test failures,
  LSP, DAP, and trace/profile events.
- [x] Include offsets on parser syntax diagnostics instead of line/column-only
  parse errors.
- [x] Preserve original file line/column positions for compiled project method
  and trigger bodies.
- [x] Emit statement trace line/column alongside source offsets so DAP and
  profile reports can consume real source positions.
- [x] Attach statement-level source positions to VM assertion/runtime stacks and
  test failure reports.
- [x] Add large-project compatibility fixtures that prove parse/index/check
  behavior across enterprise repositories.
- [x] Ensure unsupported syntax and semantic features return stable diagnostics
  instead of parser/VM panics.

## 2. Apex Runtime Core

- [x] Complete class and instance execution fidelity for real service/domain
  classes.
- [x] Finish properties, getters/setters, initializer blocks, static
  initializers, static field ordering, and static reset behavior.
- [x] Execute static and instance field initializer expressions in source order,
  interleaved with initializer blocks and reset behavior.
- [x] Complete constructor chaining, default constructors, overloaded
  constructors, and `this(...)`/`super(...)` behavior.
- [x] Complete virtual dispatch, overrides, interfaces, abstract classes, and
  inherited member lookup.
- [x] Add runtime virtual dispatch coverage through superclass-typed and
  interface-typed references.
- [x] Resolve `super` method calls from the declaring class, not the runtime
  receiver class.
- [x] Prefer inherited concrete methods before interface fallback methods.
- [x] Resolve inherited static fields and static methods through subclass names.
- [x] Block abstract method invocation at runtime.
- [x] Reject interface, enum, and abstract-class instantiation in the VM.
- [x] Add enum method baselines and explicit user object `toString()` dispatch.
- [x] Use user object `toString()` for debug and assertion message display.
- [x] Add qualified nested type symbols and a nested class method/static member
  execution baseline.
- [x] Pin user object identity equality behavior.
- [x] Implement inner classes, nested types, and user object values with
  Salesforce-like equality/debug behavior.
- [x] Resolve relative nested type names inside owning classes for constructors,
  declarations, returns, and implemented interfaces.
- [x] Execute nested class constructors, instance fields, methods, static
  methods, and static fields through qualified and relative references.
- [x] Execute nested interfaces and nested enum values/methods, including chained
  enum member calls such as `Outer.Choice.Two.name()`.
- [x] Preserve identity equality for nested user objects and existing
  user-object debug/toString behavior.
- [x] Complete exception hierarchy semantics, typed catch matching, multi-catch
  behavior, rethrow behavior, stack traces, and file/line reporting.
- [x] Support ordered multiple `catch` blocks in addition to pipe-style
  multi-catch clauses.
- [x] Normalize `System.*Exception` names against unqualified Apex catch types.
- [x] Preserve original throw stacks across catch/rethrow and expose
  `getTypeName`, `getLineNumber`, and `getStackTraceString`.
- [x] Complete control-flow edge cases for loops, `switch`, `break`,
  `continue`, `return`, `finally`, and exception unwinding.
- [x] Cover `finally` execution across return, return override, and uncaught
  throw unwinding in the VM.
- [x] Treat `break` inside `switch` as switch-local while still propagating
  `continue`, `return`, and `throw` to surrounding loops/methods.
- [x] Preserve and override loop signals through `finally`, including
  break/continue preservation, continue overriding break, and finally-thrown
  exceptions overriding pending returns.
- [x] Cover enhanced-for break/continue/finally signal behavior.
- [x] Implement access modifiers and namespace/package boundaries at runtime,
  not only in sema.
- [x] Support Apex numeric, decimal, boolean, string, collection, null, enum,
  and object coercion rules closely enough for enterprise code.
- [x] Coerce declared locals, method params/returns, object fields,
  collection members, and schema-backed DML storage values through a shared VM
  assignability path.
- [x] Reject invalid String/Boolean, narrowing numeric, collection generic, and
  schema field coercions in VM/sema coverage.
- [x] Enforce a class/interface object assignability baseline for VM locals,
  fields, params, returns, and overload matching.
- [x] Add no-panic guards around all VM execution paths for malformed or
  unsupported user code.

## 3. Apex Test Semantics And Async

- [x] Make `@TestSetup` match Salesforce transaction behavior exactly,
   including setup data visibility and rollback.
- [x] Run `@TestSetup` once per test class into an org snapshot, then clone that
  snapshot for each test method.
- [x] Restore governor windows around `Test.startTest()` and `Test.stopTest()`
   with Salesforce-compatible counter behavior.
- [x] Preserve pre-`startTest` counters, reset the inner window, drain async work
  at `stopTest`, and restore the outer counter window for post-stop code.
- [x] Complete per-test transaction rollback for all storage mutations,
   triggers, async jobs, and platform side effects.
- [x] Complete static reset behavior across test methods, test setup, async
   drain, and nested execution.
- [x] Reset statics before each drained Queueable job so async execution starts
  with a fresh transaction-shaped static state.
- [x] Complete `System.runAs` user/profile identity behavior for supported local
  test modes.
- [x] Complete broader `System.runAs` permission, sharing, and mixed-DML
  enforcement for supported modes.
- [x] Scope `FeatureManagement.checkPermission` to supported `runAs` user
  permission lists, enforce mixed-DML guards, and pin local tests to
  system-sharing mode.
- [x] Implement `@future` execution and stopTest drain behavior.
- [x] Implement Batchable execution, batch chunking, finish behavior, and
   observable async records where useful.
- [x] Implement Schedulable execution and direct scheduling model.
- [x] Drain Queueable jobs at `Test.stopTest()` with deterministic job IDs,
  error propagation, and fresh async static state.
- [x] Complete Queueable chaining limits and durable async job state where useful.
- [x] Improve assertion failures and runtime errors with precise file/line
   stack traces.
- [x] Add enterprise test fixtures for trigger-heavy, selector/service/domain,
  async-heavy, describe-heavy, and namespace-heavy projects.
- [x] Add an async-heavy compatibility test fixture that covers future, batch,
  schedule, chained Queueable, `AsyncApexJob`, and `CronTrigger` behavior.

## 4. SObjects, SOQL, DML, And Triggers

- [x] Complete typed SObject field access, dynamic `get`/`put`, absent-field
  behavior, explicit null behavior, and system fields.
  - [x] Support `SObject.put` previous-value returns, `isSet`, `clear`, and
    `getPopulatedFieldsAsMap` with explicit-null field tracking.
  - [x] Populate and expose common system fields (`CreatedDate`, `CreatedById`,
    `LastModifiedDate`, `LastModifiedById`, `SystemModstamp`, `OwnerId`, and
    `IsDeleted`) on DML-mutated and SOQL-projected SObjects.
- [ ] Complete schema describe objects, field describes, record type info,
  picklists, relationship metadata, and common describe-heavy code paths.
  - [x] Load Metadata API picklist values and expose `SObjectField.getDescribe`
    with common field metadata plus `getPicklistValues` entries.
  - [x] Load Metadata API record type files and expose
    `DescribeSObjectResult.getRecordTypeInfos`,
    `getRecordTypeInfosByName`, `getRecordTypeInfosByDeveloperName`, and common
    `RecordTypeInfo` methods with deterministic local record type IDs.
- [x] Expand static SOQL parsing/execution with `AND`/`OR`, `IN`/`NOT IN`,
  `LIKE`, comparison operators, `NOT`, and parenthesized conditions.
  - **Limitation**: Apex compiler does not support chained method calls
    (e.g., `obj.getErrors().get(0)`); intermediate variables are required.
  - **Limitation**: SOQL string literals inside Apex `[SELECT ...]` were missing
    quotes due to a compiler lexer bug; this has been fixed, but complex string
    escapes inside SOQL literals may still have edge cases.
- [ ] Complete dynamic SOQL binding and runtime parse/error behavior for
  `Database.query`.
  - [x] Support dynamic binds beside operators, dotted bind paths, collection
    binds, date literal colons, and catchable `QueryException` parse errors.
- [x] Add relationship child subqueries.
  - [x] Support child relationship query projection with metadata-driven
    relationship names, child filters, ordering, limits, and VM list row shape.
- [ ] Expand parent relationship traversal and polymorphic relationship
  behavior.
  - [x] Support multi-hop parent relationship fields and filters, including VM
    nested SObject row projection.
- [x] Add aggregates: `COUNT`, `COUNT(field)`, `COUNT_DISTINCT`, `SUM`, `MIN`,
  `MAX`, `AVG`, `GROUP BY`, `ROLLUP`, `CUBE`, and `HAVING`.
  - [x] Support no-`GROUP BY` `COUNT()`, `COUNT(field)`, `COUNT_DISTINCT`,
    `SUM`, `MIN`, `MAX`, and `AVG` with `AggregateResult.exprN` fields.
  - [x] Support `GROUP BY`, `HAVING` on aggregate expressions, grouped field
    projection, and grouped result ordering/limits for aggregate rows.
  - [x] Support aggregate aliases on `AggregateResult` rows while preserving
    `exprN` fields.
  - [x] Support `ROLLUP`, `CUBE`, and `GROUPING(field)` subtotal metadata.
- [x] Add complex predicates: `IN`, `NOT IN`, `LIKE`, boolean combinations,
  null semantics, and comparison operators (`>`, `<`, `>=`, `<=`).
  - [x] Support common date literals including `TODAY`, `YESTERDAY`,
    `TOMORROW`, `LAST_N_DAYS:n`, `NEXT_N_DAYS:n`, and month/year ranges.
  - [x] Support semi-joins and anti-joins with single-field subqueries in
    `IN`/`NOT IN` predicates.
  - [x] Match SOQL `LIKE` and `NOT LIKE` ASCII letters case-insensitively.
  - [x] Support comma-separated `ORDER BY ASC` and `ORDER BY DESC` for normal,
    aggregate, and child relationship query rows.
  - [x] Support explicit `NULLS FIRST` and `NULLS LAST` ordering modifiers.
  - [x] Support `FIELDS(ALL)`, `FIELDS(STANDARD)`, and `FIELDS(CUSTOM)` field
    projection expansion.
  - [x] Parse and execute `FOR UPDATE` as a local lock marker.
  - [x] Support `ALL ROWS` queries that include soft-deleted records.
  - [x] Parse and execute `WITH SECURITY_ENFORCED`, `WITH USER_MODE`, and
    `WITH SYSTEM_MODE` as local security-mode markers.
  - [x] Support baseline `TYPEOF` relationship projection for parent lookup
    branches.
  - **Limitation**: Formula-adjacent predicate behavior remains incomplete.
- [ ] Add SOQL features commonly used by real projects: security enforcement,
  lock contention behavior, and advanced query row shape fidelity.
- [ ] Wire SQLite planning or indexed execution where needed without changing
  Salesforce-visible behavior.
- [ ] Complete Apex DML statements: `insert`, `update`, `delete`, `upsert`,
  `undelete`, and `merge`.
  - [x] Support soft delete visibility and undelete restoration for VM/SOQL
    paths.
  - [x] Support baseline `merge` statement and `Database.merge` execution with
     duplicate soft delete, child lookup reparenting, and `MergeResult` shape.
  - [x] Fire supported merge trigger hooks for master `before/after update` and
    duplicate `before/after delete` contexts with rollback on trigger errors.
- [x] Improve `Database.insert/update/delete/upsert/undelete` result fidelity
  with structured `Database.Error` objects carrying `statusCode`, `message`, and
  `fields` arrays; add `Database.UpsertResult.isCreated()`.
  - [x] Preserve multiple object-level and field-level `addError` calls as
    multiple `Database.Error` entries on `SaveResult`/`MergeResult`.
  - [x] Cascade soft-delete child records from relationship metadata.
  - **Limitation**: Full merge loser relationship result details and full
    undelete edge-case parity remain incomplete.
  - **Limitation**: The VM `Database.Error` shape covers the most common status
    codes; full Salesforce status-code parity is not yet complete.
- [x] Complete external-ID upsert and ID/object mismatch behavior.
  - [x] Support implicit external-ID matching for upsert when an external ID field
    is populated and reject ID/object key-prefix mismatches.
  - [x] Support explicit `upsert rows Field__c` and
    `Database.upsert(rows, Field__c, ...)` field-token overloads.
- [ ] Implement validation rules, required fields, uniqueness, foreign-key
  behavior, and relationship constraints where representable locally.
  - [x] Enforce required/unknown fields, unique fields, lookup reference
    existence, and restricted-delete lookup constraints.
  - **Limitation**: Formula-backed validation rules, owner/sharing side effects,
    and broad relationship constraints remain incomplete.
- [ ] Complete trigger ordering, before/after state, bulk execution,
  recursion behavior, operation type, maps, old/new values, and rollback on
  failures.
  - [x] Support trigger operation flags, `Trigger.size`, nullable unavailable
    contexts, and `Trigger.newMap`/`Trigger.oldMap` for supported operations.
  - [x] Preserve bulk partial-success result alignment when before triggers
    filter failed rows before DML, including after-trigger execution for
    successful rows.
  - [x] Enforce a deterministic trigger recursion depth guard with catchable
    `DmlException` rollback.
- [ ] Implement `addError` behavior on SObjects and fields.
  - [x] Support object-level `SObject.addError`, `hasErrors`, and `getErrors`
    in before-trigger DML with row-level `SaveResult` error shaping and
    all-or-none rollback.
  - [x] Support field-level `someRecord.Field__c.addError(...)` with
    `Database.Error.getFields()` attribution.
  - [x] Preserve multiple addError calls as multiple `Database.Error` entries.
  - **Limitation**: Advanced addError overload parity remains incomplete.
- [ ] Add trigger fixtures covering insert/update/delete/upsert/undelete,
  all-or-none failures, partial success, recursion, and bulk batches.
  - [x] Add compatibility fixture coverage for failed-first bulk insert partial
    success, before-trigger mutation, and after-trigger execution.
  - [x] Add compatibility fixture coverage for recursive trigger limit rollback.

## 5. Governor Limits And Platform APIs

- [ ] Make SOQL query and row counters Salesforce-compatible for supported
  query paths.
- [ ] Make DML statement and row counters Salesforce-compatible for supported
  DML paths.
- [ ] Improve heap size approximation and expose predictable diagnostics for
  unsupported heap fidelity.
- [ ] Improve CPU accounting beyond statement counts while keeping runs
  deterministic.
- [ ] Complete callout, email, async, queueable, future, batch, and scheduled
  counters.
  - [x] Track separate future, queueable, batch, scheduled, and email invocation
    counters while preserving the aggregate async job counter.
  - [x] Expose supported public `Limits` getters for aggregate async jobs,
    future calls, queueable jobs, and email invocations with max values.
- [ ] Add configurable strict/permissive limit modes for CLI, tests, server,
  and compatibility fixtures.
- [ ] Complete `System`, `Test`, `Database`, `Schema`, `Limits`, and `JSON`
  APIs used by enterprise tests.
- [ ] Complete common `String`, `Pattern`, `Matcher`, `Date`, `Datetime`,
  `Time`, `Math`, `Decimal`, `EncodingUtil`, and `Crypto` behavior.
- [ ] Complete HTTP/callout mock behavior: request/response types,
  `HttpCalloutMock`, callout limits, and test isolation.
- [ ] Complete `UserInfo`, `FeatureManagement`, `Messaging`, `ApexPages`,
  `URL`, and `PageReference` basics.
- [ ] Add stable unsupported-feature errors for every unimplemented standard
  library method.
- [ ] Generate and publish a standard-library coverage matrix.

## 6. Storage, Fixtures, And Persistence

- [ ] Performance-tune SQLite-backed storage for large fixture sets.
- [x] Add migrations/versioning for persistent databases.
  - [x] Add a SQLite migration runner backed by `PRAGMA user_version`, record
    applied migrations, and expose schema version in DB inspection summaries.
- [ ] Add stronger transaction boundaries across CLI tests, server requests,
  DML failures, triggers, and async drains.
- [ ] Complete fixture alias resolution for polymorphic and relationship-heavy
  data.
- [ ] Expand deterministic platform data for users, profiles, roles,
  permission sets, permission assignments, record types, and org settings.
- [ ] Add fixture reset endpoints that can reset data, users, platform state,
  limits, and async queues deterministically.
- [ ] Add persistent server database lifecycle docs and operational checks.
- [ ] Add import/export compatibility tests for `oaer db seed/reset/export/
  inspect`.
  - [x] Add a DB lifecycle compatibility fixture that seeds SQLite storage,
    inspects schema/data counts, exports the fixture shape, and verifies reset
    behavior.
- [ ] Add fixture schemas for enterprise selector/service/domain test suites.

## 7. Developer Experience

- [ ] Add true live VM pause hooks for DAP at stable source locations.
- [ ] Make breakpoints drive execution rather than only serving debug
  snapshots.
- [ ] Complete DAP stepping: step in, step over, step out, pause, continue, and
  disconnect semantics.
- [ ] Complete DAP scopes and variable rendering for SObjects, user objects,
  statics, collections, exceptions, and trigger context.
- [ ] Complete watch expression evaluation against VM context.
- [x] Add VS Code launch/task examples and editor documentation.
- [ ] Expand `oaer lsp` with incremental document sync.
- [ ] Add LSP semantic tokens, definition, references, rename, and richer
  completion.
- [ ] Make LSP diagnostics match `oaer check` and test results consistently.
- [ ] Add native OS watcher backends for `oaer test --watch`.
- [ ] Add incremental re-indexing and affected-test dependency graph updates.
- [ ] Add in-flight VM/test cancellation for watch reruns.
- [ ] Stabilize watch JSON stream for editor/test UI consumers.
- [ ] Expand profile/trace events for SOQL, DML, describe, callouts, limits,
  methods, triggers, and async.
- [ ] Add native reports that fully replace apexrr-style analysis for local
  runtime data.

## 8. Local API Server

- [ ] Complete auth/user context stubs enough for local integrations.
- [ ] Expand Salesforce-like error response shapes and status codes.
- [ ] Complete `/services/data` resource discovery for commonly used REST
  resources.
- [ ] Expand SObject REST resources: describe, layout-adjacent metadata where
  useful, recent, query, queryAll, and record CRUD edge cases.
- [ ] Expand Tooling API coverage beyond `executeAnonymous` and query
  delegation.
- [ ] Add more REST resources used by local integrations and editor tooling.
- [ ] Add Composite API coverage beyond baseline sObject insert, including
  all-or-none rollback and reference ID behavior.
- [ ] Add Bulk API approximations if needed by local integration tests.
- [ ] Ensure anonymous Apex runs against the same persistent server database,
  transaction boundaries, user context, and limits.
- [ ] Add server fixture reset endpoints for test data, org state, limits, and
  async queues.
- [ ] Add black-box server compatibility fixtures for CRUD, query,
  executeAnonymous, composite, errors, auth stubs, and persistence.

## 9. Compatibility, Hardening, And Release

- [x] Generate a public compatibility dashboard from `internal/capability`.
- [x] Add CI gates for compatibility matrix drift and MVP readiness.
- [ ] Build black-box fixtures against Salesforce behavior for every supported
  language/runtime/data/server feature.
  - [x] Add storage DB lifecycle coverage to the compatibility fixture runner.
- [ ] Add enterprise fixtures for trigger-heavy, selector/service/domain,
  async-heavy, describe-heavy, namespace-heavy, and package-style projects.
- [x] Add fixture coverage for unsupported-feature diagnostics so failures are
  stable and intentional.
- [x] Add panic recovery and no-panic tests around parser, sema, VM, SOQL, DML,
  test runner, watcher, LSP, DAP, fixture loading, and server routes.
- [x] Add benchmarks for parser, project indexing, sema, tests, SOQL, DML,
  triggers, storage seed/export, server routes, LSP, and watch mode.
- [ ] Add stress tests for large projects, large fixtures, bulk DML, and
  describe-heavy execution.
- [x] Add release binaries for supported platforms.
- [x] Add checksums and signed or verifiable release artifacts.
- [x] Add install docs for Homebrew/manual/CI usage.
- [x] Add known-gaps docs generated from the compatibility matrix.
- [x] Add upgrade/release notes and compatibility policy by API version.
- [x] Add smoke tests that install the built binary and run parser, exec, test,
  db, server, lsp diagnostics, profile, and compat commands.

## Beyond Parity

These should come after the core runtime is credible and the parity gate is
green.

- [ ] First-class query plan reporting for SOQL.
- [ ] Per-statement cost attribution for SOQL, DML, describe, callouts, limits,
  triggers, async, and validation behavior.
- [ ] Fixture anonymizer for exporting useful local fixtures without leaking
  sensitive data.
- [ ] Deterministic replay bundles containing source, metadata, fixtures,
  clock, user context, limits mode, command, and trace data.
- [ ] SARIF output for CI findings from parser, sema, compatibility, limits,
  and profiling checks.
- [ ] Compatibility dashboard by Salesforce API version.
- [ ] Plugin-style platform API extensions for project-specific or
  package-specific APIs.
- [ ] Fuzz testing for parser, sema, VM, SOQL, DML, fixture loading, and server
  request handling.
- [ ] Mutation testing for VM, SOQL, DML, triggers, and test semantics.
- [ ] Query-plan regression tracking across fixture/database changes.
- [ ] Per-statement optimization suggestions for SOQL/DML/describe-heavy Apex.
- [ ] Replayable performance budgets for CI.
- [ ] Optional alternate persistence backends for larger shared CI fixtures.
- [ ] Rich compatibility reports that explain why a project is blocked and
  which unsupported features are highest impact.

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
- [x] Auto-register `docs/fixtures/*.json` in `internal/compat` tests so
  parallel fixture additions no longer collide in one-off test functions.

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
- [x] Pin unsupported `AsyncOptions` mutator/accessor calls with typed
  diagnostics and black-box fixture coverage.
- [x] Pin unsupported queueable finalizer registration/context getters with
  typed diagnostics and black-box fixture coverage.
- [x] Support local `System.abortJob` for queued Queueable/Scheduled test jobs
  before `Test.stopTest`, with typed diagnostics for completed/unknown aborts.

## 4. SObjects, SOQL, DML, And Triggers

- [x] Complete typed SObject field access, dynamic `get`/`put`, absent-field
  behavior, explicit null behavior, and system fields.
  - [x] Support `SObject.put` previous-value returns, `isSet`, `clear`, and
    `getPopulatedFieldsAsMap` with explicit-null field tracking.
  - [x] Populate and expose common system fields (`CreatedDate`, `CreatedById`,
    `LastModifiedDate`, `LastModifiedById`, `SystemModstamp`, `OwnerId`, and
    `IsDeleted`) on DML-mutated and SOQL-projected SObjects.
- [x] Complete schema describe objects, field describes, record type info,
  picklists, relationship metadata, and common describe-heavy code paths.
  - [x] Support `SObjectType.getDescribe`, `DescribeSObjectResult.fields.getMap`,
    and child relationship describe basics.
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
- [x] Complete dynamic SOQL binding and runtime parse/error behavior for
  `Database.query`.
  - [x] Support dynamic binds beside operators, dotted bind paths, collection
    binds, date literal colons, and catchable `QueryException` parse errors.
  - [x] Support `Database.queryWithBinds` with map-provided scalar and collection
    binds plus catchable missing-bind errors.
- [x] Add relationship child subqueries.
  - [x] Support child relationship query projection with metadata-driven
    relationship names, child filters, ordering, limits, and VM list row shape.
- [x] Expand parent relationship traversal and polymorphic relationship
  behavior.
  - [x] Support multi-hop parent relationship fields and filters, including VM
    nested SObject row projection.
  - [x] Load multi-target relationship metadata and resolve polymorphic SOQL
    `TYPEOF`/parent projections from the actual referenced record type.
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
- [x] Add SOQL features commonly used by real projects: security enforcement,
  lock contention behavior, and advanced query row shape fidelity.
  - [x] Validate projected fields and parent relationship `WHERE` fields for
    `WITH SECURITY_ENFORCED`, `WITH USER_MODE`, and `WITH SYSTEM_MODE` queries
    and return catchable `QueryException`s for unavailable fields, including
    relationship fields that are not present on every configured parent target.
  - [x] Mark `FOR UPDATE` result records with a local lock marker and serialize
    queried SObjects with `attributes.type` and `attributes.url`.
  - [x] Return catchable `QueryException`s when `FOR UPDATE` hits an already
    locked local row.
  - **Limitation**: Security enforcement is local field-availability validation rather
    than full CRUD/FLS/sharing enforcement.
- [x] Wire SQLite planning or indexed execution where needed without changing
  Salesforce-visible behavior.
  - [x] Rebuild runtime index sets from object index definitions and use
    single-field equality indexes as SOQL candidate sets.
- [x] Complete Apex DML statements: `insert`, `update`, `delete`, `upsert`,
  `undelete`, and `merge`.
  - [x] Support soft delete visibility and undelete restoration for VM/SOQL
    paths.
  - [x] Support baseline `merge` statement and `Database.merge` execution with
    duplicate soft delete, child lookup reparenting, and `MergeResult` shape
    including `getUpdatedRelatedIds()`.
  - [x] Fire supported merge trigger hooks for master `before/after update` and
    duplicate `before/after delete` contexts with rollback on trigger errors.
  - [x] Run after-trigger contexts only for rows that survive partial-success
    engine validation.
- [x] Improve `Database.insert/update/delete/upsert/undelete` result fidelity
  with structured `Database.Error` objects carrying `statusCode`, `message`, and
  `fields` arrays; add `Database.UpsertResult.isCreated()`.
  - [x] Preserve multiple object-level and field-level `addError` calls as
    multiple `Database.Error` entries on `SaveResult`/`MergeResult`.
  - [x] Cascade soft-delete child records from relationship metadata.
  - [x] Return `ENTITY_IS_NOT_DELETED` SaveResult details when undeleting active
    records.
  - [x] Preserve `Database.undelete(..., false)` mixed-row result alignment for
    deleted, active, missing, and ID/object-mismatched rows, with `allOrNone`
    rollback on later undelete failures.
  - [x] Carry merge loser IDs from the DML engine into
    `MergeResult.getMergedRecordIds()`, including partial-success list results.
  - **Limitation**: Broader undelete edge-case parity remains incomplete.
  - **Limitation**: The VM `Database.Error` shape covers the most common status
    codes; full Salesforce status-code parity is not yet complete.
- [x] Complete external-ID upsert and ID/object mismatch behavior.
  - [x] Support implicit external-ID matching for upsert when an external ID field
    is populated and reject ID/object key-prefix mismatches.
  - [x] Support explicit `upsert rows Field__c` and
    `Database.upsert(rows, Field__c, ...)` field-token overloads.
- [x] Implement validation rules, required fields, uniqueness, foreign-key
  behavior, and relationship constraints where representable locally.
  - [x] Enforce required/unknown fields, unique fields, lookup reference
    existence, and restricted-delete lookup constraints.
  - [x] Reject DML writes and explicit nulls to formula-backed calculated fields
    with `INVALID_FIELD_FOR_INSERT_UPDATE` result shaping.
  - [x] Load and enforce simple Metadata API validation rules with stable
    `FIELD_CUSTOM_VALIDATION_EXCEPTION` error shaping.
  - **Limitation**: Complex validation-rule formulas, owner/sharing side effects,
    and broad relationship constraints remain incomplete.
- [x] Complete trigger ordering, before/after state, bulk execution,
  recursion behavior, operation type, maps, old/new values, and rollback on
  failures.
  - [x] Support trigger operation flags, `Trigger.size`, nullable unavailable
    contexts, and `Trigger.newMap`/`Trigger.oldMap` for supported operations.
  - [x] Preserve bulk partial-success result alignment when before triggers
    filter failed rows before DML, including after-trigger execution for
    successful rows.
  - [x] Enforce a deterministic trigger recursion depth guard with catchable
    `DmlException` rollback.
  - [x] Run supported after-undelete trigger contexts while skipping unsupported
    before-undelete invocation.
  - **Limitation**: Complete platform trigger ordering across all automation
    types remains outside the local trigger runtime.
- [x] Implement `addError` behavior on SObjects and fields.
  - [x] Support object-level `SObject.addError`, `hasErrors`, and `getErrors`
    in before-trigger DML with row-level `SaveResult` error shaping and
    all-or-none rollback.
  - [x] Support field-level `someRecord.Field__c.addError(...)` with
    `Database.Error.getFields()` attribution.
  - [x] Preserve multiple addError calls as multiple `Database.Error` entries.
  - [x] Support common optional `escapeHtml` overloads and field `addError` on
    unset-but-valid SObject fields.
  - **Limitation**: UI rendering details for escaped addError text remain
    outside the local runtime.
- [x] Add trigger fixtures covering insert/update/delete/upsert/undelete,
  all-or-none failures, partial success, recursion, and bulk batches.
  - [x] Add compatibility fixture coverage for failed-first bulk insert partial
    success, before-trigger mutation, and after-trigger execution.
  - [x] Add compatibility fixture coverage for recursive trigger limit rollback.
  - [x] Add compatibility fixture coverage for upsert insert/update trigger
    contexts and after-undelete trigger context.

## 5. Governor Limits And Platform APIs

- [x] Make SOQL query and row counters Salesforce-compatible for supported
  query paths.
  - [x] Count projected child relationship rows toward `Limits.getQueryRows()`
    while preserving one query count for the parent SOQL statement.
- [x] Make DML statement and row counters Salesforce-compatible for supported
  DML paths.
  - [x] Count cascade-deleted child records toward `Limits.getDmlRows()` for
    supported relationship metadata.
  - [x] Accept documented `Limits.getDML*` getter casing aliases backed by the
    same tracked DML statement and row counters.
- [x] Improve heap size approximation and expose predictable diagnostics for
  unsupported heap fidelity.
  - [x] Recompute deterministic live heap usage after statements so mutated
    locals and collections are reflected in `Limits.getHeapSize()`.
  - **Limitation**: Byte-exact Salesforce heap accounting remains unsupported.
- [x] Improve CPU accounting beyond statement counts while keeping runs
  deterministic.
  - [x] Add deterministic SOQL and DML row-work costs on top of per-statement
    CPU accounting.
  - **Limitation**: Wall-clock Salesforce CPU parity remains unsupported.
- [x] Complete callout, email, async, queueable, future, batch, and scheduled
  counters.
  - [x] Track separate future, queueable, batch, scheduled, and email invocation
    counters while preserving the aggregate async job counter.
  - [x] Expose supported public `Limits` getters for aggregate async jobs,
    future calls, queueable jobs, and email invocations with max values.
  - [x] Expose supported public `Limits` getters for batch and scheduled jobs
    with max values.
- [x] Add configurable strict/permissive limit modes for CLI, tests, server,
  and compatibility fixtures.
  - [x] Wire `--limit-mode` through `oaer exec`, `oaer test`, and `oaer server`
    Tooling `executeAnonymous`.
  - [x] Add `limitMode` support for compatibility exec/test fixtures.
  - [x] Pin savepoint rollback `Limits` getters as explicit unsupported
    diagnostics until rollback-specific accounting and caps are modeled.
  - [x] Pin publish-immediate DML `Limits` getters as explicit unsupported
    diagnostics until platform event publish accounting and caps are modeled.
- [x] Complete `System`, `Test`, `Database`, `Schema`, `Limits`, and `JSON`
  APIs used by enterprise tests.
  - [x] Add common JSON overloads for `serialize(value, suppressApexObjectNulls)`,
    `serializePretty`, and `deserializeStrict`.
  - [x] Reject duplicate object fields in `JSON.deserializeStrict` before typed
    mapping.
  - [x] Reject unknown SObject fields in `JSON.deserializeStrict` with catchable
    `JSONException` while preserving non-strict `JSON.deserialize` behavior.
  - [x] Map JSON.deserialize SObject fields through local schema field types for
    Date, Datetime, numeric, Boolean, Id, and reference edges.
  - [x] Reject `JSONGenerator.writeFieldName(...)` inside arrays with catchable
    `JSONException` while preserving array writes after the failed call.
  - [x] Reject `JSONGenerator.writeEndObject()` inside arrays with catchable
    `JSONException` while preserving array writes after the failed call.
  - [x] Reject `JSONGenerator.writeEndArray()` inside objects with catchable
    `JSONException` while preserving object writes after the failed call.
  - [x] Reject `JSONGenerator` writes after `getAsString()`/close with
    catchable `JSONException` while preserving the closed generator state.
  - [x] Add common `Test.isRunningTest()` and deterministic
    `Test.getStandardPricebookId()` support.
  - [x] Add `Database.getQueryLocator(String)` for supported SOQL and batch
    start scopes.
  - [x] Add basic `Type.forName(...)` and `Type.newInstance()` factory support.
  - [x] Add `Database.setSavepoint()` and `Database.rollback(...)` for local
    org-state snapshots.
  - [x] Add `Schema.describeSObjects(...)` basics plus local describe access
    booleans for SObjects and fields.
  - **Limitation**: Broader standard-library method parity remains tracked by
    the common stdlib and unsupported-error rows below.
- [x] Complete common `String`, `Pattern`, `Matcher`, `Date`, `Datetime`,
  `Time`, `Math`, `Decimal`, `EncodingUtil`, and `Crypto` behavior.
  - [x] Add common `String` helpers for trim, search, replacement, split/join,
    blank checks, and case-insensitive equality.
  - [x] Pin one-field `String.escapeCsv`/`unescapeCsv` behavior for plain,
    comma, quote, doubled-quote, CR, and LF values.
  - [x] Add deterministic `String.unescapeHtml3/4` coverage for selected
    high-use named entities (`nbsp`, `copy`, `reg`, `trade`, `euro`, `mdash`,
    `ndash`, `hellip`, `bull`, `ldquo`, `rdquo`, `lsquo`, `rsquo`, `cent`,
    `pound`, `yen`, `sect`, `para`, `middot`) while leaving remaining unknown
    names unchanged.
  - [x] Add XML-version escape handling for `String.escapeXml10/11` control
    ranges, including invalid-code-point removal and numeric control escapes.
  - [x] Preserve malformed, null, out-of-range, and surrogate numeric entities in
    `String.unescapeXml*` while unescaping valid decimal and hex references.
  - [x] Add `Pattern.compile`, `Pattern.matches`, and basic `Matcher`
    `find`/`matches`/`group` behavior.
  - [x] Add `Pattern.quote` for local Go-regexp literal matching of common
    metacharacters.
  - [x] Add common `Date`, `Datetime`, and `Time` factories, parsing,
    arithmetic, and component helpers.
  - [x] Add common numeric `Math` helpers, `Decimal` scale/conversion helpers,
    URL encoding helpers, and MD5/SHA1/SHA-256 digest coverage.
  - **Limitation**: Exact locale, broad timezone, rounding-mode, charset, and
    full Java-regex parity remain outside the current local subset. The current
    named-zone slice is limited to deterministic `America/Los_Angeles` and
    `America/New_York` DST formatting, offsets, and current-user timezone
    formatting.
- [x] Complete HTTP/callout mock behavior: request/response types,
  `HttpCalloutMock`, callout limits, and test isolation.
  - [x] Add common `HttpRequest`/`HttpResponse` endpoint, method, header,
    timeout, status, and body/blob accessors.
  - **Limitation**: Local execution remains mock-first; real outbound network
    callout transport is intentionally not modeled.
- [x] Complete `UserInfo`, `FeatureManagement`, `Messaging`, `ApexPages`,
  `URL`, and `PageReference` basics.
  - [x] Add common `UserInfo` org/session/locale/timezone getters, including
    `TimeZoneSidKey` handoff for the modeled UTC, America/Los_Angeles, and
    America/New_York slice.
  - [x] Add `Messaging.SingleEmailMessage` setters and structured
    `SendEmailResult` basics.
  - [x] Add `ApexPages` message storage, current page, `PageReference`, and
    deterministic org `URL` basics.
  - **Limitation**: Full Visualforce navigation/rendering and production session
    semantics remain outside the local VM subset.
- [x] Add stable unsupported-feature errors for every unimplemented standard
  library method.
  - [x] Return typed `UnsupportedFeature` runtime errors for unimplemented
    VM/stdlib calls while preserving fixture-compatible message text.
  - [x] Keep ordinary runtime errors out of unsupported-feature classification.
- [x] Generate and publish a standard-library coverage matrix.
  - [x] Add `oaer compat stdlib` with Markdown, JSON, output, and drift-check
    modes backed by `internal/capability`.
  - [x] Publish generated coverage at `docs/STDLIB_COVERAGE.md`.

## 6. Storage, Fixtures, And Persistence

- [x] Performance-tune SQLite-backed storage for large fixture sets.
  - [x] Reuse prepared inserts inside SQLite save transactions and set
    performance-oriented connection pragmas.
  - [x] Add large-fixture SQLite save/load coverage and a 5,000-record storage
    benchmark.
- [x] Add migrations/versioning for persistent databases.
  - [x] Add a SQLite migration runner backed by `PRAGMA user_version`, record
    applied migrations, and expose schema version in DB inspection summaries.
- [x] Add stronger transaction boundaries across CLI tests, server requests,
  DML failures, triggers, and async drains.
  - [x] Run mutating server requests against cloned org state and commit only
    after successful execution and persistence.
  - [x] Roll back Tooling `executeAnonymous` mutations on runtime errors and
    REST mutations on persistence failures.
  - [x] Serialize server request handling to prevent concurrent clone/commit
    lost updates.
- [x] Complete fixture alias resolution for polymorphic and relationship-heavy
  data.
  - [x] Track fixture aliases with object type and generated ID for every record.
  - [x] Support qualified `Object.alias` refs and reject ambiguous short aliases.
  - [x] Validate `fieldRefs` against reference field target metadata.
- [x] Expand deterministic platform data for users, profiles, roles,
  permission sets, permission assignments, record types, and org settings.
  - [x] Seed deterministic `Organization`, `Profile`, `UserRole`, `User`,
    `PermissionSet`, `PermissionSetAssignment`, and `RecordType` objects.
  - [x] Add locale/timezone/language, role, and permission metadata fields for
    local user/org state.
  - [x] Materialize object record-type metadata as deterministic `RecordType`
    records.
- [x] Add fixture reset endpoints that can reset data, users, platform state,
  limits, and async queues deterministically.
  - [x] Support full reset plus scoped `data`, `users`, `platform`, `limits`,
    and `async` reset requests through path, query, and JSON body scopes.
  - [x] Keep data resets from clearing deterministic platform users/org state.
  - [x] Rebuild user/platform baseline records deterministically for users and
    platform resets.
- [x] Add persistent server database lifecycle docs and operational checks.
  - [x] Document `oaer server --db` startup, DB seed/inspect/export/reset
    preparation, server fixture/reset endpoints, and restart persistence checks.
  - [x] Document operational checks for saved mutations and rollback-on-failure
    commit boundaries.
- [x] Add import/export compatibility tests for `oaer db seed/reset/export/
  inspect`.
  - [x] Add a DB lifecycle compatibility fixture that seeds SQLite storage,
    inspects schema/data counts, exports the fixture shape, and verifies reset
    behavior.
  - [x] Re-import the exported fixture during compatibility execution and assert
    imported record, user, profile, and Account counts.
- [x] Add fixture schemas for enterprise selector/service/domain test suites.
  - [x] Extend compatibility `check` fixtures to write schema metadata files,
    load project schema, and report schema object counts.
  - [x] Add an enterprise selector/service/domain fixture with Account, Contact,
    and custom Invoice metadata.

## 7. Developer Experience

- [x] Add true live VM pause hooks for DAP at stable source locations.
  - [x] Add VM debug hooks that pause on statement line/column locations,
    expose stack/locals snapshots, support step pauses, and allow stop/continue
    actions.
- [x] Make breakpoints drive execution rather than only serving debug
  snapshots.
  - [x] Convert DAP breakpoints into VM debug breakpoints and add a DAP
    execution helper that runs to the first live breakpoint before the statement
    executes.
- [x] Complete DAP stepping: step in, step over, step out, pause, continue, and
  disconnect semantics.
  - [x] Add a live DAP session that blocks a VM goroutine at pauses and releases
    it through continue, pause, disconnect, step-in, step-over, and step-out
    commands.
  - [x] Use stack depth so step-over skips method bodies, step-in enters method
    bodies, and step-out returns to the caller frame.
- [x] Complete DAP scopes and variable rendering for SObjects, user objects,
  statics, collections, exceptions, and trigger context.
  - [x] Split DAP variables into Locals, Statics, and Trigger scopes, render
    object/SObject/exception fields as named child variables, preserve nested
    collection children, and expose static class fields from live VM pauses.
- [x] Complete watch expression evaluation against VM context.
  - [x] Evaluate paused-context watch expressions for locals, dotted object
    fields, static fields, `Trigger.*` context values, trigger shorthand roots,
    list/set numeric indexes, map string keys, and nested combinations without
    re-running VM code.
- [x] Add VS Code launch/task examples and editor documentation.
- [x] Expand `oaer lsp` with incremental document sync.
  - [x] Handle `textDocument/didOpen`, `didChange`, and `didClose`, apply
    full-document or ranged incremental text edits to in-memory overlays, publish
    parse diagnostics from open buffers, and clear diagnostics on close.
- [x] Add LSP semantic tokens, definition, references, rename, and richer
  completion.
  - [x] Advertise semantic-token, definition, references, prepare-rename, and
    rename providers, return semantic tokens from indexed declarations, resolve
    definitions from cursor words to Apex/schema symbols, scan project/open
    buffers for references, build workspace edits for rename, and include
    members, schema fields, and Apex keywords in completion.
- [x] Make LSP diagnostics match `oaer check` and test results consistently.
  - [x] Publish project sema/type diagnostics through the same diagnostic model
    used by `oaer check`, overlay open-buffer parse diagnostics while editing,
    restore project diagnostics on close, and expose test-result diagnostics
    from failure stack frames.
- [x] Add native OS watcher backends for `oaer test --watch`.
  - [x] Add `fsnotify` native watching with recursive directory registration,
    automatic fallback to polling in `auto` mode, explicit
    `--watch-backend auto|native|poll` selection, backend reporting in
    `watch.started`, and polling/native backend tests.
- [x] Add incremental re-indexing and affected-test dependency graph updates.
  - [x] Reuse the existing type index for Apex-only watch changes by replacing
    changed/deleted class and trigger symbols, fall back to full reload for
    schema metadata, and build a source-scanned dependency graph so production
    class changes select dependent tests before falling back to all tests.
- [x] Add in-flight VM/test cancellation for watch reruns.
  - [x] Thread context cancellation through the Apex test runner and VM
    instruction loop, run watch test executions asynchronously, cancel stale
    in-flight runs when a newer rerun starts, and suppress late results from
    canceled runs by run ID.
- [x] Stabilize watch JSON stream for editor/test UI consumers.
  - [x] Add `schemaVersion: 1` to each newline-delimited watch event, keep
    `runId` present on run events, emit `testClasses` as a stable array on
    `watch.run_started`, and cover the wire shape with exact JSON tests.
- [x] Expand profile/trace events for SOQL, DML, describe, callouts, limits,
  methods, triggers, and async.
  - [x] Keep existing statement/method/SOQL/DML trace events, add describe,
    callout, email, async enqueue/run, trigger invocation, and final governor
    limit summary events, and expand profile attribution for platform/resource
    counters.
- [x] Add native reports that fully replace apexrr-style analysis for local
  runtime data.
  - [x] Extend `oaer profile analyze` JSON and Markdown output with native
    runtime sections for hot events, categories, statements, methods, SOQL, DML,
    triggers, describe, callouts, async, platform events, and governor/resource
    summary counters.

## 8. Local API Server

- [ ] Complete auth/user context stubs enough for local integrations.
  - [x] Make identity/userinfo choose deterministic local users via
    `X-OAER-User-Id`, `Authorization: Bearer <userId>`, default platform user,
    and lexicographic fallback without enforcing auth globally.
  - [x] Thread selected server user context into Tooling executeAnonymous
    without enabling Apex test-only context.
  - [x] Add conservative OpenID-style userinfo and identity URL shapes while
    preserving deterministic local user selection and no token issuance.
- [ ] Expand Salesforce-like error response shapes and status codes.
  - [x] Return Salesforce-shaped `METHOD_NOT_ALLOWED` errors with `Allow: GET`
    for discovery, identity, and userinfo routes instead of accidental success.
- [ ] Complete `/services/data` resource discovery for commonly used REST
  resources.
  - [x] Return root API version discovery entries from `/services/data` and
    `/services/data/` with Salesforce-shaped `version`, `label`, and `url`
    fields.
  - [x] Advertise the local Bulk jobs namespace while returning explicit
    unsupported errors for unmodeled jobs.
  - [x] Advertise common unsupported top-level REST namespace links only for
    routes that return deterministic unsupported errors.
  - [x] Return conservative discovery payloads for advertised Tooling and Bulk
    Jobs roots instead of falling through to unknown routes.
- [ ] Expand SObject REST resources: describe, layout-adjacent metadata where
  useful, recent, query, queryAll, and record CRUD edge cases.
  - [x] Return Salesforce-like object resource metadata for
    `/sobjects/{Object}` with stable describe/recent URLs and common `urls`
    entries instead of leaking internal storage definitions.
  - [x] Enrich `/sobjects/{Object}/describe` with conservative object, field,
    record type, picklist, and child relationship payload keys from local
    storage metadata.
  - [x] Return flat record GET payloads with `attributes.type`, `attributes.url`,
    and `Id` instead of leaking internal storage value wrappers.
  - [x] Return REST and Tooling query rows as SObject-shaped payloads with
    `attributes`, `Id`, and flat fields instead of internal storage records.
  - [x] Return explicit unsupported SOQL query-plan `explain` responses instead
    of treating query-plan probes as malformed or normal SOQL execution.
  - [x] Add explicit full-layout unsupported responses and an empty compact
    layouts stub for local tooling probes.
  - [x] Cover common compact layout describe aliases with the same conservative
    empty local stub.
  - [x] Add explicit approval-layout and named-layout unsupported responses with
    advertised object resource URLs.
  - [x] Return explicit unsupported default-values responses and malformed-ID
    errors for literal row-template placeholders advertised by SObject resources.
  - [x] Add an empty list-view collection stub plus explicit unsupported
    describe/results responses and advertised list-view object URLs for common
    `/listviews`, `/describe`, and `/results` probes.
  - [x] Add explicit object-scoped quick action unsupported responses for
    collection, detail, and default-value probes with advertised object URLs.
  - [x] Add conservative `/sobjects/{Object}/updated` and
    `/sobjects/{Object}/deleted` resources backed by local record system
    timestamps, soft-delete flags, deterministic ID ordering, optional
    start/end bounds, and Salesforce-shaped malformed-date errors.
  - [x] Add conservative `limit=` handling for global and object recent
    resources, including default, malformed, and over-limit behavior.
  - [x] Cover missing/deleted record GET, missing DELETE, null PATCH, and method
    `Allow` headers in server tests.
  - [x] Add conservative external-ID SObject GET, PATCH upsert, and DELETE
    routes backed by local DML upsert/delete behavior.
  - [x] Add REST record `fields=` projection on ID and external-ID GET routes
    with field validation and blank-field preservation.
- [ ] Expand Tooling API coverage beyond `executeAnonymous` and query
  delegation.
  - [x] Return stable unsupported errors for common Tooling sObject discovery,
    Tooling sObject describe, and Tooling completions routes.
  - [x] Return stable unsupported errors for common Tooling metadata object
    probes (`ApexClass`, `ApexTrigger`, `ApexPage`, `ApexComponent`,
    `StaticResource`), debugger/log/test sObject probes (`ApexLog`,
    `TraceFlag`, `DebugLevel`, `ApexTestQueueItem`, `ApexTestResult`,
    `ApexCodeCoverage`, `ApexCodeCoverageAggregate`, `ApexOrgWideCoverage`,
    `ContainerAsyncRequest`, `MetadataContainer`), test-run orchestration
    endpoints, and coverage probes instead of falling through to unknown
    tooling routes.
  - [x] Delegate Tooling `queryAll` through the SOQL query handler and return
    explicit unsupported errors for Tooling search probes.
  - [x] Return Tooling `query`/`queryAll` pagination continuations under
    `/tooling/query/{locator}` with GET-only queryMore method boundaries.
  - [x] Return a conservative Tooling root discovery payload and pin Apex test
    run/suite Tooling records as explicit unsupported local stubs.
- [ ] Add more REST resources used by local integrations and editor tooling.
  - [x] Return a deterministic, conservative `/limits` payload with common
    Salesforce limit names and stable `Max`/`Remaining` fields for local client
    probes without claiming live org accounting.
  - [x] Return stable unsupported errors for common unmodeled top-level REST
    namespaces such as Connect, Chatter, Analytics, Wave, Metadata, Support,
    Process, Actions, Apps, AppMenu, Tabs, Theme, and QuickActions.
  - [x] Return stable unsupported errors for common OAuth token, revoke,
    introspect, and authorize probes without issuing local tokens.
  - [x] Return explicit Apex REST unsupported stubs for common custom REST
    endpoint methods and `METHOD_NOT_ALLOWED` for unsupported methods.
  - [x] Harden REST Search/SOSL and common unsupported REST namespace methods so
    non-GET probes receive deterministic `METHOD_NOT_ALLOWED` responses.
- [ ] Add Composite API coverage beyond baseline sObject insert, including
  all-or-none rollback, reference ID behavior, and conservative local
  collection update/delete mutators.
  - [x] Return stable unsupported errors for generic composite subrequest
    orchestration, batch, tree, and graph routes instead of fake composite
    success.
  - [x] Model conservative Composite sObject collection update/delete with DML
    Update/Delete, allOrNone rollback, reference IDs for update, and ID-located
    delete query parameters.
  - [x] Implement conservative typed Composite sObject upsert
    `/composite/sobjects/{Object}/{ExternalIdField}` backed by local DML
    external-ID upsert, with per-row results, reference IDs, created flags, and
    all-or-none rollback.
  - [x] Add conservative typed Composite sObject collection retrieve with ID
    selection and field projection handling.
- [ ] Add Bulk API approximations if needed by local integration tests.
  - [x] Add explicit Bulk API v2 query and ingest job stubs with Salesforce-shaped
    unsupported errors instead of fake job success.
  - [x] Return deterministic unsupported responses for common Bulk API v2 job
    record, results, batches, successfulResults, failedResults, and
    unprocessedrecords probes.
  - [x] Harden Bulk API v2 method boundaries for common query and ingest route
    shapes while preserving explicit unsupported responses for allowed methods.
  - [x] Return a conservative Bulk Jobs root discovery payload for query and
    ingest families while keeping job execution unsupported.
- [ ] Ensure anonymous Apex runs against the same persistent server database,
  transaction boundaries, user context, and limits.
  - [x] Add black-box server fixture evidence that Tooling
    `executeAnonymous` commits successful DML into queryable persistent server
    state, rolls back runtime-error and strict-limit mutations, honors selected
    bearer user context, and accepts fixture-level strict limit mode.
  - [x] Accept Tooling `executeAnonymous` form-encoded POST bodies while
    preserving JSON POST, query-string GET, and malformed JSON failure behavior.
- [ ] Add server fixture reset endpoints for test data, org state, limits, and
  async queues.
  - [x] Add local-only `oaer/state` and `oaer/inspect` summaries with supported
    reset scope metadata.
  - [x] Report `limits` and `async` reset scopes as accepted no-ops until limit
    and async queues become persisted server state.
- [ ] Add black-box server compatibility fixtures for CRUD, query,
  executeAnonymous, composite, errors, auth stubs, and persistence.
  - [x] Cover SObject object-resource metadata and unknown-object resource
    errors in the server black-box fixture.
  - [x] Cover SObject updated/deleted resource happy paths in the server
    black-box fixture.
  - [x] Cover Tooling and Bulk unsupported route shapes in the server black-box
    fixture.
  - [x] Cover common Bulk API v2 job subroute unsupported shapes in the server
    black-box fixture.
  - [x] Cover common Tooling metadata/debugger/log/test object, test-run,
    coverage, and search unsupported route shapes in the server black-box
    fixture.
  - [x] Cover Tooling `queryAll` SOQL delegation in the server black-box
    fixture.
  - [x] Cover Tooling `query`/`queryAll` pagination continuations and GET-only
    queryMore boundaries in the server black-box fixture.
  - [x] Cover generic composite unsupported route shape in the server black-box
    fixture.
  - [x] Cover Composite sObject collection update/delete and typed Composite
    sObject external-ID upsert create/update behavior in the server black-box
    fixture.
  - [x] Cover common top-level REST namespace unsupported route shapes, including
    utility AppMenu and QuickActions probes, in the server black-box fixture.
  - [x] Cover Tooling `executeAnonymous` persistence, rollback, selected bearer
    user, and strict-limit edges in the server black-box fixture.
  - [x] Cover common OAuth unsupported auth-stub route shapes in the server
    black-box fixture.
  - [x] Cover representative `/limits` payload keys in the server black-box
    fixture.
  - [x] Cover REST SObject external-ID create, update, get, and delete in the
    server black-box fixture.
  - [x] Cover REST record field projection, SOQL query-plan unsupported probes,
    Composite sObject retrieve, search/Bulk method boundaries, SObject
    list-view stubs, recent `limit=`, Apex REST stubs, and
    executeAnonymous form-body behavior in the server black-box fixture.
  - [x] Cover SObject approval/named layout stubs, compact layout describe
    aliases, object-scoped quick action stubs, and REST namespace method
    boundaries in the server black-box fixture.
  - [x] Cover Tooling/Bulk root discovery, conservative OpenID identity shapes,
    empty list-view collection stubs, and Apex test Tooling record stubs in the
    server black-box fixture.

## 9. Compatibility, Hardening, And Release

- [x] Generate a public compatibility dashboard from `internal/capability`.
- [x] Add CI gates for compatibility matrix drift and MVP readiness.
- [x] Build black-box fixtures against Salesforce behavior for every supported
  language/runtime/data/server feature.
  - [x] Add storage DB lifecycle coverage to the compatibility fixture runner.
  - [x] Add server black-box fixture execution for version discovery, CRUD, SOQL
    query, Tooling `executeAnonymous`, composite insert, Salesforce-shaped error
    arrays, OAuth userinfo/id stubs, scoped reset/state, no-op reset reporting,
    and SQLite persistence.
- [x] Add enterprise fixtures for trigger-heavy, selector/service/domain,
  async-heavy, describe-heavy, namespace-heavy, and package-style projects.
  - [x] Add trigger-heavy and describe-heavy test fixtures.
  - [x] Add namespace-heavy/package-directory check fixture with SFDX namespace
    and multiple package directories.
  - [x] Keep async-heavy coverage through the async semantics fixture and
    selector/service/domain coverage through the schema-aware enterprise fixture.
- [x] Add fixture coverage for unsupported-feature diagnostics so failures are
  stable and intentional.
- [x] Add panic recovery and no-panic tests around parser, sema, VM, SOQL, DML,
  test runner, watcher, LSP, DAP, fixture loading, and server routes.
- [x] Add benchmarks for parser, project indexing, sema, tests, SOQL, DML,
  triggers, storage seed/export, server routes, LSP, and watch mode.
- [x] Add stress tests for large projects, large fixtures, bulk DML, and
  describe-heavy execution.
  - [x] Add bounded normal-suite stress tests for large type-index builds,
    SQLite fixture round-trips, bulk DML partial results, and repeated describe
    execution.
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

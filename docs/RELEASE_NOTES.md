# Release Notes

## Unreleased

Compatibility status:

- MVP readiness: not ready.
- Required MVP capabilities are still partial or unsupported.
- See [`COMPATIBILITY_DASHBOARD.md`](COMPATIBILITY_DASHBOARD.md) and
  [`KNOWN_GAPS.md`](KNOWN_GAPS.md) for generated status.

Release engineering:

- Added tag-driven release artifact builds for macOS, Linux, and Windows.
- Added `SHA256SUMS.txt` checksum generation for release artifacts.
- Added manual, CI, and future Homebrew installation guidance.
- Added editor integration docs with VS Code tasks, debug launch examples, LSP
  wiring, watch mode, and report commands.
- Added a fail-fast `oaer compat mvp --require-ready` gate and CI visibility
  for machine-readable MVP readiness.
- Added compatibility fixture support and smoke coverage for expected
  unsupported-feature diagnostics.
- Added `check` compatibility fixture execution and an enterprise-style
  multi-class fixture covering parse/index/check behavior.
- Added parser diagnostic-count fixtures plus type-index and sema panic recovery
  diagnostics for malformed project inputs.
- Added method/constructor parameter type diagnostics and expanded VM exception
  fidelity for multi-catch, bare rethrow, catchable null dereference, and
  malformed IR guards.
- Completed the exception hierarchy baseline with ordered catch blocks,
  `System.*Exception` name normalization, original-stack rethrow preservation,
  and `getTypeName`, `getLineNumber`, and `getStackTraceString`.
- Added a conservative method-body sema baseline for local declaration types,
  constructor references, simple assignments, project method calls, and
  known-receiver overload arity/simple argument type matching.
- Extended method-body sema with duplicate-local diagnostics, unknown call
  argument variable diagnostics, inherited/interface/`super` method lookup, and
  private/protected method-call visibility diagnostics through inheritance
  chains with token-level ranges for body diagnostics, including `@TestVisible`
  method access from test classes.
- Extended sema visibility diagnostics to known user-object field reads, and
  resolved namespace-token schema aliases such as `pkg__Thing__c` for namespaced
  project metadata.
- Added namespace-token custom object and field alias resolution through VM
  SObject construction, direct field access, `get`/`put`, DML validation, and
  SOQL projection/where clauses.
- Expanded data-fidelity coverage for SOQL complex predicates, numeric
  comparison semantics, `Database.Error` result shapes, and
  `Database.UpsertResult.isCreated()`.
- Added no-`GROUP BY` SOQL aggregate support for `COUNT(field)`,
  `COUNT_DISTINCT`, `SUM`, `MIN`, `MAX`, and `AVG` with `AggregateResult`
  `exprN` fields.
- Added SOQL `GROUP BY` with grouped field projection, aggregate `HAVING`, and
  ordering/limits over grouped aggregate rows.
- Added SOQL aggregate aliases on `AggregateResult` rows while preserving
  `exprN` compatibility.
- Added SOQL `GROUP BY ROLLUP`, `GROUP BY CUBE`, and `GROUPING(field)` subtotal
  metadata for aggregate result rows.
- Added common SOQL date literals, including day, month/year, and `*_N_DAYS:n`
  ranges, for Date and Datetime comparisons.
- Added SOQL semi-join and anti-join predicate support for single-field
  subqueries in `IN` and `NOT IN` filters.
- Added SOQL child relationship subquery projection with metadata-driven
  relationship names and VM `List<SObject>` row shapes.
- Made SOQL `LIKE` and `NOT LIKE` matching case-insensitive for ASCII letters.
- Added comma-separated SOQL `ORDER BY ASC` and `ORDER BY DESC` handling for
  regular, aggregate, and child relationship query rows.
- Added SOQL `ORDER BY NULLS FIRST` and `NULLS LAST` modifiers.
- Added SOQL `FIELDS(ALL)`, `FIELDS(STANDARD)`, and `FIELDS(CUSTOM)` projection
  expansion.
- Added SOQL `FOR UPDATE` parsing and execution as a local lock marker.
- Added SOQL `ALL ROWS` support for querying soft-deleted records.
- Added SOQL `WITH SECURITY_ENFORCED`, `WITH USER_MODE`, and
  `WITH SYSTEM_MODE` parsing as local security-mode markers.
- Added baseline SOQL `TYPEOF` relationship projection for parent lookup
  branches.
- Added DML fidelity for implicit external-ID upsert, unique-field checks,
  lookup reference validation, ID/object mismatch errors, soft delete visibility,
  and undelete restoration.
- Added explicit external-ID upsert support for `upsert rows Field__c` and
  `Database.upsert(rows, Field__c, allOrNone)` field tokens.
- Added baseline DML merge support for the `merge` statement and
  `Database.merge`, including duplicate soft-delete, child lookup reparenting,
  and `Database.MergeResult` accessors.
- Added cascade soft-delete behavior from relationship metadata, including
  Metadata API `deleteConstraint` loading for local fixtures.
- Added object-level and field-level `SObject.addError`, `hasErrors`, and
  `getErrors` handling in before-trigger DML, including row-level `SaveResult`
  error shaping and `Database.Error.getFields()` attribution.
- Preserved source ranges through parser syntax diagnostics, compiled project
  method/trigger bodies, VM statement traces, runtime/test failure stacks, DAP
  stack frames, and profile source ranges.
- Added sema checks for invalid `override` markers, abstract methods declared
  on concrete classes, and missing concrete interface/abstract implementations.
- Added method-body sema diagnostics for local initializer and simple assignment
  type mismatches.
- Added method-body sema diagnostics for simple return type mismatches.
- Added sema and VM guards for non-void methods that fall through without a
  return value.
- Added simple binary expression typing in sema for numeric arithmetic, string
  concatenation, comparisons, and boolean operators.
- Added a sema numeric widening baseline for method-call matching from
  `Integer` to `Long`, `Decimal`, and `Double`, plus decimal-literal argument
  typing.
- Added a sema and VM object assignability baseline for class inheritance and
  interfaces across locals, assignments, returns, params, fields, and overload
  matching.
- Added sema return-type inference for known receiver method calls and chained
  constructor call expressions.
- Added sema and VM overload specificity baselines for exact matches, narrowest
  numeric widening, and nearest class/interface ancestors ahead of `Object`.
- Completed the overload specificity baseline with pairwise candidate comparison,
  ambiguous overload diagnostics/errors, and `null` calls choosing a strictly
  narrower applicable overload.
- Added an IR-backed method-body sema pass that checks scoped local reads across
  declarations, assignments, conditions, returns, calls, loops, switch, and
  try/catch/finally bodies.
- Extended the IR-backed method-body sema pass with Boolean condition checks and
  scoped declaration, assignment, and return type checks.
- Extended the IR-backed method-body sema pass with known user-object field
  read/write validation, including inherited fields.
- Extended the IR-backed method-body sema pass with known receiver and
  same-class method-call validation for unknown methods and argument mismatches.
- Extended the IR-backed method-body sema pass with constructor-call validation
  for unknown types, non-instantiable types, and argument mismatches.
- Completed inherited/interface/virtual/super sema coverage for this/super field
  and method return inference, assignments, returns, interface receivers, and
  superclass-typed virtual calls.
- Added IR-backed non-void return path analysis for `if`, `switch`, and
  try/catch control flow.
- Added inherited instance fields to method-body sema scopes.
- Added constructor chaining validation in sema for `this(...)`/`super(...)`
  placement, arity, and non-instantiable interface/enum/abstract constructor
  calls.
- Added namespace-qualified type resolution in sema, a small visibility
  diagnostic baseline, namespace-qualified class-name parsing in the VM, and
  runtime namespace checks that require global class and member access across
  namespace boundaries.
- Added qualified nested type symbols and a nested class construction, method,
  and static member execution baseline.
- Completed the inner/nested type runtime baseline with owner-relative nested
  type resolution, nested constructors/fields/methods/static members, nested
  interfaces, nested enum values/methods, and nested user-object identity
  equality coverage.
- Added Apex static and instance initializer block execution for project classes,
  including static reset behavior that reapplies static initializer blocks.
- Added `this(...)` and `super(...)` constructor chaining for supported project
  classes, with runtime guards for interface/enum/abstract instantiation.
- Added a runtime guard that blocks abstract method invocation.
- Added VM property getter/setter body execution, source-ordered field
  initialization/reset metadata, runtime visibility and namespace access checks,
  protected visibility through inheritance chains, `@TestVisible` method access
  from test classes, and overload selection by argument types with numeric and
  class/interface specificity baselines.
- Completed class/instance runtime fidelity for field initializer expressions,
  initializer block ordering, static reset, runtime access modifiers, and
  namespace boundaries.
- Added runtime virtual dispatch coverage through superclass-typed and
  interface-typed references.
- Completed runtime dispatch for declaring-class `super` calls, inherited
  concrete methods ahead of interface fallback methods, and inherited static
  fields/methods through subclass names.
- Added interface fallback method lookup, enum `name`/`ordinal`/`values`, and
  interface-based exception catch matching in the VM.
- Added VM coverage for `finally` execution across return, return override, and
  uncaught throw unwinding.
- Completed the control-flow edge baseline for loop `break`/`continue`, switch
  local `break`, enhanced-for signals, and `finally` preservation/override of
  pending loop, return, and throw signals.
- Completed the coercion baseline for numeric widening, null/object/enum
  assignment, invalid String/Boolean and narrowing rejection, collection member
  generics, method params/returns, fields, and schema-backed DML storage.
- Added explicit object `toString()` dispatch, including user-defined overrides,
  a default object fallback, and debug/assert message display.
- Added runtime coverage for user object identity equality.
- Added a VM Decimal baseline with decimal literals, integer/decimal arithmetic,
  assignment checks, storage conversion, and JSON number conversion.
- Completed the supported test-fidelity baseline: `@TestSetup` now runs once per
  class into a setup data snapshot, each test method gets an isolated org/VM
  clone with static reset, `Test.startTest()`/`Test.stopTest()` restore governor
  windows, `System.runAs` scopes UserInfo user/profile identity,
  FeatureManagement permission checks, and the supported mixed-DML guard,
  Queueable drain starts with fresh async statics, and assertion/runtime stacks
  keep file/line frames.
- Completed the supported async test baseline with `@future` method draining,
  Batchable start/execute chunking/finish, Schedulable execution, Queueable
  chaining limits, local `AsyncApexJob`/`CronTrigger` records, and an async-heavy
  compatibility fixture.
- Added binary smoke coverage for parser, exec, test, db, server, LSP
  diagnostics, profile, and compatibility commands.

Upgrade notes:

- No migration is required for this unreleased preview state.
- Persistent database and fixture formats are still preview interfaces.

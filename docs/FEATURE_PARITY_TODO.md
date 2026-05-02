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
- [ ] Model local variables, scopes, expressions, statements, method calls, and
  constructor calls in sema.
- [ ] Resolve overloads with Apex-compatible conversion and specificity rules.
- [ ] Resolve inherited members, interface members, virtual/override methods,
  and `super` references in sema.
- [ ] Enforce visibility: `private`, `protected`, `public`, `global`, test
  visibility, and package boundaries.
- [ ] Resolve namespaces for managed-package style references, custom metadata,
  custom objects, custom fields, and package-local symbols.
- [ ] Preserve stable source ranges through parser, sema, VM, test failures,
  LSP, DAP, and trace/profile events.
- [ ] Add large-project compatibility fixtures that prove parse/index/check
  behavior across enterprise repositories.
- [ ] Ensure unsupported syntax and semantic features return stable diagnostics
  instead of parser/VM panics.

## 2. Apex Runtime Core

- [ ] Complete class and instance execution fidelity for real service/domain
  classes.
- [x] Finish properties, getters/setters, initializer blocks, static
  initializers, static field ordering, and static reset behavior.
- [x] Complete constructor chaining, default constructors, overloaded
  constructors, and `this(...)`/`super(...)` behavior.
- [ ] Complete virtual dispatch, overrides, interfaces, abstract classes, and
  inherited member lookup.
- [ ] Implement inner classes, nested types, enums, and user object values with
  Salesforce-like equality/string/debug behavior.
- [ ] Complete exception hierarchy semantics, typed catch matching, multi-catch
  behavior, rethrow behavior, stack traces, and file/line reporting.
- [ ] Complete control-flow edge cases for loops, `switch`, `break`,
  `continue`, `return`, `finally`, and exception unwinding.
- [ ] Implement access modifiers and namespace/package boundaries at runtime,
  not only in sema.
- [ ] Support Apex numeric, decimal, boolean, string, collection, null, enum,
  and object coercion rules closely enough for enterprise code.
- [x] Add no-panic guards around all VM execution paths for malformed or
  unsupported user code.

## 3. Apex Test Semantics And Async

- [ ] Make `@TestSetup` match Salesforce transaction behavior exactly,
  including setup data visibility and rollback.
- [ ] Restore governor windows around `Test.startTest()` and `Test.stopTest()`
  with Salesforce-compatible counter behavior.
- [ ] Complete per-test transaction rollback for all storage mutations,
  triggers, async jobs, and platform side effects.
- [ ] Complete static reset behavior across test methods, test setup, async
  drain, and nested execution.
- [ ] Complete `System.runAs` profile, user, permission, sharing, and mixed-DML
  behavior for supported modes.
- [ ] Implement `@future` execution and stopTest drain behavior.
- [ ] Implement Batchable execution, batch chunking, finish behavior, and
  observable async records where useful.
- [ ] Implement Schedulable execution and direct scheduling model.
- [ ] Complete Queueable behavior: chaining limits, job IDs, error handling,
  and durable job state where useful.
- [ ] Improve assertion failures and runtime errors with precise file/line
  stack traces.
- [ ] Add enterprise test fixtures for trigger-heavy, selector/service/domain,
  async-heavy, describe-heavy, and namespace-heavy projects.

## 4. SObjects, SOQL, DML, And Triggers

- [ ] Complete typed SObject field access, dynamic `get`/`put`, absent-field
  behavior, explicit null behavior, and system fields.
- [ ] Complete schema describe objects, field describes, record type info,
  picklists, relationship metadata, and common describe-heavy code paths.
- [ ] Complete static SOQL parsing/execution for common selector-layer queries.
- [ ] Complete dynamic SOQL binding and runtime parse/error behavior for
  `Database.query`.
- [ ] Add relationship child subqueries.
- [ ] Expand parent relationship traversal and polymorphic relationship
  behavior.
- [ ] Add aggregates: `COUNT`, `COUNT(field)`, `COUNT_DISTINCT`, `SUM`, `MIN`,
  `MAX`, `AVG`, `GROUP BY`, `ROLLUP`, `CUBE`, and `HAVING`.
- [ ] Add complex predicates: `IN`, `NOT IN`, `LIKE`, boolean combinations,
  date literals, null semantics, semi-joins, anti-joins, and formula-adjacent
  behavior where locally representable.
- [ ] Add SOQL features commonly used by real projects: `FIELDS()`, `TYPEOF`,
  security clauses, `FOR UPDATE` handling, and query row shape fidelity.
- [ ] Wire SQLite planning or indexed execution where needed without changing
  Salesforce-visible behavior.
- [ ] Complete Apex DML statements: `insert`, `update`, `delete`, `upsert`,
  `undelete`, and `merge`.
- [ ] Complete `Database.insert/update/delete/upsert/undelete` result fidelity,
  error arrays, `allOrNone`, partial success, and status codes.
- [ ] Complete external-ID upsert and ID/object mismatch behavior.
- [ ] Implement validation rules, required fields, uniqueness, foreign-key
  behavior, and relationship constraints where representable locally.
- [ ] Complete trigger ordering, before/after state, bulk execution,
  recursion behavior, operation type, maps, old/new values, and rollback on
  failures.
- [ ] Implement `addError` behavior on SObjects and fields.
- [ ] Add trigger fixtures covering insert/update/delete/upsert/undelete,
  all-or-none failures, partial success, recursion, and bulk batches.

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
- [ ] Add migrations/versioning for persistent databases.
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

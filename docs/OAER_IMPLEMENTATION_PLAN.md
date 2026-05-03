# OAER Detailed Implementation Plan

## Objective

Deliver a clean-room, open source local Apex runtime that can run real Apex
tests, execute anonymous Apex, support schema-aware SOQL/DML/triggers, expose
debugging and language tooling, and act as a local Salesforce-compatible API
server.

This plan assumes the product name `oaer` during development. Performance
analysis is implemented natively in this project; `apexrr` is not a dependency.

## Definition Of Complete

`oaer` is complete enough for a v1 release when it can:

- run a representative enterprise Apex test suite locally without a Salesforce
  org connection
- load local Salesforce metadata and schema exports
- execute common Apex language constructs, SObjects, SOQL, DML, triggers, and
  async Apex with reliable test isolation
- enforce or report governor limits
- produce stable JSON, JUnit, trace, and profiling output
- support VS Code debugging through DAP
- support core LSP features: diagnostics, symbols, hover, completion,
  go-to-definition, and references
- run a local API server for SObject CRUD and query
- publish a compatibility dashboard that identifies supported, partial, and
  unsupported Apex/Salesforce features by API version

## Full-Featured MVP Target

MVP means an aer-parity local Apex development loop, not a thin demo. A feature
may have a baseline package and still fail the MVP gate if it is not usable from
real Apex projects through the CLI/runtime.

The MVP gate is machine-readable through `oaer compat mvp` and
`oaer compat matrix --json`. MVP is not ready until every `requiredForMVP`
capability in `internal/capability` is `supported`.

Required MVP capabilities:

- real Apex class execution: methods, constructors, statics, properties,
  control flow, and exceptions
- real Apex test execution: class dispatch, `@TestSetup`, `startTest/stopTest`,
  `runAs`, per-test isolation, and async draining basics
- Apex-integrated SObject construction and field access
- static and dynamic SOQL from Apex, including binds and relationships for
  common selector-layer queries
- Apex DML statements and `Database.*` methods, including partial-success
  result shapes
- trigger invocation and `Trigger.*` context for insert/update/delete flows
- governor counters and strict/permissive enforcement for SOQL, DML, rows, heap,
  CPU approximation, callouts, and async counts
- persistent fixture seed/export/reset workflow
- usable `oaer test --json`, JUnit, `oaer test --watch`, `oaer lsp`,
  `oaer exec/test --debug`, native profile reports, and `oaer server`
- generated compatibility dashboard and release packaging

Current implementation snapshot as of 2026-05-02:

- The repo has broad baselines for phases 0-15, including parser/project/schema
  indexing, VM execution, Apex test running, SObject/SOQL/DML/trigger
  integration, governor/platform API basics, SQLite fixtures, watch mode, LSP,
  DAP snapshot sessions, profile analysis, and a Salesforce-shaped local API
  server.
- `oaer compat mvp` is still expected to report not ready. Many required
  features are intentionally marked `partial` because unsupported edge cases
  must fail loudly instead of silently claiming Salesforce parity.
- The highest-risk remaining work is runtime fidelity: full language semantics,
  richer SOQL/DML/trigger behavior, async breadth, complete platform APIs,
  debugging pause hooks, and enterprise compatibility fixtures.

## Planning Assumptions

- Initial implementation language: Go, matching the current repo.
- Initial storage backend: SQLite.
- Initial parser: public `apexfmt`/ANTLR parser behind an internal adapter.
- Initial execution strategy: interpreter, not compiler.
- Initial compatibility strategy: black-box tests against Salesforce behavior
  using Apex code and metadata the project owns.
- Initial team size assumption for estimates: 2 senior engineers full time,
  with occasional Salesforce domain review. A solo build is possible but will
  stretch the timeline substantially.

Effort bands:

- XS: 1-3 days
- S: 1 week
- M: 2-4 weeks
- L: 1-2 months
- XL: 3+ months

## Workstream Map

The project has eight long-running workstreams:

1. Clean-room governance and compatibility
2. CLI, project loading, and configuration
3. Parser, AST, symbol table, and semantic analysis
4. Interpreter VM and Apex standard library
5. Schema, SObjects, SOQL, DML, and triggers
6. Tests, async execution, and governor limits
7. Debugging, profiling, LSP, watch mode, and editor integration
8. Local API server, fixtures, release engineering, and documentation

Do not wait for every foundational piece to be perfect before shipping
vertical slices. Each milestone should run one more realistic Apex scenario
end to end.

## Phase 0: Governance And Repository Foundation

Goal: make the project safe to build and easy to contribute to.

Effort: S-M.

Current status as of 2026-05-02: complete for the Phase 0 baseline. The repo
now has a Go module, Apache-2.0 license text, `cmd/oaer`, shared diagnostics,
minimal `oaer.yml` loading, compatibility fixture types, a parser-smoke fixture,
clean-room, compatibility, and architecture docs, and GitHub Actions metadata
for `go vet ./...`, `go test ./...`, and `go test -race ./...`. CLI plumbing
now covers `version`, `help`, `doctor`, `parse`, `inspect`, `schema`, `check`,
`exec`, `test`, `profile`, `server`, `db`, `lsp`, and `compat`; compatibility
fixtures can be validated/executed, and capability readiness is reported by
`oaer compat matrix` and `oaer compat mvp`.

### Deliverables

- Decide whether `oaer` lives in this repo under `cmd/oaer` or in a sibling
  repo.
- Choose license. Recommended default: Apache-2.0 unless a dependency forces a
  different choice.
- Add `docs/CLEAN_ROOM.md`.
- Add `docs/COMPATIBILITY.md`.
- Add `docs/ARCHITECTURE.md`.
- Add issue labels for runtime areas: `parser`, `type-system`, `vm`, `soql`,
  `dml`, `schema`, `tests`, `limits`, `dap`, `lsp`, `server`, `compat`.
- Add CI for `go test ./...`, linting, race tests where practical, and
  generated artifact checks.
- Add baseline command skeleton:
  - `oaer version`
  - `oaer help`
  - `oaer doctor`

### Implementation Tasks

- Create `cmd/oaer/main.go`.
- Create `internal/oaercli` or equivalent for CLI plumbing.
- Add a project config file format, likely `oaer.yml`.
- Define structured error and diagnostic types shared by parser, sema, VM, and
  CLI.
- Create `internal/compat` fixture schema:
  - Apex source
  - metadata/schema
  - seed data
  - command or invocation
  - expected stdout/stderr/result/error
  - expected storage side effects
  - expected limit counters where known

### Exit Criteria

- `go test ./...` passes.
- `oaer doctor` prints environment and dependency status.
- Clean-room policy is documented.
- A compatibility fixture can be committed and executed, even if it only checks
  parser output initially.

## Phase 1: Parser Adapter And Source Model

Goal: parse Apex source reliably and expose a stable internal source model.

Effort: M.

Current status as of 2026-05-02: complete for the Phase 1 baseline. `internal/apexast` wraps
`github.com/octoberswimmer/apexfmt/parser` behind an internal source model,
and `oaer parse <paths...> --json` can parse `.cls` and `.trigger` files from
explicit files or directories. The small example project
`example-projects/src-nmb-nutpl-develop` parses cleanly: 135 Apex files, zero
diagnostics. The large example project `example-projects/src-nmb-nu-develop`
parses cleanly: 2,964 Apex files, zero diagnostics, in about 9 seconds. The
parser adapter includes a clean source-preserving workaround for valid Apex
methods named `void`, which the upstream generated grammar treats too strictly.
The package includes declaration walking utilities, line/column mapping, and
file URI conversion.

### Deliverables

- `internal/apexast` parser facade.
- File-level AST with stable node kinds and source ranges.
- Syntax diagnostics with file, line, column, and excerpt.
- Visitor/walker utilities.
- Snapshot tests for representative Apex syntax.
- Parser compatibility corpus from public and project-owned samples.

### Implementation Tasks

- Add `apexfmt` dependency or generated public grammar dependency.
- Wrap parser output in internal types:
  - compilation unit
  - class declaration
  - interface declaration
  - enum declaration
  - trigger declaration
  - method declaration
  - field/property declaration
  - statement nodes
  - expression nodes
  - SOQL/SOSL expression nodes if parser exposes them
- Preserve comments only where needed for doc hover or pragmas.
- Build source range utilities:
  - byte offsets
  - line/column mapping
  - file URI conversion
- Add golden tests for:
  - annotations
  - generics
  - nested classes
  - properties
  - triggers
  - static SOQL
  - dynamic SOQL strings as plain expressions
  - `with sharing`, `without sharing`, `inherited sharing`
  - namespaces and custom object names

### Exit Criteria

- `oaer parse <paths...> --json` can parse a large local Salesforce project.
- Parser reports all syntax errors without panicking.
- Existing regex discovery can be reproduced using AST data.

## Phase 2: Project Loader, Metadata Loader, And Symbol Table

Goal: understand a Salesforce project as a typed collection of Apex and
metadata.

Effort: M-L.

Current status as of 2026-05-02: complete for the Phase 2 baseline. `internal/project` discovers SFDX
package directories and Apex/object/field metadata files. `internal/schema`
loads custom objects and custom fields from Metadata API XML. `internal/typesys`
builds a first symbol index for top-level Apex classes, interfaces, enums,
triggers, members, test annotations, and schema objects. CLI commands now include
`oaer inspect symbols --project <root> --json` and
`oaer schema load --project <root> --json`. The small example project
`example-projects/src-nmb-nutpl-develop` indexes cleanly: 134 types, 1 trigger,
3 objects, 13 fields, zero diagnostics. The large example project
`example-projects/src-nmb-nu-develop` indexes cleanly in about 9 seconds:
2,898 types, 65 triggers, 168 objects, zero diagnostics.

### Deliverables

- `internal/project`: SFDX/metadata project discovery.
- `internal/schema`: metadata model for objects, fields, relationships, record
  types, permissions, users, profiles, and package namespace settings.
- `internal/typesys`: symbol table for Apex declarations.
- Duplicate symbol and namespace diagnostics.
- `oaer inspect symbols`.
- `oaer schema load`.

### Implementation Tasks

- Read `sfdx-project.json`, package directories, and Metadata API layouts.
- Load `.cls`, `.trigger`, and related `*-meta.xml`.
- Load custom object metadata:
  - `objects/*.object-meta.xml`
  - `fields/*.field-meta.xml`
  - record types
  - validation rules
  - compact layouts only as metadata, not behavior yet
- Model standard objects with a seed schema bundle.
- Implement namespace handling:
  - default package namespace
  - namespaced custom objects and fields
  - package-local references
  - external managed package stubs
- Build symbol table:
  - classes/interfaces/enums/triggers
  - methods and overloads
  - fields/properties
  - constructors
  - annotations
  - visibility modifiers
  - inheritance relationships
- Add basic reference resolver.

### Exit Criteria

- `oaer inspect symbols --project <root>` lists classes, methods, triggers,
  SObjects, and fields.
- Duplicate classes and unresolved top-level symbols are reported cleanly.
- Large project indexing completes within an acceptable time budget.

## Phase 3: Semantic Analysis And Type Checking

Goal: type-check enough Apex to lower it into an executable representation.

Effort: L.

Current status as of 2026-05-02: complete for the Phase 3 baseline.
`internal/sema` builds a known-type catalog from Apex symbols, nested type
declarations, schema objects, builtin Apex types, and a starter Salesforce
platform type catalog. `oaer check --project <root> --json` validates
declaration/member type references, method and constructor references,
visibility, overrides, namespace/schema aliases, constructor chaining,
non-instantiable constructor calls, overload specificity, object assignability,
and a conservative IR-backed method-body baseline for locals, assignments,
returns, scoped reads, Boolean conditions, user-object field reads/writes, known
method calls, all-path non-void returns, and constructor calls. Full expression
typing and full flow analysis remain incomplete. The small example project
`example-projects/src-nmb-nutpl-develop` checks cleanly: 134 types, 1 trigger, 3
objects, zero diagnostics. The large example project
`example-projects/src-nmb-nu-develop` checks cleanly in about 9 seconds: 2,898
types, 65 triggers, 168 objects, zero diagnostics.

### Deliverables

- `internal/sema`: semantic analyzer.
- Type model for Apex primitives, collections, SObjects, user classes,
  interfaces, enums, exceptions, and null.
- Method overload resolution.
- Assignment, cast, and coercion rules.
- Diagnostics suitable for CLI and LSP.
- `oaer check <paths...>`.

### Implementation Tasks

- Implement type identity and assignability:
  - primitives: Boolean, Integer, Long, Double, Decimal, String, Id, Blob,
    Date, Datetime, Time
  - Object
  - SObject
  - typed SObjects
  - List, Set, Map
  - user classes and interfaces
  - enums
  - exception types
- Implement method lookup:
  - static vs instance
  - inherited members
  - overload selection
  - constructors
  - property getters/setters
- Implement expression checks:
  - literals
  - variables
  - field access
  - method calls
  - constructors
  - casts
  - `instanceof`
  - arithmetic/logical/comparison operators
  - ternary
  - collection indexing
- Implement statement checks:
  - variable declarations
  - assignment
  - if/switch
  - loops
  - try/catch/finally
  - return
  - throw
  - DML statements as semantic nodes
  - SOQL assignment shape
- Add a standard-library signature catalog.

### Exit Criteria

- `oaer check` accepts simple real Apex test classes.
- Type checker can identify test methods and entry points without regex.
- Semantic diagnostics are deterministic and source-located.

## Phase 4: IR And Minimal Interpreter VM

Goal: execute non-database Apex code.

Effort: L.

Current status as of 2026-05-02: complete for the Phase 4 baseline.
`internal/ir` defines a compact instruction/expression representation, and
`internal/vm` executes the supported Apex subset through `oaer exec` and the
test runner. Supported now: variables and expressions; primitive, collection,
enum, null, and user-object values; class/method execution; constructors and
constructor chaining; instance/static fields; property accessors; initializer
blocks; inheritance, interface, virtual, and `super` dispatch; nested classes,
interfaces, and enums; visibility and namespace enforcement; overload matching;
typed coercion; `if`, `while`, `for`, enhanced `for`, `do/while`, `break`,
`continue`, and `switch`; ordered catch blocks, multi-catch, try/catch/finally,
rethrow, stack accessors, and common exception hierarchy matching; SObject
construction and field access; static and dynamic SOQL entry points; DML
statements; trigger dispatch; governor counters; common platform APIs; trace
events; and debug snapshots. Broader platform APIs, expression fidelity, data
edge cases, and debugger pause hooks remain MVP work.

### Deliverables

- `internal/ir`: lowered executable form.
- `internal/vm`: interpreter with stack frames, heap values, exceptions, and
  trace hooks.
- Core runtime values:
  - primitives
  - null
  - user objects
  - lists, sets, maps
  - enums
  - exceptions
- Core `System` assertions and debug output.
- `oaer exec` for simple anonymous Apex.

### Implementation Tasks

- Lower sema-checked AST to IR:
  - statements
  - expressions
  - method calls
  - control flow
  - exception handlers
  - source locations for each executable instruction
- Implement execution context:
  - call stack
  - lexical scopes
  - static storage
  - heap/object identity
  - transaction context placeholder
  - user context placeholder
  - clock abstraction
- Implement dispatch:
  - static calls
  - virtual calls
  - constructors
  - super calls
  - property accessors
- Implement base library:
  - `System.assert`, `assertEquals`, `assertNotEquals`
  - `System.debug`
  - string operations
  - math/decimal basics
  - date/datetime basics
  - JSON serialize/deserialize for simple shapes
  - collection methods used by common tests

### Exit Criteria

- `oaer exec 'System.assertEquals(2, 1 + 1);'` passes.
- Hand-written tests for classes, inheritance, exceptions, and collections pass.
- VM can emit an execution trace with source locations.

## Phase 5: Minimal Test Runner

Goal: discover and run Apex tests without database behavior.

Effort: M.

Current status as of 2026-05-02: complete for the Phase 5 baseline.
`internal/apextest` discovers `@isTest` classes, `@isTest` methods, and legacy
`testMethod` methods from the symbol index. `oaer test` compiles project helper
classes and triggers, runs constructor and instance method bodies, executes
`@TestSetup`, clones org state per test, resets statics, supports
`Test.startTest()`/`Test.stopTest()`, `System.runAs`, Queueable/Future/Batch/
Scheduled draining, durable async job records, assertion stack frames, substring
filtering, and console/JSON/JUnit reporters. Richer auth/profile semantics,
broader permission behavior, and unsupported edge-case fidelity remain MVP work.

### Deliverables

- `internal/apextest`: test discovery, scheduling, isolation, and execution.
- `internal/testreport`: console, JSON, and JUnit result output.
- `oaer test`.
- `oaer test --json`.
- JUnit XML output.
- Basic test isolation.
- `@isTest` and test method support.

### Implementation Tasks

- Discover:
  - `@isTest` classes
  - `@isTest` methods
  - legacy `testMethod`
  - `@TestSetup` methods as pending unsupported or basic supported
- Implement result states:
  - pass
  - fail
  - skipped
  - compile error
  - runtime error
  - unsupported feature
- Add test filtering:
  - class name
  - method name
  - glob/substr
- Add isolation:
  - reset statics between tests where required
  - reset storage/org state between tests
  - deterministic clock option
- Add console reporter, JSON reporter, JUnit reporter.

### Exit Criteria

- A no-SObject Apex test suite runs locally.
- Failing assertions produce useful stack traces and file/line locations.
- `oaer test --json` is stable enough for editor integration.

## Phase 6: SObject Model And Schema Runtime

Goal: represent Salesforce records and schema at runtime.

Effort: L.

Current status as of 2026-05-02: complete for the Phase 6 baseline.
`internal/storage` defines the org/object/record contract, fixture envelope,
deterministic ID generation, 15/18-character ID validation, clone helpers for
transaction snapshots, fixture alias/reference resolution, deterministic
platform users/profiles/permissions, SQLite-backed persistence for object
definitions, records, and ID sequences, and schema migrations/versioning with DB
inspection summaries. `internal/sobject` adds runtime SObject values with field
maps, explicit null tracking, `get`/`put` behavior, record conversion, schema
describe registry, field describe basics, picklist values, record type describe
information, relationship metadata, deterministic object key prefixes, and
conversion to storage object definitions. Apex syntax integration for
`new Account(Name='Acme')`, typed field access, dynamic field access, DML Id
propagation, and simple relationship projection is wired into the VM. Richer
describe behavior and permission semantics remain incomplete.

### Deliverables

- Runtime SObject value model.
- Typed SObject construction and dynamic field access.
- ID generation and object key prefixes.
- `Schema` describe APIs for common use.
- Fixture import/export format.

### Implementation Tasks

- Implement SObject value:
  - object API name
  - field map
  - explicit null tracking
  - relationship fields
  - clone behavior
  - `get`, `put`, typed field access
- Implement ID service:
  - deterministic test IDs
  - 15/18-character handling where practical
  - object prefix mapping
- Implement schema APIs:
  - `Schema.getGlobalDescribe`
  - `SObjectType.getDescribe`
  - field describe basics
  - record type info basics
- Add fixture format:
  - JSON records
  - object name
  - field values
  - relationship references
  - user/profile context
- Add `oaer db seed` and `oaer db export`.

### Exit Criteria

- Apex code can construct, mutate, and inspect SObjects.
- Basic describe-heavy code runs.
- Fixture data can be loaded and inspected from CLI.

## Phase 7: SOQL Engine

Goal: execute static and dynamic SOQL over local storage.

Effort: XL.

Current status as of 2026-05-02: complete for the Phase 7 baseline.
`internal/soql` parses and executes supported in-memory queries over
`storage.OrgState`: `SELECT` field projections, `FROM`, equality/inequality and
comparison `WHERE` predicates, boolean combinations, binds, date literals,
semi-joins, anti-joins, child relationship subqueries, multi-hop parent
relationship fields and filters, `ORDER BY`, `LIMIT`, `OFFSET`, `COUNT()`,
broader aggregates, `GROUP BY`, `ROLLUP`, `CUBE`, `HAVING`, `FIELDS()`,
`TYPEOF`, soft-deleted row visibility, and security-mode markers. Static SOQL
and dynamic `Database.query` are wired into the VM, projected records preserve
Salesforce-like field absence, and limit counters are updated. SQLite planning,
lock contention, security enforcement, and advanced polymorphic relationship
behavior remain incomplete.

### Deliverables

- `internal/soql`: parser, binder, planner, executor.
- SQLite-backed query execution.
- Bind variable support.
- Relationship traversal support.
- Query limit accounting hooks.
- Query explain output.

### Implementation Tasks

- Start with supported SOQL:
  - `SELECT fields FROM Object`
  - `WHERE` with comparisons, booleans, null, `IN`, `LIKE`
  - `ORDER BY`
  - `LIMIT` and `OFFSET`
  - parent relationship fields
  - child subqueries
  - bind variables
- Add dynamic `Database.query(String)`:
  - parse runtime string
  - bind variables from frame
  - report parse errors like Apex exceptions
- Add result materialization:
  - typed SObjects
  - relationship SObjects
  - child relationship lists
  - aggregate result placeholder
- Add query plan bridge:
  - translate to SQL
  - run SQLite
  - attach explain plan to trace/profiling metadata
- Add later support:
  - aggregates
  - `GROUP BY`
  - `HAVING`
  - `TYPEOF`
  - `FIELDS()`
  - security clauses
  - polymorphic references

### Exit Criteria

- Common selector-layer queries run against fixture data.
- Query results match Salesforce shape for covered features.
- Unsupported SOQL returns explicit unsupported diagnostics, not panics.

## Phase 8: DML, Transactions, And Triggers

Goal: mutate local storage with Salesforce-like DML semantics and trigger
execution.

Effort: XL.

Current status as of 2026-05-02: complete for the Phase 8 baseline.
`internal/dml` supports insert, update, delete, upsert, undelete, and merge over
`storage.OrgState`; required, unknown-field, unique-field, lookup-reference, and
ID/object mismatch validation; deterministic ID assignment; all-or-none and
partial-success result records for `Database.*`; single and multi-entry
`Database.Error` shaping; and rollback-by-snapshot transaction wrappers. Apex
DML syntax and `Database.*` methods are wired into the VM, including implicit
and explicit external-ID upsert, soft delete/undelete, merge hooks, cascade
soft-delete metadata, object-level and field-level `addError`, and before/after
trigger invocation with `Trigger.new`, `Trigger.old`, maps, flags, operation
type, size, supported bulk partial-success alignment, and recursion rollback.
Validation-rule formulas, full merge loser relationship result details, full
undelete edge-case parity, and exact bulk ordering remain incomplete.

### Deliverables

- `internal/storage`: SQLite store and transaction manager.
- `internal/dml`: DML pipeline.
- Trigger discovery, ordering, and invocation.
- `Database.SaveResult`, `DeleteResult`, and partial-success behavior.
- Trigger context variables.

### Implementation Tasks

- Implement storage:
  - schema migrations
  - records table
  - indexes for object and ID
  - transaction boundaries
  - rollback on exception
  - test transaction reset
- Implement DML operations:
  - insert
  - update
  - upsert
  - delete
  - undelete
  - merge later
- Implement DML validation:
  - required fields
  - unknown fields
  - ID/object mismatch
  - duplicate external IDs later
  - validation rules later
- Implement trigger runtime:
  - before insert/update/delete
  - after insert/update/delete/undelete
  - `Trigger.new`, `old`, maps, operation flags, size
  - recursion behavior
  - bulk execution
- Implement DML statement syntax and `Database.*` methods.

### Exit Criteria

- CRUD-heavy tests pass.
- Trigger tests pass for covered trigger events.
- Rollback semantics are correct for failed transactions.

## Phase 9: Full Test Semantics, Async, And Governor Limits

Goal: make local tests meaningful for Salesforce development.

Effort: L-XL.

Current status as of 2026-05-02: complete for the Phase 9 baseline.
`@TestSetup`, per-test cloned org state, static reset, `startTest`/`stopTest`,
`runAs`, Queueable/Future/Batch/Scheduled draining at `stopTest`, durable
`AsyncApexJob`/`CronTrigger` records, assertion stack frames, governor counters,
and strict/permissive limit modes are implemented for the supported runtime
subset. Exact Salesforce accounting, configurable per-test caps, broader
permission semantics, and unsupported edge-case fidelity remain incomplete.

### Deliverables

- `@TestSetup` behavior.
- `Test.startTest` and `Test.stopTest`.
- `System.runAs`.
- Queueable, Future, Batchable, Schedulable execution.
- Governor counters and strict enforcement mode.
- Per-test isolation with storage rollback.

### Implementation Tasks

- Test setup:
  - run once per class
  - snapshot setup data
  - restore before each test
- `startTest/stopTest`:
  - reset relevant limits
  - drain async at stop
  - enforce call ordering
- Async:
  - `System.enqueueJob`
  - `@future`
  - `Database.executeBatch`
  - `System.schedule` direct scheduling model
  - `AsyncApexJob` records where useful
- Limits:
  - SOQL queries
  - query rows
  - DML statements
  - DML rows
  - heap approximation
  - CPU approximation
  - callouts
  - queueable/future/batch counts
- Strict/permissive modes:
  - strict: throw on limit breach
  - permissive: report breach but continue where possible

### Exit Criteria

- Tests using setup data and async patterns pass.
- Limit counters are observable through `Limits.*`.
- CI can fail on unsupported features or limit breaches by configuration.

## Phase 10: Platform Library Expansion

Goal: cover the standard-library and platform APIs used by real projects.

Effort: ongoing, starts after Phase 4 and continues through v1.

Current status as of 2026-05-02: complete for the Phase 10 baseline. The VM has
common `System`, `Test`, `Database`, `Limits`, `Schema`, `JSON`, date/time,
math, encoding, crypto, user-info, feature-management, messaging, ApexPages,
and HTTP/callout mock surfaces for common tests. These APIs are intentionally
partial; unsupported methods should produce stable unsupported-feature errors
instead of panics.

### Priority 1 APIs

- `System`
- `Test`
- `Limits`
- `Database`
- `Schema`
- `JSON`
- `String`, `Pattern`, `Matcher`
- `Date`, `Datetime`, `Time`
- `Math`
- `EncodingUtil`
- `Crypto`

### Priority 2 APIs

- `Http`, `HttpRequest`, `HttpResponse`, `HttpCalloutMock`
- `Messaging`
- `FeatureManagement`
- `UserInfo`
- `URL`
- `PageReference`
- `ApexPages` basics
- `ConnectApi` stubs where common

### Priority 3 APIs

- `EventBus`
- Platform event approximations
- Approval process stubs
- DataWeave clean substitute
- Flow invocation approximations
- Metadata/tooling edge APIs

### Exit Criteria

- Standard library coverage is tracked in a generated matrix.
- Unsupported methods fail with stable `UnsupportedFeatureError` diagnostics.
- Contributors can add methods without modifying VM internals.

## Phase 11: Debug Adapter Protocol

Goal: debug Apex in standard editors.

Effort: L.

Current status as of 2026-05-02: complete for the Phase 11 baseline.
`internal/dap` implements DAP content-length framing, request decoding,
response/event encoding, and an in-memory handler for initialize,
setBreakpoints, configurationDone, threads, stackTrace, scopes, variables,
continue, next, pause, and disconnect. It can render primitive and collection
variables from VM snapshots and evaluate simple expressions against those
snapshots. `oaer exec --debug` and `oaer test --debug` start DAP snapshot
sessions. Live VM suspension, breakpoint-driven execution control, launch/attach
transport fidelity, and step-in/out semantics remain incomplete.

### Deliverables

- `oaer test --debug`.
- `oaer exec --debug`.
- DAP server with source breakpoints, stepping, scopes, variables, watch eval,
  and call stack.
- VS Code launch/task examples.

### Implementation Tasks

- Add VM pause hooks at IR instruction boundaries.
- Map IR instructions to source locations.
- Implement DAP requests:
  - initialize
  - launch/attach
  - setBreakpoints
  - configurationDone
  - threads
  - stackTrace
  - scopes
  - variables
  - continue
  - next
  - stepIn
  - stepOut
  - pause
  - evaluate
  - disconnect
- Add variable renderers:
  - primitives
  - collections
  - SObjects
  - user objects
  - statics
- Add watch expression evaluator using sema and VM context.

### Exit Criteria

- VS Code can set breakpoints, step through tests, inspect variables, and
  evaluate simple expressions.
- Debug mode does not change non-debug execution results.

## Phase 12: Profiling, Trace, And Performance Analysis

Goal: expose runtime cost in open, useful formats.

Effort: M-L.

### Deliverables

- Chrome Trace Event output.
- pprof-compatible profile output or converter.
- Statement-level events for SOQL, DML, describe, callout, heap, and limits.
- `oaer profile`.
- Native trace/profile analysis reports.

Current status as of 2026-05-02: complete for the Phase 12 baseline. `oaer exec
--trace` writes Chrome Trace Event JSON, and `internal/profile` plus `oaer
profile analyze` aggregate native trace events into ranked JSON or Markdown
reports with statement, method, SOQL, and DML categories plus SOQL/DML row
deltas. No external apexrr dependency is used. pprof output, wall-clock
attribution, and richer statement metadata remain incomplete.

### Implementation Tasks

- Add trace hooks in VM:
  - method begin/end
  - statement execution
  - SOQL begin/end
  - DML begin/end
  - describe call
  - callout
  - async job
  - trigger entry/exit
- Add event metadata:
  - file/line
  - class/method
  - governor deltas
  - query text hash
  - row counts
  - storage explain plan pointer
- Add profile aggregation:
  - inclusive method time
  - self time
  - call count
  - query count
  - DML count
  - heap approximation
- Add native JSON/Markdown reports for trace/profile analysis.

### Exit Criteria

- Trace files open in Perfetto or Chrome trace viewers.
- Native `oaer` reports can rank hot methods/statements from trace output.
- Statement-level cost is visible for supported operations.

## Phase 13: Watch Mode And Affected Test Selection

Goal: make the runtime useful in day-to-day development.

Effort: M.

Current status as of 2026-05-02: complete for the Phase 13 baseline.
`internal/watch` classifies Apex and metadata files, snapshots file state by
modtime and size, diffs changes, emits stable JSON event structs, and performs
conservative affected-test selection from the symbol index. `oaer test
--watch` runs a polling watcher with debounce, reruns, JSON event stream, and
context cancellation; `--watch-once` is available for deterministic tests.
Native OS watcher backends, incremental re-indexing, and in-flight VM
cancellation remain incomplete.

### Deliverables

- `oaer test --watch`.
- Incremental project index.
- Dependency graph for affected test selection.
- Stable JSON stream for editor/test UI.

### Implementation Tasks

- File watcher over `.cls`, `.trigger`, and metadata files.
- Incremental reparse and recheck.
- Dependency graph:
  - symbol references
  - trigger-to-object mapping
  - test references
  - conservative fallback to all tests
- Watch runner:
  - debounce saves
  - cancel in-flight run
  - preserve last good index where possible
  - emit structured events

### Exit Criteria

- Editing a class reruns directly affected tests.
- Metadata changes invalidate relevant schema and tests.
- Watch mode handles syntax errors without exiting.

## Phase 14: LSP Server

Goal: provide editor intelligence from the same parser and type system.

Effort: L.

Current status as of 2026-05-02: complete for the Phase 14 baseline.
`internal/lsp` implements stdio JSON-RPC/LSP transport through `oaer lsp` plus
request handling for initialize/shutdown, diagnostics payloads, incremental text
document sync with open-buffer overlays, document symbols, workspace symbols,
semantic tokens, definition, references, rename, hover, and completion for Apex
types, SObjects, members, fields, and keywords from the existing project index.
Diagnostics parity with `oaer check`, deeper context-aware completion, and
large-project behavior remain incomplete.

### Deliverables

- `oaer lsp`.
- Diagnostics.
- Semantic tokens.
- Document symbols.
- Workspace symbols.
- Hover.
- Completion.
- Go-to-definition.
- References.

### Implementation Tasks

- Implement JSON-RPC/LSP server.
- Share project index with CLI.
- Add LSP features in order:
  - initialize/shutdown
  - text document sync
  - publish diagnostics
  - document symbols
  - semantic tokens
  - definition
  - hover
  - completion
  - references
  - rename later
- Add schema-aware completion:
  - SObject names
  - field names
  - relationship names
  - enum-like picklist metadata where useful

### Exit Criteria

- Editor features work without org access.
- LSP diagnostics match `oaer check`.
- LSP can serve a large project without repeated full re-indexes on every edit.

## Phase 15: Local Salesforce-Compatible Server

Goal: expose local data and execution through Salesforce-shaped endpoints.

Effort: L.

Current status as of 2026-05-02: complete for the Phase 15 baseline.
`internal/server` provides an HTTP handler backed by `storage.OrgState`,
`internal/dml`, `internal/soql`, and the VM. It supports `/services/data`,
sObject CRUD, normal REST JSON payload decoding, `query`/`queryAll`,
describe/recent, limits, OAuth userinfo and `/id` stubs, Tooling
`executeAnonymous`, Tooling query delegation, composite sObject insert,
Salesforce-shaped error arrays, OAER fixture/reset endpoints, and optional
SQLite persistence through `oaer server --db`. Full auth, Tooling object
coverage, Composite Graph, Bulk API, broader REST resources, and exact error
fidelity remain incomplete.

### Deliverables

- `oaer server`.
- REST SObject CRUD.
- REST query endpoint.
- Anonymous Apex execute endpoint.
- Basic auth/user-context stub.
- Fixture reset endpoints for tests.

### Implementation Tasks

- Implement HTTP server and route model.
- Add endpoints:
  - `/services/data`
  - `/services/data/vXX.X/sobjects/<Object>`
  - `/services/data/vXX.X/sobjects/<Object>/<Id>`
  - `/services/data/vXX.X/query`
  - `/services/data/vXX.X/tooling/executeAnonymous` or local equivalent
  - health and reset endpoints
- Add response shapes close to Salesforce for supported endpoints.
- Use same storage, schema, SOQL, DML, VM, and limits stack as CLI.
- Add server fixtures for integration tests.

### Exit Criteria

- Local clients can query and mutate fixture SObjects through HTTP.
- Anonymous Apex can run against the same server database.
- Server behavior is covered by compatibility tests.

## Phase 16: Enterprise Compatibility And v1 Hardening

Goal: close enough gaps for serious adoption.

Effort: XL and ongoing.

### Deliverables

- Public compatibility dashboard.
- API-versioned behavior flags.
- Managed package fixtures.
- Performance benchmarks.
- Fuzz and mutation tests for parser/VM/SOQL.
- Release candidate process.

### Implementation Tasks

- Build compatibility matrix generator:
  - language feature
  - standard library method
  - SOQL feature
  - DML behavior
  - metadata behavior
  - debug/LSP feature
- Add benchmark suites:
  - parser/index time
  - test execution time
  - SOQL query time
  - DML bulk operation time
  - watch invalidation time
- Add enterprise fixtures:
  - namespace-heavy project
  - trigger-heavy project
  - selector/service/domain pattern project
  - async-heavy project
  - describe-heavy project
- Harden errors:
  - no panics on user code
  - stable unsupported-feature errors
  - actionable diagnostics
- Prepare releases:
  - changelog
  - binary builds
  - checksums
  - docs site
  - migration guide from org-based local workflows

### Exit Criteria

- v1 compatibility scope is documented and true.
- Known gaps are tracked and exposed at runtime.
- Runtime is stable enough for CI use on supported features.

## Milestone Release Plan

### M0: Skeleton

Includes Phases 0-1 basics.

User value: contributors can run the CLI and parse projects.

### M1: Static Apex Tooling

Includes Phase 2 and enough Phase 3 for symbol diagnostics.

User value: local project indexing, duplicate detection, unresolved symbol
reports, and AST-backed discovery.

### M2: No-DB Test Runner

Includes Phases 4-5.

User value: pure logic tests run locally.

### M3: Local Data Runtime

Includes Phases 6-8 basics.

User value: SObject/SOQL/DML/trigger tests run for common CRUD paths.

### M4: Salesforce Test Semantics

Includes Phase 9 and high-priority Phase 10 APIs.

User value: meaningful Apex test suites with setup data, async, and limits.

### M5: Developer Experience

Includes Phases 11-14.

User value: debug, profile, watch, and editor features.

### M6: Local Platform

Includes Phase 15.

User value: local Salesforce-compatible server for integration workflows.

### M7: v1

Includes Phase 16.

User value: documented compatibility, stable releases, and CI adoption.

## Critical Path

The critical path is:

1. Parser adapter
2. Symbol table
3. Semantic analysis
4. IR
5. VM
6. SObject model
7. SOQL
8. DML
9. Test semantics
10. Debug/profile/LSP/server on top

DAP, LSP, watch mode, profiling, and server work should not start as full
features until the VM has stable source locations and execution hooks.

## Parallelizable Work

These can run in parallel after Phase 0:

- CLI/project loading and parser adapter
- metadata/schema loader and standard object seed schema
- compatibility fixture runner
- documentation and clean-room process

These can run in parallel after Phase 4:

- standard library method expansion
- test runner reporters
- trace format
- storage schema design

These can run in parallel after Phase 8:

- DAP
- LSP
- watch mode
- local API server
- native trace/profile analysis

## Testing Strategy

### Unit Tests

- Parser adapters
- Type assignability
- Overload resolution
- VM instruction behavior
- Collection/library methods
- SOQL parser and binder
- DML validation
- Limit counters

### Golden Tests

- Diagnostics
- JSON test output
- JUnit output
- Trace output
- LSP responses
- DAP protocol flows

### Compatibility Tests

- Run equivalent Apex in Salesforce and `oaer`.
- Compare:
  - return values
  - exceptions
  - stack traces where stable
  - DML side effects
  - SOQL result shape
  - trigger context
  - async outcomes
  - limit counters where practical

### Fuzz Tests

- Apex parser does not panic.
- SOQL parser does not panic.
- JSON serialization round trips.
- VM rejects invalid IR safely.

### Performance Tests

- Large project parse/index.
- Bulk insert/update trigger execution.
- Selector query performance.
- Watch invalidation latency.
- Debug stepping overhead.

## Runtime Compatibility Policy

Every feature should be classified as:

- supported: implemented and covered by compatibility tests
- partial: works for common cases, documented gaps
- stub: method exists but returns a controlled placeholder or unsupported error
- unsupported: explicit diagnostic before or during runtime
- unknown: not yet evaluated

Unsupported behavior should fail loudly and predictably. Silent wrong behavior
is worse than an unsupported-feature error.

## Data And Fixture Plan

Fixture system requirements:

- deterministic IDs
- deterministic clock
- deterministic user context
- seed records by object
- relationship references by alias
- anonymization support
- import from Salesforce CLI query output
- export from local SQLite
- reset per test
- package as replay bundle

Replay bundle layout:

```
bundle/
  oaer.yml
  schema/
  source/
  data/
  invocation.json
  expected.json
```

## Observability Plan

Runtime should expose:

- structured diagnostics
- structured test events
- trace events
- profile samples/aggregates
- limit snapshots
- query explain plans
- unsupported-feature inventory
- coverage of platform API methods invoked during a run

This observability is not optional. It is how the project avoids claiming
Salesforce parity where none exists.

## Documentation Plan

Docs needed before v1:

- installation
- quickstart
- project configuration
- schema loading
- fixture loading
- running tests
- debugging in VS Code
- profiling
- watch mode
- local server
- compatibility matrix
- unsupported features
- contributor guide
- clean-room guide
- architecture overview
- adding standard library methods
- adding SOQL features
- adding compatibility fixtures

## Staffing Plan

Minimum practical staffing:

- Runtime lead: VM, IR, sema, execution model
- Salesforce semantics lead: SObjects, SOQL, DML, triggers, tests, limits
- Tooling lead, part time until Phase 11: DAP, LSP, watch, editor integration
- Compatibility/release owner, part time: fixtures, CI, docs, dashboard

With one engineer, build in this strict order:

1. parser
2. symbols
3. sema
4. VM
5. no-DB tests
6. SObjects
7. SOQL
8. DML/triggers
9. test semantics
10. debugger/LSP/server

## Top Risks And Mitigations

- Wrong Apex semantics: mitigate with compatibility fixtures before feature
  claims.
- SOQL complexity: phase feature support and keep unsupported errors explicit.
- DML/trigger edge cases: start with bulk behavior and transaction rollback
  tests early.
- Debugger retrofitting cost: attach source locations to IR from the first VM
  milestone.
- LSP churn: defer advanced LSP until sema stabilizes.
- Release drift: version compatibility by Salesforce API version and schema
  bundle version.
- Legal contamination: keep clean-room docs and avoid proprietary internals as
  implementation references.

## Original First 30 Days Target

Historical target for a fresh build, retained for roadmap context:

1. Create `cmd/oaer`.
2. Add clean-room and architecture docs.
3. Add parser adapter.
4. Add `oaer parse`.
5. Add project loader for SFDX package directories.
6. Add symbol table for classes, methods, fields, triggers.
7. Add `oaer inspect symbols`.
8. Convert existing regex entry-point discovery to AST discovery.
9. Add the first compatibility fixture format and runner.
10. Publish M0/M1 project board.

## Original First 90 Days Target

Historical target for a fresh two-engineer build, retained for roadmap context:

1. Finish basic semantic analysis.
2. Add IR lowering for core statements and expressions.
3. Implement the initial VM.
4. Implement core `System`, collections, strings, dates, and assertions.
5. Implement no-DB test runner with JSON and JUnit output.
6. Add basic SObject value model.
7. Add schema loading for custom objects and fields.
8. Start SOQL parser/binder for simple queries.
9. Add trace hooks and source-location stack traces.
10. Release M2.

## v1 Completion Checklist

- `oaer test` works on supported enterprise fixture projects.
- `oaer exec` works for anonymous Apex over local data.
- `oaer test --json` and JUnit are stable.
- `oaer test --debug` works in VS Code.
- `oaer lsp` provides core editor features.
- `oaer server` supports local SObject CRUD and query.
- SOQL/DML/triggers/async/limits have documented compatibility status.
- Unsupported features are explicit.
- Compatibility dashboard is generated in CI.
- Release binaries and checksums are published.
- Docs cover installation, operation, contribution, and known gaps.

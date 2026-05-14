# Comprehensive Apex Parity Follow-Up Plan

Status date: 2026-05-14.

This plan follows `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`. The local-test plan
is the near-term product path: make the six enterprise example projects run
locally with scratch-org-like test behavior. This follow-up plan is broader. It
tracks the work needed for `oaer` to become a comprehensive Apex compatibility
runtime and tooling stack, beyond the first local-test claim.

Current checkpoint: `src-nmb-nutpl-develop` is the first green
example-project runtime sentinel at `total=761 pass=761`. The active
local-test closure target is now `sf-cred-pkg-develop`; the May 14, 2026
runtime baseline is `total=4268 pass=2993 unsupported=157 runtimeGap=594
assertFail=524 compileError=0 internalError=0`. Keep this plan behind the
local-test closure work until the active example-project blocker frontier is
green or explicitly accepted.

The goal is not to clone every Salesforce service. The goal is to make public
Apex language behavior, public platform APIs, metadata-driven data behavior, and
developer tooling predictable enough that real Apex projects can use `oaer` as
their default local feedback loop. For unsupported external platform behavior,
`oaer` should provide explicit diagnostics or deterministic local contracts
instead of silent no-ops.

## Relationship To Existing Plans

Use the plans in this order:

1. `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`: enterprise example-project local
   test execution and performance.
2. `docs/POST_PARITY_TODO.md`: post-parity backlog for old projects and local
   test blockers.
3. This document: comprehensive Apex parity after the example projects are
   running well.

Do not move a capability to `supported` until it has fixture or corpus coverage.
Use public Salesforce behavior, public grammars, owned fixtures, scratch-org
black-box probes, and the generated capability matrix as the evidence base.

## North-Star Claims

The comprehensive parity target should be split into explicit release claims:

| Claim | Meaning | Primary gate |
| --- | --- | --- |
| Enterprise local tests | Six example projects run locally with scratch-org-like results in bounded time. | `compat local-tests --check docs/fixtures/local-tests-corpus.json` |
| Apex language parity | Parser, sema, and VM accept and execute common production Apex language constructs. | Corpus parse/check/run parity gates plus parser shadow tests |
| Data platform parity | SOQL, SOSL fences, DML, transactions, triggers, storage, and describe match public platform behavior for supported objects. | Data/trigger/SOQL fixture suites |
| Platform API parity | Common `System`, `Database`, `Schema`, `Test`, `Messaging`, `ConnectApi`, `Auth`, `Site`, `Cache`, `Metadata`, and callout APIs have supported or explicit unsupported behavior. | Stdlib/capability matrix plus black-box fixtures |
| Tooling parity | LSP, DAP, watch, profile, server, and reports are reliable enough for daily development. | Tooling smoke and editor fixture suites |
| Release-ready parity | Docs, generated dashboards, and compatibility gates agree with implementation. | `go test ./...`, smoke, compat docs checks |

## Parallel Workstreams

These workstreams are intentionally larger than the enterprise-test phases.
They should be run by squad agents in separate worktrees with disjoint write
sets.

### P1: Apex Language Front End

Primary scope: `internal/apexast`, `internal/sema`, `internal/typesys`,
`internal/ir`, parser fixtures.

Goals:

- Accept Apex syntax used by large modern and legacy packages.
- Preserve stable ranges for every diagnostic, runtime trace, LSP item, and DAP
  frame.
- Make sema strict enough to catch real developer mistakes without rejecting
  valid Salesforce Apex.

Tasks:

- Finish compiler/parser support for all common Apex statements and expression
  contexts: multi-variable `for` initializers, nested assignment expressions,
  collection/map literals, safe navigation, casts, ternaries, chained calls,
  `switch`, all loop forms, try/multi-catch/finally, annotations, sharing
  modifiers, and nested declarations.
- Complete expression typing for arithmetic, Boolean, string, enum, SObject,
  generic collection, map, set, list, `Object`, null, and overloaded methods.
- Make all symbol and member lookup paths case-insensitive where Apex is
  case-insensitive.
- Resolve nested classes, inherited nested types, interfaces, abstract methods,
  and namespace-qualified names consistently in parser, sema, and VM.
- Add corpus shadow checks against example projects and owned minimal fixtures
  for every newly supported syntax family.

Validation:

```bash
go test ./internal/apexast ./internal/typesys ./internal/sema ./internal/ir ./internal/vm
go run ./cmd/oaer parse ./example-projects --json
go run ./cmd/oaer check --project ./example-projects/src-nmb-nutpl-develop --json
```

Exit criteria:

- Parser failures across the example corpus are zero.
- Sema false positives from scratch-org-passing code are tracked as bugs and
  driven down by fixtures.

### P2: VM Execution Semantics

Primary scope: `internal/vm`, `internal/apextest`, `internal/ir`.

Goals:

- Execute supported Apex with Salesforce-like call dispatch, state, exception,
  static lifecycle, and test isolation.
- Keep unsupported behavior explicit and typed.

Tasks:

- Finish static initialization, static reset, `@testSetup`, per-test org clone,
  per-test static state, `Test.startTest/stopTest`, async drain, and rollback
  composition.
- Complete method dispatch for overloaded, inherited, interface-typed,
  superclass-typed, namespace-qualified, nested, dynamic receiver, and
  `Object`-typed calls.
- Finish constructor semantics, object identity, equality/hash behavior, map/set
  key behavior, clone/deepClone, and collection iterators.
- Complete exception semantics for platform exception hierarchy, catch ordering,
  stack frames, line numbers, causes, rethrow, and finally unwinding.
- Add runtime profile hooks so slow tests can be attributed to compile, setup,
  DML, SOQL, trigger, automation, async, or assertion work.

Validation:

```bash
go test ./internal/vm ./internal/apextest
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --json
```

Exit criteria:

- Runtime failures in scratch-org-passing tests are categorized into known
  parity families, then reduced through fixtures.

### P3: Standard Library And Platform APIs

Primary scope: `internal/vm`, `internal/capability`, `docs/STDLIB_COVERAGE.md`.

Goals:

- Provide broad deterministic behavior for Apex APIs commonly used in
  enterprise code.
- Mark every incomplete surface as `partial`, `stub`, or `unsupported` with a
  stable diagnostic.

Tasks:

- Complete core `System`, `String`, numeric, `Date`, `Datetime`, `Time`,
  `TimeZone`, `Blob`, `EncodingUtil`, `Crypto`, `JSON`, `Type`, `Pattern`, and
  collection APIs to the level needed by real packages.
- Complete `Test` APIs: `createStub`, `setMock`, callout mocks, fixed search
  results, standard pricebook, test clock where possible, async semantics, and
  test-visible metadata context.
- Complete common platform namespaces used by tests: `Schema`, `Database`,
  `Messaging`, `ApexPages`, `PageReference`, `Site`, `Network`, `Auth`,
  `ConnectApi`, `Cache`, `Metadata`, `FeatureManagement`, `UserInfo`, `URL`,
  `RestContext`, and `Http*`.
- Add unsupported fences for real network transport, browser rendering, OAuth
  exchanges, external services, irreversible org mutation, and cloud-only APIs.
- Keep the generated stdlib coverage document synchronized after every status
  change.

Validation:

```bash
go test ./internal/vm ./internal/capability
go run ./cmd/oaer compat stdlib --check docs/STDLIB_COVERAGE.md
go run ./cmd/oaer compat mvp --json
```

Exit criteria:

- No API is marked `supported` without fixture evidence.
- Top enterprise runtime failures are no longer core stdlib gaps.

### P4: Metadata And Org Shape

Primary scope: `internal/metadata`, `internal/schema`, `internal/storage`,
`internal/resource`, `internal/project`, generated schema scripts.

Goals:

- Load real project metadata in source and legacy formats.
- Build a deterministic local org shape that matches what tests can observe.

Tasks:

- Complete source and legacy loading for objects, fields, record types, value
  sets, business processes, validation rules, compact layouts, layouts,
  profiles, permission sets, tabs, applications, quick actions, labels,
  translations, workflows, flows, custom metadata, named credentials, remote
  sites, static resources, content assets, email templates, and sites.
- Keep generated standard object/field schema current from public/scratch-org
  describe output, including Sales Cloud, Service Cloud, Person Accounts,
  Products, Pricebooks, Activities, Files, Campaigns, Cases, Users, Queues, and
  common Health/credentialing objects where public metadata is available.
- Preserve explicit project metadata over generated defaults.
- Add org-feature overlays for Person Accounts, multi-currency, state/country
  picklists, communities, platform cache, chatter, and common package features.
- Add deterministic fixture IDs and stable ordering for metadata-derived
  records.

Validation:

```bash
go test ./internal/metadata ./internal/schema ./internal/storage ./internal/resource ./internal/project
go run ./cmd/oaer schema load --project ./example-projects/src-nmb-nu-develop --json
go run ./cmd/oaer compat post-parity --project ./example-projects --json
```

Exit criteria:

- Metadata load failures are structured and rare.
- Runtime describe behavior matches the loaded project metadata and generated
  standard schema.

### P5: SOQL, SOSL Fences, And Query Engine

Primary scope: `internal/soql`, `internal/sobject`, `internal/storage`,
`internal/vm`.

Goals:

- Match SOQL behavior needed by selector-heavy enterprise packages.
- Provide explicit unsupported behavior for SOSL and cloud-only search until
  local search is intentionally implemented.

Tasks:

- Finish SOQL parsing for subqueries, relationship paths, polymorphic
  references, aliases, aggregate expressions, date literals, query options,
  `TYPEOF`, `FIELDS`, `WITH SECURITY_ENFORCED`, `WITH USER_MODE`,
  `FOR UPDATE`, `ALL ROWS`, semi-joins, anti-joins, and dynamic bind shapes.
- Complete row shaping for SObjects, `AggregateResult`, relationship fields,
  parent and child records, attributes URLs, explicit nulls, and unselected
  field errors.
- Add basic selectivity/index behavior and performance safeguards for large
  fixture datasets.
- Add precise `QueryException` diagnostics for unsupported grammar or
  unsupported execution semantics.
- Fence SOSL with stable unsupported diagnostics or add a deterministic local
  search model later as a separate claim.

Validation:

```bash
go test ./internal/soql ./internal/sobject ./internal/storage ./internal/vm
go run ./cmd/oaer test --project example-projects/NPSP-rel-3.237 --filter <selector-sentinel> --json
```

Exit criteria:

- Selector/service tests fail on real assertions or unsupported cloud behavior,
  not query parser/runtime gaps.

### P6: DML, Transactions, Triggers, And Declarative Automation

Primary scope: `internal/dml`, `internal/automation`, `internal/storage`,
`internal/vm`, `internal/trace`.

Goals:

- Match Salesforce-visible save order for tests.
- Compose Apex triggers, DML validation, Workflow, Flow, Process Builder,
  rollback, async scheduling, and side effects.

Tasks:

- Complete DML operations and `Database.*` result objects for insert, update,
  upsert, delete, undelete, merge, partial success, external IDs, duplicate
  rules where modeled, validation rules, lookup integrity, cascades, and row
  errors.
- Finish trigger contexts for before/after operations, bulk behavior, old/new
  maps, recursion limits, addError, savepoints, and rollback.
- Expand Workflow support: criteria, field updates, email alerts, task/outbound
  unsupported fences, recursion, limits, and trace output.
- Expand Flow and Process Builder support: decision, assignment, formula,
  collection variables, loops, record lookup/create/update/delete, invocable
  Apex, subflow fences, and transaction rollback.
- Add deterministic side-effect capture for emails, files, events where
  locally modeled, and async jobs.

Validation:

```bash
go test ./internal/dml ./internal/automation ./internal/storage ./internal/vm
go run ./cmd/oaer test --project example-projects/src-nmb-nu-develop --filter <automation-sentinel> --json
```

Exit criteria:

- Declarative side effects observed by Apex tests match scratch-org behavior for
  modeled metadata.

### P7: Async, Limits, And Runtime Windows

Primary scope: `internal/vm`, `internal/apextest`, `internal/storage`,
`internal/capability`.

Goals:

- Make asynchronous Apex and governor limits predictable in local tests.

Tasks:

- Complete `@future`, Queueable, Batchable, Schedulable, chained queueables,
  `AsyncApexJob`, `CronTrigger`, `System.schedule`, abort behavior, finalizer
  unsupported fences, and async exception surfacing.
- Match `Test.startTest/stopTest` limit windows and async drain ordering.
- Improve governor counters for SOQL, DML, heap, CPU, callouts, emails,
  queueables, futures, batches, scheduled jobs, publish events, and describe
  calls.
- Add strict/permissive mode coverage for every counter family.

Validation:

```bash
go test ./internal/vm ./internal/apextest
go run ./cmd/oaer compat run docs/fixtures/async-*.json
go run ./cmd/oaer compat run docs/fixtures/limits-*.json
```

Exit criteria:

- Async-heavy tests get stable pass/fail/unsupported outcomes without hidden
  background state leaks.

### P8: Security, Sharing, Permissions, And User Context

Primary scope: `internal/vm`, `internal/storage`, `internal/soql`,
`internal/dml`, `internal/schema`.

Goals:

- Model the parts of Salesforce security that tests commonly assert, while
  fencing full org security until explicitly implemented.

Tasks:

- Complete user/profile/permission set loading and current-user context.
- Add object and field permission checks where `WITH SECURITY_ENFORCED`,
  user-mode DML/query, describe accessibility, and test assertions need them.
- Model sharing mode enough for with/without/inherited sharing tests and
  user-context-sensitive queries.
- Implement `System.runAs` semantics for identity, mixed DML relaxation, and
  test transaction behavior.
- Add explicit unsupported diagnostics for full role hierarchy, territory,
  restriction rules, scoping rules, and external sharing models until supported.

Validation:

```bash
go test ./internal/vm ./internal/storage ./internal/soql ./internal/dml
go run ./cmd/oaer test --project example-projects/nams-workspace --filter <security-sentinel> --json
```

Exit criteria:

- Security-sensitive local tests either match scratch-org behavior or fail with
  a precise unsupported security capability.

### P9: Tooling, Server, Editor, And Developer Loop

Primary scope: `internal/lsp`, `internal/dap`, `internal/watch`,
`internal/profile`, `internal/server`, `internal/oaercli`.

Goals:

- Make the local runtime useful during daily development, not only in batch
  tests.

Tasks:

- Improve `oaer test --changed-since`, `--watch`, caching, invalidation, and
  affected-test selection.
- Make LSP diagnostics, hover, completion, definition, references, rename, and
  semantic tokens reflect the same symbol/sema/runtime model as tests.
- Extend DAP from snapshot debugging toward controlled execution where feasible.
- Expand local Salesforce-shaped server resources for Tooling, Composite, REST
  data APIs, metadata describes, and reset/fixture endpoints.
- Add profile output for parse, check, compile, setup, test execution, SOQL,
  DML, trigger, automation, async, and heap hotspots.

Validation:

```bash
go test ./internal/lsp ./internal/dap ./internal/watch ./internal/profile ./internal/server ./internal/oaercli
scripts/smoke.sh
```

Exit criteria:

- Focused local runs are fast enough to replace a scratch-org deploy/test loop
  for supported surfaces.

### P10: Compatibility Evidence, Docs, And Release Hardening

Primary scope: `internal/compat`, `internal/capability`, `docs`, `scripts`,
CI workflows.

Goals:

- Keep implementation, docs, generated dashboards, and release claims in sync.

Tasks:

- Add black-box scratch-org probe fixtures for behavior that is hard to infer
  from docs alone.
- Maintain generated docs:

```bash
go run ./cmd/oaer compat dashboard --output docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/oaer compat gaps --output docs/KNOWN_GAPS.md
go run ./cmd/oaer compat stdlib --output docs/STDLIB_COVERAGE.md
```

- Add dashboards for local-test corpus, enterprise runtime blockers, stdlib
  coverage, parser corpus status, and performance.
- Require every `supported` capability to cite fixture/corpus evidence.
- Add panic recovery and hardening tests for malformed Apex, malformed
  metadata, malformed fixtures, and malformed API requests.
- Keep release notes honest: preview until MVP and broader parity gates are
  green.

Validation:

```bash
go test ./...
go run ./cmd/oaer compat mvp --require-ready
go run ./cmd/oaer compat dashboard --check docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/oaer compat gaps --check docs/KNOWN_GAPS.md
go run ./cmd/oaer compat stdlib --check docs/STDLIB_COVERAGE.md
scripts/smoke.sh
```

Exit criteria:

- A release claim can be traced to commands and fixtures, not prose.

## Execution Order

The fastest practical path is:

1. Finish the enterprise example-project runtime plan.
2. Stabilize corpus baselines and timeout-safe reporting.
3. Clear dynamic proxy, compiler syntax, and standard schema token
   blockers because they unblock many failures at once.
4. Drive SOQL/DML/test-isolation gaps from broad project baselines.
5. Add declarative automation and platform API depth.
6. Harden performance, watch mode, LSP/DAP/server, and release gates.
7. Only then broaden to less common Salesforce APIs.

## Non-Goals Until Explicitly Claimed

These surfaces should remain explicit unsupported behavior unless a later plan
promotes them:

- Real network callouts without mocks.
- Browser rendering of Visualforce, Aura, LWC, or Experience Cloud.
- Real OAuth, SSO, external identity, or connected-app flows.
- Full Salesforce sharing/territory/restriction-rule enforcement.
- Full Metadata API deploy orchestration against real orgs.
- Full SOSL search ranking and external search backends.
- Cloud-only services such as Einstein, OmniStudio, Data Cloud, or external
  managed service runtimes.

## Definition Of Done

Comprehensive Apex parity is credible only when:

- `go test ./...` and smoke pass.
- The six enterprise example projects run locally in bounded time with
  scratch-org-like outcomes.
- Generated compatibility docs are in sync.
- Every `supported` capability has test evidence.
- Unsupported features fail with stable diagnostics.
- Local focused runs are materially faster than scratch-org deploy/test cycles.

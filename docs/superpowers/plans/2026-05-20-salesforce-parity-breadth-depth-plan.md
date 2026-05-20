# Salesforce Parity Breadth And Depth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build OAER toward Salesforce-like Apex execution where the same Apex project run in Salesforce and OAER produces the same test outcomes, observable side effects, and normalized traces.

**Architecture:** Use a batch-first implementation strategy. Build broad platform slices from public Salesforce behavior, stub catalogs, project metadata, scratch-org describes, and black-box oracle runs before using enterprise projects as corpus gates. Validate with owned fixtures, then large example projects, then future external projects.

**Tech Stack:** Go 1.26, `cmd/oaer`, `internal/apexast`, `internal/typesys`, `internal/sema`, `internal/ir`, `internal/vm`, `internal/apextest`, `internal/schema`, `internal/storage`, `internal/soql`, `internal/dml`, `internal/probe`, `internal/compat`, Salesforce CLI scratch orgs, Tooling API `executeAnonymous`, Apex test logs.

---

## Strategy

The current example projects are not the product boundary. They are a good saw log. They contain thousands of real Apex tests and enough old and new Salesforce shape to expose broad gaps.

This plan avoids the slow loop of fixing one failing enterprise test at a time. Instead it builds broad slices first, then uses enterprise projects as acceptance gates. Each slice includes compile shape, runtime behavior, metadata shape, side effects, trace output, and fixtures.

The work has two tracks:

1. **Breadth track:** make OAER accept and classify as much Salesforce Apex and org shape as practical. This covers symbols, generated stubs, standard objects, metadata, package dependencies, and explicit unsupported fences.
2. **Depth track:** make supported behavior match Salesforce. This covers VM semantics, DML, SOQL, triggers, test isolation, platform APIs, debug/oracle traces, and final data state.

The harness must compare Salesforce and OAER at stable observation points. Raw debug logs help diagnose. They should not be the only oracle. The durable oracle is normalized JSON: test result, exception, stack, debug payloads, SOQL/DML events, limits, async drain, emitted side effects, and selected final records.

## Current Inventory

Current checked example-project signals:

| Project | Role | Current frontier |
| --- | --- | --- |
| `src-nmb-nutpl-develop` | Fast VM/mock sentinel | Green: `761 pass=761`. |
| `sf-cred-pkg-develop` | Large runtime sentinel | Green: `4274 pass=4274`. |
| `src-nmb-nc-develop` | Legacy commerce/membership | Compile frontier: `znu.Address`. |
| `nams-workspace` | Multi-package workspace | Compile frontier: `znu.Pluggable`. |
| `src-nmb-nu-develop` | Large legacy package | Compile frontier around field/static resolution, e.g. `Status__c`. |
| `NPSP-rel-3.237` | Nonprofit domain corpus | Compile frontier: standard/generated type `CampaignMemberStatus`. |

Observed platform usage across example projects is concentrated in:

| Surface | Approximate references |
| --- | ---: |
| `System` | 58k |
| `Test` | 34k |
| `Schema` | 8k |
| custom metadata / `__mdt` | 6.5k |
| `Database` | 5k |
| `ConnectApi` | 4k |
| `ApexPages` / `PageReference` | 7k combined |
| `JSON` | 2.3k |
| HTTP mocks | 1.1k |
| `ContentVersion` / static resources | 900 combined |
| `Cache`, `Messaging`, `Auth`, `Site`, `Network` | smaller but important |

This says the high-return platform bundles are:

1. Apex language and compiler acceptance.
2. Test runner semantics.
3. Schema/metadata/standard objects.
4. Database/SOQL/DML/transaction behavior.
5. Visualforce/PageReference/ApexPages.
6. JSON, HTTP mocks, files/resources, email.
7. Managed package dependencies.
8. Primary ConnectApi and other platform namespaces.

## Release Claims

Do not claim "perfect Salesforce" as one undivided milestone. Use release claims that can be proved:

| Claim | Meaning | Gate |
| --- | --- | --- |
| Shape breadth | Projects compile or fail with typed dependency/unsupported diagnostics. | `compat local-tests --blockers-only` on all example projects. |
| Runtime breadth | All discovered tests receive pass/fail/unsupported/runtime_gap outcomes, not compile gaps. | `docs/fixtures/local-tests-example-projects.json`. |
| Behavioral depth | Supported tests match Salesforce outcome and normalized trace. | New `compat oracle-tests` gate. |
| Enterprise parity | Current enterprise corpus matches Salesforce for supported claims. | Per-project Salesforce-vs-OAER baseline. |
| Expansion ready | Adding a new project gives a structured inventory, not a new custom implementation path. | New project onboarding command and report. |

## Workstream A: Salesforce Oracle Harness

**Purpose:** Build the measuring stick before deeper runtime claims.

**Files:**

- Create: `internal/oracle/model.go`
- Create: `internal/oracle/normalize.go`
- Create: `internal/oracle/diff.go`
- Create: `internal/oracle/salesforce_runner.go`
- Create: `internal/oracle/local_runner.go`
- Create: `internal/oracle/report.go`
- Create: `internal/oracle/*_test.go`
- Modify: `internal/oaercli/compat.go`
- Modify: `internal/probe/tooling_snippet.go`
- Modify: `internal/apextest`
- Create: `docs/fixtures/oracle/*.json`
- Create: `docs/ORACLE_PARITY.md`

### Batch A1: Normalized Observation Model

- [ ] Define `OracleRun` with project, org alias, test class, test method, status, exception, stack, debug payloads, events, limits, side effects, final records, and timings.
- [ ] Define `OracleEvent` types: `method_call`, `soql`, `dml`, `trigger`, `flow`, `workflow`, `email`, `file`, `async`, `limit`, `assert`, `exception`, `debug`, `unsupported`.
- [ ] Define stable sorting and redaction rules for IDs, timestamps, generated usernames, stack line noise, async job IDs, and org IDs.
- [ ] Add unit tests proving normalization removes unstable IDs but preserves object type, field names, event order, exception type, and result value.

Validation:

```bash
go test ./internal/oracle
```

### Batch A2: Salesforce Runner

- [ ] Add a runner that deploys or assumes deployed source, runs selected Apex tests in the scratch org, and fetches `ApexTestResult`, `ApexTestQueueItem`, `ApexLog`, and relevant Tooling records.
- [ ] Add a runner mode for anonymous Apex snippets, used for narrow language/platform probes.
- [ ] Add finest logging only for targeted classes/methods. Do not collect finest logs for whole-project full runs by default.
- [ ] Parse Apex logs into normalized events for SOQL, DML, method entry/exit, exceptions, limits, and `USER_DEBUG`.
- [ ] Support opt-in `System.debug('OAER_ORACLE:' + JSON.serialize(payload))` markers for precise state capture.

Validation:

```bash
go test ./internal/oracle ./internal/probe
go run ./cmd/oaer compat oracle-tests --project example-projects/src-nmb-nutpl-develop --target-org oaer-probe-lab --filter <small-test> --golden-only --json
```

### Batch A3: OAER Runner And Diff

- [ ] Make `oaer test` emit the same `OracleRun` shape for selected tests.
- [ ] Add VM trace hooks where missing for SOQL, DML, trigger dispatch, email capture, file/content capture, async enqueue/drain, limit increments, and unsupported fences.
- [ ] Add `compat oracle-tests` to run Salesforce and OAER, then produce `pass`, `trace_mismatch`, `state_mismatch`, `exception_mismatch`, `unsupported`, `compile_gap`, and `infrastructure_error`.
- [ ] Persist compact artifacts under `.oaer/runs/<run-id>/oracle/`.

Validation:

```bash
go test ./internal/oracle ./internal/apextest ./internal/vm
go run ./cmd/oaer compat oracle-tests --project example-projects/src-nmb-nutpl-develop --target-org oaer-probe-lab --filter <small-test> --json
```

Exit criteria:

- Oracle diff works on at least one passing NUTPL test.
- A forced mismatch produces a readable event diff.
- Raw logs are retained as evidence, but normalized JSON drives pass/fail.

## Workstream B: Compile And Shape Breadth

**Purpose:** Make broad Salesforce-shaped projects compile or fail with precise dependency/unsupported diagnostics.

**Files:**

- Modify: `internal/apexast`
- Modify: `internal/typesys`
- Modify: `internal/sema`
- Modify: `internal/schema`
- Modify: `internal/project`
- Modify: `internal/config`
- Modify: generated schema/stub scripts
- Create/modify: `testdata/local-tests/shape-breadth-*`

### Batch B1: Apex Language Acceptance

- [ ] Implement broad parser/compiler support for common Apex constructs: multi-variable `for` initializers, nested collection literals, nested map literals, safe navigation, casts, ternaries, all loop forms, switch, try/multi-catch/finally, inherited nested types, enum constants, annotations, sharing modifiers, and owner-relative references.
- [ ] Add corpus tests that parse and check all example projects without dumping full logs.
- [ ] Add fixture suites for every syntax family accepted by Salesforce.

Validation:

```bash
go test ./internal/apexast ./internal/typesys ./internal/sema ./internal/ir ./internal/vm
go run ./cmd/oaer parse ./example-projects --json
go run ./cmd/oaer compat local-tests --project ./example-projects --blockers-only --top-failures 50 --json
```

### Batch B2: Standard Object And Field Breadth

- [ ] Expand generated standard schema from scratch-org describes and checked stub overlays.
- [ ] Cover Campaign, CampaignMember, CampaignMemberStatus, Product, Pricebook, Opportunity, Order, Case, Activity, User, Profile, Group, Queue, PermissionSet, Content/File, Territory, Person Account, and common Service/Sales objects.
- [ ] Preserve feature overlays for Person Accounts, communities, multi-currency, state/country picklists, Platform Cache, and Sites.
- [ ] Make missing standard metadata produce `standard_schema_missing`, not ordinary unknown type errors.

Validation:

```bash
go test ./internal/schema ./internal/storage ./internal/typesys ./internal/sema
go run ./cmd/oaer schema load --project ./example-projects/NPSP-rel-3.237 --json
go run ./cmd/oaer compat local-tests --project ./example-projects/NPSP-rel-3.237 --blockers-only --top-failures 20 --json
```

### Batch B3: Managed Package Dependency Artifacts

- [ ] Implement source-backed and artifact-backed managed package dependency loading from `oaer.yml`.
- [ ] Export only subscriber-visible `global` Apex contracts across namespaces.
- [ ] Load namespaced objects, fields, labels, resources, custom metadata, and dependency schema before consumer projects.
- [ ] Add explicit outcomes: `dependency_missing`, `dependency_version_mismatch`, `dependency_load_error`, `dependency_access_denied`.
- [ ] Build a first `znu` artifact path sufficient for NC and NAMS shape, then generalize it.

Validation:

```bash
go test ./internal/config ./internal/project ./internal/typesys ./internal/sema ./internal/schema ./internal/vm ./internal/compat
go run ./cmd/oaer test --project testdata/local-tests/managed-package-consumer --json
go run ./cmd/oaer compat local-tests --project example-projects/src-nmb-nc-develop --blockers-only --top-failures 20 --json
go run ./cmd/oaer compat local-tests --project example-projects/nams-workspace --blockers-only --top-failures 20 --json
```

Exit criteria:

- The four compile-frontier projects move from compile gaps into runtime outcomes or typed dependency diagnostics.
- No project-specific runtime branches are added.

## Workstream C: VM And Test Runner Depth

**Purpose:** Make supported Apex run with Salesforce-like lifecycle, state, and exceptions.

**Files:**

- Modify: `internal/vm`
- Modify: `internal/ir`
- Modify: `internal/apextest`
- Modify: `internal/testreport`
- Create/modify: `testdata/local-tests/vm-*`

### Batch C1: Test Lifecycle Semantics

- [ ] Implement complete `@testSetup` behavior, per-test cloned org state, static reset, test user context, `System.runAs`, test clock handling, rollback boundaries, and deterministic setup data.
- [ ] Complete `Test.startTest/stopTest` for limit reset, async drain, scheduled/batch/queueable/future execution, and nested unsupported fences.
- [ ] Add structured outcome categories for `assert_fail`, `runtime_gap`, `unsupported`, `compile_gap`, `state_mismatch`, and `trace_mismatch`.

Validation:

```bash
go test ./internal/apextest ./internal/vm ./internal/testreport
go run ./cmd/oaer test --project testdata/local-tests/org-like-runner --json
```

### Batch C2: Dispatch, Object, And Exception Semantics

- [ ] Finish overloaded, inherited, interface-typed, superclass-typed, namespace-qualified, nested-class, dynamic receiver, and `Object`-typed call dispatch.
- [ ] Finish constructor chaining, static initialization ordering, object identity, `equals`, `hashCode`, map/set key behavior, clone/deepClone, collection iterators, and `Iterable`.
- [ ] Finish platform exception hierarchy, catch ordering, stack frames, line numbers, rethrow, causes, and finally unwinding.

Validation:

```bash
go test ./internal/vm
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --json
go run ./cmd/oaer test --project example-projects/sf-cred-pkg-develop --parallel 4 --json
```

### Batch C3: Mock Framework Semantics

- [ ] Complete `System.StubProvider` and `Test.createStub` method metadata, argument capture, return dispatch, exception propagation, void calls, and object identity.
- [ ] Implement matcher lifecycle patterns used by enterprise mock frameworks: custom matchers, combined matchers, ordered verification, any-order verification, never/times verification, and exception stubbing.
- [ ] Make `HttpCalloutMock`, `WebServiceMock`, and request/response-shaped callout mocks converge on the same invocation recording model.

Validation:

```bash
go test ./internal/vm ./internal/apextest
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --filter Mock --json
```

Exit criteria:

- Green sentinels stay green.
- New runtime outcomes expose real behavior gaps, not framework plumbing gaps.

## Workstream D: Data Platform Depth

**Purpose:** Match Salesforce-visible behavior for SOQL, DML, triggers, transactions, and describes.

**Files:**

- Modify: `internal/soql`
- Modify: `internal/dml`
- Modify: `internal/storage`
- Modify: `internal/sobject`
- Modify: `internal/schema`
- Modify: `internal/vm`
- Create/modify: `testdata/local-tests/data-platform-*`

### Batch D1: SOQL And Describe

- [ ] Expand SOQL support for relationship fields, subqueries, semi/anti joins where practical, aggregates, group by, order/nulls, limit/offset, date literals, bind expressions, `TYPEOF` fences, query locator iteration, and security-mode fences.
- [ ] Make describe calls reflect loaded metadata, record types, picklists, field sets, child relationships, accessible/updateable flags, and feature overlays.
- [ ] Add describe oracle probes for high-use `Schema.*` calls.

Validation:

```bash
go test ./internal/soql ./internal/schema ./internal/sobject ./internal/vm
go run ./cmd/oaer compat oracle-tests --suite schema-soql --target-org oaer-probe-lab --json
```

### Batch D2: DML, Triggers, Transactions

- [ ] Expand insert/update/upsert/delete/undelete/merge behavior, partial success, allOrNone, SaveResult/Error shapes, duplicate rules fences, assignment rules fences, validation rules, required fields, lookup validation, owner/defaults, autonumber, audit fields, and rollback snapshots.
- [ ] Complete trigger order, before/after contexts, `Trigger.*` variables, recursion behavior, per-transaction static state, and bulk list handling.
- [ ] Add transaction event traces for DML, trigger, workflow, flow, async, and rollback.

Validation:

```bash
go test ./internal/dml ./internal/storage ./internal/vm ./internal/apextest
go run ./cmd/oaer test --project testdata/local-tests/data-platform-dml --json
```

Exit criteria:

- Test data setup in enterprise projects behaves like Salesforce for supported objects.
- Final record-state oracle diffs pass on owned fixtures.

## Workstream E: Platform API Bundles

**Purpose:** Build breadth and depth across platform APIs by namespace, not one method at a time.

**Files:**

- Modify: `internal/vm/stdlib.go`
- Modify: `internal/vm/vm.go`
- Modify: `internal/capability`
- Modify: `docs/STDLIB_COVERAGE.md`
- Modify: generated platform stub tools
- Create/modify: `testdata/local-tests/platform-*`

### Batch E1: Core Apex APIs

- [ ] Complete high-use `System`, assertions, `String`, `Blob`, `EncodingUtil`, `Crypto`, numeric, `Date`, `Datetime`, `Time`, `TimeZone`, `Type`, `JSON`, `Pattern`, `Matcher`, collections, and exception classes.
- [ ] Use stub probe full results to fill broad return/default/error contracts.
- [ ] Keep unsupported methods typed and explicit.

Validation:

```bash
go test ./internal/vm ./internal/capability
go run ./cmd/oaer compat stdlib --check docs/STDLIB_COVERAGE.md
go run ./cmd/oaer probe summarize probes/output/stub-full/gap-report.json --top-stub
```

### Batch E2: Enterprise Platform APIs

- [ ] Build broad contracts for `Database`, `Schema`, `Test`, `ApexPages`, `PageReference`, `Messaging`, `Http*`, `UserInfo`, `URL`, `FeatureManagement`, `Cache`, `Auth`, `Site`, `Network`, `ContentVersion`, static resources, email templates, and files.
- [ ] Implement test-visible behavior. Add explicit fences for real browser rendering, OAuth exchange, real network calls, external service calls, and cloud-only mutation.
- [ ] Add oracle probes for common methods that are easy to compare in a scratch org.

Validation:

```bash
go test ./internal/vm ./internal/apextest ./internal/resource
go run ./cmd/oaer test --project testdata/local-tests/platform-apis --json
go run ./cmd/oaer compat oracle-tests --suite platform-apis --target-org oaer-probe-lab --json
```

### Batch E3: ConnectApi Scope

- [ ] Keep ConnectApi breadth shape broad enough to compile real code.
- [ ] Deepen only primary enterprise-use areas first: Chatter feeds/pages, NamedCredentials/ExternalCredentials, Organization settings, managed content, common feed/photo/binary input types, and high-use enum/value DTOs.
- [ ] For other ConnectApi classes, provide explicit unsupported or deterministic DTO behavior, not silent nulls.

Validation:

```bash
go test ./internal/vm ./internal/typesys ./internal/sema
go run ./cmd/oaer compat local-tests --project ./example-projects --blockers-only --top-failures 50 --json
```

Exit criteria:

- Core and enterprise API bundles have fixture coverage.
- Stub full gaps shrink by families, not by individual probe IDs.

## Workstream F: Metadata, Files, UI, And Declarative Automation

**Purpose:** Support what Apex tests can observe from Salesforce org shape.

**Files:**

- Modify: `internal/project`
- Modify: `internal/schema`
- Modify: `internal/storage`
- Modify: `internal/resource`
- Modify: `internal/vm`
- Modify: Flow/Workflow support packages if present
- Create/modify: `testdata/local-tests/metadata-*`

### Batch F1: Metadata Loader Breadth

- [ ] Load source and legacy metadata for objects, fields, record types, value sets, business processes, labels, translations, tabs, apps, profiles, permission sets, custom metadata, static resources, content assets, named credentials, remote sites, sites, pages, components, workflows, flows, email templates, reports, and dashboards.
- [ ] Preserve project metadata over generated defaults.
- [ ] Add warnings for loaded-but-not-executable metadata, separate from hard failures.

Validation:

```bash
go test ./internal/project ./internal/schema ./internal/storage ./internal/resource
go run ./cmd/oaer compat post-parity --project ./example-projects --json --require-ready
```

### Batch F2: Test-Visible UI And Declarative Behavior

- [ ] Complete Visualforce page registry, `Page.*`, `PageReference`, `ApexPages.StandardController`, `StandardSetController`, messages, params, redirects, and view-state fences.
- [ ] Implement workflow field updates and flow record lookup/create/update shapes used by tests.
- [ ] Capture emails, files, content versions, and generated side effects inside rollback-aware test transactions.

Validation:

```bash
go test ./internal/vm ./internal/apextest ./internal/storage
go run ./cmd/oaer test --project testdata/local-tests/ui-controller-contracts --json
go run ./cmd/oaer test --project testdata/local-tests/workflow --json
go run ./cmd/oaer test --project testdata/local-tests/flow --json
```

Exit criteria:

- Apex tests can observe metadata-backed behavior without UI rendering.
- Declarative automation has trace events and rollback behavior.

## Workstream G: Corpus Expansion And Release Gates

**Purpose:** Make future projects cheap to add and hard to misclassify.

**Files:**

- Modify: `internal/compat`
- Modify: `internal/projectscan`
- Modify: `internal/profile`
- Modify: `docs/fixtures/local-tests-example-projects.json`
- Create: `docs/fixtures/oracle-example-projects.json`
- Create: `scripts/baseline-oracle-example-projects.mjs`
- Create: `docs/PROJECT_ONBOARDING.md`

### Batch G1: Project Onboarding Inventory

- [ ] Add a command or script that scans a new SFDX project and reports Apex volume, test count, metadata types, scratch-org features, package dependencies, namespace usage, platform API usage, and first compile/runtime frontier.
- [ ] Emit a project readiness report with `compile_shape`, `metadata_shape`, `runtime_surface`, `oracle_ready`, and `unsupported_surface` sections.
- [ ] Store compact baselines in `docs/fixtures`.

Validation:

```bash
go test ./internal/compat ./internal/projectscan
go run ./cmd/oaer compat project-inventory --project ./example-projects/sf-cred-pkg-develop --json
```

### Batch G2: Corpus Gates

- [ ] Define three corpus gates: fast owned fixtures, medium sentinel projects, large enterprise projects.
- [ ] Add a nightly/full command for large oracle comparison.
- [ ] Add a developer command for focused oracle comparison on one test class/method.
- [ ] Track trend history for pass/fail/unsupported/runtime_gap/trace_mismatch/state_mismatch.

Validation:

```bash
go test ./...
go run ./cmd/oaer compat local-tests --check docs/fixtures/local-tests-corpus.json
go run ./cmd/oaer compat local-tests --check docs/fixtures/local-tests-example-projects.json
go run ./cmd/oaer compat oracle-tests --check docs/fixtures/oracle-example-projects.json
```

Exit criteria:

- Adding a seventh enterprise project produces a comparable report in one command.
- Regressions show which surface moved.

## Implementation Order

This order favors breadth first, then depth, then expansion:

1. **Oracle harness skeleton:** normalized model, local runner, Salesforce runner, diff report.
2. **Shape breadth batch:** Apex language acceptance, standard schema breadth, managed package artifacts.
3. **Core runtime batch:** test lifecycle, dispatch/object/exception semantics, mock framework semantics.
4. **Data platform batch:** SOQL/describe, DML/triggers/transactions.
5. **Platform API batch:** core stdlib, enterprise APIs, scoped ConnectApi depth.
6. **Metadata/declarative batch:** UI controller contracts, resources/files/email, workflow/flow.
7. **Corpus expansion batch:** project inventory, oracle baselines, trend gates.

Each batch should be implemented in a worktree. Each batch should land broad tests before running the full enterprise gate. A good batch may take longer than a small patch. That is intended.

## Parallel Squad Layout

| Squad | Write scope | First deliverable |
| --- | --- | --- |
| Oracle | `internal/oracle`, `internal/oaercli`, `internal/probe`, trace output | `compat oracle-tests` working on one NUTPL test. |
| Shape | `internal/apexast`, `internal/typesys`, `internal/sema`, `internal/schema`, `internal/project` | Four compile-frontier projects move to runtime/dependency outcomes. |
| VM | `internal/vm`, `internal/ir`, `internal/apextest` | Broad lifecycle/dispatch/exception fixture suite green. |
| Data | `internal/soql`, `internal/dml`, `internal/storage`, `internal/sobject` | SOQL/DML/trigger oracle fixtures green. |
| Platform | `internal/vm/stdlib.go`, generated stubs, `internal/capability` | Core platform API bundle fixtures green. |
| Metadata | metadata/resource/UI/declarative packages | Visualforce, resources, files/email, workflow/flow fixtures green. |
| Corpus | `internal/compat`, `internal/projectscan`, docs fixtures/scripts | New-project onboarding report and corpus trend gates. |

## Debug Log Policy

Use debug logs as a microscope, not as the whole measuring stick.

- Finest logs are valuable for targeted tests and probes.
- Full-project finest logs are too noisy and slow for default runs.
- Parse logs into normalized events.
- Prefer explicit `OAER_ORACLE:` JSON debug payloads where a test needs exact state comparison.
- Keep raw logs as artifacts for investigation.
- Do not make pass/fail depend on raw line-for-line log text.

The useful Salesforce logging levels for targeted runs:

```text
ApexCode=FINEST
ApexProfiling=FINE
Database=INFO or FINE
Workflow=FINE
Validation=INFO
Callout=INFO
System=DEBUG
Visualforce=INFO
```

## Definition Of Done

A platform surface is "supported" only when:

- OAER compiles representative Apex accepted by Salesforce.
- OAER runtime output matches Salesforce for owned fixtures.
- Oracle diff passes for at least one black-box scratch-org probe where practical.
- Enterprise corpus tests that use the surface pass or move to a different blocker.
- Unsupported cloud-only behavior has a typed diagnostic.
- Docs and generated capability reports match implementation.

## Main Validation Commands

Fast checks:

```bash
go test ./internal/oracle ./internal/apextest ./internal/vm ./internal/sema ./internal/typesys
go run ./cmd/oaer compat local-tests --project example-projects/src-nmb-nutpl-develop --parallel 4 --json
```

Medium checks:

```bash
go test ./internal/soql ./internal/dml ./internal/storage ./internal/schema ./internal/project ./internal/compat
go run ./cmd/oaer compat local-tests --project example-projects/sf-cred-pkg-develop --parallel 4 --timeout 30000 --top-failures 20 --json
go run ./cmd/oaer compat post-parity --project ./example-projects --json --require-ready
```

Full checks:

```bash
go test ./...
go run ./cmd/oaer compat local-tests --check docs/fixtures/local-tests-corpus.json
go run ./cmd/oaer compat local-tests --check docs/fixtures/local-tests-example-projects.json
go run ./cmd/oaer compat oracle-tests --check docs/fixtures/oracle-example-projects.json
go run ./cmd/oaer compat dashboard --check docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/oaer compat gaps --check docs/KNOWN_GAPS.md
go run ./cmd/oaer compat stdlib --check docs/STDLIB_COVERAGE.md
```

## First Batch To Start

Start with two worktrees in parallel:

1. **Oracle harness skeleton.** This makes every later depth decision measurable.
2. **Shape breadth batch.** This unlocks more enterprise projects so the oracle has more timber to work.

The first shape batch should include managed package artifacts, standard schema breadth for NPSP, and broad sema fixes for field/static resolution. That is the fastest way to move from two green projects to the full enterprise corpus.


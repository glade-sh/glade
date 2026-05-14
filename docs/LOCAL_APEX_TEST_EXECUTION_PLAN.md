# Local Apex Test Execution Plan

Status date: 2026-05-14.

This plan turns the broad post-parity backlog into squad-sized implementation
phases for full local Apex test execution. The target is not merely loading
large projects. The target is to run Apex tests locally with org-like behavior:
the same project metadata resolves, the same setup data and test transactions
apply, the same platform APIs are available where tests use them, the same
declarative side effects fire when they matter, and the result clearly reports
which tests pass, fail, or are blocked by an explicit unsupported feature.

The current server-example gate is green:

```text
pass=101 fail=0 unsupported=0 missing=0
```

The owned local-test corpus gate is green for the checked baseline, including
intentional unsupported classifications. The broader post-parity inventory is
also green for the checked `example-projects` corpus. A May 7, 2026 inventory
from the current checkout reports:

```text
filesScanned=50457 findings=0 testBlockingFindings=0 surfaces=0
```

That inventory is implementation-aware as of this checkpoint. It suppresses
surfaces only when the project metadata, static model, or runtime fallback can
resolve them generally: generated standard object/field metadata, loaded
labels/translations, managed-package and platform label fallbacks, loaded static
resources/content assets, named credential and remote site endpoints,
namespace-tolerant custom metadata type references, legacy presentation
metadata, Visualforce page/component metadata, Visualforce controller and action
contracts, Aura/LWC Apex discovery, Workflow rules, and the modeled
record-lookup/record-create Flow shapes are no longer reported as broad
post-parity blockers. The zero-blocker scan is a readiness inventory result,
not a claim that every Salesforce UI, metadata, or automation behavior is fully
implemented at runtime.

Use this document for parallel squad planning. Use
`docs/POST_PARITY_TODO.md` as the exhaustive backlog and capability boundary.
Use `docs/MANAGED_PACKAGE_DEPENDENCY_PLAN.md` for source-backed and
version-pinned installed package dependency handling.
Use `docs/APEX_PARITY_FOLLOWUP_PLAN.md` for the broader Apex language,
runtime, platform API, tooling, and release-hardening roadmap after the
enterprise example-project local-test path is under control.

Current execution focus as of May 14, 2026: drive
`example-projects/sf-cred-pkg-develop` to zero local-test blockers. The latest
accurate local run used:

```bash
go build -o /private/tmp/oaer-perf ./cmd/oaer
/private/tmp/oaer-perf compat local-tests \
  --project ./example-projects/sf-cred-pkg-develop \
  --parallel 4 \
  --top-failures 60 \
  --json
```

Measured result:

```text
total=4268 pass=3146 unsupported=98 runtimeGap=445 assertFail=579 compileError=0 internalError=0 durationMs=269872
```

The next blocker frontier is not compile loading. It is runtime fidelity:
JSON/SObject describe shape gaps, standard metadata/relationship coverage,
remaining DML behavior, and the top assert-failure families now reached after
the UUID, nested enum, Java-regex lookahead, and `Account.ChildAccounts`
fixes. `docs/plans/2026-05-07-src-nmb-nu-develop-squad-plan.md` is retained as
historical planning for the larger NU project but is no longer the active
frontier while `sf-cred-pkg-develop` is the zero-blocker target.

## Execution Objective

The product goal is a local edit-test loop for Apex projects that is much
faster than deploying to Salesforce and waiting for platform test execution.
That means the first release claim is not "complete Salesforce." The first
release claim is:

- Load a large Salesforce-shaped project without project-specific patches.
- Run its Apex tests locally through the same command developers use while
  editing code.
- Match Salesforce-visible behavior for the metadata, DML, SOQL, trigger,
  async, controller, platform API, and declarative surfaces those tests touch.
- Classify unsupported behavior separately from real test failures.
- Keep the common local loop fast enough that developers can run focused tests
  continuously while changing Apex.

Target command shape:

```bash
oaer test --project . --filter MyClassTest --json
oaer test --project . --changed-since main --json
oaer test --project . --watch --watch-backend auto --json
```

The compatibility commands below are the engineering gates. The user-facing
success path is still `oaer test`.

## Milestone Ladder

These milestones are ordered by developer value, not by feature count.

| Milestone | Claim | Required gate |
| --- | --- | --- |
| M0: Server examples green | Local Salesforce-shaped API probes work for the checked corpus. | `go run ./cmd/oaer compat server-examples --json` reports no fail, unsupported, or missing probes. |
| M1: Local-test gate exists | Every discovered test method receives `pass`, `fail`, `unsupported`, `load_error`, `compile_error`, or `internal_error`. | `go run ./cmd/oaer compat local-tests --project testdata/local-tests/basic --json` |
| M2: Metadata-resolved tests | Legacy objects, custom metadata records, labels, resources, endpoints, and presentation metadata load well enough that metadata load/resolve blockers fall sharply. | `compat local-tests` plus `compat post-parity --json` show reduced `load` and `resolve` blockers. |
| M3: Controller-test ready | Visualforce controller tests, `Page.*`, `PageReference`, `ApexPages`, Aura Apex discovery, and LWC Apex imports execute or produce precise unsupported diagnostics. | `compat local-tests --project testdata/local-tests/ui-controller-contracts --json` |
| M4: Platform-test ready | `System.Callable`, `Test.createStub`, Site/Network/Auth, ConnectApi org settings, Platform Cache, and endpoint resolution work for local tests. | `compat local-tests --project testdata/local-tests/platform-apis --json` |
| M5: Side-effect ready | Files, email templates, captured emails, and rollback-visible side effects behave like test transaction state. | `compat local-tests --project testdata/local-tests/files-email --json` |
| M6: Declarative-test ready | Workflow and Flow side effects run inside the DML/test transaction with traceable decisions and rollback. | `compat local-tests --project testdata/local-tests/workflow --json` and `.../flow --json` |
| M7: Legacy-project-test ready | Owned corpus fixtures modeled after the example projects are green, and remaining unsupported surfaces are outside the documented claim. | `compat local-tests --check docs/fixtures/local-tests-corpus.json` |

M0 through the checked M7 corpus gate are green. The corpus now includes passing
coverage for local tab describe metadata through `Schema.describeTabs()`. The
remaining work is to broaden the owned corpus and deepen runtime fidelity for
surfaces that are outside the current fixture claim.

## Speed Requirements

Local execution must be faster because it avoids deploy, org scheduling, and
remote test startup. Preserve that advantage as features are added:

- Focused test run: load only the selected project packages and execute the
  filtered test class or method.
- Changed-test run: use the existing dependency graph and watcher machinery to
  select affected tests for changed Apex or metadata.
- Warm watch run: reuse parsed source, type indexes, schema registries, metadata
  registries, and compiled IR when inputs are unchanged.
- Per-test isolation: clone org/test state cheaply using storage snapshots, not
  full project reloads.
- Unsupported classification: stop at the first capability-specific blocker for
  a test method instead of burning time in broad fallback execution.
- Trace/profile on demand: collect detailed traces only for failures, blockers,
  or explicit profiling flags.

Performance gates should be added once M1 exists:

```bash
oaer test --project testdata/local-tests/basic --filter PassingTest --json
oaer test --project testdata/local-tests/org-like-runner --changed-since main --json
oaer test --project testdata/local-tests/org-like-runner --watch --watch-once --json
```

The exact millisecond budget should be set from baseline measurements on the
owned fixtures, then tightened as caching lands.

## Enterprise Example-Project Runtime Gap Plan

The six checked `example-projects` directories are the runtime parity target for
the next phase. The current static/readiness inventory is green, and the fast
NUTPL runtime sentinel now passes end to end. The other five example projects
still stop at measured compile-gap frontiers, so the project does not yet have
full runtime support for all example projects. Treat scratch-org pass results as
the behavioral target: failures in these projects are OAER parity gaps unless
proven otherwise.

Current example-project set:

| Project | Runtime role |
| --- | --- |
| `NPSP-rel-3.237` | Large nonprofit domain/trigger/service corpus with heavy builders, metadata, SOQL, and fluent APIs. |
| `src-nmb-nc-develop` | Large legacy package with extensive custom metadata, UI/controller contracts, and currently expensive local runtime execution. |
| `src-nmb-nu-develop` | Large legacy package with Workflow/Flow, Visualforce/Aura, and broad metadata shape. |
| `src-nmb-nutpl-develop` | Smaller mock-framework-heavy package that gives fast focused feedback on VM behavior. |
| `sf-cred-pkg-develop` | Large credentialing package with namespaces, HTTP/callout tests, map/list literal usage, and service models. |
| `nams-workspace` | Large workspace with namespaced test setup, mock framework usage, metadata, endpoint, and UI controller surfaces. |

Current measured runtime frontier:

| Gate | Current signal |
| --- | --- |
| Static/readiness inventory | `go run ./cmd/oaer compat post-parity --project ./example-projects --json` reports `filesScanned=50457 findings=0 testBlockingFindings=0 surfaces=0`. |
| Fast runtime sentinel | `go run ./cmd/oaer compat local-tests --project example-projects/src-nmb-nutpl-develop --timeout 30000 --top-failures 8 --json` reports `total=761 pass=761 fail=0 unsupported=0 loadError=0 compileError=0 internalError=0`. |
| Six-project runtime baseline | `node scripts/baseline-local-tests-example-projects.mjs` records one green project and five compile-gap frontiers in `docs/fixtures/local-tests-example-projects.json`. |
| Scale runtime sentinel | `go run ./cmd/oaer compat local-tests --project example-projects/src-nmb-nc-develop --timeout 30000 --top-failures 8 --json` returns structured compile-gap output rather than timing out or stack-overflowing; the current top blocker is missing `znu.Address`. |
| Unit/regression suite | `go test ./...` must stay green after every merge. |

### Runtime Closure Phases

These phases should be tackled by parallel squad agents with disjoint write
sets. Each phase must add owned fixtures or targeted package tests before
claiming support. Do not add project-specific branches in runtime code.

#### Phase E0: Managed Package Dependency Artifacts

Goal: load installed managed package dependencies before compiling consuming
enterprise projects.

Primary write scope: `internal/config`, `internal/project`,
`internal/typesys`, `internal/sema`, `internal/schema`, `internal/vm`,
`internal/compat`, `docs/MANAGED_PACKAGE_DEPENDENCY_PLAN.md`.

Current blockers:

- `src-nmb-nc-develop` references installed package Apex such as `znu.Address`.
- `nams-workspace` references installed package Apex such as `znu.Pluggable`
  and installed package schema such as `znu__CartItemLine__c`.
- Treating those as ordinary current-project compile gaps hides the real
  prerequisite: OAER needs the `znu` managed package contract loaded first.

Tasks:

- Add explicit first-iteration dependency config in `oaer.yml`, mapping a
  namespace to a local source project root and optional package version.
- Build a source-backed managed package artifact model for Apex contracts,
  schema, labels, resources, custom metadata, and other test-visible metadata.
- Load dependency artifacts before current project package directories.
- Resolve `namespace.Type`, nested dependency types, `namespace__Object__c`,
  namespaced fields, labels, resources, and custom metadata through dependency
  registries.
- Enforce managed-package boundaries: consuming packages can access dependency
  Apex only through `global` top-level types and `global` members.
- Split missing package, version mismatch, dependency load error, and
  dependency access denial from ordinary compile/runtime gaps in JSON output.

Validation:

```bash
go test ./internal/config ./internal/project ./internal/typesys ./internal/sema ./internal/schema ./internal/vm ./internal/compat
go run ./cmd/oaer test --project testdata/local-tests/managed-package-consumer --json
go run ./cmd/oaer test --project testdata/local-tests/managed-package-access --json
go run ./cmd/oaer compat local-tests --project example-projects/src-nmb-nc-develop --timeout 30000 --top-failures 8 --json
go run ./cmd/oaer compat local-tests --project example-projects/nams-workspace --timeout 30000 --top-failures 8 --json
```

Exit criteria:

- Missing `znu` setup reports `dependency_missing` rather than unknown type or
  unknown SObject compile gaps.
- Source-backed `znu` setup moves `znu.Address`, `znu.Pluggable`, and
  `znu__*` blockers to successful resolution or package-scoped diagnostics.
- Cross-namespace access rejects dependency `public` APIs and allows dependency
  `global` APIs.
- No project-specific `znu` stubs or runtime branches are added.

#### Phase E1: Local-Test Corpus Baseline And Triage

Goal: make the six-project runtime gap measurable and stable.

Primary write scope: `internal/compat`, `internal/apextest`, `docs/fixtures`,
`docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`.

Tasks:

- Add or refresh a checked `local-tests-corpus` baseline that records summary
  counts and top blocker families per example project.
- Add `--top-failures`, `--timeout`, and `--profile-on-timeout` support to the
  local-test compatibility/reporting path if missing.
- Split local runtime outcomes into `assert_fail`, `runtime_gap`,
  `unsupported`, `compile_gap`, `internal_error`, and `timeout`.
- Persist a small focused sentinel for each project so future runs do not need
  the full corpus to detect regressions.
- Add a timeout-safe runner path so large projects never hang a squad lane
  indefinitely.

Validation:

```bash
go test ./internal/compat ./internal/apextest ./internal/testreport
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --json
go run ./cmd/oaer test --project example-projects/src-nmb-nc-develop --filter <focused-sentinel> --json
go run ./cmd/oaer compat post-parity --project ./example-projects --json
```

E1 baseline artifact:

- Added `docs/fixtures/local-tests-example-projects.json` as the timeout-safe
  six-project runtime baseline.
- Generated with
  `node scripts/baseline-local-tests-example-projects.mjs`, which runs
  `go run ./cmd/oaer compat local-tests --project <project> --json --timeout 30000 --top-failures 8`
  per project and records compact summaries plus top blocker families.
- `compat local-tests --timeout`, `--top-failures`, and
  `--profile-on-timeout` are now implemented in the compatibility reporting
  path, so large-project runs return structured timeout outcomes instead of
  relying on shell-level process termination.
- Static/readiness gate remains green:
  `go run ./cmd/oaer compat post-parity --project ./example-projects --json --require-ready`
  reports `filesScanned=50457 findings=0 testBlockingFindings=0 surfaces=0`.

Measured May 7, 2026 E1 baseline after native timeout/top-failure reporting,
standard-schema refreshes, package-aware duplicate-symbol handling, sema
frontier fixes, and the NUTPL runtime closure:

| Project | Result | Top blocker family |
| --- | --- | --- |
| `NPSP-rel-3.237` | `total=3986 compileGap=3986`, completed in 26.1s wall time. | Unknown standard object/type `CampaignMemberStatus` from `CON_AddToCampaign.cls:42`. |
| `src-nmb-nc-develop` | `total=9761 compileGap=9761`, completed in 30.1s wall time. | Unknown managed Apex type `znu.Address` from `AddressMessage.cls:8`. |
| `src-nmb-nu-develop` | `total=11526 compileGap=11526`, completed in 112.8s wall time. | Static/member resolution gap: `ARTransaction` references unknown `ORDER_ITEM_PARAM`. |
| `src-nmb-nutpl-develop` | `total=761 pass=761`, completed in 50.9s wall time. | Green runtime sentinel; no fail, unsupported, load, compile, or internal errors. |
| `sf-cred-pkg-develop` | `total=4268 compileGap=4268`, completed in 42.4s wall time. | Same-package duplicate top-level symbol `BaseSingleProviderProfileInfo`, which remains a true source duplicate. |
| `nams-workspace` | `total=5716 compileGap=5716`, completed in 52.0s wall time. | Unknown managed Apex type `znu.Pluggable` from `AddProgramsToProformaOrderHelper.cls:30`. |

First E1 implementation notes:

- Outcome taxonomy now distinguishes `assert_fail`, `runtime_gap`,
  `compile_gap`, `unsupported`, `internal_error`, and `timeout`.
- The NUTPL blocker frontier moved past missing `JSONParser`, `JSONToken`,
  `InstallContext`, `InstallHandler`, schema describe aliases, public built-in
  exception types, XML stream types, custom exception inherited constructors,
  multiline returns, catch locals, enum static values, implicit chained
  collection calls, `Type.class.toString()`, list bracket indexing, collection
  calls on field paths, list-initializer argument splitting, owner-relative enum
  access, enum `name()`, common String fluent APIs, basic `Dom.Document` /
  `Dom.XmlNode` signatures, and comment-safe return scanning.
- NAMS duplicate symbols are now package-aware: duplicate class names across
  sibling package directories are allowed, while same-package duplicates remain
  blockers.
- NAMS moved past the generated `znu__CartItemLine__c` placeholder object gap by
  inferring missing referenced managed-package custom objects from lookup
  metadata.
- NPSP moved past the generated `CampaignMember` and `OpportunityContactRole`
  standard-schema gaps; the next frontier is managed/generated type `SoapType`.
- Added a clean-room `testdata/local-tests/apexmocks-proxy` fixture for
  `Test.createStub` / `System.StubProvider` proxy lifecycle, method metadata,
  argument capture, return dispatch, void calls, and stub object identity.
- Superseded the old E4 quick-scan note by folding standard-object coverage into
  `docs/STANDARD_OBJECT_SCHEMA.md` and the generated SObject stub field overlay.

Exit criteria:

- Every project has a reproducible local runtime baseline.
- Timeouts are reported as structured outcomes, not long-running shell probes.
- The plan tracks top blocker families from measured output, not stale notes.

#### Phase E2: Dynamic Proxy And Mock Framework Semantics

Goal: make `System.StubProvider` / `Test.createStub`-backed mock frameworks
pass locally and unlock mock-heavy enterprise tests.

Primary write scope: `internal/vm`, `internal/apextest`, focused fixtures.

Current blockers:

- Matcher registration/clear state is lost or observed at the wrong time:
  matcher count errors dominate NUTPL.
- Invocation recording misses calls, causing mock-verification failures.
- `System.StubProvider` and `Test.createStub` need Salesforce-like method-call
  metadata, argument capture, return dispatch, and exception propagation.
- Object-key equality and map/set semantics need to preserve Apex object
  identity where mocks use objects as keys.

Tasks:

- Implement full `Test.createStub` / `System.StubProvider` invocation metadata
  for method name, return type, parameter types, args, and stubbed object.
- Fix matcher lifecycle so matcher factory calls register state that survives
  through the subsequent mocked method call and is cleared at Salesforce-like
  boundaries.
- Preserve object identity/equality for map and set keys used by mock
  invocations.
- Add support for common mock-framework invocation patterns: ordered
  verification, any-order verification, never/times verification, custom
  matchers, combined matchers, and exception stubbing.
- Add focused generic fixtures modeled on observed enterprise mock-framework
  patterns without copying an entire project into tests.

Validation:

```bash
go test ./internal/vm ./internal/apextest
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --filter AnyOrderTest --json
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --filter InOrderTest --json
```

Exit criteria:

- NUTPL mock-framework matcher-count errors are gone.
- `Wanted but not invoked` failures drop to real assertion mismatches or pass.
- No framework-specific or project-specific shortcuts exist in production
  runtime code.

#### Phase E3: Compiler And Apex Language Fidelity

Goal: remove syntax/compiler gaps that Salesforce accepts and enterprise tests
use heavily.

Primary write scope: `internal/vm/compiler.go`, `internal/apexast`,
`internal/sema`, parser/IR tests.

Current blockers:

- Multi-variable `for` initializers such as
  `for (Integer i = 0, j = size; i < j; i++)` fail in compiled method bodies.
- Some collection/map/list initializers still reject valid Apex shapes in
  larger packages.
- Some lexer/coercion paths treat Apex identifiers or enum-ish constants as the
  wrong scalar type, such as integer assignment failures from string tokens.
- Chained assignment is now supported, but similar assignment-expression
  contexts need corpus coverage.

Tasks:

- Add compiler support for comma-separated variable declarations in `for`
  initializers and update IR lowering accordingly.
- Broaden list/set/map initializer support for nested generics and object/SObject
  values used in service tests.
- Harden enum/static-field parsing for nested classes and all case-insensitive
  Apex identifier paths.
- Add corpus-derived compiler fixtures for the syntax seen in all six projects.

Validation:

```bash
go test ./internal/vm ./internal/sema ./internal/apexast
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --filter HtmlElement --json
go run ./cmd/oaer compat post-parity --project ./example-projects --json
```

Exit criteria:

- Multi-variable `for` initializer failures are gone.
- Compiler failures in the example-project runtime baseline are below the next
  measured blocker family.

#### Phase E4: Standard Schema And SObject Token Completion

Goal: make generated standard object support usable at runtime, not only in
static inventory.

Primary write scope: `internal/storage`, `internal/sobject`, `internal/vm`,
schema fixtures.

Current blockers:

- Runtime references such as `OpportunityLineItem.SObjectType` should resolve
  through generated describe metadata or the SObject stub field overlay.
- Standard object field tokens, relationship fields, child relationships,
  record type describes, and standard pricebook/product objects need continued
  sentinel coverage.
- Project-defined record types must remain authoritative over generated
  baseline record types.

Tasks:

- Ensure every generated standard object exposes `<Object>.SObjectType` and
  `<Object>.<Field>` tokens through VM lookup paths.
- Keep the stub field overlay generated from public Apex stubs so standard
  object field coverage moves in batches rather than one missing field at a
  time.
- Add runtime describe coverage for Opportunity, OpportunityLineItem, Product2,
  Pricebook2, PricebookEntry, Lead, Campaign, Case, Task, Event, User, Group,
  QueueSObject, Content*, and common Health/Sales Cloud references present in
  the six projects.
- Preserve explicit project metadata over generated schema overlays for fields,
  record types, labels, and relationships.
- Add SObject constructor/coercion coverage for generated standard fields and
  relationship references.

Validation:

```bash
go test ./internal/storage ./internal/sobject ./internal/vm ./internal/soql
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --json
go run ./cmd/oaer test --project example-projects/sf-cred-pkg-develop --filter <schema-heavy-sentinel> --json
```

Exit criteria:

- Standard `SObjectType` runtime lookup failures disappear from the six-project
  baseline.
- Fixture tests prove generated schema overlays do not clobber project
  metadata.

#### Phase E5: Data, DML, Mixed DML, And Test Isolation

Goal: align local test transaction behavior with scratch-org behavior for setup
data, mixed DML, rollback, and test-visible side effects.

Primary write scope: `internal/dml`, `internal/storage`, `internal/apextest`,
`internal/vm`.

Current blockers:

- Some tests report mixed-DML failures where scratch orgs pass, likely because
  `System.runAs`, setup-object classification, or test transaction isolation is
  too strict or in the wrong phase.
- Some helper APIs see null state such as `Test.Database.hasRecords` because
  setup/test fixture state is not initialized like the project expects.
- Large project execution needs cheaper org cloning and better per-test
  isolation.

Tasks:

- Audit setup-object classification against the standard schema and known setup
  objects used by the six projects.
- Match Salesforce `System.runAs` mixed-DML relaxation for tests.
- Ensure `@testSetup` data, static state reset, savepoints, async drain, and
  rollback happen in the correct order.
- Add deterministic support for test helper data APIs discovered in the corpus,
  but model general behavior instead of naming project helpers.
- Profile and optimize org/test state cloning for large projects.

Validation:

```bash
go test ./internal/dml ./internal/storage ./internal/apextest ./internal/vm
go run ./cmd/oaer test --project example-projects/src-nmb-nc-develop --filter <mixed-dml-sentinel> --json
go run ./cmd/oaer test --project example-projects/nams-workspace --filter <setup-sentinel> --json
```

Exit criteria:

- Mixed-DML failures in the baseline are either gone or classified as precise
  unsupported behavior with a fixture-backed reason.
- Large project focused tests run in bounded time.

#### Phase E6: SOQL, Relationship, And Dynamic Access Fidelity

Goal: make enterprise selector/service tests behave like scratch-org tests.

Primary write scope: `internal/soql`, `internal/vm`, `internal/sobject`,
`internal/sema`.

Common blocker families:

- Unknown `get`, `contains`, fluent builder, and relationship access calls often
  mean receiver type inference or dynamic SObject/map access is too shallow.
- Relationship queries, polymorphic references, aggregate rows, and dynamic
  fields need runtime depth across NPSP and credentialing packages.

Tasks:

- Improve receiver inference and runtime dispatch for `Object`, `SObject`,
  `Map`, `List`, aggregate rows, and dynamic JSON/SObject maps.
- Broaden relationship SOQL support for parent/child, polymorphic, aliases, and
  relationship field projection.
- Make fluent builder return-type handling robust for nested and inherited
  methods.
- Add focused fixtures from NPSP-style selector/domain patterns.

Validation:

```bash
go test ./internal/soql ./internal/sobject ./internal/vm ./internal/sema
go run ./cmd/oaer test --project example-projects/NPSP-rel-3.237 --filter <selector-sentinel> --json
```

Exit criteria:

- Unknown collection/SObject dynamic access failures drop sharply in NPSP and
  the credentialing package.

#### Phase E7: Platform APIs, Resources, And UI Controller Test Contracts

Goal: make non-rendering controller/service tests pass when they depend on
Salesforce platform context.

Primary write scope: `internal/vm`, `internal/resource`, `internal/uicontroller`,
`internal/visualforce`, `internal/apextest`.

Tasks:

- Finish test-facing `PageReference`, `ApexPages`, standard controller, and
  controller extension behavior used by the six projects.
- Complete deterministic static resource/content asset URL behavior for
  `URLFOR`, `$Resource`, and LWC resource imports.
- Fill remaining `Site`, `Network`, `Auth`, `ConnectApi.Organization`,
  Platform Cache, endpoint, named credential, and remote-site test contracts.
- Keep unsupported diagnostics for browser rendering, real callouts, OAuth,
  and external network behavior outside the local-test claim.

Validation:

```bash
go test ./internal/vm ./internal/resource ./internal/uicontroller ./internal/visualforce
go run ./cmd/oaer test --project example-projects/src-nmb-nu-develop --filter <ui-controller-sentinel> --json
go run ./cmd/oaer test --project example-projects/sf-cred-pkg-develop --filter <endpoint-sentinel> --json
```

Exit criteria:

- Controller/resource/platform-context failures are no longer top blockers in
  any of the six project baselines.

#### Phase E8: Declarative Runtime Side Effects

Goal: execute Workflow, Flow, and Process Builder behavior that affects Apex
test assertions.

Primary write scope: `internal/automation`, `internal/dml`, `internal/storage`,
`internal/vm`.

Tasks:

- Extend current Workflow support for criteria, field updates, email alerts,
  recursion, and rollback.
- Extend Flow support for richer nodes, decisions, assignments, loops, record
  lookups/creates/updates/deletes, invocable Apex, formulas, and collection
  variables.
- Model Process Builder variants as Flow-like automation where public metadata
  shape allows.
- Emit trace events for automation decisions and side effects.

Validation:

```bash
go test ./internal/automation ./internal/dml ./internal/storage ./internal/vm
go run ./cmd/oaer test --project example-projects/src-nmb-nu-develop --filter <automation-sentinel> --json
go run ./cmd/oaer test --project example-projects/nams-workspace --filter <automation-sentinel> --json
```

Exit criteria:

- Declarative side-effect differences are no longer top blockers for broad
  local test execution.

#### Phase E9: Full-Corpus Performance And Release Gate

Goal: make full local runs practical and turn the six projects into a durable
release gate.

Primary write scope: `internal/apextest`, `internal/project`, `internal/watch`,
`internal/profile`, `internal/compat`.

Tasks:

- Profile full runs for `src-nmb-nc-develop`, `src-nmb-nu-develop`,
  `sf-cred-pkg-develop`, `nams-workspace`, and NPSP.
- Cache parsed source, semantic model, compiled methods, metadata registries,
  describe registries, and immutable org baselines across test methods.
- Add bounded parallel test execution where static state and org clone
  semantics allow.
- Add `compat local-tests --check docs/fixtures/local-tests-corpus.json` as the
  promotion gate for the six-project corpus.
- Publish a dashboard that separates `pass`, `assert_fail`, `runtime_gap`,
  `unsupported`, and `timeout` by project and blocker family.

Validation:

```bash
go test ./...
go run ./cmd/oaer compat local-tests --check docs/fixtures/local-tests-corpus.json
go run ./cmd/oaer test --project example-projects/src-nmb-nutpl-develop --json
go run ./cmd/oaer test --project example-projects/src-nmb-nc-develop --json
go run ./cmd/oaer test --project example-projects/src-nmb-nu-develop --json
go run ./cmd/oaer test --project example-projects/sf-cred-pkg-develop --json
go run ./cmd/oaer test --project example-projects/nams-workspace --json
go run ./cmd/oaer test --project example-projects/NPSP-rel-3.237 --json
```

Exit criteria:

- Full runs complete with structured JSON in bounded time.
- Remaining non-pass outcomes are either zero or intentionally documented as
  outside the local-test support claim.
- The scratch-org passing result and local result are close enough that local
  failures can be treated as actionable developer feedback.

## Principles

- Build general platform behavior, not project-specific routing.
- Prefer explicit unsupported diagnostics over silent no-ops.
- Add owned compatibility fixtures before claiming a surface is supported.
- For ambiguous Apex runtime behavior, run a minimal anonymous Apex probe
  against `nu-dx-org` with `sf apex run --target-org nu-dx-org` before deciding
  the local behavior. Treat the scratch-org result as the oracle and capture it
  in a focused regression or compatibility fixture.
- Keep local test execution separate from browser/UI rendering. Apex tests need
  controller contracts, page references, labels, resources, and metadata
  resolution first; full Visualforce/Aura/LWC serving comes later.
- Treat org-like behavior as a transaction problem: setup, test method clone,
  DML, triggers, async drain, workflow/flow side effects, rollback, limits, and
  captured outbound side effects must compose.
- Keep every phase measurable by a command that future agents can rerun.

## Target Command Surface

The main new gate should be a local-test compatibility command:

```bash
go run ./cmd/oaer compat local-tests --project path/to/project --json
```

The JSON should classify each test method into one terminal outcome:

- `pass`: completed with matching local runtime behavior.
- `fail`: test assertion, uncaught Apex exception, DML validation error, or
  other runtime failure that would be a real test failure.
- `unsupported`: blocked by a known unsupported OAER capability.
- `load_error`: project metadata or Apex source could not be loaded.
- `compile_error`: sema/type/indexing failure before execution.
- `internal_error`: OAER bug, panic recovery, or malformed diagnostic.

Each blocked test should include:

- project label
- class and method
- phase: `load`, `compile`, `setup`, `execute`, `async`, `declarative`,
  `side_effect`, or `assert`
- capability ID
- source file and line when available
- top stack frame when available
- short error text
- related metadata file when the blocker comes from metadata

This command becomes the primary progress meter for full support. The existing
`compat post-parity` command remains the broad static inventory.

## Phase 0: Baseline And Worktree Setup

Goal: establish reproducible parallel work from current `main`.

Shared setup:

```bash
git status --short --branch
go test ./...
go run ./cmd/oaer compat server-examples --json
go run ./cmd/oaer compat post-parity --json
```

Parallel agents should work in separate worktrees with non-overlapping write
sets. Suggested branch names:

| Lane | Branch | Primary write scope |
| --- | --- | --- |
| Gate | `codex/local-test-gate` | `internal/compat`, `internal/oaercli`, docs |
| Metadata core | `codex/local-test-metadata-core` | `internal/metadata`, `internal/schema`, `internal/project` |
| Metadata resources | `codex/local-test-resources` | `internal/metadata`, `internal/storage`, `internal/vm` resource APIs |
| UI contracts | `codex/local-test-ui-contracts` | `internal/visualforce`, `internal/uicontroller`, `internal/vm`, `internal/apextest` |
| Platform APIs | `codex/local-test-platform-apis` | `internal/vm`, `internal/apextest`, `internal/storage` |
| Declarative | `codex/local-test-declarative` | `internal/automation`, `internal/dml`, `internal/trace` |

Exit criteria:

- All agents can run `go test ./...`.
- No lane introduces concrete example-project package names, object names, or
  domains into source, tests, or docs.
- Each lane has a focused validation command and a known merge order.

## Phase 1: Local-Test Gate And Reporting

Goal: add the gate that converts broad findings into test-execution readiness.

Primary lane: Gate.

Tasks:

- Add `compat local-tests` CLI routing.
- Reuse project discovery, metadata load, symbol index, sema, and Apex test
  discovery rather than creating a parallel project loader.
- Emit a stable JSON schema with summary counts and per-test outcomes.
- Add `--blockers-only`, `--project`, `--class`, `--method`, and `--json`
  filters.
- Classify blocker stage: load, compile, setup, execute, async, declarative,
  side effect, or assert.
- Include capability IDs from existing scanner and runtime diagnostics.
- Add a Markdown summary mode after JSON is stable.
- Add small owned fixture projects that intentionally cover pass, fail,
  unsupported, load error, compile error, and internal-error recovery paths.

Non-overlap guidance:

- This lane should not implement new platform behavior except tiny hooks needed
  to classify existing diagnostics.
- Other lanes can use temporary focused tests before the command exists, then
  plug into this gate after merge.

Validation:

```bash
go test ./internal/compat ./internal/oaercli ./internal/apextest
go run ./cmd/oaer compat local-tests --project testdata/local-tests/basic --json
```

Exit criteria:

- The command can run against a project without panicking.
- Every discovered test method receives one terminal outcome.
- Unsupported behavior is reported as unsupported, not as a generic failure.
- The output is stable enough for future baseline files.

## Phase 2: Read-Only Metadata Ingestion

Goal: make project load and resolution match the org metadata shape tests
expect before runtime semantics are attempted.

Primary lanes: Metadata core, Metadata resources.

### Phase 2A: Legacy Object And Custom Metadata Records

Tasks:

- Load legacy `.object` files alongside source-format `object-meta.xml`.
- Merge the generated Salesforce standard object schema baseline before project
  custom-field deltas so Account, Contact, Lead, Opportunity, Orders, Quotes,
  Products, Activities, files, and platform objects resolve consistently.
- Load custom fields, record types, validation rules, compact layouts, and
  business processes from both legacy and source-format layouts where present.
- Load legacy custom metadata record `.md` files into schema/storage fixtures.
- Preserve namespace and relationship metadata for custom metadata types.
- Add deterministic IDs and stable ordering for loaded custom metadata records.
- Make SOQL over loaded custom metadata records work through existing storage
  and SOQL paths.

Validation:

```bash
go test ./internal/metadata ./internal/schema ./internal/storage ./internal/soql
go run ./cmd/oaer compat local-tests --project testdata/local-tests/custom-metadata --json
```

### Phase 2B: Labels, Translations, Resources, And Endpoints

Tasks:

- Load `.labels` and translation files into a label registry.
- Add VM support for resolving `Label.SomeName` and namespaced label forms.
- Load static resources and content assets as metadata records with deterministic
  local URLs.
- Implement local `URLFOR($Resource...)` behavior needed by Apex tests and
  Visualforce controller assertions.
- Load `.namedCredential` and `.remoteSite` metadata as endpoint configuration.
- Expose endpoint lookup to callout mocks without performing real network
  authorization.

Validation:

```bash
go test ./internal/metadata ./internal/vm ./internal/visualforce
go run ./cmd/oaer compat local-tests --project testdata/local-tests/resources-labels --json
```

### Phase 2C: Permissions And Presentation Metadata

Tasks:

- Discover profiles, permission sets, tabs, layouts, web links, quick actions,
  global value sets, standard value sets, applications, and flexipages as
  read-only project metadata inputs.
- Load profiles and permission sets into the existing read-only metadata
  registry; layouts and compact layouts are available through the local server
  source metadata path.
- Add registry-backed loaders for tabs, web links, quick actions, global value
  sets, standard value sets, applications, and flexipages before treating those
  surfaces as supported.
- Support describe/controller lookups that need this metadata.
- Keep enforcement conservative: if a permission rule is not modeled, report a
  capability-specific unsupported diagnostic instead of allowing silently.

Current status:

- Custom metadata Phase 2A has an owned local-test fixture and is expected to
  pass through the corpus gate.
- Phase 2C has an owned `presentation-metadata` fixture that loads representative
  profile, permission set, tab, layout, compact layout, web link, quick action,
  global value set, standard value set, application, and flexipage files. The
  test now exercises `Schema.describeTabs()` and asserts local tab describe
  values.

Validation:

```bash
go test ./internal/metadata ./internal/schema ./internal/sobject ./internal/vm
go run ./cmd/oaer compat local-tests --project testdata/local-tests/presentation-metadata --json
```

Exit criteria for Phase 2:

- Static load blockers for legacy object, custom metadata, labels, resources,
  and endpoint metadata are represented as loaded metadata or explicit
  unsupported diagnostics.
- The local-test gate shows fewer `load_error` and metadata-resolution
  `unsupported` outcomes.

## Phase 3: Test-Facing UI Controller Contracts

Goal: support Apex tests that touch Visualforce, Aura, or LWC controller paths
without rendering a browser UI.

Primary lane: UI contracts.

### Phase 3A: Visualforce Page And Component Index

Tasks:

- Parse and index `.page` and `.component` metadata.
- Resolve page names, controller classes, standard controllers, extensions,
  component attributes, and assign-to bindings.
- Resolve `Page.SomePage` references to a local `PageReference`.
- Add source locations for page/controller metadata diagnostics.

Validation:

```bash
go test ./internal/visualforce ./internal/uicontroller ./internal/sema
go run ./cmd/oaer compat local-tests --project testdata/local-tests/visualforce-index --json
```

### Phase 3B: PageReference And ApexPages Test State

Tasks:

- Implement `PageReference` URL, redirect, parameters, headers, cookies, and
  request-body stubs needed by tests.
- Implement `ApexPages.currentPage()` isolation per test method and server
  request.
- Support `ApexPages.addMessage`, message retrieval, and severity constants.
- Reset page state between test methods and inside test transaction clones.

Validation:

```bash
go test ./internal/vm ./internal/apextest ./internal/visualforce
go run ./cmd/oaer compat local-tests --project testdata/local-tests/page-reference --json
```

### Phase 3C: Controller Invocation Contracts

Tasks:

- Instantiate custom controllers and controller extensions with supported
  constructor shapes.
- Add a minimal standard-controller model for SObject-backed controller tests.
- Support component attribute binding where Apex tests instantiate or inspect
  component-facing controller state.
- Discover Aura `@AuraEnabled` Apex methods and LWC Apex imports as test-facing
  entry points.
- Add JSON/wrapper serialization shapes used by Aura/LWC controller tests.

Validation:

```bash
go test ./internal/uicontroller ./internal/vm ./internal/apextest
go run ./cmd/oaer compat local-tests --project testdata/local-tests/ui-controller-contracts --json
```

Exit criteria for Phase 3:

- Tests that reference `Page.*`, `PageReference`, `ApexPages`, standard
  controllers, controller extensions, Aura Apex methods, or LWC Apex imports can
  either execute or fail with a precise unsupported diagnostic.
- No Visualforce/Aura/LWC rendering server is required for this phase.

## Phase 4: Test-Visible Platform APIs

Goal: implement platform APIs commonly used inside tests and controller/service
test setup.

Primary lane: Platform APIs.

Tasks:

- Implement `System.Callable` dispatch for ordinary Apex classes.
- Implement `System.StubProvider` and `Test.createStub` for method interception
  shapes used in tests.
- Add deterministic `Site`, `Network`, Community, and guest/current-site
  context.
- Support `$Site.Template` through the same metadata/context path used by
  Visualforce tests.
- Implement `Auth.*` methods used by tests with deterministic local behavior.
- Implement `ConnectApi.Organization.getSettings()` and common organization
  settings fields.
- Add Platform Cache basics: org/session partitions, get/put/remove, TTL
  handling where tests observe it.
- Connect named credential and remote site metadata to callout mock endpoint
  resolution.

Validation:

```bash
go test ./internal/vm ./internal/apextest ./internal/storage
go run ./cmd/oaer compat local-tests --project testdata/local-tests/platform-apis --json
```

Exit criteria:

- Platform API blockers move from broad unsupported counts to either passing
  local behavior or narrower documented unsupported methods.
- Stubs and callables execute user Apex, not hard-coded project names.

## Phase 5: Files, Email, And Captured Side Effects

Goal: support data and messaging side effects that Apex tests commonly assert.

Primary lanes: Metadata resources, Platform APIs.

Tasks:

- Implement `Attachment`, `Document`, `ContentVersion`, `ContentDocument`, and
  `ContentDocumentLink` binary-body behavior on top of storage.
- Add deterministic body/content handling for DML, SOQL projection, and delete.
- Expand email template merge context for target object, related object,
  current user, labels, and simple custom fields.
- Capture outbound email side effects with recipients, subject, plain/html body,
  template ID, target object ID, related object ID, and save-as-activity flags.
- Account for email limits in strict and permissive limit modes.
- Roll back captured side effects with the test transaction.

Validation:

```bash
go test ./internal/storage ./internal/dml ./internal/vm ./internal/apextest
go run ./cmd/oaer compat local-tests --project testdata/local-tests/files-email --json
```

Exit criteria:

- Tests can assert file records and captured email effects without real
  transport or filesystem leakage.
- Side effects participate in rollback and test isolation.

## Phase 6: Declarative Automation In Test Transactions

Goal: match org save-order behavior where tests rely on Workflow, Flow, or
Process Builder-style side effects.

Primary lane: Declarative.

### Phase 6A: Workflow Rules

Tasks:

- Load Workflow Rule metadata, field updates, email alerts, outbound messages,
  and task actions.
- Evaluate rule criteria against records during DML.
- Apply field updates with recursive save-order behavior.
- Capture workflow email alerts as email side effects.
- Add rollback and trace events for every decision and action.

Validation:

```bash
go test ./internal/automation ./internal/dml ./internal/apextest ./internal/trace
go run ./cmd/oaer compat local-tests --project testdata/local-tests/workflow --json
```

### Phase 6B: Flow And Process Builder

Tasks:

- Load flow metadata needed for record-triggered and autolaunched flows.
- Support variables, assignments, decisions, record lookups, record updates, and
  Apex invocable action calls used by tests.
- Route `@InvocableMethod` calls into ordinary Apex.
- Preserve transaction rollback and trace events.
- Report unsupported flow nodes precisely.

Validation:

```bash
go test ./internal/automation ./internal/vm ./internal/dml ./internal/apextest
go run ./cmd/oaer compat local-tests --project testdata/local-tests/flow --json
```

Exit criteria:

- DML-driven tests observe modeled Workflow/Flow mutations and captured side
  effects.
- Unsupported automation nodes report the exact node type and metadata source.

### Declarative Coverage Still Partial

The Phase 6 fixtures now cover the first practical Workflow, Flow, and Process
Builder-shaped execution paths, but they are not full declarative automation
parity. Current supported slices:

- Workflow rules can load criteria and field updates from metadata and apply
  field updates during DML/test transactions.
- Workflow email alerts can load basic alert metadata, use local email template
  metadata, increment email invocation limits, and capture a local side effect
  through the VM DML automation path.
- Flow metadata can model active record-triggered and Process Builder-shaped
  DML automation when `start.object` is present.
- Flow start filters, simple decision conditions, formula-backed criteria,
  assignments, `$Record` source-field copies, typed literal values, and field
  update formulas can drive same-record mutations.
- Flow Apex action calls can invoke static `@InvocableMethod` methods through
  the VM for the modeled list-input shape.
- Unsupported Flow nodes report stable `OAERAUTO002` diagnostics with node type,
  node name, and metadata file.

Keep these remaining surfaces tracked before claiming
`declarative-automation-test-ready`:

- Workflow email alerts still need full recipient expansion, target/related
  record semantics, richer template rendering, rollback-specific regression
  coverage, and trace details for every matched/skipped alert.
- Workflow task actions and outbound messages need captured side-effect records
  and rollback behavior; they should not perform real transport or create
  project-specific shortcuts.
- Workflow rule criteria still need broader formula support, boolean filters,
  time-dependent actions, recursive save-order coverage, and trace events for
  matched/skipped rules and applied actions.
- Flow now models routed decision branches, default routes, record lookups, and
  record creates for the delete-propagation shape in the checked corpus. Flow
  still needs record deletes, collection operations, loops, screens, subflows,
  scheduled paths, pause elements, platform event triggers, and broader
  before-save/fast-field-update ordering.
- Flow decisions and assignments still need richer typed variables beyond
  `$Record` fields, `$Record__Prior`, relationship references, collection
  assignments, and precise traces for every decision outcome and assignment.
- Flow and Process Builder Apex actions still need richer invocable marshaling
  for custom request/response DTOs, multiple arguments, return values, and
  unsupported action signatures.
- Process Builder-shaped flows need more corpus-backed fixtures because their
  metadata often uses Flow XML shapes that differ from hand-authored
  record-triggered flows.

## Phase 7: Org-Like Test Runner Fidelity

Goal: tighten the core `oaer test` behavior once metadata and side-effect
surfaces exist.

Initial implementation status:

- Added `testdata/local-tests/org-like-runner` as the runner-fidelity fixture.
- Added local platform-event trigger delivery for `EventBus.publish(...)` so
  after-insert platform event triggers participate in the same per-test VM
  transaction and trace stream.
  It verifies `@TestSetup` data cloning, static reset between test methods,
  `Test.startTest`/`Test.stopTest` queueable/future/batch/scheduled drain,
  current local `EventBus.publish` success behavior, savepoint rollback,
  trigger ordering, Workflow field updates, and Flow field updates in one
  composed local test run.
- DML automation ordering now matches the supported test transaction path:
  before triggers run before the write, after triggers observe the post-write
  record before declarative automation, then Workflow and Flow field updates run
  inside the same rollback-able transaction.
- Platform-event trigger delivery now has owned fixture coverage for the local
  synchronous after-insert trigger path used by tests. Broader Salesforce event
  bus semantics, publish callbacks, replay IDs, and asynchronous subscriber
  ordering remain outside the current claim.
- Added `oaer test --compat-json` so the user-facing test command can emit the
  same readiness-shaped per-test outcome schema as `compat local-tests`.

Primary lanes: Gate, Platform APIs, Declarative.

Tasks:

- Ensure `@TestSetup` data is cloned exactly once per test method.
- Verify static state reset across test methods and namespace/package
  boundaries.
- Verify `Test.startTest`/`Test.stopTest` limit windows and async drain order.
- Verify queueable, future, batch, scheduled, and platform-event-like async
  paths used by tests.
- Verify DML save-order composition: validation, before triggers, DML write,
  after triggers, workflow, flow, async enqueue, rollback.
- Added per-test trace/profile summaries for blocked local-test outcomes and
  `--slow-test-ms` slow-test capture for `oaer test --compat-json` and
  `compat local-tests`.
- Add `oaer test --compat-json` or equivalent output compatible with the
  local-test gate.

Validation:

```bash
go test ./internal/apextest ./internal/vm ./internal/dml ./internal/automation
go run ./cmd/oaer compat local-tests --project testdata/local-tests/org-like-runner --json
go run ./cmd/oaer test --project testdata/local-tests/org-like-runner --compat-json
```

Exit criteria:

- Local test execution has a single transaction discipline shared by Apex, DML,
  triggers, async, Workflow, Flow, captured email, files, and rollback.
- Failing tests are distinguishable from unsupported OAER behavior.

## Phase 8: Corpus Baselines And Release Gates

Goal: make the work durable and measurable against large anonymized examples.

Initial implementation status:

- Added `docs/fixtures/local-tests-corpus.json` as the first checked baseline
  for the owned local-test corpus.
- Added `compat local-tests --check <path>` so the gate reruns every project in
  the baseline and fails if readiness, summary counts, or stable test outcomes
  drift.
- The first baseline covers owned metadata/resources, Visualforce controller,
  platform/API, files/email, Workflow, Flow, and org-like runner fixtures.
- Added `compat ui-controllers --check
  docs/fixtures/ui-controller-discovery.json` for Aura/LWC controller discovery
  without adding browser rendering or action endpoint semantics.
- The checked 13-project corpus now covers owned metadata/resources,
  presentation-metadata unsupported classification, Visualforce controller
  contracts, Aura/LWC discovery, VM-level Aura/LWC action invocation, platform
  APIs, named-credential and remote-site callout matching, local Metadata API
  custom object/field deployment, files/email, Workflow, Flow, and org-like
  runner fidelity.
- Larger anonymized corpus projects and generated local-test dashboard files
  remain future release-hardening work. VM-level Aura/LWC action dispatch now
  has JSON-shaped return and `AuraHandledException` error contracts ready for
  fixture expansion.
- The broad post-parity readiness inventory is green for the checked
  `example-projects` corpus:
  `filesScanned=50457 findings=0 testBlockingFindings=0 surfaces=0`.
- CI now keeps the release-hardening gates live with `go test ./...`,
  `compat local-tests --check docs/fixtures/local-tests-corpus.json`,
  `compat post-parity --json --require-ready`, `compat ui-controllers --check
  docs/fixtures/ui-controller-discovery.json`, and generated stdlib coverage
  drift checks.
- Added `docs/fixtures/post-parity-trace-events.json` as a stable trace
  fixture for Flow, Visualforce controller, and Metadata deploy events.

Primary lane: Gate.

Tasks:

- Add anonymized owned fixture projects modeled after the large-project corpus
  surfaces, not copied project code.
- Add baseline files for local-test gate counts.
- Add `--check` mode for `compat local-tests`.
- Add dashboards for local-test readiness separate from MVP readiness.
- Add docs that define these release labels:
  - `server-examples-green`
  - `mvp-ready`
  - `legacy-project-test-ready`
  - `declarative-automation-test-ready`
  - Done in `docs/RELEASE_POLICY.md`; each label is tied to a checked gate and
    explicitly avoids broader Salesforce runtime claims.
- Add CI-friendly focused jobs for quick fixtures and optional large corpus
  scans.

Validation:

```bash
go test ./...
go run ./cmd/oaer compat local-tests --project testdata/local-tests/org-like-runner --json
go run ./cmd/oaer compat local-tests --project testdata/local-tests/resources-labels --json
go run ./cmd/oaer compat local-tests --project testdata/local-tests/ui-controller-contracts --json
go run ./cmd/oaer compat local-tests --project testdata/local-tests/platform-apis --json
go run ./cmd/oaer compat local-tests --project testdata/local-tests/files-email --json
go run ./cmd/oaer compat local-tests --project testdata/local-tests/workflow --json
go run ./cmd/oaer compat local-tests --project testdata/local-tests/flow --json
go run ./cmd/oaer compat local-tests --check docs/fixtures/local-tests-corpus.json
go run ./cmd/oaer compat ui-controllers --check docs/fixtures/ui-controller-discovery.json
go run ./cmd/oaer compat post-parity --json --require-ready
```

Exit criteria:

- The project can claim local Apex test execution support only when the
  local-test gate is green for the owned corpus and remaining unsupported
  surfaces are outside the documented claim.

## Release Labels

Use these labels consistently in release notes, dashboards, and issue triage:

- `server-examples-green`: the Salesforce-shaped local API route corpus passes
  with no failing, unsupported, or missing probes.
- `mvp-ready`: every capability required by `oaer compat mvp --require-ready`
  is `supported`, and generated compatibility docs are in sync.
- `legacy-project-test-ready`: `compat local-tests --check
  docs/fixtures/local-tests-corpus.json` is green for the owned local-test
  corpus, and larger-project unsupported surfaces are outside the documented
  support claim.
- `declarative-automation-test-ready`: Workflow, Flow, and Process
  Builder-shaped automation fixtures cover the declared save-order,
  side-effect, rollback, trace, and unsupported-diagnostic surfaces.

## Merge Strategy

Use small worktree merges. Merge order should usually be:

1. Gate/reporting skeleton.
2. Metadata core.
3. Metadata resources.
4. UI controller contracts.
5. Platform APIs.
6. Files/email side effects.
7. Declarative automation.
8. Runner fidelity and corpus baselines.

After each merge:

```bash
go test ./...
go run ./cmd/oaer compat server-examples --json
go run ./cmd/oaer compat post-parity --json
```

When `compat local-tests` exists, add:

```bash
go run ./cmd/oaer compat local-tests --project testdata/local-tests/basic --json
```

Clean up merged worktrees and branches immediately after their commits are on
the integration branch.

## Current Squad Completion Snapshot

The first broad blocker-reduction squad has completed. The integrated result
cleared the checked post-parity blocker frontier for:

- UI and org presentation metadata resolution.
- Visualforce controller, page, component, extension, action, and `Page.*`
  reference discovery.
- Label/resource resolution, including platform `Site` labels and discovered
  external managed-package label namespaces.
- Workflow sibling field-update metadata and modeled Flow record lookup/create
  shapes.

The next squad should not repeat those scanner-readiness lanes. It should focus
on runtime-depth gaps that remain outside the zero-blocker inventory claim:
richer Visualforce controller execution, advanced Flow interviews and invocable
actions, broader Metadata API mutation behavior, trace/debug visibility, and
larger owned enterprise fixtures.

## Historical First Squad

The first squad started with four lanes:

| Lane | Why first |
| --- | --- |
| Gate/reporting | Creates the scoreboard for all later work. |
| Legacy object/custom metadata records | Legacy custom metadata type references now resolve through loaded schema; the remaining work is record loading and legacy source behavior. |
| Labels/resources/endpoints | Scanner resolution is mostly in place; remaining work is runtime behavior and namespaced edge cases. |
| Visualforce index/PageReference | Attacks the highest-count controller-test blocker without requiring rendering. |

That ordering has already been executed for scanner-readiness work. Use it as
context for why the current implementation is layered; do not treat it as the
next work queue.

## First Work Package: M1 To M3

This is the first parallel batch to schedule. It creates the scoreboard, removes
the most common metadata blockers, and makes controller tests runnable without
browser rendering.

### Lane A: Local-Test Gate

Owner scope: `internal/compat`, `internal/oaercli`, `internal/apextest`, docs.

Deliverables:

- Add `oaer compat local-tests`.
- Reuse `oaer test` discovery and execution; do not add a second test runner.
- Emit stable JSON with project summary, test outcomes, blocker stage,
  capability ID, source location, related metadata file, and timing.
- Add `--project`, `--class`, `--method`, `--blockers-only`, `--json`, and
  later `--check`.
- Add fixtures for pass, fail, unsupported, load error, compile error, and panic
  recovery/internal error.

Validation:

```bash
go test ./internal/compat ./internal/oaercli ./internal/apextest
go run ./cmd/oaer compat local-tests --project testdata/local-tests/basic --json
```

Merge requirement: this lane merges first. Other lanes may add temporary tests,
but they should migrate to `compat local-tests` after this lands.

### Lane B: Legacy Metadata And Custom Metadata Records

Owner scope: `internal/project`, `internal/schema`, `internal/storage`,
`internal/soql`, `internal/vm`.

Deliverables:

- Load legacy `.object` files and source-format object metadata through one
  normalized schema model.
- Load legacy custom metadata record `.md` files into deterministic local
  storage records.
- Preserve namespace, relationship, record type, and field metadata needed by
  Apex code and SOQL.
- Make SOQL over loaded custom metadata records work in tests.
- Report unsupported metadata shapes by capability ID instead of generic load
  errors.

Validation:

```bash
go test ./internal/project ./internal/schema ./internal/storage ./internal/soql ./internal/vm
go run ./cmd/oaer compat local-tests --project testdata/local-tests/custom-metadata --json
```

Expected movement: reduce `custommetadata.legacy-records` and
`metadata.legacy-source` blockers first.

### Lane C: Labels, Resources, And Endpoints

Owner scope: `internal/project`, `internal/schema`, `internal/storage`,
`internal/vm`, Visualforce/resource helpers if added.

Deliverables:

- Load custom labels and translations into a registry.
- Resolve `Label.Name` and namespaced label forms in Apex execution.
- Load static resources and content assets with deterministic local URLs.
- Add the test-facing `URLFOR($Resource...)` behavior needed by controller
  assertions.
- Load named credentials and remote site settings as endpoint metadata.
- Connect endpoint metadata to HTTP callout mock resolution without performing
  real network authorization.

Validation:

```bash
go test ./internal/schema ./internal/storage ./internal/vm
go run ./cmd/oaer compat local-tests --project testdata/local-tests/resources-labels --json
```

Expected movement: reduce `labels.localization`, `staticresources.urlfor`, and
`endpoint.metadata` blockers.

### Lane D: Visualforce/PageReference Controller Contracts

Owner scope: `internal/visualforce`, `internal/uicontroller`,
`internal/apextest`, `internal/vm`, `internal/sema`.

Deliverables:

- Index `.page` and `.component` files with controller, standard controller,
  extension, and component attribute metadata.
- Resolve `Page.SomePage` to deterministic `PageReference` values.
- Implement `ApexPages.currentPage()`, parameters, messages, severities, and
  per-test reset.
- Instantiate custom controllers and extensions for supported constructor
  shapes.
- Add a minimal standard-controller model for SObject-backed tests.

Validation:

```bash
go test ./internal/visualforce ./internal/uicontroller ./internal/apextest ./internal/vm
go run ./cmd/oaer compat local-tests --project testdata/local-tests/page-reference --json
go run ./cmd/oaer compat local-tests --project testdata/local-tests/ui-controller-contracts --json
```

Expected movement: reduce `visualforce.controller-test` and
`visualforce.component-test` blockers without implementing markup rendering.

### Integration Gate For The Batch

After each lane merge:

```bash
go test ./...
go run ./cmd/oaer compat server-examples --json
go run ./cmd/oaer compat post-parity --json
go run ./cmd/oaer compat local-tests --project testdata/local-tests/basic --json
```

After all four lanes merged, the post-parity readiness inventory moved to:

```text
filesScanned=50457 findings=0 testBlockingFindings=0 surfaces=0
```

The outcome is not "all Salesforce behavior is implemented"; it is a shift from
load/resolve blocker discovery toward narrower runtime-depth and fixture
hardening work.

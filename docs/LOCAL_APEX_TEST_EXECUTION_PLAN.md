# Local Apex Test Execution Plan

Status date: 2026-05-06.

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
not green. A May 6, 2026 inventory from the current checkout reports:

```text
filesScanned=51067 findings=4712 testBlockingFindings=4712 surfaces=15
```

That inventory is implementation-aware as of this checkpoint: generated
standard object/field metadata, loaded labels/translations, loaded static
resources/content assets, named credential and remote site endpoints, and
namespace-tolerant custom metadata type references are suppressed when the
project metadata resolves them. Passive LWC files, resolved LWC Apex imports,
supported Visualforce runtime references, registered `Page.*` references,
static resource metadata, endpoint metadata, discovered read-only presentation
metadata files, custom-object `Name` and dynamic field-map references, and
recognized Lightning client modules no longer appear as broad post-parity
blocker surfaces in the current inventory. The scanner also resolves existing
Visualforce controller classes, standard controller objects, controller
extensions, action methods, and component action attributes through the
best-effort Visualforce index and Apex symbol table; the remaining
Visualforce findings are unresolved page/controller/action contracts or
component metadata work. Remaining findings should be treated as the next
implementation frontier rather than stale scanner noise.

Use this document for parallel squad planning. Use
`docs/POST_PARITY_TODO.md` as the exhaustive backlog and capability boundary.

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

M0 through the first owned M7 corpus gate were green before the Phase 2C
presentation-metadata fixture was added. The corpus now intentionally includes a
presentation-metadata fixture that is not ready yet because tab describe is
reported as an explicit unsupported capability instead of being silently
modeled. The remaining work is to broaden the owned corpus and close the
documented unsupported surfaces before making a large-project local-test
execution claim.

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

## Principles

- Build general platform behavior, not project-specific routing.
- Prefer explicit unsupported diagnostics over silent no-ops.
- Add owned compatibility fixtures before claiming a surface is supported.
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
  test intentionally exercises `Schema.describeTabs()` and currently expects an
  `unsupported` outcome until tab describe metadata is modeled.

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
- Flow still needs record lookups, creates/deletes, collection operations,
  loops, screens, subflows, scheduled paths, pause elements, platform event
  triggers, and before-save/fast-field-update ordering.
- Flow decisions and assignments still need multi-branch routing, typed
  variables beyond `$Record` fields, `$Record__Prior`, relationship references,
  collection assignments, and precise traces for every decision outcome and
  assignment.
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
- The checked corpus now covers owned metadata/resources, presentation-metadata
  unsupported classification, Visualforce controller contracts, Aura/LWC
  discovery, VM-level Aura/LWC action invocation, platform APIs, files/email,
  Workflow, Flow, and org-like runner fidelity.
- Larger anonymized corpus projects and generated local-test dashboard files
  remain future release-hardening work. VM-level Aura/LWC action dispatch now
  has JSON-shaped return and `AuraHandledException` error contracts ready for
  fixture expansion.

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

## Suggested First Squad

Start with four lanes:

| Lane | Why first |
| --- | --- |
| Gate/reporting | Creates the scoreboard for all later work. |
| Legacy object/custom metadata records | Legacy custom metadata type references now resolve through loaded schema; the remaining work is record loading and legacy source behavior. |
| Labels/resources/endpoints | Scanner resolution is mostly in place; remaining work is runtime behavior and namespaced edge cases. |
| Visualforce index/PageReference | Attacks the highest-count controller-test blocker without requiring rendering. |

Do not start declarative automation first. Workflow and Flow need metadata,
storage, DML, transaction, and side-effect hooks to be stable before their
behavior can be meaningful.

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

After all four lanes merge, record the before/after blocker movement in
`docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md` or a generated local-test dashboard.
The expected outcome is not "all tests pass"; it is a measurable shift from
load/resolve blockers toward narrower execute-time blockers.

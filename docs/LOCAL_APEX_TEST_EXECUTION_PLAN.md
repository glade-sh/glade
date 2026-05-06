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

The broader local-test support gate is not green. The current post-parity
inventory reports:

```text
filesScanned=51482 findings=41532 testBlockingFindings=41532 surfaces=19
```

Use this document for parallel squad planning. Use
`docs/POST_PARITY_TODO.md` as the exhaustive backlog and capability boundary.

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

- Load profiles, permission sets, tabs, layouts, web links, quick actions,
  global value sets, and flexipages into read-only metadata registries.
- Support describe/controller lookups that need this metadata.
- Keep enforcement conservative: if a permission rule is not modeled, report a
  capability-specific unsupported diagnostic instead of allowing silently.

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

## Phase 7: Org-Like Test Runner Fidelity

Goal: tighten the core `oaer test` behavior once metadata and side-effect
surfaces exist.

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
- Add per-test trace/profile output for blocked or slow tests.
- Add `oaer test --compat-json` or equivalent output compatible with the
  local-test gate.

Validation:

```bash
go test ./internal/apextest ./internal/vm ./internal/dml ./internal/automation
go run ./cmd/oaer compat local-tests --project testdata/local-tests/org-like-runner --json
```

Exit criteria:

- Local test execution has a single transaction discipline shared by Apex, DML,
  triggers, async, Workflow, Flow, captured email, files, and rollback.
- Failing tests are distinguishable from unsupported OAER behavior.

## Phase 8: Corpus Baselines And Release Gates

Goal: make the work durable and measurable against large anonymized examples.

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
go run ./cmd/oaer compat local-tests --project testdata/local-tests/corpus-a --json
go run ./cmd/oaer compat local-tests --project testdata/local-tests/corpus-b --json
go run ./cmd/oaer compat local-tests --check docs/fixtures/local-tests-corpus.json
```

Exit criteria:

- The project can claim local Apex test execution support only when the
  local-test gate is green for the owned corpus and remaining unsupported
  surfaces are outside the documented claim.

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
| Legacy object/custom metadata | Removes the biggest load/resolve blocker family. |
| Labels/resources/endpoints | Unblocks common controller and callout-test setup. |
| Visualforce index/PageReference | Attacks the highest-count controller-test blocker without requiring rendering. |

Do not start declarative automation first. Workflow and Flow need metadata,
storage, DML, transaction, and side-effect hooks to be stable before their
behavior can be meaningful.

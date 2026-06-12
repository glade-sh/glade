# Local Apex Second Tier Packets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. The coordinating agent should also use dispatching-parallel-agents and give each implementer a separate worktree.

**Goal:** Implement the next useful `partial` surfaces after the best local Apex and LWC packets land. These items are valuable, but they carry heavier Salesforce semantics, lower local-test hit rate, or more edge behavior.

**Architecture:** Use parallel second-tier worktrees. Each lane owns a narrow runtime family, focused tests, and row recommendations. A final integration lane updates `/Users/matt/Dev/glade-tools` and generated docs only after product behavior is merged.

**Tech Stack:** Go 1.26, Glade VM/runtime under `internal/vm`, storage under `internal/storage`, first-party compatibility tooling in `/Users/matt/Dev/glade-tools`.

---

## Scope

This plan starts after the best-next packet plan has either landed or been skipped by the coordinator. These are second-tier because they need deeper semantic judgment:

- Business hours calendars need holidays, time zones, and overnight windows.
- Approval needs local process records and workitem state.
- QuickAction needs metadata-backed defaults and local action execution.
- Messaging needs template, attachment, and send-option edge behavior.
- Limits and `Test.loadData` need exact accounting and CSV edge handling.
- Crypto, Encoding, Pattern, String, and Decimal need Java/Salesforce edge parity.
- WebService and ExternalService should remain mock-only but can materialize richer generated shapes.

Live transport, live identity, live Trailblazer, Answers, Tooling, GraphQL, Pub/Sub, and external product services stay out of scope.

## Parallel Worktree Setup

Run from `/Users/matt/Dev/glade`:

```bash
git worktree add -b codex/second-businesshours ../glade-second-businesshours HEAD
git worktree add -b codex/second-approval ../glade-second-approval HEAD
git worktree add -b codex/second-quickaction ../glade-second-quickaction HEAD
git worktree add -b codex/second-messaging-limits ../glade-second-messaging-limits HEAD
git worktree add -b codex/second-stdlib-polish ../glade-second-stdlib-polish HEAD
git worktree add -b codex/second-webservice-external ../glade-second-webservice-external HEAD
git worktree add -b codex/second-stdlib-integration ../glade-second-stdlib-integration HEAD
```

Expected: seven worktrees are created and each has a `codex/` branch.

Each subagent must:

- Use GPT-5.5 medium if available. If not, use the strongest GPT-5 coding agent available.
- Work only inside its assigned worktree.
- Write failing or strengthening tests first.
- Keep service-only surfaces explicitly unsupported.
- Avoid generated docs unless assigned to integration.
- Return changed files, exact tests, row recommendations, and remaining gaps.

## Phase 0: Second-Tier Baseline

- [ ] **Step 1: Capture counts and partial rows**

Run from `/Users/matt/Dev/glade`:

```bash
awk -F'|' 'NR>8 && /^\|/ {
  area=$2; api=$3; status=$4; notes=$5
  gsub(/^[ \t]+|[ \t]+$/,"",area)
  gsub(/^[ \t]+|[ \t]+$/,"",api)
  gsub(/`/,"",api)
  gsub(/`/,"",status)
  gsub(/^[ \t]+|[ \t]+$/,"",status)
  gsub(/^[ \t]+|[ \t]+$/,"",notes)
  if (status=="partial") print area "\t" api "\t" notes
}' docs/STDLIB_COVERAGE.md > /tmp/glade-second-tier-partial-before.tsv
```

Expected on the current baseline: 78 partial rows. If the best-next plan has already landed, record the new number and proceed from the live file.

- [ ] **Step 2: Confirm generated docs are clean**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go run ./cmd/glade-tools stdlib --check ../glade/docs/STDLIB_COVERAGE.md
go test ./internal/capability
```

Expected: docs are up to date and capability tests pass.

## Phase 1A: BusinessHours Calendar Semantics Packet

**Subagent lane:** `../glade-second-businesshours`

**Rows touched:**

- `BusinessHours.add(String, Datetime, Long)`
- `BusinessHours.addGmt(String, Datetime, Long)`
- `BusinessHours.diff(String, Datetime, Datetime)`
- `BusinessHours.isWithin(String, Datetime)`
- `BusinessHours.nextStartDate(String, Datetime)`

**Target status:** Stay `partial` unless holidays, inactive/default business hours, overnight windows, split daily windows, time zone handling, negative diffs, and null/error behavior are covered. Good row notes are a win even without `supported`.

**Files:**

- Modify: `internal/vm/business_hours_runtime.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/storage/standard_fields.go`
- Modify: `internal/storage/model_test.go`
- Inspect generated schema output only if existing repo tooling regenerates it: `internal/storage/standard_schema_generated.go`

- [ ] **Step 1: Add tests for real calendar edges**

In `internal/vm/platform_test.go`, add tests for:

- default active BusinessHours record when ID is null
- inactive BusinessHours record rejection
- two windows on the same day
- overnight window, such as Friday 22:00 to Saturday 02:00
- holiday exclusion
- negative `diff`
- `add` versus `addGmt` around the org timezone

- [ ] **Step 2: Run focused failures**

Run:

```bash
go test ./internal/vm -run 'Test.*BusinessHours'
```

Expected before implementation: new edge tests fail.

- [ ] **Step 3: Implement storage-backed calendar math**

In `internal/vm/business_hours_runtime.go`:

- Load BusinessHours records by ID or default active record.
- Parse local day start/end fields from storage metadata.
- Support multiple windows by normalizing day schedules into intervals.
- Support overnight windows by splitting them at midnight.
- Exclude Holiday records that relate to the BusinessHours calendar when that relationship is present.
- Keep unknown holiday/service-calendar shapes explicit with a local unsupported diagnostic.

- [ ] **Step 4: Verify this lane**

Run:

```bash
go test ./internal/vm -run 'Test.*BusinessHours'
go test ./internal/storage -run 'Test.*BusinessHours'
git diff --check
```

Expected: all commands exit 0.

**Done report:** exact supported calendar edges and remaining partial boundaries.

## Phase 1B: Approval Local Process Packet

**Subagent lane:** `../glade-second-approval`

**Rows touched:**

- `Approval.process(Approval.ProcessRequest)`
- `Approval.process(Approval.ProcessRequest, Boolean)`
- `Approval.lock`
- `Approval.unlock`
- `Approval.isLocked`

**Target status:** Keep approval `partial` unless local ProcessInstance and ProcessInstanceWorkitem records exist with submit, approve, reject, lock, unlock, allOrNone, and error behavior. Do not model real approval routing.

**Files:**

- Modify: `internal/vm/approval_process_runtime.go`
- Modify: `internal/vm/dispatch.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/vm/sobject_soql_sharing.go`
- Modify: `internal/storage/standard_fields.go`
- Inspect and modify when process records need storage helpers: `internal/storage/model.go`
- Inspect and modify when end-to-end local test runner coverage owns the failure: `internal/apextest/runner_test.go`

- [ ] **Step 1: Add process-record tests**

In `internal/vm/platform_test.go`, add tests proving:

- `Approval.process(ProcessSubmitRequest)` inserts a local `ProcessInstance`.
- submit creates a local `ProcessInstanceWorkitem`.
- `Approval.isLocked(recordId)` returns true after submit.
- `ProcessWorkitemRequest` approve/reject completes the workitem and updates result status.
- `allOrNone=false` returns a failed result instead of throwing when the request is invalid.
- `allOrNone=true` throws for missing object ID.

- [ ] **Step 2: Run focused failures**

Run:

```bash
go test ./internal/vm -run 'Test.*Approval'
```

Expected before implementation: new process-record assertions fail.

- [ ] **Step 3: Implement local process records**

In `internal/vm/approval_process_runtime.go`:

- Build `Approval.ProcessResult` from stored local state.
- Insert ProcessInstance and ProcessInstanceWorkitem records through storage helpers.
- Store lock state in VM/test context or in local record metadata, matching existing lock/unlock behavior.
- Keep process definition routing deterministic and local.
- Do not add assignment rules, email notifications, delegated approvers, or live process routing.

- [ ] **Step 4: Verify this lane**

Run:

```bash
go test ./internal/vm -run 'Test.*Approval'
go test ./internal/storage -run 'Test.*(ProcessInstance|Approval|Lock)'
git diff --check
```

Expected: all commands exit 0.

**Done report:** approval local model, records created, and remaining live-routing boundaries.

## Phase 1C: QuickAction Metadata Execution Packet

**Subagent lane:** `../glade-second-quickaction`

**Rows touched:**

- `QuickAction.describeAvailableActions`
- `QuickAction.describeAvailableQuickActions(String)`
- `QuickAction.describeQuickActions(List<String>)`
- `QuickAction.retrieveQuickActionTemplate(String,Id)`
- `QuickAction.retrieveQuickActionTemplates(List<String>,Id)`
- `QuickAction.performQuickAction`
- `QuickAction.performQuickAction(QuickAction.QuickActionRequest)`
- `QuickAction.performQuickAction(QuickAction.QuickActionRequest,Boolean)`
- `QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>)`
- `QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>,Boolean)`
- `Test.newSendEmailQuickActionDefaults(Id,Id)`

**Target status:** Stay `partial` unless local metadata-backed Create, Update, and SendEmail quick actions run through DML/template behavior. No live UI action service.

**Files:**

- Modify: `internal/vm/request_runtime.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/storage/model.go`
- Modify: `internal/storage/fixture.go`
- Modify: `internal/metadata/metadata.go`
- Inspect and modify when the quick action fixture needs predefined values: `testdata/local-tests/presentation-metadata/force-app/main/default/quickActions/Widget__c.New.quickAction-meta.xml`

- [ ] **Step 1: Add metadata-backed quick action tests**

In `internal/vm/platform_test.go`, add tests for:

- describe reads quick actions from local metadata, not only hardcoded defaults
- retrieve template pre-fills target object fields
- perform create action inserts a local SObject
- perform update action updates a local SObject
- perform send-email action returns local result and increments email limits
- invalid request returns failed `QuickActionResult` when `allOrNone=false`

- [ ] **Step 2: Run focused failures**

Run:

```bash
go test ./internal/vm -run 'Test.*QuickAction'
```

Expected before implementation: metadata-backed perform assertions fail.

- [ ] **Step 3: Implement local action execution**

In `internal/vm/request_runtime.go`:

- Resolve local quick action metadata through storage/project metadata.
- Build template SObjects from target object and predefined values.
- For Create and Update actions, call existing DML paths.
- For SendEmail actions, reuse `Messaging.SingleEmailMessage` local validation and result paths.
- Keep unsupported action types explicit.

- [ ] **Step 4: Verify this lane**

Run:

```bash
go test ./internal/vm -run 'Test.*QuickAction'
go test ./internal/metadata ./internal/storage -run 'Test.*QuickAction'
git diff --check
```

Expected: all commands exit 0.

**Done report:** quick action types modeled and unsupported action types left explicit.

## Phase 1D: Messaging, Templates, Limits, and Test.loadData Packet

**Subagent lane:** `../glade-second-messaging-limits`

**Rows touched:**

- `Messaging.SingleEmailMessage`
- `Messaging.sendEmail`
- `Messaging.renderStoredEmailTemplate(String,String,String,Messaging.AttachmentRetrievalOption)`
- `Messaging.renderStoredEmailTemplate(String,String,String,Messaging.AttachmentRetrievalOption,Boolean)`
- `Limits.get*`
- `Test.loadData`
- `Test.startTest`
- `Test.stopTest`

**Target status:** Move rows only if local behavior is exact enough. Email delivery stays unmodeled. Attachment retrieval can be local-template-only.

**Files:**

- Modify: `internal/vm/email_runtime.go`
- Modify: `internal/vm/limits.go`
- Modify: `internal/vm/runtime_state.go`
- Modify: `internal/vm/test_support_runtime.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/apextest/runner_test.go`
- Modify: `internal/resource/resource.go`
- Modify: `internal/storage/model.go`
- Modify: `testdata/local-tests/files-email/force-app/main/default/classes/FilesEmailTest.cls`

- [ ] **Step 1: Add email template and send-option tests**

In `internal/vm/platform_test.go`, add tests for:

- SingleEmailMessage getters for every supported setter.
- `sendEmail(messages, false)` returns per-message errors and increments limits once.
- `sendEmail(messages, true)` throws or fails all when one message is invalid, matching existing local semantics.
- renderStoredEmailTemplate merges `whoId`, `whatId`, and local template fields.
- attachment retrieval option is accepted and returns a deterministic local attachment list only when local template resources exist.

- [ ] **Step 2: Add limit accounting tests**

Add tests for:

- SOQL, SOSL, DML, DML rows, callout, email, queueable, batch, scheduled, savepoint, and runAs counters.
- `Test.startTest()` resets the active window.
- `Test.stopTest()` restores outer counters plus drained async counters.
- failed DML and failed email paths account the same way as successful local paths when Salesforce would count them.

- [ ] **Step 3: Add Test.loadData CSV edge tests**

Add tests for:

- quoted commas
- escaped quotes
- blank fields
- CRLF input
- Date, Datetime, Boolean, Decimal, Id, and lookup values
- static resource not found
- bad header diagnostic

- [ ] **Step 4: Run focused failures**

Run:

```bash
go test ./internal/vm -run 'Test.*(Messaging|Limits|LoadData|StartTest|StopTest)'
go test ./internal/apextest -run 'Test.*(Messaging|Limits|LoadData|FilesEmail)'
```

Expected before implementation: new edge tests fail.

- [ ] **Step 5: Implement local behavior**

In `internal/vm/email_runtime.go`:

- Reuse existing DTO getter/setter pattern.
- Keep no-delivery boundary clear.
- Materialize renderStoredEmailTemplate results from local EmailTemplate/resource data only.

In `internal/vm/limits.go` and `internal/vm/runtime_state.go`:

- Keep counters per execution window.
- Make start/stop window behavior explicit and tested.

In `internal/vm/test_support_runtime.go`:

- Use Go CSV parsing for `Test.loadData`.
- Route records through DML so validation, defaulting, and counters stay shared.

- [ ] **Step 6: Verify this lane**

Run:

```bash
go test ./internal/vm -run 'Test.*(Messaging|Limits|LoadData|StartTest|StopTest)'
go test ./internal/apextest -run 'Test.*(Messaging|Limits|LoadData|FilesEmail)'
go test ./internal/resource ./internal/storage
git diff --check
```

Expected: all commands exit 0.

**Done report:** exact counters modeled, Test.loadData CSV edges covered, and email transport boundary.

## Phase 1E: Core Stdlib Edge Polish Packet

**Subagent lane:** `../glade-second-stdlib-polish`

**Rows touched:**

- `Crypto.generateDigest`
- `EncodingUtil.urlEncode`
- `EncodingUtil.urlDecode`
- `Pattern.compile`
- `Pattern.matches`
- `Matcher.find`
- `Matcher.group`
- `Matcher.matches`
- `String.split`
- `Decimal.round`
- `Decimal.setScale`

**Target status:** Promote only narrow rows whose edge behavior matches Salesforce enough. Keep regex rows `partial` if Go regexp cannot model Java regex features.

**Files:**

- Modify: `internal/vm/stdlib.go`
- Modify: `internal/vm/regex_test.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/vm/stdlib_test.go`
- Inspect and modify when Decimal value representation blocks exact rounding: `internal/vm/value.go`

- [ ] **Step 1: Add Crypto and Encoding tests**

Add tests for:

- SHA-512 digest
- SHA-256 case-insensitive algorithm names
- unsupported digest algorithm diagnostic
- UTF-8 charset accepted
- unsupported charset rejected
- URL encode/decode plus signs, spaces, unicode, and percent errors

- [ ] **Step 2: Add Decimal tests**

Add tests for:

- `setScale(0)`, `setScale(2)`, and larger scale
- negative scale rejection or supported behavior, based on Salesforce-observed semantics
- `RoundingMode.HALF_UP`, `HALF_DOWN`, `HALF_EVEN`, `UP`, `DOWN`, `CEILING`, `FLOOR`
- negative number rounding

- [ ] **Step 3: Add String and Pattern tests**

Add tests for:

- `String.split` as Java regex split, not literal split
- split limit behavior: zero, positive, negative
- Pattern flags currently accepted
- unsupported Java regex features produce `UnsupportedFeature`, not silent wrong matches
- Matcher group state after repeated `find()`

- [ ] **Step 4: Run focused failures**

Run:

```bash
go test ./internal/vm -run 'Test.*(Crypto|Encoding|Decimal|Pattern|Matcher|StringSplit)'
```

Expected before implementation: new edge tests fail.

- [ ] **Step 5: Implement supported edges only**

In `internal/vm/stdlib.go`:

- Add digest algorithms only when Go standard library supports exact output.
- Validate charset names. Accept UTF-8 and aliases that Salesforce accepts.
- Implement Decimal rounding modes with deterministic decimal arithmetic. Do not use float shortcuts.
- Use existing regex translation helpers for Java-compatible features.
- Keep unsupported regex constructs explicit.

- [ ] **Step 6: Verify this lane**

Run:

```bash
go test ./internal/vm -run 'Test.*(Crypto|Encoding|Decimal|Pattern|Matcher|StringSplit)'
go test ./internal/vm -run TestExecPatternMatcherStdlib
git diff --check
```

Expected: all commands exit 0.

**Done report:** rows that can move to supported, regex features still partial, and exact unsupported diagnostics.

## Phase 1F: WebServiceCallout and ExternalService Mock Packet

**Subagent lane:** `../glade-second-webservice-external`

**Rows touched:**

- `WebServiceCallout.invoke(Object,Object,Map,List)`
- `WebServiceCallout.invoke(Object,Object,Map<String,Object>,List<String>)`
- `Test.getExternalService()`
- `Test.setMock`
- HTTP request/response rows only if mock response shape needs them

**Target status:** Stay `partial`. The useful target is richer mock-only generated-wrapper behavior, not outbound SOAP or external-service transport.

**Files:**

- Modify: `internal/vm/vm.go`
- Modify: `internal/vm/test_support_runtime.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/vm/platform_passive_members.go`
- Modify: `internal/vm/construct_runtime.go`
- Inspect and modify when overload shape tests fail: `internal/typesys/standard_symbols_test.go`

- [ ] **Step 1: Add generated SOAP wrapper tests**

In `internal/vm/platform_test.go`, add a local generated wrapper shape:

- request class with nested fields
- response class with nested fields
- registered `WebServiceMock`
- `WebServiceCallout.invoke` writes into response map/list shape
- callout limit increments exactly once
- missing mock creates deterministic empty response shell

- [ ] **Step 2: Add ExternalService harness tests**

Add tests proving:

- `Test.getExternalService()` returns a local harness object.
- generated external-service facade methods can be invoked only when a local test stub/mock is registered.
- no registered stub returns a stable unsupported diagnostic.

- [ ] **Step 3: Run focused failures**

Run:

```bash
go test ./internal/vm -run 'Test.*(WebServiceCallout|ExternalService|CalloutMock)'
```

Expected before implementation: new wrapper materialization assertions fail.

- [ ] **Step 4: Implement mock-only materialization**

In `internal/vm/vm.go`:

- Preserve existing `WebServiceMock` dispatch.
- Reflect response map/list keys into local response class fields.
- Materialize nested generated response objects where type metadata is available.
- Keep outbound SOAP transport unsupported.

In `internal/vm/test_support_runtime.go`:

- Store external-service test stubs by service name and operation name.
- Dispatch only from test context.
- Return explicit unsupported diagnostics outside local test harness behavior.

- [ ] **Step 5: Verify this lane**

Run:

```bash
go test ./internal/vm -run 'Test.*(WebServiceCallout|ExternalService|CalloutMock)'
go test ./internal/typesys -run 'TestStandardPlatformSymbols.*(WebService|ExternalService|Http)'
git diff --check
```

Expected: all commands exit 0.

**Done report:** mock-only generated shapes modeled and transport boundaries.

## Phase 2: Second-Tier Integration Lane

**Subagent lane:** `../glade-second-stdlib-integration`

Start after second-tier product lanes merge.

**Files:**

- Modify: `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`
- Modify: `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go`
- Add or modify fixtures under `/Users/matt/Dev/glade-tools/docs/fixtures/`
- Regenerate: `docs/STDLIB_COVERAGE.md`
- Regenerate: `docs/COMPATIBILITY_DASHBOARD.md`
- Regenerate: `docs/KNOWN_GAPS.md`

- [ ] **Step 1: Build a row move table**

Create `/tmp/glade-second-tier-row-moves.tsv`:

```text
area	api	before	after	test_evidence	remaining_gap
```

Include one row for every catalog edit. No blank evidence fields.

- [ ] **Step 2: Update catalog and guard tests**

In `/Users/matt/Dev/glade-tools/internal/capability/stdlib.go`:

- Promote only behavior-backed rows.
- Keep second-tier rows `partial` when Salesforce edge behavior is still unmodeled.
- Write notes that name the modeled local behavior and the missing service or edge behavior.

In `/Users/matt/Dev/glade-tools/internal/capability/capability_test.go`:

- Add guard tests for rows moved to `supported`.
- Add guard tests that live-service rows remain unsupported.

- [ ] **Step 3: Add fixture evidence**

Add or update fixtures:

- `core-runtime-businesshours-calendar-evidence.json`
- `data-runtime-approval-local-process-evidence.json`
- `ui-quickaction-local-execution-evidence.json`
- `core-runtime-messaging-limits-load-data-evidence.json`
- `core-stdlib-edge-polish-evidence.json`
- `integration-webservice-external-mock-evidence.json`

Each fixture must name exact `surfaceId` values. Do not rely on broad behavior labels.

- [ ] **Step 4: Run glade-tools tests**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go test ./internal/capability ./internal/compat ./internal/surfaceledger
```

Expected: all packages pass.

- [ ] **Step 5: Regenerate docs**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go run ./cmd/glade-tools stdlib --output ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --output ../glade/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --output ../glade/docs/KNOWN_GAPS.md
```

Expected: generated docs update only where rows or notes changed.

- [ ] **Step 6: Verify generated docs**

Run:

```bash
go run ./cmd/glade-tools stdlib --check ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --check ../glade/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --check ../glade/docs/KNOWN_GAPS.md
```

Expected:

```text
../glade/docs/STDLIB_COVERAGE.md: up to date
../glade/docs/COMPATIBILITY_DASHBOARD.md: up to date
../glade/docs/KNOWN_GAPS.md: up to date
```

## Phase 3: Coordinator Merge and Verification

- [ ] **Step 1: Merge product lanes**

Run from `/Users/matt/Dev/glade`:

```bash
git merge --no-ff codex/second-businesshours
git merge --no-ff codex/second-approval
git merge --no-ff codex/second-quickaction
git merge --no-ff codex/second-messaging-limits
git merge --no-ff codex/second-stdlib-polish
git merge --no-ff codex/second-webservice-external
```

Expected: conflicts are limited to `internal/vm/platform_test.go`, dispatch/runtime files, and shared catalog notes after integration.

- [ ] **Step 2: Merge integration lane**

Run:

```bash
git merge --no-ff codex/second-stdlib-integration
```

Expected: catalog, fixtures, and generated docs merge after product behavior.

- [ ] **Step 3: Run focused product gates**

Run:

```bash
go test ./internal/vm
go test ./internal/apextest
go test ./internal/storage ./internal/resource ./internal/typesys ./internal/soql
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 4: Run glade-tools gates**

Run from `/Users/matt/Dev/glade-tools`:

```bash
go test ./internal/capability ./internal/compat ./internal/surfaceledger
go run ./cmd/glade-tools stdlib --check ../glade/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --check ../glade/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --check ../glade/docs/KNOWN_GAPS.md
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 5: Run broad gate if focused gates pass**

Run from `/Users/matt/Dev/glade`:

```bash
go test ./...
```

Expected: exit 0.

- [ ] **Step 6: Cleanup worktrees after merge**

Run from `/Users/matt/Dev/glade`:

```bash
git worktree remove ../glade-second-businesshours
git worktree remove ../glade-second-approval
git worktree remove ../glade-second-quickaction
git worktree remove ../glade-second-messaging-limits
git worktree remove ../glade-second-stdlib-polish
git worktree remove ../glade-second-webservice-external
git worktree remove ../glade-second-stdlib-integration
git branch -d codex/second-businesshours codex/second-approval codex/second-quickaction codex/second-messaging-limits codex/second-stdlib-polish codex/second-webservice-external codex/second-stdlib-integration
```

Expected: worktrees and merged local branches are removed.

## Final Done Criteria

- Every edited row has test evidence.
- No service-only row is promoted through passive shape or no-op behavior.
- Generated docs match the `glade-tools` catalog.
- Final report includes before/after partial counts, unsupported count, exact tests, skipped tests, and remaining row notes.

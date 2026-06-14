# Full Local Support Roadmap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the remaining worthwhile partial support lanes by implementing complete deterministic local contracts and splitting Salesforce-hosted behavior into exact unsupported rows.

**Architecture:** Product runtime, server, LSP, DAP, profile, and docs changes live in `glade`. Capability catalogs, oracle probes, fixture runners, surface ledgers, and generated support reports live in sibling `glade-tools`. A lane is complete only when every implemented behavior is fixture-backed and every hosted-only behavior has an exact unsupported diagnostic; no new `partial` rows are allowed.

**Tech Stack:** Go, Glade VM/runtime, local org storage, local Salesforce-shaped REST server, DAP/LSP/profile/watch packages, `glade-tools` compat fixtures, scratch-org oracle probes against `oaer-probe-max`, generated support reports.

---

## Current Baseline

Run these first from local `main`:

```bash
cd /Users/matt/Dev/glade
git status --short
git log --oneline -5
rg -n 'partial|unsupported' docs/COMPATIBILITY_DASHBOARD.md docs/STDLIB_COVERAGE.md docs/COMPATIBILITY.md

cd /Users/matt/Dev/glade-tools
git status --short
git log --oneline -5
go run ./cmd/glade-tools stdlib --check /Users/matt/Dev/glade/docs/STDLIB_COVERAGE.md
```

Expected baseline:

```text
/Users/matt/Dev/glade/docs/STDLIB_COVERAGE.md: up to date
docs/STDLIB_COVERAGE.md has 263 supported rows, 19 unsupported rows, and 0 partial rows.
docs/COMPATIBILITY_DASHBOARD.md has 9 partial post-MVP lanes.
```

Do not start from stale worktrees. Create fresh worktrees:

```bash
cd /Users/matt/Dev/glade
git worktree add .worktrees/full-local-support -b codex/full-local-support main

cd /Users/matt/Dev/glade-tools
git worktree add /Users/matt/Dev/glade/.worktrees/full-local-support-tools -b codex/full-local-support-tools main
```

## Hard Rule

No squad may leave a row as `partial`.

When a Salesforce behavior is fully representable from local project files, local org state, fixtures, or deterministic VM state, implement the whole local contract and mark it `supported`.

When behavior depends on Salesforce-hosted identity, rendering, search ranking, email transport, external network transport, admin-computed permissions, live package install state, or exact hosted governor accounting, do not fabricate it. Add or keep an explicit `unsupported` row and test the diagnostic.

When a current row mixes local and hosted behavior, split it into two rows:

```text
Supported row: complete deterministic local contract.
Unsupported row: exact hosted-service boundary.
```

## Parallel Squads

Use a coordinator plus six squads. Each squad gets a fresh subagent and commits its own lane.

- Coordinator: baseline, row guards, docs generation, merge order, final tests.
- Squad A: BusinessHours and Approval.
- Squad B: Messaging, Test.loadData, HTTP client certificates.
- Squad C: Async, governor profiles, and Limits.
- Squad D: Schema, Type, Search, and local metadata/search breadth.
- Squad E: Local API server breadth.
- Squad F: DAP, LSP, profile, watch, fixture expansion, release automation.

Do not let two squads edit the same file at the same time. If two squads need `internal/vm/dispatch.go`, the coordinator merges one patch first, then hands the updated file to the next squad.

## Shared File Map

Product worktree:

- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/business_hours_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/business_hours_runtime_test.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/approval_process_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/approval_process_runtime_test.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/email_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/email_runtime_test.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/test_support_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/test_support_runtime_test.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/async_job_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/limits.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/method_test.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/soql_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/search_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/describe_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/vm/platform_passive_members.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/server/server.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/server/server_test.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/lsp`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/gladecli/dap_command.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/profile/profile.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/internal/watch`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/docs/STDLIB_COVERAGE.md`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/docs/COMPATIBILITY_DASHBOARD.md`
- `/Users/matt/Dev/glade/.worktrees/full-local-support/docs/COMPATIBILITY.md`

Tools worktree:

- `/Users/matt/Dev/glade/.worktrees/full-local-support-tools/internal/capability/stdlib.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support-tools/internal/capability/capability.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support-tools/internal/capability/capability_test.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support-tools/internal/oracleprobe/stdlib_cases.go`
- `/Users/matt/Dev/glade/.worktrees/full-local-support-tools/internal/compat`
- `/Users/matt/Dev/glade/.worktrees/full-local-support-tools/internal/surfaceledger`
- `/Users/matt/Dev/glade/.worktrees/full-local-support-tools/docs/fixtures`

## Task 1: Coordinator Guardrails

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/full-local-support-tools/internal/capability/capability_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/full-local-support/docs/COMPATIBILITY_DASHBOARD.md`
- Modify: `/Users/matt/Dev/glade/.worktrees/full-local-support/docs/COMPATIBILITY.md`

- [ ] **Step 1.1: Add a no-new-partial stdlib guard**

Add this test in `internal/capability/capability_test.go`:

```go
func TestStdlibCatalogHasNoPartialRows(t *testing.T) {
	for _, row := range StdlibEntries() {
		if row.Status == StatusPartial {
			t.Fatalf("%s %s is partial: %s", row.Area, row.API, row.Notes)
		}
	}
}

func testHoliday(id string, fields map[string]storage.Value) storage.Record {
	full := map[string]storage.Value{
		"Id":   storage.IDValue(storage.ID(id)),
		"Name": storage.StringValue(id),
	}
	for field, value := range fields {
		full[field] = value
	}
	return storage.Record{ID: storage.ID(id), Object: "Holiday", Fields: full}
}

func testOperatingHoursHoliday(id, holidayID, operatingHoursID string) storage.Record {
	return storage.Record{
		ID:     storage.ID(id),
		Object: "OperatingHoursHoliday",
		Fields: map[string]storage.Value{
			"Id":               storage.IDValue(storage.ID(id)),
			"HolidayId":        storage.IDValue(storage.ID(holidayID)),
			"OperatingHoursId": storage.IDValue(storage.ID(operatingHoursID)),
		},
	}
}

func testSeedOperatingHoursHolidayLinks(t *testing.T, org *storage.OrgState, links ...storage.Record) {
	t.Helper()
	if len(links) == 0 {
		return
	}
	storage.EnsureStandardObject(org, "OperatingHoursHoliday")
	object := org.Objects["OperatingHoursHoliday"]
	for _, link := range links {
		object.Records[link.ID] = link
	}
	org.Objects["OperatingHoursHoliday"] = object
}
```

- [ ] **Step 1.2: Run the guard**

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support-tools
go test ./internal/capability -run TestStdlibCatalogHasNoPartialRows -count=1
```

Expected:

```text
ok  	github.com/glade-sh/glade/tools/internal/capability
```

- [ ] **Step 1.3: Add dashboard exit criteria**

At the top of `docs/COMPATIBILITY_DASHBOARD.md`, add a short "Full Local Support Exit Criteria" section:

```markdown
## Full Local Support Exit Criteria

No post-MVP lane may stay `partial`. Each lane must be split into a complete local `supported` row plus exact hosted-service `unsupported` rows where needed. Local supported rows must cite deterministic tests, fixture evidence, or generated docs.
```

- [ ] **Step 1.4: Commit guardrails**

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support-tools
git add internal/capability/capability_test.go
git commit -m "test: guard stdlib catalog against partial rows"

cd /Users/matt/Dev/glade/.worktrees/full-local-support
git add docs/COMPATIBILITY_DASHBOARD.md
git commit -m "docs: define full local support exit criteria"
```

## Task 2: Squad A - Full Local BusinessHours Calendar Model

**Goal:** Replace the current BusinessHours hosted holiday fence with complete local support for seeded BusinessHours, Holiday, and OperatingHoursHoliday records.

**Files:**

- Modify: `internal/vm/business_hours_runtime.go`
- Modify: `internal/vm/business_hours_runtime_test.go`
- Modify: `docs/STDLIB_COVERAGE.md`
- Modify in tools: `internal/capability/stdlib.go`
- Add fixture in tools: `docs/fixtures/core-runtime-businesshours-full-local-calendar.json`

- [ ] **Step 2.1: Write failing tests for all supported holiday shapes**

Add test cases to `internal/vm/business_hours_runtime_test.go`:

```go
func TestExecBusinessHoursFullLocalHolidayCalendar(t *testing.T) {
	tests := []struct {
		name        string
		holidays    []storage.Record
		links       []storage.Record
		expr        string
		wantLiteral string
	}{
		{
			name: "partial day closes only the time window",
			holidays: []storage.Record{testHoliday("0HoPartial000001AAA", map[string]storage.Value{
				"ActivityDate":        storage.DateValue("2026-06-15"),
				"IsAllDay":            storage.BooleanValue(false),
				"StartTimeInMinutes":  storage.IntegerValue(12 * 60),
				"EndTimeInMinutes":    storage.IntegerValue(13 * 60),
			})},
			expr:        "BusinessHours.diff('01m000000000001AAA', Datetime.newInstanceGmt(2026, 6, 15, 18, 30, 0), Datetime.newInstanceGmt(2026, 6, 15, 20, 30, 0))",
			wantLiteral: "3600000",
		},
		{
			name: "yearly recurrence closes matching date",
			holidays: []storage.Record{testHoliday("0HoYearly000001AAA", map[string]storage.Value{
				"ActivityDate":          storage.DateValue("2026-06-15"),
				"IsAllDay":              storage.BooleanValue(true),
				"IsRecurrence":          storage.BooleanValue(true),
				"RecurrenceType":        storage.StringValue("RecursYearly"),
				"RecurrenceStartDate":   storage.DateValue("2026-06-15"),
				"RecurrenceEndDateOnly": storage.DateValue("2028-06-15"),
			})},
			expr:        "BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2027, 6, 15, 16, 0, 0))",
			wantLiteral: "false",
		},
		{
			name: "linked holiday affects one calendar only",
			holidays: []storage.Record{testHoliday("0HoLinked000001AAA", map[string]storage.Value{
				"ActivityDate": storage.DateValue("2026-06-15"),
				"IsAllDay":     storage.BooleanValue(true),
			})},
			links: []storage.Record{testOperatingHoursHoliday("0OHLinked000001AAA", "0HoLinked000001AAA", "01m000000000001AAA")},
			expr:        "BusinessHours.isWithin('01m000000000001AAA', Datetime.newInstanceGmt(2026, 6, 15, 16, 0, 0))",
			wantLiteral: "false",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous("System.assertEquals(" + tt.wantLiteral + ", " + tt.expr + ");")
			if err != nil {
				t.Fatal(err)
			}
			org := testBusinessHoursOrg(t, tt.holidays...)
			testSeedOperatingHoursHolidayLinks(t, &org, tt.links...)
			machine := New(nil)
			machine.Org = &org
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
```

- [ ] **Step 2.2: Run tests and confirm failure**

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support
go test ./internal/vm -run BusinessHoursFullLocalHolidayCalendar -count=1
```

Expected: FAIL on current unsupported fences or wrong holiday math.

- [ ] **Step 2.3: Implement full local calendar expansion**

In `business_hours_runtime.go`:

- Replace the string-key holiday map with a day closure structure that can hold all-day and minute windows.
- Expand recurring holidays for the query window used by `isWithin`, `add`, `addGmt`, `diff`, and `nextStartDate`.
- Support these recurrence types from `Holiday`: `RecursDaily`, `RecursWeekly`, `RecursMonthly`, `RecursYearly`, plus `RecurrenceInterval`, `RecurrenceDayOfMonth`, `RecurrenceDayOfWeekMask`, `RecurrenceInstance`, `RecurrenceMonthOfYear`, `RecurrenceStartDate`, and `RecurrenceEndDateOnly`.
- Support `OperatingHoursHoliday` and seeded `BusinessHoursId` links so linked holidays affect only their calendar. Unlinked holidays remain global.
- Keep an unsupported fence only for malformed recurrence metadata that cannot be interpreted, with a message that names the exact malformed field.

- [ ] **Step 2.4: Add fixture evidence**

Create `docs/fixtures/core-runtime-businesshours-full-local-calendar.json` in `glade-tools` with Apex that covers all-day, partial-day, recurrence, linked, and unlinked behavior. Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support-tools
go run ./cmd/glade-tools compat validate docs/fixtures/core-runtime-businesshours-full-local-calendar.json
go test ./internal/compat -run 'TestRunDocumentedFixtures/core-runtime-businesshours-full-local-calendar' -count=1
```

Expected: PASS.

- [ ] **Step 2.5: Promote catalog row**

In `internal/capability/stdlib.go`, remove `BusinessHours hosted service calendar holiday expansion` after the seeded calendar model passes. If malformed local holiday metadata still needs a fence, add a separate unsupported row named `BusinessHours malformed local holiday metadata` and back it with a VM test. Regenerate docs:

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support-tools
go run ./cmd/glade-tools stdlib --output /Users/matt/Dev/glade/.worktrees/full-local-support/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools stdlib --check /Users/matt/Dev/glade/.worktrees/full-local-support/docs/STDLIB_COVERAGE.md
```

- [ ] **Step 2.6: Commit**

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support
git add internal/vm/business_hours_runtime.go internal/vm/business_hours_runtime_test.go docs/STDLIB_COVERAGE.md
git commit -m "feat: support full local BusinessHours holiday calendars"

cd /Users/matt/Dev/glade/.worktrees/full-local-support-tools
git add internal/capability/stdlib.go docs/fixtures/core-runtime-businesshours-full-local-calendar.json
git commit -m "test: add full local BusinessHours calendar fixture"
```

## Task 3: Squad A - Seeded Approval Engine

**Goal:** Implement complete local approval routing for seeded process metadata and work items, while keeping hosted rule evaluation and notifications explicit unsupported.

**Files:**

- Modify: `internal/vm/approval_process_runtime.go`
- Add: `internal/vm/approval_process_runtime_test.go`
- Modify in tools: `internal/capability/stdlib.go`
- Add fixture in tools: `docs/fixtures/core-runtime-approval-local-engine-full.json`

- [ ] **Step 3.1: Write failing approval tests**

Create tests that seed `ProcessDefinition`, `ProcessNode`, `ProcessInstance`, and `ProcessInstanceWorkitem` records in local org storage. Cover:

- Submit request creates a ProcessInstance and first ProcessInstanceWorkitem.
- Workitem approve updates status to Approved.
- Workitem reject updates status to Rejected.
- `allOrNone=false` returns a failed ProcessResult with structured errors when metadata is missing.
- `allOrNone=true` raises a catchable DmlException or ApprovalException consistent with the existing runtime error model.

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support
go test ./internal/vm -run ApprovalProcessLocalEngine -count=1
```

Expected: FAIL before implementation.

- [ ] **Step 3.2: Implement the local approval state machine**

In `approval_process_runtime.go`, add:

- `approvalLocalProcessDefinition(objectName string) (storage.Record, bool)`
- `approvalCreateProcessInstance(objectID storage.ID, definition storage.Record) (storage.ID, error)`
- `approvalCreateWorkitem(instanceID storage.ID, actorID storage.ID) (storage.ID, error)`
- `approvalApplyWorkitemAction(workitemID storage.ID, action string, comments string) (Value, error)`

The local engine must mutate `vm.Org` through the same storage path used by DML so rollback snapshots and test isolation work.

- [ ] **Step 3.3: Keep hosted behavior explicit unsupported**

If a request needs criteria evaluation, delegated approver resolution, queue routing, email notification, or absent process metadata, return:

```text
unsupported call "Approval.process hosted approval engine routing"
```

Do not silently approve or fabricate approvers.

- [ ] **Step 3.4: Add fixture and catalog split**

Add fixture `core-runtime-approval-local-engine-full.json`. Keep `Approval.process hosted approval engine routing` unsupported. Ensure supported rows describe only the full seeded local engine.

- [ ] **Step 3.5: Verify and commit**

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support
go test ./internal/vm -run Approval -count=1

cd /Users/matt/Dev/glade/.worktrees/full-local-support-tools
go test ./internal/capability -count=1
go run ./cmd/glade-tools stdlib --output /Users/matt/Dev/glade/.worktrees/full-local-support/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools stdlib --check /Users/matt/Dev/glade/.worktrees/full-local-support/docs/STDLIB_COVERAGE.md
```

Commit product and tools changes separately.

## Task 4: Squad B - Messaging Full Local Template and Send Capture

**Goal:** Complete all local Messaging behavior that can be represented by local EmailTemplate, ContentDocument, Attachment, SingleEmailMessage, MassEmailMessage, and SendEmailOptions records.

**Files:**

- Modify: `internal/vm/email_runtime.go`
- Modify: `internal/vm/email_runtime_test.go`
- Modify: `internal/vm/platform_passive_members.go`
- Modify in tools: `internal/capability/stdlib.go`
- Add fixtures in tools:
  - `docs/fixtures/core-runtime-messaging-render-template-full-local.json`
  - `docs/fixtures/core-runtime-messaging-send-capture-full-local.json`

- [ ] **Step 4.1: Write failing render template tests**

Cover:

- Classic text and HTML templates.
- Merge fields for `whoId` and `whatId`.
- `Messaging.AttachmentRetrievalOption.METADATA_ONLY`, `BODY`, and `NONE`.
- Local ContentDocument/ContentVersion and Attachment expansion.
- `updateEmailTemplateUsage=true` updates a deterministic local counter only if a local field exists; otherwise split usage mutation into unsupported.

Run:

```bash
go test ./internal/vm -run 'MessagingRenderStoredEmailTemplateFullLocal' -count=1
```

Expected: FAIL before implementation.

- [ ] **Step 4.2: Implement full local template rendering**

Use existing `renderEmailTemplateText` and storage lookup helpers. Add deterministic attachment expansion from local storage only. Do not call network or hosted services.

- [ ] **Step 4.3: Write failing send capture tests**

Cover:

- Single and mass email capture.
- To/cc/bcc/replyTo/sender/displayName/templateId/targetObjectId/whatId/entityAttachments/fileAttachments.
- `SendEmailOptions` fields that affect local capture.
- Limits email invocation increments.
- Unsupported real delivery remains fenced.

- [ ] **Step 4.4: Implement full local send capture**

Make `Messaging.sendEmail` return complete local `SendEmailResult` values for locally captured messages. Store captured messages in VM test harness state so tests can assert bodies, recipients, and attachments.

- [ ] **Step 4.5: Split hosted rows**

Keep these hosted-only unsupported rows:

```text
Messaging.renderStoredEmailTemplate Salesforce content attachment service
Messaging.sendEmail delivery transport
```

Remove "send options" from unsupported wording once local options are complete. Remove the content attachment service row only if local `ContentDocument`, `ContentVersion`, and `Attachment` records fully cover the row and the fixture proves it.

- [ ] **Step 4.6: Verify and commit**

```bash
go test ./internal/vm -run 'Messaging|Email' -count=1
go test ./internal/vm ./internal/resource ./internal/storage -count=1
```

Regenerate stdlib docs through `glade-tools`, then commit product and tools changes separately.

## Task 5: Squad B - Test.loadData Full Local CSV Contract

**Goal:** Finish the local Test.loadData behavior for packaged static resources and relationship external-ID expansion.

**Files:**

- Modify: `internal/vm/test_support_runtime.go`
- Add or modify: `internal/vm/test_support_runtime_test.go`
- Modify: `internal/resource`
- Modify in tools: `internal/capability/stdlib.go`
- Add fixture in tools: `docs/fixtures/core-runtime-test-load-data-full-local.json`

- [ ] **Step 5.1: Write failing tests**

Cover:

- `Test.loadData(Account.SObjectType, 'pkg__Accounts')` resolves a static resource under a local package namespace.
- CSV relationship columns such as `Parent__r.External_Id__c` resolve local external IDs.
- Bad package namespace returns exact unsupported or missing-resource diagnostic.
- Bad relationship external ID returns structured DML-style error.

- [ ] **Step 5.2: Implement resource namespace resolution**

Use the local project package namespace and static resource registry. Never query a live package.

- [ ] **Step 5.3: Implement relationship external-ID expansion**

Before DML insert, expand relationship columns into lookup IDs using local schema metadata and storage records. Enforce ambiguity and missing-record errors.

- [ ] **Step 5.4: Verify and commit**

```bash
go test ./internal/vm -run 'TestLoadData' -count=1
go test ./internal/resource ./internal/storage -count=1
```

Update catalog and docs. Remove the unsupported row if packaged namespace and relationship external-ID expansion are complete locally.

## Task 6: Squad B - HTTP Client Certificate Local Store

**Goal:** Implement a complete deterministic local client-certificate store for `HttpRequest.setClientCertificateName` and `setClientCertificate`.

**Files:**

- Modify: `internal/vm/platform_passive_members.go`
- Add or modify: `internal/vm/platform_test.go`
- Modify: `internal/config` if a config-backed certificate fixture path is needed.
- Modify in tools: `internal/capability/stdlib.go`

- [ ] **Step 6.1: Write failing tests**

Cover:

- Name lookup succeeds when local config defines a certificate.
- Name lookup fails with exact unknown-certificate diagnostic.
- Inline certificate/password stores deterministic request metadata.
- `Http.send` passes certificate metadata to `HttpCalloutMock` inspection.

- [ ] **Step 6.2: Implement local certificate metadata**

Do not parse private keys unless needed for local mock inspection. Store name, certificate text, password presence, and source. The complete local contract is "available to mocks and recorded on HttpRequest"; real TLS is unsupported.

- [ ] **Step 6.3: Split catalog row**

Replace `HttpRequest client certificate methods` unsupported with:

```text
supported: HttpRequest client certificate local mock metadata
unsupported: HttpRequest client certificate real TLS handshake
```

- [ ] **Step 6.4: Verify and commit**

```bash
go test ./internal/vm -run 'HttpRequest|HttpSend|ClientCertificate' -count=1
```

## Task 7: Squad C - Async Duplicate Signature and Deterministic Delay

**Goal:** Complete local Queueable duplicate signature locking and deterministic delay semantics for `System.enqueueJob`.

**Files:**

- Modify: `internal/vm/async_job_runtime.go`
- Modify: `internal/vm/platform_http_cache_resources.go`
- Modify: `internal/vm/method_test.go`
- Modify in tools: `internal/capability/stdlib.go`

- [ ] **Step 7.1: Write failing tests**

Cover:

- Two queueables with the same `QueueableDuplicateSignature` in one transaction reject the second enqueue.
- Different signatures enqueue.
- Delay values are recorded on local `AsyncApexJob`.
- Test.stopTest drains only due jobs when deterministic clock is advanced.

- [ ] **Step 7.2: Implement duplicate signature registry**

Add per-transaction duplicate signature state to VM runtime state. Reset it on transaction boundary and rollback.

- [ ] **Step 7.3: Implement deterministic delay clock**

Use VM-local time. Do not sleep wall clock. Store `NotBefore` on local async job records and add test helper hooks to advance deterministic time.

- [ ] **Step 7.4: Split hosted row**

Replace `System.enqueueJob wall-clock delay and duplicate signature enforcement` with:

```text
supported: System.enqueueJob local duplicate signature and deterministic delay
unsupported: System.enqueueJob hosted wall-clock queue scheduling
```

- [ ] **Step 7.5: Verify and commit**

```bash
go test ./internal/vm -run 'Queueable|Enqueue|Async' -count=1
go test ./internal/vm ./internal/gladecli ./internal/server -run 'Limit|Async|Queueable|ExecuteAnonymous' -count=1
```

## Task 8: Squad C - Configurable Governor Profiles

**Goal:** Turn the partial Limits lane into complete configurable local profiles plus exact unsupported hosted accounting.

**Files:**

- Modify: `internal/vm/limits.go`
- Modify: `internal/vm/runtime_state.go`
- Modify: `internal/gladecli/exec_command.go`
- Modify: `internal/gladecli/test_command.go`
- Modify: `internal/server/server_command.go`
- Modify: `internal/server/server.go`
- Add tests in `internal/vm/method_test.go`, `internal/gladecli/cli_test.go`, and `internal/server/server_test.go`
- Modify docs: `docs/COMPATIBILITY_DASHBOARD.md`, `docs/COMPATIBILITY.md`, `docs/STDLIB_COVERAGE.md`

- [ ] **Step 8.1: Define supported local profile contract**

Supported local profiles:

```text
default: current permissive deterministic counters
strict-sync: sync Apex-style caps for SOQL, rows, DML, heap, CPU, callouts, email
strict-async: async Apex-style caps for async execution
custom: explicit cap values from CLI/server config
```

Unsupported:

```text
Exact Salesforce governor accounting profiles
```

- [ ] **Step 8.2: Write failing tests**

Cover profile parsing, cap enforcement, JSON output, server executeAnonymous, and test runner behavior.

- [ ] **Step 8.3: Implement profiles**

Add named profile resolver:

```go
func LimitCapsForProfile(name string) (LimitCaps, bool)
```

Wire it to CLI flags and server config. Existing explicit cap flags must override profile defaults.

- [ ] **Step 8.4: Update dashboard**

Split dashboard lane:

```text
supported: limits.configurable-local-profiles
unsupported: limits.exact-salesforce-accounting
```

- [ ] **Step 8.5: Verify and commit**

```bash
go test ./internal/vm -run 'Limits|LimitProfile' -count=1
go test ./internal/gladecli -run 'Limit|Exec|Test' -count=1
go test ./internal/server -run 'Limit|ExecuteAnonymous' -count=1
```

## Task 9: Squad D - Local Metadata, Type, and Search Breadth

**Goal:** Finish useful local metadata/search support and split hosted search/admin behavior out.

**Files:**

- Modify: `internal/vm/describe_runtime.go`
- Modify: `internal/vm/soql_runtime.go`
- Add or modify: `internal/vm/search_runtime.go`
- Modify: `internal/vm/platform_passive_members.go`
- Modify tests in `internal/vm/vm_test.go`, `internal/vm/method_test.go`, `internal/vm/soql_test.go`
- Modify in tools: `internal/capability/stdlib.go`

- [ ] **Step 9.1: Implement local package namespace Type.forName**

Support local package namespace aliases from project metadata and generated platform aliases. Keep live package reflection unsupported.

- [ ] **Step 9.2: Implement local data category metadata if project files exist**

Load data category group metadata from local project files. Describe methods return local data category records. Hosted data category service lookup stays unsupported.

- [ ] **Step 9.3: Implement local SOSL scopes**

Support deterministic local SOSL token search for:

```text
IN ALL FIELDS
NAME FIELDS
EMAIL FIELDS
PHONE FIELDS
RETURNING object(field list WHERE ... ORDER BY ... LIMIT ...)
```

Ranking is deterministic and documented as lexical/local. Hosted ranking, stemming, synonyms, and external indexes stay unsupported.

- [ ] **Step 9.4: Verify and commit**

```bash
go test ./internal/vm -run 'TypeForName|Describe|DataCategory|Search|SOSL' -count=1
go test ./internal/soql -count=1
```

Update catalog rows and generated docs.

## Task 10: Squad E - Local API Server Breadth

**Goal:** Promote `server.rest-breadth` by implementing complete local support for the next useful REST families and splitting hosted-only families out.

**Files:**

- Modify: `internal/server/server.go`
- Modify: `internal/server/server_test.go`
- Create helper files:
  - `internal/server/composite_tree.go`
  - `internal/server/bulk_query.go`
  - `internal/server/layouts.go`
  - `internal/server/metadata_jobs.go`
- Modify docs: `docs/COMPATIBILITY_DASHBOARD.md`, `docs/COMPATIBILITY.md`

- [ ] **Step 10.1: Composite Tree**

Write tests for Account plus child Contact tree create, reference IDs, all-or-none rollback, error arrays, and unsupported nested tree shapes. Implement `/composite/tree/{object}`.

- [ ] **Step 10.2: Composite Graph**

Write tests for graph orchestration over already supported subrequests, reference substitution, partial graph failure, and response shape. Implement `/composite/graph`.

- [ ] **Step 10.3: Bulk API query locator paging**

Write tests for query job create, status, page locator, CSV chunks, invalid locator, and complete state. Implement locator paging without background workers.

- [ ] **Step 10.4: Layout and defaults read surface**

Write tests for local layout/default values from project metadata. Implement read-only endpoints. Keep hosted UI layout calculation unsupported.

- [ ] **Step 10.5: Metadata deploy/retrieve job stubs**

Implement deterministic local job records for deploy/retrieve validation of local metadata. Do not perform hosted deployment. Unsupported row names hosted deploy execution.

- [ ] **Step 10.6: Verify and commit**

```bash
go test ./internal/server -run 'Composite|Bulk|Layout|Metadata|REST' -count=1
go test ./internal/server -count=1
```

Update dashboard:

```text
supported: server.rest-breadth.local-expanded
unsupported: server.rest-breadth.hosted-auth-live-org-deploy
```

## Task 11: Squad F - DevEx Lanes

**Goal:** Promote DAP, LSP, profile, and watch partial lanes by completing the named local behavior.

**Files:**

- Modify: `internal/gladecli/dap_command.go`
- Modify: `internal/gladecli/dap_cache.go`
- Modify: `internal/lsp`
- Modify: `internal/profile/profile.go`
- Modify: `internal/watch`
- Modify tests in matching packages.
- Modify docs: `docs/COMPATIBILITY_DASHBOARD.md`, `docs/COMPATIBILITY.md`

- [ ] **Step 11.1: DAP live IDE orchestration**

Implement and test:

- Launch config accepts project root, class name, method name, and anonymous body.
- DAP starts compile/run, emits initialized/stopped/terminated in stable order.
- Test method launch resets DB writes after test completion.
- Disconnect cancels in-flight run.

Run:

```bash
go test ./internal/gladecli -run 'DAP|Debug' -count=1
```

- [ ] **Step 11.2: LSP context completion**

Implement and test ranked completions for:

- SOQL SELECT, WHERE, ORDER BY, GROUP BY.
- Apex member access after local variable, SObject, list, map, and schema token.
- Annotation and test method context.

Run:

```bash
go test ./internal/lsp -run 'Completion|Context' -count=1
```

- [ ] **Step 11.3: pprof-compatible profile output**

Implement `glade profile analyze --format pprof`. Include CPU samples from trace durations and stable function labels. Keep hosted Apex Replay Debugger output unsupported if named.

Run:

```bash
go test ./internal/profile ./internal/gladecli -run 'Profile|Pprof' -count=1
```

- [ ] **Step 11.4: Watch profile/trace reports**

Make watch mode emit profile summary events when profile tracing is enabled. Add JSON event tests and CLI tests.

Run:

```bash
go test ./internal/watch ./internal/gladecli -run 'Watch|Profile|Trace' -count=1
```

- [ ] **Step 11.5: Update dashboard**

Change these rows from partial to supported when tests pass:

```text
dap.live-ide-orchestration
lsp.context-completion
profile.pprof-and-timing
watch.profile-trace-reports
```

Commit each sub-lane separately.

## Task 12: Squad F - Fixture Expansion and Release Automation

**Goal:** Promote release lanes by making fixture expansion and release automation complete local workflows.

**Files:**

- Modify in tools: `internal/compat`, `internal/surfaceledger`, `internal/toolcli`
- Modify product scripts: `scripts/release-build.sh`, `scripts/smoke.sh`
- Modify docs: `docs/COMPATIBILITY_DASHBOARD.md`, `docs/RELEASE_NOTES.md`, release docs.

- [ ] **Step 12.1: Fixture expansion**

Add a fixture manifest gate that proves every supported stdlib/dashboard row has at least one fixture or package test. Add explicit unsupported fixture coverage for every unsupported row.

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support-tools
go test ./internal/compat ./internal/surfaceledger ./internal/toolcli -run 'Fixture|Coverage|Unsupported' -count=1
```

- [ ] **Step 12.2: Release automation**

Finish local release proof:

- Build archive for current platform.
- Generate checksums.
- Verify unpacked `glade version`, `glade doctor`, and parser smoke.
- Verify the VS Code extension package with the repo's existing extension package command when `contrib/vscode-glade/package.json` is present; otherwise record `VS Code extension package: not present` in the release manifest.
- Emit a machine-readable release manifest.

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support
scripts/release-build.sh
scripts/smoke.sh
```

- [ ] **Step 12.3: Update dashboard**

Change these rows from partial to supported:

```text
compat.fixture-expansion
release.distribution-automation
```

Commit tools and product docs separately.

## Task 13: Coordinator Final Report Generation

**Files:**

- Modify: `docs/STDLIB_COVERAGE.md`
- Modify: `docs/COMPATIBILITY_DASHBOARD.md`
- Modify: `docs/COMPATIBILITY.md`
- Modify site docs if generated docs changed.

- [ ] **Step 13.1: Regenerate all checked reports**

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support-tools
go run ./cmd/glade-tools stdlib --output /Users/matt/Dev/glade/.worktrees/full-local-support/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools stdlib --check /Users/matt/Dev/glade/.worktrees/full-local-support/docs/STDLIB_COVERAGE.md
```

Regenerate the dashboard with the existing tools command:

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support-tools
go run ./cmd/glade-tools dashboard --output /Users/matt/Dev/glade/.worktrees/full-local-support/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools dashboard --check /Users/matt/Dev/glade/.worktrees/full-local-support/docs/COMPATIBILITY_DASHBOARD.md
```

- [ ] **Step 13.2: Assert zero partial dashboard rows**

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support
if rg -n '\\| `partial` \\|' docs/COMPATIBILITY_DASHBOARD.md docs/STDLIB_COVERAGE.md; then
  echo "partial rows remain" >&2
  exit 1
fi
```

Expected: no output and exit 0.

- [ ] **Step 13.3: Assert unsupported rows have exact diagnostics**

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support
rg -n 'unsupported' docs/STDLIB_COVERAGE.md docs/COMPATIBILITY_DASHBOARD.md docs/COMPATIBILITY.md
```

For every unsupported row, confirm one of these exists:

- VM test asserting `UnsupportedFeature`.
- Server test asserting Salesforce-shaped unsupported response.
- Compat fixture ending in `unsupported.json`.

- [ ] **Step 13.4: Commit docs**

```bash
git add docs/STDLIB_COVERAGE.md docs/COMPATIBILITY_DASHBOARD.md docs/COMPATIBILITY.md site/docs-src
git commit -m "docs: mark full local support lanes complete"
```

## Task 14: Final Verification

Run from product worktree:

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support
go test ./internal/vm ./internal/resource ./internal/storage -count=1
go test ./internal/server -count=1
go test ./internal/lsp ./internal/profile ./internal/gladecli -count=1
go test ./... -count=1
scripts/smoke.sh
```

Run from tools worktree:

```bash
cd /Users/matt/Dev/glade/.worktrees/full-local-support-tools
go test ./internal/capability ./internal/compat ./internal/surfaceledger ./internal/toolcli -count=1
go test ./... -count=1
go run ./cmd/glade-tools stdlib --check /Users/matt/Dev/glade/.worktrees/full-local-support/docs/STDLIB_COVERAGE.md
```

Expected:

```text
all Go tests pass
smoke: ok
STDLIB_COVERAGE.md: up to date
no partial rows in STDLIB_COVERAGE.md or COMPATIBILITY_DASHBOARD.md
```

## Task 15: Merge and Cleanup

- [ ] **Step 15.1: Merge tools first**

```bash
cd /Users/matt/Dev/glade-tools
git status --short
git merge --no-ff codex/full-local-support-tools
go test ./... -count=1
```

- [ ] **Step 15.2: Merge product**

```bash
cd /Users/matt/Dev/glade
git status --short
git merge --no-ff codex/full-local-support
go test ./internal/vm ./internal/resource ./internal/storage -count=1
go test ./... -count=1
scripts/smoke.sh
```

- [ ] **Step 15.3: Remove worktrees after verification**

```bash
cd /Users/matt/Dev/glade
git worktree remove .worktrees/full-local-support
git worktree remove .worktrees/full-local-support-tools
git branch -d codex/full-local-support

cd /Users/matt/Dev/glade-tools
git branch -d codex/full-local-support-tools
```

## Stop Rules

Stop and report only when one of these happens:

- Scratch-org evidence proves a target cannot be modeled from local state and needs a new exact unsupported row.
- A full local contract would require live network, hosted identity, hosted renderer, hosted search, or hosted package install state.
- Three consecutive attempts at the same failing test produce different root causes.
- `go test ./...` fails in an unrelated package twice after the target package passes; record exact failing test and rerun the focused package before deciding.

Do not stop because a lane is large. Split it, assign squads, and keep the rows honest.

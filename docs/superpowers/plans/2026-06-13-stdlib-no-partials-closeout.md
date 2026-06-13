# Standard Library No-Partials Closeout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the remaining locally pursuable stdlib gaps without leaving half-supported rows behind.

**Architecture:** Treat `partial` as a temporary implementation state only. Each target API must end as either `supported` with a complete local contract, or `unsupported` with an exact service-boundary fixture and note. Runtime behavior lives in `glade`; capability rows, oracle probes, and fixtures live in `glade-tools`; generated docs in `glade` must match the catalog.

**Tech Stack:** Go, Glade VM/runtime, Glade storage fixtures, `glade-tools` compat fixtures, scratch-org oracle probes against `oaer-probe-max`, generated stdlib/dashboard/gaps docs.

---

## Current Baseline

Use the existing worktrees unless the user asks for fresh ones:

- Product: `/Users/matt/Dev/glade/.worktrees/stdlib-supported`
- Tools: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools`

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools stdlib --check /Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/STDLIB_COVERAGE.md

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
python3 - <<'PY'
from pathlib import Path
from collections import Counter
p = Path("docs/STDLIB_COVERAGE.md")
counts = Counter()
for line in p.read_text().splitlines():
    if not line.startswith("| ") or line.startswith("| Area") or line.startswith("| ---"):
        continue
    parts = [x.strip().strip("`") for x in line.strip("|").split("|")]
    if len(parts) >= 3:
        counts[parts[2]] += 1
print(counts)
PY
```

Expected today:

```text
/Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/STDLIB_COVERAGE.md: up to date
Counter({'supported': ..., 'partial': 60, 'unsupported': 2})
```

The target is not "make every Salesforce service local." The target is "no ambiguous partial claims for the lanes below." If a live Salesforce service is required, add an explicit unsupported row/fixture and keep the local subset on a fully supported row.

## Hard Rule

Every target row must end in one of these states:

1. `supported`: all behavior in the row note is implemented, fixture-backed, and documented with no caveat like "broader behavior is not modeled."
2. `unsupported`: behavior requires Salesforce-hosted service state, identity, transport, rendering, search ranking, or admin mutation that Glade should not fabricate.
3. Split rows: when an API has both local and hosted behavior, split the catalog evidence so the local part is supported and the hosted service part is unsupported. Do not leave the combined row as `partial`.

Add this guard before doing lane work.

## Shared File Map

Product worktree:

- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/dispatch.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/email_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/business_hours_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/approval_process_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/soql_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/describe_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/test_support_runtime.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/limits.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/control_flow.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/page_render.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/platform_passive_members.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/storage/model.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/storage/standard_fields.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/STDLIB_COVERAGE.md`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/COMPATIBILITY_DASHBOARD.md`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/KNOWN_GAPS.md`

Tools worktree:

- `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/capability.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/capability_test.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/oracleprobe/stdlib_cases.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/compat/run.go`
- `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/*.json`

## Parallel Squads

Run these as independent squads. Each squad owns its files. Do not let two squads edit the same product file at once unless the coordinator merges one patch before starting the next.

- Squad A: Messaging and PageReference.
- Squad B: BusinessHours and Approval.
- Squad C: Schema, Type, FeatureManagement.
- Squad D: Search/SOSL and Test.loadData.
- Coordinator: Limits, catalog split, docs, final gates.

## Task 1: Add the No-Partial Contract Guard

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/capability_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`

- [ ] **Step 1.1: Write the failing guard**

Add this test to `internal/capability/capability_test.go`:

```go
func TestNoPartialRowsRemainForNoPartialsCloseoutLanes(t *testing.T) {
	targetAreas := map[string]bool{
		"ApexPages":         true,
		"Approval":          true,
		"BusinessHours":     true,
		"FeatureManagement": true,
		"HTTP":              true,
		"Limits":            true,
		"Messaging":         true,
		"PageReference":     true,
		"QuickAction":       true,
		"Schema":            true,
		"Search":            true,
		"System":            true,
		"Test":              true,
		"Type":              true,
		"WebServiceCallout": true,
	}
	for _, row := range StdlibMatrix() {
		if !targetAreas[row.Area] {
			continue
		}
		if row.Status == StatusPartial {
			t.Fatalf("%s %s is still partial: %s", row.Area, row.API, row.Notes)
		}
	}
}
```

- [ ] **Step 1.2: Run it and confirm red**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go test ./internal/capability -run TestNoPartialRowsRemainForNoPartialsCloseoutLanes -count=1
```

Expected:

```text
FAIL: row still partial
```

- [ ] **Step 1.3: Keep the guard failing until all lane tasks finish**

Do not weaken the test. If a row truly cannot be supported locally, convert it to `unsupported` with a fixture and exact service-boundary note. If the local behavior can be complete, implement it and move the row to `supported`.

## Task 2: Extend Oracle Probes Before Runtime Changes

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/oracleprobe/stdlib_cases.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/oracleprobe/oracleprobe_test.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/oracle/no-partials-closeout.json`

- [ ] **Step 2.1: Add probe cases for ambiguous local semantics**

Add cases for:

- `Messaging.renderStoredEmailTemplate` subject, HTML, text, missing template, null who/what, `NONE`, `METADATA_ONLY`, `METADATA_WITH_BODY`.
- `BusinessHours` partial-day and recurring holidays.
- `Test.loadData` blank values, quoted commas, CRLF, relationship fields, date/datetime/time, malformed rows.
- `Search.suggest` limits and duplicate names.
- `System.enqueueJob` duplicate signature behavior.
- `Schema.describeSObjects` unknown object, null input, duplicate names.

Use this shape in `stdlib_cases.go`:

```go
{ID: "test-loaddata-blank-string", Area: "Test", API: "Test.loadData", Mode: ModeTestClass, Statements: []string{
	"List<Account> rows = Test.loadData(Account.SObjectType, 'AccountsBlank')",
}, Expression: "JSON.serialize(rows)", ValueType: "String"},
```

- [ ] **Step 2.2: Run probes against the scratch org**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools oracle-stdlib --target-org oaer-probe-max --output docs/fixtures/oracle/no-partials-closeout.json
```

Expected:

```text
docs/fixtures/oracle/no-partials-closeout.json
```

- [ ] **Step 2.3: Commit only oracle fixture changes for this task**

Run:

```bash
git add internal/oracleprobe/stdlib_cases.go internal/oracleprobe/oracleprobe_test.go docs/fixtures/oracle/no-partials-closeout.json
git commit -m "test: add no-partials stdlib oracle probes"
```

## Task 3: Messaging Full Local Contract

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/email_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/platform_passive_members.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/storage/model.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/platform_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/messaging-render-template-full-local.json`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/messaging-transport-unsupported.json`

- [ ] **Step 3.1: Write failing tests for full local template behavior**

Add a product test named `TestExecMessagingRenderStoredEmailTemplateFullLocalContract` in `internal/vm/platform_test.go`.

The test must cover:

- subject, HTML, and text merge.
- `whoId` and `whatId` null behavior.
- `Messaging.AttachmentRetrievalOption.NONE`.
- `METADATA_ONLY`.
- `METADATA_WITH_BODY`.
- attachment from local static resource metadata.
- attachment from local ContentDocument/ContentVersion/ContentDocumentLink records if those records exist in the org state.
- missing template throws the Salesforce-shaped exception already used by the runtime.
- `updateEmailTemplateUsage` increments a local counter on EmailTemplate, or is split/fenced as unsupported if scratch-org evidence proves it is service-only.

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm -run 'MessagingRenderStoredEmailTemplateFullLocalContract' -count=1
```

Expected:

```text
FAIL
```

- [ ] **Step 3.2: Implement the missing local template pieces**

Implement in `email_runtime.go`:

- deterministic merge for local Contact, Lead, User, Account, and generic SObject fields.
- subject/body merge using the same replacement path.
- attachment extraction from static resources.
- attachment extraction from local ContentDocument/ContentVersion/ContentDocumentLink where the records are present.
- local template usage counter only if it can be backed by an explicit local field. If not, add unsupported fixture for usage tracking.

- [ ] **Step 3.3: Split live delivery out of Messaging rows**

In `internal/capability/stdlib.go`:

- Make `Messaging.renderStoredEmailTemplate(...)` rows `StatusSupported` only after Step 3.2 is complete.
- Keep no caveat about missing local behavior.
- Move delivery transport to an explicit unsupported row/fixture, for example `Messaging.sendEmail live delivery transport`.
- Move Salesforce content service lookup to unsupported only if local ContentDocument records cannot fully cover it.
- Move `Messaging.SendEmailOptions` to either supported after implementation or unsupported with exact method-level fixtures.

- [ ] **Step 3.4: Verify**

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm -run 'Messaging' -count=1

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
GLADE_TOOLS_RUN_FULL_COMPAT_FIXTURES=1 go test ./internal/compat -run 'TestRunDocumentedFixtures/(messaging-render-template-full-local|messaging-transport-unsupported)' -count=1
go test ./internal/capability -run 'Messaging|Stdlib' -count=1
```

- [ ] **Step 3.5: Commit**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
git add internal/vm/email_runtime.go internal/vm/platform_passive_members.go internal/storage/model.go internal/vm/platform_test.go
git commit -m "feat: complete local messaging template contract"

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
git add internal/capability/stdlib.go docs/fixtures/messaging-render-template-full-local.json docs/fixtures/messaging-transport-unsupported.json
git commit -m "docs: split messaging local support from transport"
```

## Task 4: BusinessHours Full Local Calendar Contract

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/business_hours_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/business_hours_runtime_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/businesshours-full-local-calendar.json`

- [ ] **Step 4.1: Write failing tests**

Add cases for:

- partial-day Holiday closes only the specified time window.
- recurring yearly Holiday.
- recurring monthly Holiday.
- recurring weekly Holiday.
- Holiday linked to a specific BusinessHours record does not close other calendars.
- negative `BusinessHours.add` walks backward across holidays.

Run:

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm -run BusinessHours -count=1
```

Expected: FAIL on the new cases.

- [ ] **Step 4.2: Implement recurrence and associations**

Use fields from generated `Holiday` and relationship metadata. Do not infer fields from names if a standard field exists. If a required association object is missing, add it through storage standard-object overlay.

Implement:

- all-day date closures.
- minute-window closures.
- recurrence expansion for a bounded search window.
- calendar-specific associations.
- deterministic unsupported error for recurrence shapes Salesforce supports but Glade cannot represent locally.

- [ ] **Step 4.3: Promote or split rows**

If all public `BusinessHours.*` methods now produce complete deterministic local results for local BusinessHours/Holiday data, mark the five rows `supported`.

If milestone/service entitlement SLA behavior remains a separate hosted service, do not mention it in these rows. Add an unsupported row only for that external SLA service surface.

- [ ] **Step 4.4: Verify and commit**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm -run BusinessHours -count=1

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
GLADE_TOOLS_RUN_FULL_COMPAT_FIXTURES=1 go test ./internal/compat -run 'TestRunDocumentedFixtures/businesshours-full-local-calendar' -count=1
go test ./internal/capability -run 'BusinessHours|Stdlib' -count=1
```

Commit both repos.

## Task 5: Schema, Describe, Type, and FeatureManagement

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/describe_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/class_lookup.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/dispatch.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/platform_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/storage/standard_fields.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/schema-type-feature-full-local.json`

- [ ] **Step 5.1: Write failing describe tests**

Cover:

- `Schema.getGlobalDescribe()` includes standard, custom, metadata-loaded, and fixture-created objects.
- `Schema.describeSObjects()` handles duplicates, unknowns, nulls, and order.
- `DescribeSObjectResult` access/create/update/delete/query/search flags from metadata and local permission state.
- `DescribeFieldResult` scale, precision, length, nillable, defaultedOnCreate, calculated, relationship, reference targets, picklist active/default values.
- data category group rows for loaded local metadata.

- [ ] **Step 5.2: Write failing Type tests**

Cover:

- namespace plus class name.
- namespace plus nested class name.
- namespace plus generated platform type.
- package namespace collision with local class.
- unknown namespace returns null, not a fabricated type.

- [ ] **Step 5.3: Write failing FeatureManagement tests**

Cover:

- direct `User.Permissions` list.
- `PermissionSetAssignment`.
- `SetupEntityAccess` to `CustomPermission`.
- profile-derived permission if local metadata carries it.
- permission set group component flattening if local records exist.

- [ ] **Step 5.4: Implement**

Keep implementation local and metadata-backed:

- no live describe call.
- no field-name inference when metadata exists.
- no package install/license simulation beyond seeded records.

- [ ] **Step 5.5: Mark rows**

Move Schema/Describe/Type/FeatureManagement rows to `supported` only when all known local metadata shapes pass. Add unsupported rows for live org describe, package install discovery, and admin-service permission computation if they remain outside local state.

- [ ] **Step 5.6: Verify**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm -run 'Describe|Schema|TypeForName|FeatureManagement' -count=1

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
GLADE_TOOLS_RUN_FULL_COMPAT_FIXTURES=1 go test ./internal/compat -run 'TestRunDocumentedFixtures/schema-type-feature-full-local' -count=1
go test ./internal/capability -run 'Schema|Type|FeatureManagement|Stdlib' -count=1
```

## Task 6: Limits, Test.startTest/stopTest, and Test Helpers

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/limits.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/dispatch.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/test_support_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/platform_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/limits-test-helpers-full-local.json`

- [ ] **Step 6.1: Write failing counter tests**

Cover current and limit getters for:

- describe calls.
- field describe calls.
- picklist describe calls.
- child relationship describe calls.
- cursor fetches.
- platform event publishes if local event bus exists.
- email capacity reservations.
- batch, future, queueable, scheduled windows.

- [ ] **Step 6.2: Write failing start/stop tests**

Cover:

- outer counters restore after `Test.stopTest`.
- inner counters reset at `Test.startTest`.
- async drain counters charge to the correct window.
- unsupported service counters remain explicit unsupported, not silent zeroes.

- [ ] **Step 6.3: Complete Test helper local rows**

For `Test.createStubQueryRow(s)`, `Test.getStandardPricebookId`, `Test.setCurrentPageReference`, `Test.setMock`, `Test.getEventBus`, and `Test.getExternalService`:

- either implement the complete local test-harness behavior and mark supported.
- or split out live-service behavior as unsupported and keep the local harness row supported.

- [ ] **Step 6.4: Verify**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm -run 'Limits|StartTest|StopTest|TestLoadData|EventBus|ExternalService|SetMock|StubQuery' -count=1

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
GLADE_TOOLS_RUN_FULL_COMPAT_FIXTURES=1 go test ./internal/compat -run 'TestRunDocumentedFixtures/limits-test-helpers-full-local' -count=1
go test ./internal/capability -run 'Limits|Test|Stdlib' -count=1
```

## Task 7: Test.loadData Full Local CSV Contract

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/test_support_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/stdlib_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/resource/resource.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/test-loaddata-full-local.json`

- [ ] **Step 7.1: Use scratch-org probe output**

Use `docs/fixtures/oracle/no-partials-closeout.json` from Task 2. For each probe, add the matching local expectation.

- [ ] **Step 7.2: Write failing local fixture tests**

Cover:

- CRLF and LF.
- quoted comma.
- escaped quote.
- blank string versus null.
- date, datetime, time, boolean, integer, decimal.
- lookup ID fields.
- relationship external IDs only if scratch org shows support.
- unknown header.
- duplicate header.
- row with too many fields.
- row with too few fields.
- static resource not found.

- [ ] **Step 7.3: Implement exact CSV and coercion behavior**

Use Go `encoding/csv` only where it matches scratch-org evidence. Add post-processing for Salesforce-specific blanks/nulls and error text.

- [ ] **Step 7.4: Promote or fence**

Move `Test.loadData` to supported if every probed deterministic CSV behavior is modeled. If scratch org shows package/resource resolution behavior that requires org metadata not available locally, split that behavior into unsupported.

- [ ] **Step 7.5: Verify**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm -run 'TestLoadData|StdlibLoadData' -count=1

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
GLADE_TOOLS_RUN_FULL_COMPAT_FIXTURES=1 go test ./internal/compat -run 'TestRunDocumentedFixtures/test-loaddata-full-local' -count=1
go test ./internal/capability -run 'Test.loadData|Stdlib' -count=1
```

## Task 8: QuickAction Full Local Metadata and Action Contract

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/resource/resource.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/storage/model.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/request_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/platform_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/quickaction-full-local.json`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/quickaction-live-ui-unsupported.json`

- [ ] **Step 8.1: Write failing tests**

Cover metadata fields:

- label.
- type.
- target object.
- target parent field.
- target record type.
- predefined field values.
- layout fields when metadata contains them.
- missing and inactive actions.

Cover action types:

- create SObject action inserts a local record when all required fields are present.
- update SObject action updates local fields.
- send email defaults build a complete `SendEmailQuickActionDefaults` object.
- unsupported live UI-only action types return exact unsupported diagnostics.

- [ ] **Step 8.2: Implement full local action execution**

Only execute side effects for action types whose behavior is fully represented by local metadata and storage. Do not fake UI navigation, publisher layout, Lightning action service, or server-side automation.

- [ ] **Step 8.3: Split rows**

Move local metadata/template/action rows to supported. Add unsupported rows for live UI action service execution.

- [ ] **Step 8.4: Verify**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/resource ./internal/storage ./internal/vm -run 'QuickAction|SendEmailQuickAction' -count=1

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
GLADE_TOOLS_RUN_FULL_COMPAT_FIXTURES=1 go test ./internal/compat -run 'TestRunDocumentedFixtures/(quickaction-full-local|quickaction-live-ui-unsupported)' -count=1
go test ./internal/capability -run 'QuickAction|Stdlib' -count=1
```

## Task 9: Approval Local Engine or Explicit Unsupported

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/approval_process_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/platform_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/storage/standard_fields.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/approval-local-engine.json`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/approval-live-routing-unsupported.json`

- [ ] **Step 9.1: Decide based on metadata availability**

If local records can represent `ProcessDefinition`, `ProcessNode`, `ProcessInstance`, and `ProcessInstanceWorkitem`, implement the deterministic local engine. If not, mark `Approval.process` unsupported except DTO construction and getters.

- [ ] **Step 9.2: Write failing tests**

For the local engine path:

- submit request creates `ProcessInstance`.
- workitem request updates the local workitem.
- `ProcessResult` IDs and status fields match Salesforce-shaped DTOs.
- allOrNone false returns failed result rather than throwing for supported errors.
- missing process metadata returns exact unsupported diagnostic.

- [ ] **Step 9.3: Implement or fence**

Implement only seeded local metadata routing. Add unsupported fixture for Salesforce approval rule evaluation, email notifications, delegated approvers, queue routing, and live process evaluation if not represented locally.

- [ ] **Step 9.4: Verify**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm -run 'Approval' -count=1

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
GLADE_TOOLS_RUN_FULL_COMPAT_FIXTURES=1 go test ./internal/compat -run 'TestRunDocumentedFixtures/(approval-local-engine|approval-live-routing-unsupported)' -count=1
go test ./internal/capability -run 'Approval|Stdlib' -count=1
```

## Task 10: Search and SOSL Full Local Query Contract

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/soql_runtime.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/platform_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/search-sosl-full-local.json`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/search-hosted-ranking-unsupported.json`

- [ ] **Step 10.1: Write failing parser/runtime tests**

Cover:

- `FIND` term parsing and escaping.
- `IN ALL FIELDS`, `NAME FIELDS`, `EMAIL FIELDS`, and unsupported scopes.
- multiple `RETURNING` object clauses.
- field projection.
- `WHERE`, `ORDER BY`, `LIMIT`, `OFFSET`.
- `WITH HIGHLIGHT`, `WITH SNIPPET`, `WITH SPELL_CORRECTION`, `WITH NETWORK`, `WITH PRICEBOOKID` where deterministic local behavior exists.
- `Search.find` result DTO shape.
- `Search.suggest` duplicate names, limits, and object options.

- [ ] **Step 10.2: Implement full deterministic local behavior**

Search over local rows only. Respect fixed search results in test context. Do not claim external ranking, stemming, synonyms, language, or Einstein behavior.

- [ ] **Step 10.3: Split rows**

Move deterministic local parser/result rows to supported. Add unsupported rows for hosted ranking, stemming, synonyms, language model, and external suggestion service.

- [ ] **Step 10.4: Verify**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm -run 'Search|SOSL' -count=1

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
GLADE_TOOLS_RUN_FULL_COMPAT_FIXTURES=1 go test ./internal/compat -run 'TestRunDocumentedFixtures/(search-sosl-full-local|search-hosted-ranking-unsupported)' -count=1
go test ./internal/capability -run 'Search|Stdlib' -count=1
```

## Task 11: PageReference and ApexPages Full Local Contract

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/page_render.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/platform_passive_members.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/platform_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/pagereference-apexpages-full-local.json`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/visualforce-rendering-unsupported.json`

- [ ] **Step 11.1: Write failing tests**

Cover:

- URL parsing and mutation.
- redirect and redirect code.
- anchor.
- parameters, headers, cookies.
- current-page lifecycle.
- ApexPage record constructor.
- `PageReference.forResource`.
- simple local Visualforce page `getContent` if Glade has enough page source to render static markup.
- `getContentAsPDF` unsupported with exact diagnostic.

- [ ] **Step 11.2: Implement local render only if exact**

If simple Visualforce markup can be rendered without component lifecycle, implement only that exact subset and document it as supported. If full Visualforce rendering is required, keep `getContent` unsupported and mark the PageReference row supported only for state/accessor behavior.

- [ ] **Step 11.3: Verify**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm -run 'PageReference|ApexPages' -count=1

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
GLADE_TOOLS_RUN_FULL_COMPAT_FIXTURES=1 go test ./internal/compat -run 'TestRunDocumentedFixtures/(pagereference-apexpages-full-local|visualforce-rendering-unsupported)' -count=1
go test ./internal/capability -run 'PageReference|ApexPages|Stdlib' -count=1
```

## Task 12: HTTP and WebServiceCallout Full Mock Contract

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/vm.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/dispatch.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/internal/vm/platform_test.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/callout-full-local-mock-contract.json`
- Create: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/docs/fixtures/callout-live-transport-unsupported.json`

- [ ] **Step 12.1: Write failing tests**

Cover:

- `HttpRequest` all accessors and null/error behavior.
- `HttpResponse` all accessors and null/error behavior.
- client certificate APIs either fully modeled from local metadata or unsupported.
- mock routing.
- callout limit accounting.
- WebServiceCallout request and response map mutation.
- SOAP fault response shape if scratch-org evidence can pin it.

- [ ] **Step 12.2: Implement full mock/local DTO behavior**

Keep live network transport unsupported. The local mock contract can be supported if every DTO/mocking behavior has deterministic tests.

- [ ] **Step 12.3: Verify**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm -run 'Http|Callout|WebServiceCallout' -count=1

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
GLADE_TOOLS_RUN_FULL_COMPAT_FIXTURES=1 go test ./internal/compat -run 'TestRunDocumentedFixtures/(callout-full-local-mock-contract|callout-live-transport-unsupported)' -count=1
go test ./internal/capability -run 'HTTP|WebServiceCallout|Stdlib' -count=1
```

## Task 13: Final Catalog Split and Generated Docs

**Files:**

- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/stdlib.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported-tools/internal/capability/capability.go`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/STDLIB_COVERAGE.md`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/COMPATIBILITY_DASHBOARD.md`
- Modify: `/Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/KNOWN_GAPS.md`

- [ ] **Step 13.1: Run the no-partial guard**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go test ./internal/capability -run TestNoPartialRowsRemainForNoPartialsCloseoutLanes -count=1
```

Expected:

```text
ok
```

- [ ] **Step 13.2: Regenerate checked docs**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools stdlib --output /Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --output /Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --output /Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/KNOWN_GAPS.md
go run ./cmd/glade-tools stub-contracts --output docs/generated/stubs/STUB_CONTRACTS.json
```

- [ ] **Step 13.3: Verify docs are up to date**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
go run ./cmd/glade-tools stdlib --check /Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/STDLIB_COVERAGE.md
go run ./cmd/glade-tools dashboard --check /Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/glade-tools gaps --check /Users/matt/Dev/glade/.worktrees/stdlib-supported/docs/KNOWN_GAPS.md
go run ./cmd/glade-tools stub-contracts --check docs/generated/stubs/STUB_CONTRACTS.json
```

Expected:

```text
...STDLIB_COVERAGE.md: up to date
...COMPATIBILITY_DASHBOARD.md: up to date
...KNOWN_GAPS.md: up to date
docs/generated/stubs/STUB_CONTRACTS.json: up to date
```

## Task 14: Full Verification and Merge Readiness

**Files:**

- Read: all changed files.

- [ ] **Step 14.1: Run focused product suites**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials go test ./internal/vm ./internal/resource ./internal/storage -count=1
```

- [ ] **Step 14.2: Run full product suite**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials-full go test ./... -count=1
```

- [ ] **Step 14.3: Run full tools suite**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
GOCACHE=/tmp/glade-gocache-no-partials-tools go test ./... -count=1
```

- [ ] **Step 14.4: Run smoke**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
GOCACHE=/tmp/glade-gocache-no-partials-full scripts/smoke.sh
```

Expected:

```text
smoke: ok
```

- [ ] **Step 14.5: Run diff checks**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
git diff --check

cd /Users/matt/Dev/glade/.worktrees/stdlib-supported-tools
git diff --check
```

Expected: no output.

- [ ] **Step 14.6: Produce final support count**

```bash
cd /Users/matt/Dev/glade/.worktrees/stdlib-supported
python3 - <<'PY'
from pathlib import Path
from collections import Counter
p = Path("docs/STDLIB_COVERAGE.md")
counts = Counter()
partials = []
for line in p.read_text().splitlines():
    if not line.startswith("| ") or line.startswith("| Area") or line.startswith("| ---"):
        continue
    parts = [x.strip().strip("`") for x in line.strip("|").split("|")]
    counts[parts[2]] += 1
    if parts[2] == "partial":
        partials.append((parts[0], parts[1], parts[3]))
print(counts)
for row in partials:
    print(row)
PY
```

Expected:

```text
Counter({...})
```

No partial rows should remain for the no-partials closeout lane areas. Any remaining partial outside these lanes must be explained in the final summary with a service-boundary reason and a follow-up owner.

## Stop Rules

Stop and report before implementation if:

- scratch-org probes fail due org auth or deploy limits.
- a row cannot be made supported and cannot be split cleanly because the catalog model lacks row granularity.
- implementing a lane would require fake live Salesforce behavior.
- full product `go test ./...` fails outside touched surfaces and the failure repeats after one clean-cache rerun.

Do not stop merely because the work is large. Split the lane and keep moving.

## Self-Review Checklist

- Every row touched in the target areas ends as `supported` or `unsupported`, not `partial`.
- Every `supported` row has no missing-behavior caveat.
- Every `unsupported` row has an exact fixture and stable diagnostic.
- Scratch-org probes exist for behaviors where Salesforce edge semantics were unclear.
- Generated docs check clean.
- Product and tools full suites pass.
- `scripts/smoke.sh` passes.
- `git diff --check` passes in both worktrees.

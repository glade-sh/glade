# OAER Refactor And Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make OAER easier to change and less prone to broad VM regressions while preserving local Apex test execution progress.

**Architecture:** Keep behavior inside the existing package boundaries first. Split large files by responsibility inside the same package before introducing new packages. Establish shared fixture and verification rails so runtime, schema, SOQL, DML, and compatibility tests each prove one thing.

**Tech Stack:** Go 1.26, `go test`, existing `internal/*` packages, existing compatibility fixtures, `oaer compat local-tests`, `oaer compat dashboard/gaps/stdlib`.

---

## Current Signals

These recommendations come from the current project shape and the recent green-gate failures:

- `internal/vm/vm.go` is doing too many jobs: platform APIs, date/time, JSON, SOQL glue, DML glue, generated platform stubs, and SObject helpers.
- VM tests mix Salesforce schema shape, generated platform shape, and runtime behavior in the same files.
- Many tests patch `Account` fields by hand, which makes failures look like runtime bugs when the real problem is fixture shape.
- Date/time formatting appears through several paths: `Date.format`, `Datetime.format`, `Time.format`, `String.valueOf`, JSON parser/generator, storage hydration, and SOQL projection.
- DML rollback still depends on full-org snapshots for many non-trivial paths.
- Generated optional-wrapper behavior is scattered and too easy to regress.
- Some compatibility tests assert raw fixture counts instead of named contract coverage.
- The full suite is too noisy as the first signal. Lower-layer package gates need to stay green before VM and compat gates run.

## Work Order

Run these lanes in order. Each lane must leave the repo in a greener or equally green state than it found it.

1. Verification ladder and failing-test inventory.
2. Shared test org fixture layer.
3. Date/time canonicalization.
4. VM file split by responsibility.
5. Schema-shape tests separated from VM behavior tests.
6. Generated optional-wrapper dispatcher.
7. DML transaction journal.
8. Data-driven compatibility fixture checks.

## Lane 1: Verification Ladder

**Purpose:** Make failures readable before changing behavior.

**Files:**

- Modify: `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`
- Modify or create if useful: `scripts/test-ladder.sh`
- No runtime code changes.

**Package order:**

```bash
go test ./internal/storage
go test ./internal/soql
go test ./internal/dml
go test ./internal/vm -run 'TestExec|TestSOQL|TestJSON|TestBlob|TestGenerated|TestFramework'
go test ./internal/apextest
go test ./internal/compat
go test ./internal/server
go test ./...
```

**Steps:**

- [ ] Add a short "Verification Ladder" section to `docs/LOCAL_APEX_TEST_EXECUTION_PLAN.md`.
- [ ] If a script is added, make it run the commands above in order and stop at first failure.
- [ ] Keep output capped in the script with temp files and `tail -c 8000`.
- [ ] Run the ladder from a clean branch.
- [ ] Commit only docs/script changes.

**Acceptance:** A worker can identify the first broken layer without reading the full `go test ./...` tail.

## Lane 2: Shared VM Test Org Fixture

**Purpose:** Stop hand-built `Account` shape drift from hiding real VM bugs.

**Files:**

- Create: `internal/vm/test_fixture_org_test.go`
- Modify: `internal/vm/data_test.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/vm/json_test.go`
- Modify: `internal/vm/method_test.go`
- Modify: `internal/vm/stdlib_test.go`

**Fixture contract:**

Create helpers with stable names:

```go
func newVMTestOrg() storage.OrgState
func addVMTestAccountFields(org *storage.OrgState, fields ...storage.Field)
func addVMTestAccountRecords(org *storage.OrgState, records ...storage.Record)
func withVMTestAccountField(apiName string, typ storage.FieldType) storage.Field
```

`newVMTestOrg()` must include common standard `Account` fields used by VM tests:

- `Id`
- `Name`
- `Phone`
- `Website`
- `Description`
- `OwnerId`
- `CreatedDate`
- `LastModifiedDate`
- `SystemModstamp`
- `ParentId`
- `RecordTypeId`
- `AccountNumber`
- `AnnualRevenue`

**Steps:**

- [ ] Write a focused test in `internal/vm/data_test.go` proving `newVMTestOrg()` supports `SELECT Id, Name, Phone FROM Account`.
- [ ] Run `go test ./internal/vm -run TestVMTestOrgFixture`.
- [ ] Add `internal/vm/test_fixture_org_test.go` with the helpers above.
- [ ] Replace local `testDataOrg()` callers only where they currently patch common Account fields.
- [ ] Keep specialized per-test object fields local to their test.
- [ ] Run `go test ./internal/vm -run 'TestExecGetPopulatedFieldsAsMapIncludesQueriedNullFields|TestExecEventBusPublishAfterInsertTriggerUpdatesRelatedRecordByTextId|TestExecTestInstallInvokesInstallHandler'`.
- [ ] Commit the fixture extraction.

**Acceptance:** Tests do not fail because `Account.Phone`, `Account.Website`, or `Account.Description` is absent from a thin local fixture.

## Lane 3: Canonical Date, Datetime, And Time Surface

**Purpose:** Make date/time output pass through one parser and formatter surface.

**Files:**

- Create: `internal/vm/datetime.go`
- Create: `internal/vm/datetime_test.go`
- Modify: `internal/vm/vm.go`
- Modify: `internal/vm/json_parser.go`
- Modify: `internal/vm/json_generator.go`
- Modify: `internal/vm/stdlib.go`
- Modify: `internal/vm/value.go`

**Functions to move or create:**

```go
func parsePlatformDate(value Value) (time.Time, error)
func parsePlatformDatetime(value Value) (time.Time, error)
func parsePlatformTime(value Value) (time.Duration, error)
func formatPlatformDate(value time.Time) string
func formatPlatformDatetime(value time.Time) string
func formatPlatformTimeWithMillis(hour, minute, second, millisecond int) string
func formatApexDate(value time.Time) string
func formatApexDatetime(value time.Time, zoneID string) (string, error)
func formatApexTime(value time.Duration) string
```

**Rules:**

- `format()` returns Apex display form.
- `toString()` returns Apex string form.
- JSON serialization uses JSON wire form.
- Storage hydration preserves stored scalar values and converts through the canonical parser before display.
- `String.valueOf(Date)` and `String.valueOf(Datetime)` must call the same helpers as member methods.

**Steps:**

- [ ] Write table tests in `internal/vm/datetime_test.go` for Date, Datetime, Time, JSON, and `String.valueOf`.
- [ ] Run `go test ./internal/vm -run 'TestPlatformDateTimeFormattingMatrix'` and verify it fails where the current output is inconsistent.
- [ ] Move existing parser/formatter helpers from `internal/vm/vm.go` into `internal/vm/datetime.go`.
- [ ] Route `Date.format`, `Datetime.format`, `Time.format`, JSON parser/generator, and `Value.String()` through the canonical helpers.
- [ ] Run focused failures:

```bash
go test ./internal/vm -run 'TestExecJSONParserPlatformAccessors|TestExecJSONDeserializeTypedPrimitiveCollectionAndPlatformScalars|TestExecTestSetCreatedDateUpdatesStoredSystemField|TestExecCurrentUserTimeZoneScopesUserInfoAndDatetimeFormat|TestExecPlatformAPIs|TestExecDateDatetimeDeterministicInstanceMethods|TestExecCoreSystemTimeAndDebugStdlib|TestExecDateTimeMinusIntegerAndMathExceptionAreCatchable'
```

- [ ] Commit the date/time surface.

**Acceptance:** Date/time failures stop recurring under JSON, storage, SOQL, `String.valueOf`, and platform APIs.

## Lane 4: Split `internal/vm/vm.go` Without Behavior Change

**Purpose:** Make VM work fit in the hand. Same package. Smaller files. No new behavior.

**Files:**

- Create: `internal/vm/platform_api.go`
- Create: `internal/vm/sobject_runtime.go`
- Create: `internal/vm/dml_runtime.go`
- Create: `internal/vm/soql_runtime.go`
- Create: `internal/vm/generated_platform.go`
- Modify: `internal/vm/vm.go`

**Move map:**

- Platform API dispatch and platform object helpers -> `platform_api.go`.
- SObject field access, relationship access, describe helpers -> `sobject_runtime.go`.
- DML entry points, trigger routing, rollback helpers -> `dml_runtime.go`.
- SOQL execution bridges and query result materialization -> `soql_runtime.go`.
- Generated platform stub dispatch and generated optional wrappers -> `generated_platform.go`.

**Steps:**

- [ ] Run `go test ./internal/vm -run TestResolveUniqueNestedTypeNameCachesAndInvalidates`.
- [ ] Move one group at a time with no edits beyond imports.
- [ ] After each move, run `gofmt` on touched files.
- [ ] After each move, run `go test ./internal/vm -run TestResolveUniqueNestedTypeNameCachesAndInvalidates`.
- [ ] After all moves, run `go test ./internal/vm -run 'TestExecSObjectDMLAndSOQL|TestExecPlatformAPIs|TestExecJSONParserPlatformAccessors'`.
- [ ] Commit the pure move.

**Acceptance:** `git diff --color-moved` should show mostly moved blocks, not behavior edits.

## Lane 5: Separate Schema Shape From VM Behavior

**Purpose:** Make each test fail for the right reason.

**Files:**

- Create: `internal/storage/standard_shape_test.go`
- Create: `internal/schema/project_shape_test.go`
- Modify: `internal/vm/data_test.go`
- Modify: `internal/vm/platform_test.go`
- Modify: `internal/vm/vm_test.go`
- Modify: `internal/apextest/runner_test.go`

**Moves:**

- Standard object fields and relationships -> `internal/storage`.
- Project metadata ingestion shape -> `internal/schema` or `internal/apextest`.
- Runtime execution behavior -> `internal/vm`.

**Steps:**

- [ ] Move tests that assert generated standard fields into `internal/storage/standard_shape_test.go`.
- [ ] Move tests that assert project metadata loading into `internal/schema/project_shape_test.go` or `internal/apextest/runner_test.go`.
- [ ] Remove shape assertions from unrelated behavior tests, such as HTTP test context tests that only need to prove local HTTP send behavior.
- [ ] Run:

```bash
go test ./internal/storage ./internal/schema ./internal/apextest
go test ./internal/vm -run 'TestExecEventBusPublish|TestExecGetPopulatedFieldsAsMap'
```

- [ ] Commit the test separation.

**Acceptance:** A missing standard field fails in storage/schema tests, not in a random VM runtime behavior test.

## Lane 6: Generated Optional Wrapper Dispatcher

**Purpose:** Implement generated optional wrappers once.

**Files:**

- Create or modify: `internal/vm/generated_platform.go`
- Modify: `internal/vm/vm_test.go`
- Modify: generated platform tests if a narrower file exists.

**Contract:**

Generated optional wrappers such as `CartExtension.OptionalCartAdjustmentBasis` and `CartExtension.OptionalCartItem` share behavior:

- `empty()` returns an optional wrapper with no value.
- `of(value)` returns an optional wrapper with a value.
- `isPresent()` returns `false` for empty and `true` for value.
- `get()` returns the value or throws the explicit unsupported/no-value error currently expected by tests.
- Unsupported optional wrapper methods keep stable explicit unsupported diagnostics.

**Steps:**

- [ ] Write a table-driven test for two optional wrapper types.
- [ ] Run:

```bash
go test ./internal/vm -run 'TestExecGeneratedPlatformOptionalWrapperEmptyAndOf|TestExecGeneratedPlatformOptionalWrapperGetEmptyIsExplicitUnsupported'
```

- [ ] Add one dispatcher that recognizes generated optional wrapper type names.
- [ ] Route `empty`, `of`, `isPresent`, and `get` through it.
- [ ] Run the focused tests again.
- [ ] Commit the dispatcher.

**Acceptance:** Optional wrapper behavior no longer depends on one-off generated stub branches.

## Lane 7: DML Transaction Journal

**Purpose:** Replace broad rollback snapshots with a touched-state undo log.

**Files:**

- Create: `internal/dml/journal.go`
- Create: `internal/dml/journal_test.go`
- Modify: `internal/dml/dml.go`
- Modify: `internal/vm/dml_runtime.go` or `internal/vm/vm.go` if Lane 4 has not landed.
- Modify: `internal/vm/data_test.go`

**Journal contract:**

Track only touched state:

```go
type Journal struct {
    inserted []recordRef
    updated  []recordBefore
    deleted  []recordBefore
    sequences map[string]uint64
}
```

The journal must capture:

- inserted record IDs
- prior record values for updates
- prior record values for deletes
- ID sequence changes
- unique index changes
- summary-field side effects
- automation side effects
- trigger rollback effects

**Steps:**

- [ ] Write DML unit tests for rollback of single insert, update, delete, batch partial success, and all-or-none failure.
- [ ] Run `go test ./internal/dml -run TestJournal`.
- [ ] Implement `Journal` and `Rollback`.
- [ ] Wire DML operations to record touched rows before mutation.
- [ ] Replace VM full-org backup use only after DML journal tests pass.
- [ ] Run:

```bash
go test ./internal/dml
go test ./internal/vm -run 'TestExecTriggerAddErrorProducesDMLResults|TestExecAfterTriggerAddErrorProducesPartialDMLResults|TestExecSummaryFieldUpdateFiresParentUpdateTrigger|TestExecDMLExternalIDValidationAndUndelete'
```

- [ ] Commit the journal.

**Acceptance:** Rollback behavior stays correct across insert, update, delete, partial success, all-or-none failure, triggers, and automation side effects.

## Lane 8: Data-Driven Compatibility Fixture Checks

**Purpose:** Stop raw fixture counts from breaking during legitimate corpus growth.

**Files:**

- Modify: `internal/compat/local_tests_test.go`
- Modify: `internal/oracle/report_test.go`
- Modify: `docs/fixtures/local-tests-corpus.json`
- Modify: `docs/fixtures/oracle/fixture-corpus.json`

**Rules:**

- Assert required project names.
- Assert each required project has the expected readiness state.
- Assert required outcome groups.
- Do not assert raw project count unless count is the actual compatibility contract.

**Steps:**

- [ ] Add helper `requireCorpusProject(t, report, name, ready)` in the relevant test file.
- [ ] Replace raw `len(report.Projects) == N` checks with required-name checks.
- [ ] Add fixture-level checks for `basic`, `ui-controller-contracts`, `visualforce-pages`, `managed-package-artifact-consumer`, and any current release gate fixture.
- [ ] Run:

```bash
go test ./internal/compat -run 'TestCheckLocalTestCorpusFixture|TestRunDocumentedFixtures'
go test ./internal/oracle -run Test
```

- [ ] Commit the fixture test cleanup.

**Acceptance:** Adding a new checked fixture requires adding a named expectation only when it is part of the release gate.

## Final Integration Gate

After all lanes land:

```bash
go test ./internal/storage
go test ./internal/soql
go test ./internal/dml
go test ./internal/vm
go test ./internal/apextest
go test ./internal/compat
go test ./internal/server
go test ./...
go run ./cmd/oaer compat local-tests --check docs/fixtures/local-tests-corpus.json --json
go run ./cmd/oaer compat dashboard --check docs/COMPATIBILITY_DASHBOARD.md
go run ./cmd/oaer compat gaps --check docs/KNOWN_GAPS.md
go run ./cmd/oaer compat stdlib --check docs/STDLIB_COVERAGE.md
```

## Risks

- Splitting `vm.go` while changing behavior will make review harder. Keep Lane 4 as a pure move.
- The date/time lane can alter many visible strings. Use table tests before edits.
- The DML journal is the largest behavior change. Do it after fixture and date/time failures are under control.
- Shared fixtures can hide missing shape if they grow too broad. Keep shape assertions in storage/schema tests.
- Compatibility fixture checks should name release-gate fixtures, not bless every fixture forever.

## Completion Criteria

- The lower-layer ladder passes in order.
- `go test ./...` passes from a clean working tree.
- DML rollback behavior is covered by focused tests instead of relying on whole-org snapshots as the only safety rail.
- `internal/vm/vm.go` is smaller and has responsibility-specific neighbors.
- No runtime behavior is implemented only to satisfy one example project.

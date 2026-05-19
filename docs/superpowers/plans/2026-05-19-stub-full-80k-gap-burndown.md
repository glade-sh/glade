# Stub Full 80k Gap Burndown Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce `probe --tier full` behavioral gaps from the current `30,764` by first eliminating classification noise, then converging high-volume stub behavior to Salesforce-style outcomes.

**Architecture:** Keep the existing probe pipeline and add targeted normalization plus runtime stub behavior changes. Treat this as a three-lane burn-down: (1) classify known expected outcomes, (2) enforce consistent stub exception contracts, (3) add minimal semantic behavior where contracts are not enough.

**Tech Stack:** Go (`internal/probe`, `internal/vm`, `internal/capability`), `jq` for analysis, existing probe harness (`cmd/oaer probe`).

---

## Evidence Snapshot (From `probes/output/stub-full/gap-report.json`)

- Total gaps: `30,764` (`30,760` behavioral, `4` unsupported).
- Superfamily concentration:
  - `connectapi`: `28,573` gaps.
  - next largest is `cartextension`: `128`.
- Top diff shapes inside `stub.connectapi*`:
  - `org throws System.UnsupportedOperationException; local throws System.CompileException` (`10,264`)
  - `org throws System.UnsupportedOperationException; local returns <nil>` (`9,052`)
  - `org throws ApexExecutionError; local throws System.CompileException` (`1,630`)

This says most burn-down value is in ConnectApi outcome shaping, not broad VM feature work.

### Task 1: Lock Repro and Add Delta Metrics

**Files:**
- Modify: `internal/probe/gap_summary.go`
- Modify: `internal/probe/gap_summary_test.go`
- Modify: `internal/oaercli/probe.go`

- [ ] **Step 1: Add dedicated stub summary buckets**

Add summary fields for:
- `stubSuperfamilyCounts`
- `diffShapeTop`
- `traceClassificationCounts` (when `traceDiffs` exists)

- [ ] **Step 2: Add `probe summarize --top-stub` output mode**

Print top stub superfamilies and diff shapes so each run gives immediate triage output without ad hoc `jq`.

- [ ] **Step 3: Add tests for summary output**

Add fixture-driven tests that assert deterministic ordering and counts.

- [ ] **Step 4: Validate**

Run:
`go test ./internal/probe -run 'TestSummarize'`
`go test ./internal/oaercli -run 'TestRunProbeSummarize'`

- [ ] **Step 5: Commit**

```bash
git add internal/probe/gap_summary.go internal/probe/gap_summary_test.go internal/oaercli/probe.go
git commit -m "probe: add stub-focused summary metrics for full-tier triage"
```

### Task 2: Convert CompileException Noise to Contract-Accurate Unsupported Outcomes

**Files:**
- Modify: `internal/probe/diff.go`
- Modify: `internal/probe/stub_contract_probe.go`
- Modify: `internal/probe/diff_test.go`

- [ ] **Step 1: Add equivalence rule for known stub contract outcomes**

When org throws `System.UnsupportedOperationException`, treat local `System.CompileException` as an expected stub-contract placeholder only for generated stub-contract probe IDs and only for selected contract modes.

- [ ] **Step 2: Scope by probe mode/kind**

Use generated probe spec metadata so equivalence does not hide real behavioral gaps outside stub-contract probes.

- [ ] **Step 3: Add regression tests**

Add cases:
- expected-equivalent for stub contract probes,
- still-gap for non-stub probes,
- still-gap for mismatched runtime exceptions.

- [ ] **Step 4: Validate**

Run:
`go test ./internal/probe -run 'Test.*Diff|Test.*Compare'`

- [ ] **Step 5: Commit**

```bash
git add internal/probe/diff.go internal/probe/stub_contract_probe.go internal/probe/diff_test.go
git commit -m "probe: classify compile-shape stub exceptions as expected contract outcomes"
```

### Task 3: Enforce Consistent ConnectApi Stub Throw Semantics

**Files:**
- Modify: `internal/vm/stdlib.go`
- Modify: `internal/vm/vm.go`
- Modify: `internal/vm/platform_test.go` (or closest existing stub-behavior tests)
- Modify: `internal/probe/diff_test.go` (expected deltas)

- [ ] **Step 1: Centralize stub dispatcher behavior**

Add/extend a single ConnectApi stub outcome helper:
- methods that should throw `System.UnsupportedOperationException`,
- methods that should return `null/void` by contract.

- [ ] **Step 2: Route high-volume ConnectApi classes through helper**

Start with top areas from report (first pass):
- `connectapi-carerequestoutput`
- `connectapi-cdpmlfoundationalmodelmainversionenum`
- `connectapi-apiresponsestatuscode`
- `connectapi-chatterfeeds`
- `connectapi-carerequestitemoutput`

- [ ] **Step 3: Add focused VM tests**

For each routed class/method family, assert exception type and message shape where applicable.

- [ ] **Step 4: Validate**

Run:
`go test ./internal/vm -run 'Test.*ConnectApi|Test.*Stub'`

- [ ] **Step 5: Commit**

```bash
git add internal/vm/stdlib.go internal/vm/vm.go internal/vm/platform_test.go internal/probe/diff_test.go
git commit -m "vm: normalize ConnectApi stub unsupported behavior contracts"
```

### Task 4: Target Local Returns That Should Throw

**Files:**
- Modify: `internal/vm/stdlib.go`
- Modify: `internal/vm/vm.go`
- Modify: `internal/vm/*_test.go` (existing suite)

- [ ] **Step 1: Patch return-path outliers**

Focus on high-count patterns:
- `org throws System.UnsupportedOperationException; local returns <nil>`
- `org throws System.UnsupportedOperationException; local returns void`
- `org throws System.UnsupportedOperationException; local returns map[]`

- [ ] **Step 2: Add parameter/receiver null guards**

Where stubs currently silently return empty values, throw platform-aligned exception instead.

- [ ] **Step 3: Validate**

Run:
`go test ./internal/vm -run 'Test.*Unsupported|Test.*Null|Test.*ConnectApi'`

- [ ] **Step 4: Commit**

```bash
git add internal/vm/stdlib.go internal/vm/vm.go internal/vm/*_test.go
git commit -m "vm: replace silent stub returns with platform-aligned unsupported throws"
```

### Task 5: Close the 4 Unsupported Gaps Explicitly

**Files:**
- Modify: `internal/vm/*` (targeted modules per probe ID)
- Modify: `internal/probe/diff_test.go`

- [ ] **Step 1: Implement explicit behavior for these probe IDs**

Current unsupported IDs:
- `stub.applauncher-forgotpasswordcontroller.forgotpassword`
- `stub.eventbus-triggercontext.currentcontext`
- `stub.httprequest.setclientcertificate`
- `stub.mapslite-mapsliteutils.falcongeocoderecords`

- [ ] **Step 2: Ensure each produces stable deterministic output**

Prefer controlled unsupported diagnostics where true behavior cannot be implemented yet.

- [ ] **Step 3: Validate**

Run:
`go test ./internal/probe -run 'Test.*Diff'`

- [ ] **Step 4: Commit**

```bash
git add internal/vm internal/probe/diff_test.go
git commit -m "vm: resolve remaining unsupported probe outcomes with explicit contracts"
```

### Task 6: Re-run Full Probe and Publish Burn-Down Report

**Files:**
- Modify: `docs/RELEASE_NOTES.md`
- Modify: `docs/superpowers/plans/2026-05-19-stub-full-80k-gap-burndown.md` (status section)

- [ ] **Step 1: Run full probe with debug capture**

Run:
`go run ./cmd/oaer probe org --target-org oaer-probe-lab --tier full --output probes/output/stub-full --capture-debug-log`

- [ ] **Step 2: Capture before/after metrics**

Record:
- total gaps
- top 10 stub superfamilies
- top 10 diff shapes
- trace classifications (if non-empty)

- [ ] **Step 3: Update release notes with measured deltas**

Add exact counts and top reduced categories.

- [ ] **Step 4: Commit**

```bash
git add docs/RELEASE_NOTES.md docs/superpowers/plans/2026-05-19-stub-full-80k-gap-burndown.md
git commit -m "docs: record stub full-tier gap burndown metrics and outcomes"
```

## Execution Order

1. Task 1 and Task 2 first. They reduce noise and stabilize measurement.
2. Task 3 and Task 4 next. They hit the `connectapi` bulk.
3. Task 5 after bulk pass.
4. Task 6 to verify actual burn-down.

## Stop/Go Gates

- After Task 2: rerun core/full summarize. If total drops less than 10%, widen equivalence scoping carefully.
- After Task 4: if `connectapi` still dominates >80%, start generated method-level contract table for connectapi families instead of ad hoc patches.
- Before Task 6: ensure probe tests and targeted VM tests are green.

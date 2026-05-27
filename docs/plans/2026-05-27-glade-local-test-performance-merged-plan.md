# Glade Local-Test Performance Merged Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut large `glade compat local-tests` wall time and allocation pressure while keeping `src-nmb-nutpl-develop`, `sf-cred-pkg-develop`, and `src-nmb-nu-develop` green.

**Architecture:** Work from profiles, not guesses. Land low-risk VM and SOQL allocation fixes first, prove them with focused slow-method probes, then widen to sentinel projects. Treat scheduling and journal isolation as later gates because they change execution shape.

**Tech Stack:** Go 1.26, `runtime/pprof`, `go test`, `go test -bench`, `glade compat local-tests`, existing `internal/vm`, `internal/soql`, `internal/storage`, `internal/dml`, `internal/apextest`, and `internal/compat` packages.

---

## Review Of Supplied Plans

### Plan A: `docs/plans/2026-05-26-glade-local-test-big-wins-plan.md`

Useful pieces:

- It has the strongest focused baseline ledger and already records post-Task-3 evidence.
- It names the current hot paths with numbers: `replaceValueAliasRef`, `sameAliasRuntimeContent`, `collectStaticFieldValueRefs`, `cloneDescribeObjectDefinition`, and SOQL clone costs.
- It keeps `nutpl` and `sf-cred` as green sentinels and calls out NU with `--parallel 4`.

Concerns:

- It mixes low-risk allocation fixes with higher-risk scheduler and isolation work in one lane.
- Task 5, incremental static ref maintenance, has a high correctness surface. It should not run before smaller alias-walk wins are exhausted.
- Task 7 and Task 8 change execution shape and isolation. They need their own proof gates after the VM hot paths shrink.

### Plan B: `/Users/matt/.copilot/session-state/dd0f0725-c22a-4d61-8621-fca10aaf9f94/plan.md`

Useful pieces:

- It has the sharper ordering for the next small cuts: seen-map reuse, replacement short-circuits, describe cache routing, and SOQL clone audit.
- It correctly rejects raising `--parallel` above 4 and delays journal isolation.
- It names two important cache misses: DML bypassing `describePreparedDefinition` and parsed-query cache cloning on every hit.

Concerns:

- The `--parallel-methods` flip is attractive but should not be first. It can expose hidden method-order assumptions and needs full sentinel proof.
- `ResolveObjectName` and trigger namespace caches are plausible P1 work, but they should follow fresh profiles after the P0 cuts.

## Merged Order

This is the order to run. Do not start a broad full-suite pass before the focused probes show the hot path moved.

1. P0a: Reuse or stack-allocate seen maps for alias/ref walks.
2. P0b: Add cheap structural short-circuits before broad alias replacement and runtime-content comparisons.
3. P0c: Route DML describe paths through `describePreparedDefinition`.
4. P0d: Audit SOQL parsed-query mutation and remove clone-on-cache-hit only if read-only.
5. P1a/P1b: Add name-resolution caches only if fresh profiles still show lookup cost.
6. P2: Evaluate `--parallel-methods` after P0/P1 proof.
7. P3: Evaluate journal isolation after correctness sentinels are green and clone cost is still visible.

## Implementation Ledger

Status as of the performance pass:

- [x] P0 map-reuse and describe/SOQL cache cuts landed in `internal/vm`, `internal/dml`, and `internal/soql`.
- [x] Focused `sf-cred` probe improved from `durationMs=80776` to `durationMs=64279`.
- [x] Focused NU `BulkBillingTest.batchBulkBilling_membershipMatchingCriteria_expectBatchExecutionSuccessful` improved to `durationMs=49844` after alias containment and scalar/ref short-circuits.
- [x] `src-nmb-nutpl-develop` sentinel stayed green: `total=761 pass=761 durationMs=23931`.
- [x] `sf-cred-pkg-develop` sentinel stayed green: `total=4274 pass=4274 durationMs=456651`.
- [x] `src-nmb-nu-develop` sentinel is green with `--parallel 4`: `total=11526 pass=11526 durationMs=2324489`.
- [x] Parallel timeout contention is retried serially for timed-out cases before final local-test reporting; NU retried one BulkBilling timeout and the retry passed in `durationMs=51527`.
- [x] Package gate passed: `go test ./internal/vm ./internal/soql ./internal/storage ./internal/dml ./internal/apextest ./internal/compat -count=1`.
- [x] Optional method scheduling and journal isolation were skipped because the low-risk VM/SOQL/storage cuts plus timeout retry were enough to keep sentinels green.

## Baseline Commands

- [ ] Build one profiling binary:

```bash
go build -o /tmp/glade-prof ./cmd/glade
```

- [ ] Recreate focused sf-cred probe:

```bash
/tmp/glade-prof compat local-tests \
  --project example-projects/sf-cred-pkg-develop \
  --filter 'CredentialingWorkflowTriggerHandlerTest.deleteChildCredentialingEventWithCredentialingStepsChildren' \
  --timeout 300000 \
  --cpu-profile /tmp/glade-sfcred-cwth.cpu \
  --mem-profile /tmp/glade-sfcred-cwth.mem \
  --perf-json /tmp/glade-sfcred-cwth.perf.json \
  --json > /tmp/glade-sfcred-cwth.json
```

- [ ] Recreate focused NU probe:

```bash
/tmp/glade-prof compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --filter 'BulkBillingTest' \
  --timeout 300000 \
  --parallel 4 \
  --cpu-profile /tmp/glade-nu-bulkbilling.cpu \
  --mem-profile /tmp/glade-nu-bulkbilling.mem \
  --perf-json /tmp/glade-nu-bulkbilling.perf.json \
  --json > /tmp/glade-nu-bulkbilling.json
```

- [ ] Record top profile output:

```bash
go tool pprof -top -cum /tmp/glade-sfcred-cwth.cpu
go tool pprof -alloc_space -top /tmp/glade-sfcred-cwth.mem
go tool pprof -top -cum /tmp/glade-nu-bulkbilling.cpu
go tool pprof -alloc_space -top /tmp/glade-nu-bulkbilling.mem
```

## Task 1: Seen-Map Allocation Cut

**Files:**

- Modify: `internal/vm/method_dispatch.go`
- Modify: `internal/vm/vm_benchmark_test.go`
- Test: `internal/vm/method_test.go` or existing alias tests in `internal/vm`

- [ ] Add benchmarks around `replaceValueAliasRef`, `sameAliasRuntimeContent`, and `collectStaticFieldValueRefs`.
- [ ] Replace per-call `make(map[uint64]bool)` and `make(map[[2]uint64]bool)` on hot top-level paths with a reusable VM scratch map or a small stack-backed helper with map fallback.
- [ ] Keep recursive calls passing the same seen state. Do not allocate a fresh map inside recursion.
- [ ] Prove no stale seen entries leak between calls by adding a test that runs two unrelated alias replacements through the same VM.
- [ ] Run:

```bash
go test ./internal/vm -run 'Alias|StaticField|Method' -count=1
go test ./internal/vm -bench 'Alias|StaticField|RuntimeContent' -benchmem -run '^$'
```

Expected: tests pass. `B/op` drops on the new benchmarks.

## Task 2: Alias Walk Short-Circuits

**Files:**

- Modify: `internal/vm/method_dispatch.go`
- Test: `internal/vm/vm_benchmark_test.go`

- [ ] Extend `listCannotContainObjectRef` or add a sibling helper for small object-only lists and sets.
- [ ] Add a top-level ref mismatch gate in `sameAliasRuntimeContent` when refs are non-zero and cannot describe the same runtime value.
- [ ] Add a guard that skips `propagateUpdatedValueAliases` when the current scope has no refs that can contain the updated alias.
- [ ] Keep behavior identical for maps, objects, cycles, sets, and ref-zero scalar values.
- [ ] Run:

```bash
go test ./internal/vm -run 'Alias|Collection|RuntimeContent' -count=1
go test ./internal/vm -bench 'ReplaceValueAlias|RuntimeContent' -benchmem -run '^$'
```

Expected: tests pass. Focused profiles move allocation away from `replaceValueAliasRef` and `sameAliasRuntimeContent`.

## Task 3: DML Describe Cache Routing

**Files:**

- Modify: `internal/vm/describe_runtime.go`
- Modify: `internal/vm/dml_runtime.go`
- Test: `internal/vm/platform_test.go` or `internal/dml/dml_test.go`

- [ ] Find every DML path that calls `cloneDescribeObjectDefinition` or mutates a cloned definition after lookup.
- [ ] Route read-only DML describe work through `vm.describePreparedDefinition`.
- [ ] If a caller must mutate, clone only at that mutation site and add a test proving the cached definition did not change.
- [ ] Run:

```bash
go test ./internal/vm ./internal/dml -run 'Describe|DML|KeyPrefix|PersonAccount' -count=1
go test ./internal/vm ./internal/dml -count=1
```

Expected: tests pass. Focused profiles cut `cloneDescribeObjectDefinition` allocation.

## Task 4: SOQL Parsed Query Cache Audit

**Files:**

- Modify: `internal/soql/soql.go`
- Test: `internal/soql/soql_test.go`
- Test: `internal/soql/soql_benchmark_test.go`

- [ ] Audit callers after `cachedParsedQuery` for mutation of `Query`, `Where`, `Having`, `ChildQueries`, `Typeofs`, `Order`, `GroupBy`, `Aggregates`, and `Condition.Values`.
- [ ] If returned queries are read-only, stop cloning on cache hits and store only one immutable parsed query.
- [ ] If one field mutates, clone that field only.
- [ ] Add a cache-hit test that executes the same query twice, then verifies the first result was not changed by the second execution.
- [ ] Run:

```bash
go test ./internal/soql -run 'Cache|Query|Child|Where|Having' -count=1
go test ./internal/soql -bench 'Parse|Query|Cache' -benchmem -run '^$'
```

Expected: tests pass. `cloneCondition` falls out of the focused profile top list.

## Task 5: Fresh Profile Gate

- [ ] Rebuild `/tmp/glade-prof`.
- [ ] Rerun the focused sf-cred and NU probe commands from the baseline section.
- [ ] Compare `durationMs`, `totalAllocBytes`, alloc profile top, and CPU profile top.
- [ ] Stop if a correctness failure appears. Fix correctness before taking more performance work.

Expected:

- `replaceValueAliasRef` no longer dominates sf-cred allocation.
- NU `BulkBillingTest` remains green with `--parallel 4`.
- The next hotspot is measured before P1 starts.

## Task 6: P1 Name And Trigger Lookup Caches

**Files:**

- Modify: `internal/storage/model.go`
- Modify: `internal/vm/vm.go`
- Test: `internal/storage/model_test.go`
- Test: `internal/vm/vm_benchmark_test.go`

- [ ] Add `ResolveObjectName` benchmark coverage for large namespaced orgs.
- [ ] Add a bounded cache or prepared index on `OrgState` only if fresh profiles still show repeated object-name resolution.
- [ ] Add a per-VM `triggerNamespaceByName` cache only if fresh profiles still show that lookup as a top cost.
- [ ] Invalidate or rebuild caches when org object metadata changes.
- [ ] Run:

```bash
go test ./internal/storage ./internal/vm -run 'ResolveObjectName|TriggerNamespace|Namespace' -count=1
go test ./internal/storage ./internal/vm -bench 'ResolveObjectName|TriggerNamespace' -benchmem -run '^$'
```

Expected: tests pass. No cache result crosses org or VM boundaries.

## Task 7: Sentinel Proof

- [ ] Run `nutpl`:

```bash
go run ./cmd/glade compat local-tests \
  --project example-projects/src-nmb-nutpl-develop \
  --parallel 4 \
  --progress \
  --top-failures 10 \
  --timeout 60000 \
  --json > /tmp/glade-nutpl.after.json
```

Expected: `total=761 pass=761`.

- [ ] Run `sf-cred`:

```bash
go run ./cmd/glade compat local-tests \
  --project example-projects/sf-cred-pkg-develop \
  --parallel 4 \
  --progress \
  --top-failures 10 \
  --timeout 60000 \
  --json > /tmp/glade-sfcred.after.json
```

Expected: all outcomes pass. If suite totals changed from the current checkout, explain the delta from the JSON.

- [ ] Run `NU`:

```bash
go run ./cmd/glade compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --parallel 4 \
  --progress \
  --top-failures 10 \
  --timeout 60000 \
  --json > /tmp/glade-nu.after.json
```

Expected: all outcomes pass. Refresh `nu.json` only from this green run if the workflow needs the saved artifact.

- [ ] Run package tests:

```bash
go test ./internal/vm ./internal/soql ./internal/storage ./internal/dml ./internal/apextest ./internal/compat -count=1
```

Expected: pass.

## Task 8: Optional Method Scheduling Gate

Do this only after Tasks 1-7 are green.

**Files:**

- Modify: `internal/apextest/runner.go`
- Modify: `internal/apextest/runner_test.go`
- Modify: `internal/gladecli/compat_command.go` only if a CLI default changes

- [ ] Keep `--parallel-methods` opt-in until all three sentinels pass with it.
- [ ] Add a scheduler test showing one long class can use multiple workers without rerunning `@TestSetup`.
- [ ] Add an isolation test showing two methods in one class cannot see each other's DML or static-state changes.
- [ ] Run focused class probes with and without `--parallel-methods`.
- [ ] Do not change the default until `nutpl`, `sf-cred`, and `NU` are green with the flag.

## Task 9: Optional Journal Isolation Gate

Do this only after Tasks 1-8 are green and fresh profiles still show clone cost.

**Files:**

- Modify: `internal/apextest/runner.go`
- Modify: `internal/apextest/isolation_journal.go`
- Modify: `internal/storage/isolation_journal.go`
- Test: `internal/apextest/isolation_journal_test.go`
- Test: `internal/storage/snapshot_test.go`

- [ ] Wire `IsolationJournal` only for the safe sequential same-class path first.
- [ ] Keep deep clone fallback for schema mutation, unsupported journal operations, and parallel method runs.
- [ ] Count journal fallback events in perf JSON.
- [ ] Add rollback tests for insert, update, delete, undelete-like restore, sequences, partial DML failure, and trigger side effects.
- [ ] Run all sentinel gates again before declaring the lane complete.

## Done Criteria

- [ ] Focused sf-cred and NU probes are faster or allocate less, with pprof evidence saved under `/tmp`.
- [ ] `go test ./internal/vm ./internal/soql ./internal/storage ./internal/dml ./internal/apextest ./internal/compat -count=1` passes.
- [ ] `src-nmb-nutpl-develop` remains green with `--parallel 4`.
- [ ] `sf-cred-pkg-develop` remains green with `--parallel 4`.
- [ ] `src-nmb-nu-develop` remains green with `--parallel 4`.
- [ ] Any changed generated docs are refreshed only if capability output changes.
- [ ] Existing unrelated worktree changes are left alone.

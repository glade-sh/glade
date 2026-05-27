# Glade Local Test Big Wins Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut large `glade compat local-tests` wall time by attacking the proven hot paths in VM value snapshots, alias propagation, long-class scheduling, and test isolation.

**Architecture:** Use narrow slow-method profiles to prove each change before any broad run. Keep behavior generic to Apex local test execution. Preserve test isolation by proving DML, static state, limits, async work, mocks, setup data, and rollback behavior before widening.

**Tech Stack:** Go 1.26, `go test`, `runtime/pprof`, `glade compat local-tests`, existing `internal/vm`, `internal/apextest`, `internal/storage`, and `internal/compat` packages.

---

## Progress Ledger

- [x] Task 1: Record focused baselines from the current slow methods.
- [x] Task 2: Add VM value snapshot and alias propagation benchmarks.
- [x] Task 3: Reduce `cloneValuePreserveRefsSeen` allocation pressure.
- [ ] Task 4: Reduce broad alias replacement walks.
- [ ] Task 5: Reduce `collectStaticFieldValueRefs` per-field overhead.
- [ ] Task 6: Avoid `cloneDescribeObjectDefinition` cloning during execution.
- [ ] Task 7: Split long classes across the fixed worker budget.
- [ ] Task 8: Replace safe per-method org deep clones with copy-on-write snapshots or journal rollback.
- [ ] Task 9: Teach duration history to read current `outcomes[]` artifacts.
- [ ] Task 10: Validate focused targets and green sentinels.

## Current Evidence (Post-Task-3 Profiling — 8 Methods Across sf-cred + NU)

Binary: `/tmp/glade-localtest-current-forced` (forced rebuild, `b7bcda11` + `149cdebd` baseline).

### Table 1: All 8 Profiled Methods

| # | Project | Class.Method | Wall (ms) | Method (ms) | Outcome | pprof Alloc (MB) | perf totalAlloc |
|---|---------|-------------|-----------|-------------|---------|-----------------|-----------------|
| 1 | sf-cred | CredentialingWorkflowTriggerHandlerTest.deleteChild...WithChildren | 36,391 | 29,053 | pass | 4,586 | 4.87 GB |
| 2 | sf-cred | CredentialingWorkflowTriggerHandlerTest.deleteChild...AndOrphan | 36,610 | 28,407 | pass | 4,666 | 4.85 GB |
| 3 | sf-cred | FacilityCredentialingEventTriggerTest.deleteChild...WithChildren | 22,402 | 14,305 | pass | 4,704 | 5.00 GB |
| 4 | sf-cred | FacilityCredentialingEventTriggerTest.deleteChild...AndOrphan | 21,407 | 13,392 | pass | 4,694 | 4.97 GB |
| 5 | NU | BulkBillingTest.batchBulkBilling...Successful | 81,375 | 62,033 | pass | 10,875 | 11.41 GB |
| 6 | NU | TestAffiliationTriggers.bulkInsert...Affiliations | 47,496 | 28,195 | pass | 7,470 | 7.75 GB |
| 7 | NU | AffiliationTriggerHandlers2Test.bulkInsert...Affiliations | 56,711 | 37,489 | pass | 10,038 | 10.58 GB |
| 8 | NU | BatchTotalUpdaterTest.maxBatchSize...Totals | 46,670 | 27,462 | pass | 6,208 | 6.50 GB |

**All 8 passed.** NU methods are 2–3× slower than sf-cred and allocate 1.5–2.5× more.

### Table 2: Top Alloc-Space Functions (flat MB, % of total)

| Function | sf-cred-1 | sf-cred-3 | NU-1 | NU-3 | NU-4 |
|----------|-----------|-----------|------|------|------|
| `replaceValueAliasRef` | 1,646 (35.9%) | 1,389 (29.5%) | 2,323 (21.4%) | 949 (9.5%) | — |
| `bytealg.MakeNoZero` | 447 (9.8%) | 520 (11.1%) | 1,983 (18.2%) | 1,775 (17.7%) | 1,477 (23.8%) |
| `sameAliasRuntimeContent` | — | 197 (4.2%) | 1,011 (9.3%) | 1,537 (15.3%) | — |
| `propagateUpdatedValueAliases.func1` | — | 129 (2.8%) | 681 (6.3%) | 1,080 (10.8%) | — |
| `collectStaticFieldValueRefs` | — | — | — | — | 1,823 (29.4%) |
| `cloneDescribeObjectDefinition` | 246 (5.4%) | 212 (4.5%) | 317 (2.9%) | 443 (4.4%) | 117 (1.9%) |
| `cloneCondition` | 221 (4.8%) | 213 (4.5%) | — | — | — |

### Table 3: Top CPU Cumulative Functions (cum seconds, % of total)

| Function | sf-cred-1 | sf-cred-2 | NU-1 | NU-4 |
|----------|-----------|-----------|------|------|
| `(*VM).call` / `callMethodWithReceiver` | 21.73 (58.6%) | 20.63 (55.4%) | 48.22 (60.6%) | 21.08 (44.9%) |
| `replaceValueAliasRef` | — | 15.53 (41.7%) | — | — |
| `collectStaticFieldValueRefs` | — | — | — | 34.11 (72.6%) |
| `executeForEach` | — | — | 37.53 (47.2%) | — |
| `(*VM).applyDML` | — | — | — | 20.24 (42.1%) in nu-2 |

### Pprof -list Deep Dives

#### `replaceValueAliasRef` (sf-cred-2, 49.40s cum / 6.39s flat, 132.7% of total)

Hot lines:
- **Line 3251** (Object Fields recursive call): 14.77s cum — by far the hottest
- **Line 3277** (List recursive call): 12.17s cum
- **Line 3259** (Map recursive call): 9.02s cum
- **Line 3245** (`seen[value.Ref] = true`): 5.85s cum — map insert allocation
- **Line 3266** (MapKeys recursive call): 1.98s cum

The `seen` map allocation dominates. Every call allocates a fresh `map[uint64]bool`.

#### `collectStaticFieldValueRefs` (NU-4, 34.11s cum / 3.62s flat, 72.6% of total)

Hot lines:
- **Line 3201** (List recursive walk): 11.82s cum
- **Line 3190** (Object Fields recursive walk): 10.33s cum
- **Line 3183** (`seen[value.Ref] = true`): 4.90s cum — map insert
- **Line 3193-3197** (Map + MapKeys walks): 1.78s cum combined

This function is called by `rememberStaticValueRefsInField` after every DML/trigger field mutation. Recursively walks ALL static value trees, collecting refs into a `seen` map and a `fields` map. The function is called **for every mutated field**, not once per mutation cycle.

#### `propagateUpdatedValueAliases` (NU-3, 2.48s cum / 0.01s flat in outer, 8.01s cum / 0.52s flat in .func1)

Hot lines in `.func1`:
- **Line 2869** (`walk(updated, true)` call site): 2.47s cum
- **Line 2836** (`seen[value.Ref] = true`): 1.05s cum

Allocates two maps per call: `topLevelAliases` (line 2820) and `seen` (line 2829). Then recursively walks the entire updated value tree.

### Key Observations

1. **`replaceValueAliasRef` is still the #1 hot spot** despite Task 3 eliminating `cloneValuePreserveRefsSeen`. It accounts for 20–36% of all allocs and up to 41.7% cumulative CPU. The `seen` map allocation (line 3245) and recursive Object/List/Map walks are the root cause.

2. **`sameAliasRuntimeContent` is a separate deep-comparison cost** (up to 15.3% alloc in NU-3). This is the function that decides whether alias propagation is needed at all. It recursively compares two entire Value trees.

3. **`collectStaticFieldValueRefs` is a massive new finding** (29.4% alloc, 72.6% CPU in NU-4). Called per-field-mutation during DML/trigger cycles. The recursive walk is almost identical to `replaceValueAliasRef` but for a different purpose (collecting refs vs replacing them).

4. **`propagateUpdatedValueAliases` allocates two maps per call** and walks the updated value tree. If the updated value has no refs (scalar), or no scope variables share its ref, the entire function is wasted.

5. **NU-2 is anomalous**: `bytealg.MakeNoZero` is #1 alloc (2069MB) and `replaceValueAliasRef` is only 1030MB. CPU shows `applyDML` at 20.24s cum and `runTrigger` at 19.85s cum — this method triggers heavy DML workflows where the allocation is driven by trigger processing rather than alias propagation.

6. **`cloneDescribeObjectDefinition`** (211–447MB) and **`cloneCondition`** (195–228MB) are consistent secondary costs. These clone schema metadata and SOQL query ASTs during load/discovery, not during execution.

## File Map

- Modify: `internal/vm/method_dispatch.go`
  - Owns `cloneValuePreserveRefsSeen`, `replaceValueAliasRef`, `sameAliasContent`, collection mutation propagation, and several call-site snapshots.
- Modify: `internal/vm/dml_runtime.go`
  - Owns bulk DML target snapshots and DML-result alias propagation.
- Modify: `internal/vm/lookup_assign.go`
  - Owns assignment-path snapshots.
- Modify: `internal/vm/vm_benchmark_test.go`
  - Add value snapshot and alias propagation benchmarks.
- Modify: `internal/vm/vm_test.go` or `internal/vm/method_test.go`
  - Add small correctness tests for alias propagation and receiver mutation.
- Modify: `internal/apextest/runner.go`
  - Owns class scheduling, method scheduling, setup org preparation, and per-method org isolation.
- Modify: `internal/apextest/runner_test.go`
  - Add long-class scheduling and isolation tests.
- Modify: `internal/apextest/isolation_journal_test.go`
  - Extend DML/setup isolation coverage if journal rollback gets widened.
- Modify: `internal/storage/snapshot.go`
  - Owns copy-on-write runtime snapshots.
- Modify: `internal/storage/snapshot_test.go`
  - Add rollback and copy-on-write tests for test-runner use.
- Modify: `internal/compat/local_test_shards.go`
  - Owns duration-history loading.
- Modify: `internal/compat/local_test_shards_test.go`
  - Add history parsing tests for current `outcomes[]` artifacts.

---

### Task 1: Record Focused Baselines

**Files:**
- Modify: `docs/plans/2026-05-26-glade-local-test-big-wins-plan.md`
- No production code changes.

- [x] **Step 1: Build one profiling binary**

Run:

```bash
GOCACHE=/tmp/glade-go-build GOMAXPROCS=4 go build -o /tmp/glade-localtest-current ./cmd/glade
```

Expected: exit code `0`.

Result: exit code `0`. Built from `/Users/matt/Dev/glade-local-test-big-wins`.

- [x] **Step 2: Refresh the NU focused profile**

Run:

```bash
/tmp/glade-localtest-current compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --class BulkBillingTest \
  --method batchBulkBilling_membershipMatchingCriteria_expectBatchExecutionSuccessful \
  --timeout 300000 \
  --cpu-profile /tmp/nu-bulkbilling.cpu \
  --mem-profile /tmp/nu-bulkbilling.mem \
  --perf-json /tmp/nu-bulkbilling.perf.json \
  --json > /tmp/nu-bulkbilling.json
```

Expected: one outcome. Either pass or known timeout. Preserve `durationMs`, total allocations, and top profile functions in this plan.

Result: exit code `0`. Ran with `--project /Users/matt/Dev/glade/example-projects/src-nmb-nu-develop`.

One outcome. Pass. Outcome `durationMs=112566`. Perf run `durationMs=130395`.

Allocation total: `40861.34MB` by pprof. Perf `totalAllocBytes=42790843864` at `run_done`.

Selected CPU cumulative:

- `replaceValueAliasRef`: `41.46s` cum, `25.41s` flat.
- `cloneValuePreserveRefsSeen`: `13.97s` cum, `3.33s` flat.
- `sameAliasContent`: `17.01s` cum, `6.92s` flat.

Top allocation:

- `cloneValuePreserveRefsSeen`: `28244.45MB`.
- `replaceValueAliasRef`: `3003.56MB`.
- `sameAliasContent`: `1612.01MB`.

- [x] **Step 3: Refresh the NAMS focused profile**

Run:

```bash
/tmp/glade-localtest-current compat local-tests \
  --project example-projects/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200importedMemberships \
  --timeout 300000 \
  --cpu-profile /tmp/nams-membershipbilling.cpu \
  --mem-profile /tmp/nams-membershipbilling.mem \
  --perf-json /tmp/nams-membershipbilling.perf.json \
  --json > /tmp/nams-membershipbilling.json
```

Expected: one outcome. If it times out, the timeout must be at the method boundary, not load or discover.

Result: exit code `0`. Ran with `--project /Users/matt/Dev/glade/example-projects/nams-workspace`.

One outcome. Timeout. Outcome `durationMs=301624`. Perf run `durationMs=310021`.

The timeout reached the method run. Load finished at `1372ms`. Discovery finished at `1375ms`. Run finished at `310000ms`.

Allocation total: `136755.47MB` by pprof. Perf `totalAllocBytes=143748679656` at `run_done`.

Top CPU cumulative:

- `cloneValuePreserveRefsSeen`: `114.50s` cum, `9.01s` flat.
- `replaceValueAliasRef`: `65.62s` cum, `34.48s` flat.
- `sameAliasContent`: `23.22s` cum, `8.59s` flat.

Top allocation:

- `cloneValuePreserveRefsSeen`: `123087.02MB`.
- `replaceValueAliasRef`: `4066.44MB`.
- `sameAliasContent`: `1817.14MB`.

- [x] **Step 4: Print the profile tops**

Run:

```bash
go tool pprof -top -cum /tmp/nu-bulkbilling.cpu | head -80
go tool pprof -alloc_space -top /tmp/nu-bulkbilling.mem | head -80
go tool pprof -top -cum /tmp/nams-membershipbilling.cpu | head -80
go tool pprof -alloc_space -top /tmp/nams-membershipbilling.mem | head -80
```

Expected: `cloneValuePreserveRefsSeen` and `replaceValueAliasRef` remain first-order costs. If not, update this plan before cutting code.

Result: profile tops printed. `cloneValuePreserveRefsSeen` and `replaceValueAliasRef` remain first-order costs.

Note: the worktree does not contain the large `example-projects` directories. Baselines used the original checkout paths under `/Users/matt/Dev/glade/example-projects`. No symlink. No copy.

---

### Task 2: Add VM Snapshot And Alias Benchmarks

**Files:**
- Modify: `internal/vm/vm_benchmark_test.go`
- Modify: `internal/vm/method_test.go`

- [x] **Step 1: Add a large nested value benchmark**

Add a benchmark near the existing VM benchmarks:

```go
func BenchmarkCloneValuePreserveRefsLargeOrderGraph(b *testing.B) {
	root := Object("OrderGraph")
	lines := List()
	for i := 0; i < 200; i++ {
		line := Object("OrderLine")
		line.Fields["Name"] = String(fmt.Sprintf("line-%d", i))
		line.Fields["Price"] = Decimal(float64(i))
		line.Fields["Children"] = List(Object("Adjustment"), Object("Agreement"))
		lines.List = append(lines.List, line)
	}
	root.Fields["Lines"] = lines

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cloned := cloneValuePreserveRefs(root)
		if cloned.Ref != root.Ref {
			b.Fatalf("clone lost root ref")
		}
	}
}
```

- [x] **Step 2: Add an alias replacement benchmark**

Add. The final benchmark should bury the target alias after many non-matching
nested values so Task 4 measures the broad walk, not a direct child hit:

```go
func BenchmarkReplaceValueAliasLargeOrderGraph(b *testing.B) {
	root := Object("OrderGraph")
	groups := List()
	var previous Value
	var updated Value
	var targetPath string
	for groupIndex := 0; groupIndex < 50; groupIndex++ {
		group := Object("OrderGroup")
		lines := List()
		for lineIndex := 0; lineIndex < 20; lineIndex++ {
			line := Object("OrderLine")
			line.Fields["Name"] = String(fmt.Sprintf("line-%d-%d", groupIndex, lineIndex))
			line.Fields["Meta"] = Map()
			line.Fields["Meta"].Map[mapKey(String("status"))] = String("draft")
			line.Fields["Meta"].MapKeys[mapKey(String("status"))] = String("status")
			lines.List = append(lines.List, line)
		}
		if groupIndex == 49 {
			previous = List(Object("OrderLine"))
			updated = previous
			updated.List = append(updated.List, Object("OrderLine"))
			group.Fields["TargetLines"] = previous
			targetPath = "TargetLines"
		}
		group.Fields["Lines"] = lines
		groups.List = append(groups.List, group)
	}
	root.Fields["Groups"] = groups

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		seen := make(map[uint64]bool)
		replaced, changed := replaceValueAlias(root, previous, updated, seen)
		finalGroup := replaced.Fields["Groups"].List[49]
		if !changed || len(finalGroup.Fields[targetPath].List) != 2 {
			b.Fatalf("alias was not replaced")
		}
	}
}
```

- [x] **Step 3: Add a receiver alias correctness test**

Add:

```go
func TestLargeReceiverMutationPreservesAliases(t *testing.T) {
	run := runApex(t, `
public class LargeReceiverMutation {
  public class Box {
    public List<String> values = new List<String>();
  }
  public static void run() {
    Box first = new Box();
    Box second = first;
    for (Integer i = 0; i < 50; i++) {
      first.values.add('v' + i);
    }
    System.assertEquals(50, second.values.size());
  }
}`)
	if run.Summary().Failed != 0 {
		t.Fatalf("run failed: %#v", run)
	}
}
```

If `runApex` does not exist in the chosen file, use the local helper pattern already present in `internal/vm/*_test.go`. Do not invent a new test harness.

- [x] **Step 4: Run the benchmarks before implementation**

Run:

```bash
go test -run 'TestLargeReceiverMutationPreservesAliases' ./internal/vm
go test -run '^$' -bench 'Benchmark(CloneValuePreserveRefsLargeOrderGraph|ReplaceValueAliasLargeOrderGraph)$' -benchmem ./internal/vm
```

Expected: correctness passes. Benchmarks record the starting allocation and ns/op budget.

Result:

- `go test -run 'TestLargeReceiverMutationPreservesAliases' ./internal/vm`: pass.
- `BenchmarkCloneValuePreserveRefsLargeOrderGraph-12`: `137895 ns/op`, `328064 B/op`, `1604 allocs/op`.
- `BenchmarkReplaceValueAliasLargeOrderGraph-12`: `134244 ns/op`, `74347 B/op`, `20 allocs/op`.

---

### Task 3: Reduce Value Snapshot Allocation

**Files:**
- Modify: `internal/vm/method_dispatch.go`
- Modify: `internal/vm/dml_runtime.go`
- Modify: `internal/vm/lookup_assign.go`
- Modify: `internal/vm/vm_benchmark_test.go`
- Modify: `internal/vm/vm_test.go`

- [x] **Step 1: Classify snapshot call sites**

Inspect these lines and write down which need a deep snapshot and which only need an alias token:

```bash
rg -n "cloneValuePreserveRefs" internal/vm
```

Expected hot call sites:

- `internal/vm/method_dispatch.go` parameter and receiver snapshots.
- `internal/vm/dml_runtime.go` bulk DML snapshots.
- `internal/vm/lookup_assign.go` assignment root snapshots.
- `internal/vm/method_dispatch.go` `SObject.put` snapshots.

Result:

- Alias token: `method_dispatch.go` parameter and receiver snapshots. These
  call sites only need the old ref/kind to propagate a still-aliased object,
  list, set, or map back to caller scope/statics.
- Alias token: `dml_runtime.go` bulk DML list snapshots and per-target DML
  result snapshots. `populateDMLResultFields` mutates known DML targets.
- Alias token: `lookup_assign.go` assignment root snapshots. The assignment
  path has already mutated the root before propagation.
- Alias token: `method_dispatch.go` `SObject.put`, `putSObject`, and framework
  SObjectUnitOfWork relationship snapshots. These paths perform explicit field
  mutation before alias propagation.
- Deep snapshot retained: `cloneValuePreserveRefs` itself, the clone benchmark,
  and the `platform_test.go` copy helper. Production hot call sites no longer
  call the deep snapshot.

- [x] **Step 2: Add a lightweight alias snapshot helper**

Add near `cloneValuePreserveRefs`:

```go
type aliasSnapshot struct {
	ref  uint64
	kind ValueKind
}

func snapshotAlias(value Value) aliasSnapshot {
	return aliasSnapshot{ref: value.Ref, kind: value.Kind}
}

func (s aliasSnapshot) valid() bool {
	return s.ref != 0
}
```

- [x] **Step 3: Add replacement by alias snapshot**

Add:

```go
func replaceAliasSnapshot(value Value, previous aliasSnapshot, updated Value, seen map[uint64]bool) (Value, bool) {
	if !previous.valid() {
		return value, false
	}
	return replaceValueAliasRef(value, previous.ref, previous.kind, updated, seen)
}
```

- [x] **Step 4: Convert one hot path at a time**

Start with DML bulk propagation in `internal/vm/dml_runtime.go`.

Change:

```go
bulkPrevious = cloneValuePreserveRefs(value)
```

to:

```go
bulkPrevious := snapshotAlias(value)
```

Then call `replaceAliasSnapshot` or a scope helper that accepts `aliasSnapshot`.

Result:

- Added `propagateAliasSnapshotToScope` and `propagateAliasSnapshotToStatics`.
- Converted method parameter/receiver, DML bulk/per-target result,
  assignment-root, SObject `put`/`putSObject`, and framework relationship
  propagation call sites.

- [x] **Step 5: Prove after each converted call site**

Run:

```bash
go test ./internal/vm
go test -run '^$' -bench 'Benchmark(CloneValuePreserveRefsLargeOrderGraph|ReplaceValueAliasLargeOrderGraph)$' -benchmem ./internal/vm
```

Expected: tests pass. The clone benchmark may remain unchanged until all sites are converted, but focused slow-method allocation must drop before this task is complete.

Result:

- Added `TestReplaceAliasSnapshotReplacesMatchingRefAndKind`.
- Red check: `go test ./internal/vm -run TestReplaceAliasSnapshotReplacesMatchingRefAndKind` failed to compile before helper implementation with `undefined: snapshotAlias` and `undefined: replaceAliasSnapshot`.
- Green check: `go test ./internal/vm -run TestReplaceAliasSnapshotReplacesMatchingRefAndKind` passed in `0.932s`.
- Required package check: `go test ./internal/vm` failed on nested type resolver tests:
  `TestResolveUniqueNestedTypeNameFromTopLevelCurrentClass` and
  `TestResolveUniqueNestedTypeNameFallsBackToOnlyNestedSuffix`; both resolved
  `Domain` to `""`. These failures are outside the Task 3 alias paths.
- Parent check: the same two tests fail at `c259b464`, before the Task 3
  patch. With those pre-existing failures skipped,
  `go test ./internal/vm -skip 'TestResolveUniqueNestedTypeName(FromTopLevelCurrentClass|FallsBackToOnlyNestedSuffix)'`
  passed in `8.132s`.
- Alias checks: `go test ./internal/vm -run 'TestReplaceAliasSnapshotReplacesMatchingRefAndKind|TestLargeReceiverMutationPreservesAliases' -count=1`
  passed in `0.803s`.
- Code quality fix: added
  `TestAliasSnapshotMutationPropagationSkipsNoopMetadataChange`,
  `TestAliasSnapshotMutationPropagationSkipsNestedNoopMetadataChange`, and
  `TestAliasSnapshotMutationPropagationKeepsRealDataChange` after review found
  that no-op method calls could propagate callee-only metadata. Focused alias
  checks plus `TestExecMethodParameterMapPropagatesNestedCollectionAliases`
  passed. The skipped VM package check passed in `8.060s`.
- Required benchmark command plus the new alias benchmark passed:
  - `BenchmarkCloneValuePreserveRefsLargeOrderGraph-12`: `138235 ns/op`, `328063 B/op`, `1604 allocs/op`.
  - `BenchmarkReplaceValueAliasLargeOrderGraph-12`: `134535 ns/op`, `74349 B/op`, `20 allocs/op`.
  - `BenchmarkReplaceAliasSnapshotLargeOrderGraph-12`: `134619 ns/op`, `74350 B/op`, `20 allocs/op`.

- [x] **Step 6: Re-profile both slow methods**

Run the two Task 1 focused profile commands again.

Expected: `cloneValuePreserveRefsSeen` allocation drops by at least 10x on NAMS or NU. If it drops less, stop and inspect the remaining call site with `go tool pprof -list`.

Result:

- Build: `GOCACHE=/tmp/glade-go-build GOMAXPROCS=4 go build -o /tmp/glade-localtest-current ./cmd/glade` passed.
- NU focused command passed: `total=1 pass=1`, total duration `173212ms`,
  method duration `154649ms`, `run_done totalAllocBytes=15211493400`.
  Allocation pprof total was `14369.20MB`. `cloneValuePreserveRefsSeen` no
  longer appeared in the allocation top. Baseline was `28244.45MB`, so the
  clone allocation drop is greater than 10x on NU. New top allocation:
  `replaceValueAliasRef` `6576.47MB`.
- NU CPU pprof top cumulative: `replaceValueAliasRef` `110.17s` cum,
  `replaceAliasSnapshot` `94.15s` cum,
  `propagateAliasSnapshotToScope` `88.99s` cum.
- NAMS focused command still timed out: `total=1 timeout=1`, total duration
  `309002ms`, method duration `300245ms`, top frame
  `namz.PriceableOrder.createProductPricesMap`, `run_done totalAllocBytes=25900183960`.
  Allocation pprof total was `24608.78MB`. `cloneValuePreserveRefsSeen` no
  longer appeared in the allocation top. Baseline was `123087.02MB`, so the
  clone allocation drop is greater than 10x on NAMS. New top allocation:
  `replaceValueAliasRef` `15800.65MB`.
- NAMS CPU pprof top cumulative: `replaceValueAliasRef` `222.63s` cum,
  `replaceAliasSnapshot` `194.14s` cum,
  `propagateAliasSnapshotToScope` `172.65s` cum.
- Remaining hot call site from `go tool pprof -alloc_space -list replaceValueAliasRef /tmp/nams-membershipbilling.after-task3.mem`:
  line `3227` map insert into `seen` allocated `15.43GB`; recursive walks were
  `15.16GB` through object fields, `13.58GB` through list elements, and
  `4.40GB` through map values. This is Task 4 territory.

---

### Task 4: Reduce Broad Alias Replacement Walks

**Status:** Partially done. Item 4a (scope-indexed approach) was in the original plan but not yet implemented.

**Updated evidence (2026-05-26):** `replaceValueAliasRef` is still the #1 hot spot with 14.77s in Object Fields recursive walk and 5.85s in `seen` map insertion. `propagateUpdatedValueAliases` allocates 681–1080MB in NU methods.

#### 4a. Skip `propagateUpdatedValueAliases` when no scope variables alias the updated value

**Current behavior:** `propagateUpdatedValueAliases` builds a `topLevelAliases` map by scanning ALL scope variables, then recursively walks the entire updated value tree. This always allocates two maps (`topLevelAliases` and `seen`).

**Proposed change:** Add an early-return gate. If the updated value has `Ref == 0` (it's a leaf/scalar), return immediately — leaf values can never be aliased. Additionally, if no scope variable's `Ref` matches the updated value's `Ref` and none of its nested refs, skip the walk entirely.

**Estimated impact:** Eliminates 100% of `propagateUpdatedValueAliases` allocation for scalar mutations (common case: string/int/boolean assignments). For collection/object mutations where no scope alias exists, eliminates the recursive walk. This could save 680–1080MB in NU methods.

**Correctness risk:** Medium. Must handle the case where a Value has `Ref == 0` but contains nested values with refs (e.g., a newly-created list of objects). In that case, check for nested refs before skipping.

#### 4b. Avoid `seen` map allocation in `replaceValueAliasRef` for shallow walks

**Current behavior:** Every call to `replaceValueAliasRef` allocates a fresh `seen` map. The map insert at line 3245 accounts for 5.85s cum in sf-cred-2. For scope-variable-level replacements (where the previous ref is at the root of a scope variable), the walk is inherently shallow (depth ≤ 2).

**Proposed change:** Use a small stack-allocated array (e.g., `[8]uint64`) for ref tracking when depth is below a threshold. Fall back to heap `map[uint64]bool` only when the array overflows.

**Estimated impact:** Eliminates the 5.85s of `seen` map allocation overhead. Combined with the `cloneValuePreserveRefsSeen` elimination from Task 3, this could push `replaceValueAliasRef` below 5s cum in sf-cred and ~20–30s cum in NU.

**Correctness risk:** Low. The ref-tracking is strictly for cycle detection. A bounded array with fallback preserves correctness.

#### 4c. Skip `sameAliasRuntimeContent` when refs differ at the top level

**Current behavior:** `sameAliasRuntimeContent` recursively compares two Value trees to decide if alias propagation is needed. In NU-3, this allocates 1537MB (15.3%). For bulk DML and trigger-heavy methods, the two trees being compared can be very large (deeply nested SObjects with many fields/child records).

**Proposed change:** Add a shallow-compare gate before the deep recursive walk. If the top-level refs differ and the values are not collections, return `false` immediately. For collections, compare `len(List/Set/Map/Fields)` first — if lengths differ, return `false` without recursing.

**Estimated impact:** Significant for NU methods where trigger chains produce large Value trees. The shallow gate could eliminate 50–80% of deep comparisons where the trees differ early.

**Correctness risk:** Low. This is purely an optimization of the comparison logic; the result must be identical.

---

### Task 5 (NEW): Reduce `collectStaticFieldValueRefs` Per-Field Overhead

**Evidence:** NU-4 shows `collectStaticFieldValueRefs` at 34.11s cum (72.6% of total CPU) and 1823MB alloc (29.4%). This is called from `rememberStaticValueRefsInField` for **every field** mutated in a DML/trigger cycle.

**Root cause:** When a trigger modifies multiple fields on an SObject (e.g., `record.put('Field1', v1); record.put('Field2', v2);`), each `put` call invokes `rememberStaticValueRefsInField`, which calls `collectStaticFieldValueRefs`, which recursively walks the entire Value tree of that field. For an SObject with 50 fields and 10 mutated fields, this means 10 full walks of the SObject tree.

**Proposed change:** Batch field ref collection. Instead of calling `collectStaticFieldValueRefs` per field, mark the static value refs as dirty and defer collection until the end of the method call or DML cycle. Use a `staticValueRefsDirty bool` flag and a list of pending field locations. On next access, walk once.

**Estimated impact:** Could reduce `collectStaticFieldValueRefs` CPU by 5–10× in trigger-heavy methods like NU-4. The allocation would drop proportionally.

**Correctness risk:** High. The static value ref index must be consistent at all points where `propagateValueMutationToStatics` or `staticValueRefs` are read. A dirty flag must be cleared (rebuilt) before any read. The trigger/DML lifecycle must be carefully audited to ensure no read-before-rebuild gaps.

---

### Task 6 (NEW): Avoid `cloneDescribeObjectDefinition` Cloning During Execution

**Evidence:** `cloneDescribeObjectDefinition` allocates 211–447MB across all methods. This is a stable 2–5% cost that appears during load/discovery but also during execution (SOQL describe, DML describe).

**Proposed change:** Cache cloned describe definitions per object type. The describe data is read-only during execution; cloning it repeatedly is wasted work. Use a `sync.Map` or a simple `map[string]*DescribeObjectDefinition` guarded by the VM's existing mutex pattern.

**Estimated impact:** 200–400MB allocation reduction per slow method. Small but consistent win across all workloads.

**Correctness risk:** Low. Describe definitions are immutable during execution. The cache must be per-VM (not global) to preserve test isolation.

---

### Task 7: Split Long Classes Without Raising Parallelism

**Files:**
- Modify: `internal/apextest/runner.go`
- Modify: `internal/apextest/runner_test.go`
- Modify: `internal/compat/local_tests.go`
- Modify: `internal/compat/local_tests_test.go`

- [ ] **Step 1: Add a scheduler test for one long class**

Create a test that builds these planned classes:

- `LongClass` with 12 methods.
- `ShortA`, `ShortB`, `ShortC`, `ShortD` with 1 method each.

Give `ClassDurationMS["LongClass"]` a large value.

Expected: with `Parallelism: 4`, idle workers can take `LongClass` methods after setup is complete.

- [ ] **Step 2: Keep setup execution once per class**

In `runTestPlansWithSetups`, preserve this rule:

```go
setupOrg, setupRandom, setupErr, setupShared := prepareTestSetupOrg(...)
```

Only one worker may prepare setup for a class. Method workers consume method jobs after that setup is ready.

- [ ] **Step 3: Add a method work queue for long classes**

Add an internal queue type:

```go
type classMethodJob struct {
	className string
	index     int
	setupOrg storage.OrgState
	setupErr error
	setupRandom uint64
	setupShared bool
}
```

Use it only when:

- `opts.ParallelMethods` is true, or
- `opts.ClassDurationMS[className]` is above a conservative threshold and `len(classIndexes[className]) >= 24`.

- [ ] **Step 4: Preserve isolation**

Every method job must still call `runCase` with an isolated org. For parallel methods, use clone or copy-on-write snapshot. Do not share a journal across parallel methods.

- [ ] **Step 5: Validate with focused class runs**

Run:

```bash
/tmp/glade-localtest-current compat local-tests \
  --project example-projects/nams-workspace \
  --class MembershipBillingSuite \
  --parallel 4 \
  --parallel-methods \
  --timeout 300000 \
  --json > /tmp/nams-membershipbilling-class-parallel.json
```

Expected: no new leakage failures. Wall time should be lower than serial method execution when more than one slow method exists.

---

### Task 8: Use Copy-On-Write Or Journal Isolation For Safe Method Runs

**Files:**
- Modify: `internal/apextest/runner.go`
- Modify: `internal/apextest/isolation_journal_test.go`
- Modify: `internal/storage/snapshot.go`
- Modify: `internal/storage/snapshot_test.go`
- Modify: `internal/dml/*.go` only if a mutation path bypasses `EnsureMutableObjectRecords`.

- [ ] **Step 1: Add a copy-on-write method isolation test**

Add a test where the first test method mutates records, deletes records, inserts records, mutates ID sequences, and enqueues async work. The second method must see only setup data.

Expected Apex shape:

```apex
@IsTest
private class CopyOnWriteIsolationTest {
  @TestSetup static void setup() {
    insert new Account(Name = 'Fixture');
  }
  @IsTest static void firstMethod() {
    Account row = [SELECT Id, Name FROM Account LIMIT 1];
    row.Name = 'Changed';
    update row;
    insert new Account(Name = 'Extra');
  }
  @IsTest static void secondMethod() {
    System.assertEquals(1, [SELECT count() FROM Account]);
    System.assertEquals('Fixture', [SELECT Name FROM Account LIMIT 1].Name);
  }
}
```

- [ ] **Step 2: Prove current clone count**

Run:

```bash
go test -run 'TestRunSequentialMethodsIsolatesSetupOrgWithClones|Test.*Isolation' ./internal/apextest
```

Expected: current tests pass. Clone counters show per-method clone fallback where still expected.

- [ ] **Step 3: Replace deep clone with `storage.SnapshotRuntimeOrg` where safe**

Change only the sequential same-class path first. Use copy-on-write snapshots for each method and keep deep clone for parallel method runs until proven safe.

Expected code shape:

```go
methodOrg := storage.SnapshotRuntimeOrg(&setupOrg)
results[i] = runCase(..., methodOrg, ..., false, nil)
```

If this mutates `setupOrg` through shared markers, restore or re-create a clean setup snapshot before the next method.

- [ ] **Step 4: Keep journal rollback as a separate lane**

Use `storage.NewIsolationJournal`, `Mark`, and `Rollback` only when the runner can prove all mutation surfaces record before-values. Do not combine journal widening with copy-on-write widening in the same commit.

- [ ] **Step 5: Validate mutation surfaces**

Run:

```bash
go test ./internal/apextest ./internal/storage ./internal/dml
```

Expected: all pass. Any bypass of `EnsureMutableObjectRecords` must be fixed before focused project profiling.

---

### Task 9: Read Duration History From Current Artifacts

**Files:**
- Modify: `internal/compat/local_test_shards.go`
- Modify: `internal/compat/local_test_shards_test.go`

- [ ] **Step 1: Add a failing test for `outcomes[]` history**

Add:

```go
func TestLoadLocalTestDurationHistoryReadsOutcomes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	data := `{
	  "outcomes": [
	    {"class":"SlowClass","method":"a","durationMs":1000},
	    {"class":"SlowClass","method":"b","durationMs":2000},
	    {"class":"FastClass","method":"a","durationMs":10}
	  ]
	}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	durations, err := loadLocalTestDurationHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if durations["SlowClass"] != 3000 {
		t.Fatalf("SlowClass duration = %d, want 3000", durations["SlowClass"])
	}
}
```

- [ ] **Step 2: Extend the parser**

Keep `topSlowClasses` support. Add `outcomes` support:

```go
var perf struct {
	TopSlowClasses []LocalTestPerfClass `json:"topSlowClasses"`
	Outcomes []struct {
		Class string `json:"class"`
		DurationMS int64 `json:"durationMs"`
	} `json:"outcomes"`
}
```

Sum `durationMs` by class when `topSlowClasses` is empty or incomplete.

- [ ] **Step 3: Validate sharding logic**

Run:

```bash
go test ./internal/compat -run 'TestLoadLocalTestDurationHistoryReadsOutcomes|TestPlanLocalTestClassShards'
```

Expected: duration history can use `nu.json` and `nams.json` as they exist today.

---

### Task 10: Focused Validation And Sentinel Proof

**Files:**
- No new files unless tests require updates.

- [ ] **Step 1: Re-run focused slow methods**

Run:

```bash
/tmp/glade-localtest-current compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --class BulkBillingTest \
  --method batchBulkBilling_membershipMatchingCriteria_expectBatchExecutionSuccessful \
  --timeout 300000 \
  --perf-json /tmp/nu-bulkbilling.after.perf.json \
  --json > /tmp/nu-bulkbilling.after.json
```

```bash
/tmp/glade-localtest-current compat local-tests \
  --project example-projects/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200importedMemberships \
  --timeout 300000 \
  --perf-json /tmp/nams-membershipbilling.after.perf.json \
  --json > /tmp/nams-membershipbilling.after.json
```

Expected:

- NU method stays pass.
- NAMS method no longer times out, or its top profile proves the next hot path.
- Allocation from `cloneValuePreserveRefsSeen` drops by at least 10x after Tasks 3 and 4.

- [ ] **Step 2: Run slow classes only**

Use classes from `nams.json` and `nu.json`, not a full gate:

```bash
/tmp/glade-localtest-current compat local-tests \
  --project example-projects/nams-workspace \
  --class-list ProFormaOrderServiceTest,MembershipBillingSuite,GenerateHistoriesCallbackTest \
  --parallel 4 \
  --timeout 300000 \
  --json > /tmp/nams-slow-classes.after.json
```

```bash
/tmp/glade-localtest-current compat local-tests \
  --project example-projects/src-nmb-nu-develop \
  --class-list BulkBillingTest,TestAffiliationTriggers,AffiliationTriggerHandlers2Test,TestAccountTrigger \
  --parallel 4 \
  --timeout 300000 \
  --json > /tmp/nu-slow-classes.after.json
```

Expected: no new correctness failures from scheduling or isolation.

- [ ] **Step 3: Run smaller green sentinels**

Run:

```bash
/tmp/glade-localtest-current compat local-tests \
  --project example-projects/src-nmb-nutpl-develop \
  --parallel 4 \
  --timeout 60000 \
  --json > /tmp/nutpl.after.json
```

```bash
/tmp/glade-localtest-current compat local-tests \
  --project example-projects/sf-cred-pkg-develop \
  --parallel 4 \
  --timeout 60000 \
  --json > /tmp/sf-cred.after.json
```

Expected: both sentinels remain green. If they do not, stop and fix correctness before measuring speed.

- [ ] **Step 4: Run package tests**

Run:

```bash
go test ./internal/vm ./internal/apextest ./internal/storage ./internal/dml ./internal/compat
```

Expected: all pass.

---

## Risk Ledger

- **High correctness risk:** replacing deep value snapshots with alias tokens. Apex pass-by-reference behavior must still hold for lists, maps, sets, SObjects, method params, receivers, and DML result field writeback.
- **High correctness risk:** deferring `collectStaticFieldValueRefs` with a dirty flag (Task 5). The static value ref index must be consistent at all read points. Any gap between a field mutation and index rebuild could cause missed alias propagation.
- **Medium correctness risk:** method splitting within a class. Setup data, static state, async jobs, mocks, and limits must remain method-isolated.
- **Medium correctness risk:** copy-on-write test org isolation. Every write path must call `EnsureMutableObjectRecords` or equivalent before mutation.
- **Medium correctness risk:** skipping `propagateUpdatedValueAliases` when no scope aliases exist (Task 4a). Must correctly handle nested refs within newly-created values.
- **Low correctness risk:** stack-allocated ref array in `replaceValueAliasRef` (Task 4b). The ref-tracking is strictly for cycle detection; bounded array with fallback preserves correctness.
- **Low correctness risk:** shallow-compare gate in `sameAliasRuntimeContent` (Task 4c). The comparison result must be identical; the optimization is purely structural.
- **Low correctness risk:** caching `cloneDescribeObjectDefinition` (Task 6). Describe data is immutable during execution; cache must be per-VM for test isolation.
- **Low correctness risk:** duration history loader for `outcomes[]`. It only affects ordering and sharding.

## Done Criteria

- [ ] P0 (Task 4a–4c): `replaceValueAliasRef` allocation drops by 2–4×; `sameAliasRuntimeContent` allocation drops proportionally.
- [ ] P1 (Task 5): `collectStaticFieldValueRefs` CPU in NU-4 drops below 10s cum (from 34s).
- [ ] P2 (Task 6): `cloneDescribeObjectDefinition` allocation drops by 80%+ during method execution.
- [ ] Focused NAMS method no longer spends most time in `cloneValuePreserveRefsSeen` (done in Task 3).
- [ ] Focused NU method stays green and materially faster.
- [ ] Slow-class focused runs show no new correctness failures.
- [ ] `src-nmb-nutpl-develop` and `sf-cred-pkg-develop` sentinels remain green with `--parallel 4`.
- [ ] Package tests pass for `internal/vm`, `internal/apextest`, `internal/storage`, `internal/dml`, and `internal/compat`.
- [ ] No project-specific behavior was added.
- [ ] No proprietary implementation source was used.

## Priority Summary (Updated 2026-05-26)

| Priority | Task | Description | Est. Impact | Risk |
|----------|------|-------------|-------------|------|
| **P0** | 4a | Skip `propagateUpdatedValueAliases` when no scope aliases exist | 680–1080MB alloc cut in NU | Medium |
| **P0** | 4b | Stack-allocated ref array in `replaceValueAliasRef` | 5.85s CPU + alloc cut | Low |
| **P0** | 4c | Shallow-compare gate in `sameAliasRuntimeContent` | 500–1000MB alloc cut in NU | Low |
| **P1** | 5 | Defer `collectStaticFieldValueRefs` via dirty flag | 5–10× CPU cut in NU-4 | High |
| **P2** | 6 | Cache `cloneDescribeObjectDefinition` | 200–400MB alloc cut | Low |
| **P3** | 7 | Split long classes across workers | Wall-time reduction for large classes | Medium |
| **P3** | 8 | Copy-on-write org isolation | 2–5× clone overhead reduction | Medium |
| **P4** | 9 | Duration history from outcomes[] | Better scheduling | Low |

# Glade Local Test Big Wins Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut large `glade compat local-tests` wall time by attacking the proven hot paths in VM value snapshots, alias propagation, long-class scheduling, and test isolation.

**Architecture:** Use narrow slow-method profiles to prove each change before any broad run. Keep behavior generic to Apex local test execution. Preserve test isolation by proving DML, static state, limits, async work, mocks, setup data, and rollback behavior before widening.

**Tech Stack:** Go 1.26, `go test`, `runtime/pprof`, `glade compat local-tests`, existing `internal/vm`, `internal/apextest`, `internal/storage`, and `internal/compat` packages.

---

## Progress Ledger

- [ ] Task 1: Record focused baselines from the current slow methods.
- [ ] Task 2: Add VM value snapshot and alias propagation benchmarks.
- [ ] Task 3: Reduce `cloneValuePreserveRefsSeen` allocation pressure.
- [ ] Task 4: Reduce broad alias replacement walks.
- [ ] Task 5: Split long classes across the fixed worker budget.
- [ ] Task 6: Replace safe per-method org deep clones with copy-on-write snapshots or journal rollback.
- [ ] Task 7: Teach duration history to read current `outcomes[]` artifacts.
- [ ] Task 8: Validate focused targets and green sentinels.

## Current Evidence

Do not start with a full NU or NAMS run.

Use the saved artifacts first:

- `nu.json`
- `nams.json`
- `nutpl.json`
- `sf-cred.json`

Current narrow profile targets:

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

Baseline facts from the investigation:

- NAMS `MembershipBillingSuite.simplestRenewal_bulk_200importedMemberships` timed out at `300009ms`.
- That NAMS profile allocated about `193.6GB`; `175.8GB` was in `internal/vm.cloneValuePreserveRefsSeen`.
- NAMS CPU showed `cloneValuePreserveRefsSeen` at about `282.96s` cumulative and `replaceValueAliasRef` at about `250.09s` cumulative.
- NU `BulkBillingTest.batchBulkBilling_membershipMatchingCriteria_expectBatchExecutionSuccessful` passed in `118722ms`.
- That NU profile allocated about `40.9GB`; `28.5GB` was in `internal/vm.cloneValuePreserveRefsSeen`.
- Load and discover were small beside method execution.

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

- [ ] **Step 1: Build one profiling binary**

Run:

```bash
GOCACHE=/tmp/glade-go-build GOMAXPROCS=4 go build -o /tmp/glade-localtest-current ./cmd/glade
```

Expected: exit code `0`.

- [ ] **Step 2: Refresh the NU focused profile**

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

- [ ] **Step 3: Refresh the NAMS focused profile**

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

- [ ] **Step 4: Print the profile tops**

Run:

```bash
go tool pprof -top -cum /tmp/nu-bulkbilling.cpu | head -80
go tool pprof -alloc_space -top /tmp/nu-bulkbilling.mem | head -80
go tool pprof -top -cum /tmp/nams-membershipbilling.cpu | head -80
go tool pprof -alloc_space -top /tmp/nams-membershipbilling.mem | head -80
```

Expected: `cloneValuePreserveRefsSeen` and `replaceValueAliasRef` remain first-order costs. If not, update this plan before cutting code.

---

### Task 2: Add VM Snapshot And Alias Benchmarks

**Files:**
- Modify: `internal/vm/vm_benchmark_test.go`
- Modify: `internal/vm/method_test.go`

- [ ] **Step 1: Add a large nested value benchmark**

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

- [ ] **Step 2: Add an alias replacement benchmark**

Add:

```go
func BenchmarkReplaceValueAliasLargeOrderGraph(b *testing.B) {
	root := Object("OrderGraph")
	previous := List()
	for i := 0; i < 200; i++ {
		previous.List = append(previous.List, Object("OrderLine"))
	}
	updated := previous
	updated.List = append(updated.List, Object("OrderLine"))
	root.Fields["Lines"] = previous

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		seen := make(map[uint64]bool)
		replaced, changed := replaceValueAlias(root, previous, updated, seen)
		if !changed || len(replaced.Fields["Lines"].List) != 201 {
			b.Fatalf("alias was not replaced")
		}
	}
}
```

- [ ] **Step 3: Add a receiver alias correctness test**

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

- [ ] **Step 4: Run the benchmarks before implementation**

Run:

```bash
go test -run 'TestLargeReceiverMutationPreservesAliases' ./internal/vm
go test -run '^$' -bench 'Benchmark(CloneValuePreserveRefsLargeOrderGraph|ReplaceValueAliasLargeOrderGraph)$' -benchmem ./internal/vm
```

Expected: correctness passes. Benchmarks record the starting allocation and ns/op budget.

---

### Task 3: Reduce Value Snapshot Allocation

**Files:**
- Modify: `internal/vm/method_dispatch.go`
- Modify: `internal/vm/dml_runtime.go`
- Modify: `internal/vm/lookup_assign.go`
- Modify: `internal/vm/vm_benchmark_test.go`
- Modify: `internal/vm/method_test.go`

- [ ] **Step 1: Classify snapshot call sites**

Inspect these lines and write down which need a deep snapshot and which only need an alias token:

```bash
rg -n "cloneValuePreserveRefs" internal/vm
```

Expected hot call sites:

- `internal/vm/method_dispatch.go` parameter and receiver snapshots.
- `internal/vm/dml_runtime.go` bulk DML snapshots.
- `internal/vm/lookup_assign.go` assignment root snapshots.
- `internal/vm/method_dispatch.go` `SObject.put` snapshots.

- [ ] **Step 2: Add a lightweight alias snapshot helper**

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

- [ ] **Step 3: Add replacement by alias snapshot**

Add:

```go
func replaceAliasSnapshot(value Value, previous aliasSnapshot, updated Value, seen map[uint64]bool) (Value, bool) {
	if !previous.valid() {
		return value, false
	}
	return replaceValueAliasRef(value, previous.ref, previous.kind, updated, seen)
}
```

- [ ] **Step 4: Convert one hot path at a time**

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

- [ ] **Step 5: Prove after each converted call site**

Run:

```bash
go test ./internal/vm
go test -run '^$' -bench 'Benchmark(CloneValuePreserveRefsLargeOrderGraph|ReplaceValueAliasLargeOrderGraph)$' -benchmem ./internal/vm
```

Expected: tests pass. The clone benchmark may remain unchanged until all sites are converted, but focused slow-method allocation must drop before this task is complete.

- [ ] **Step 6: Re-profile both slow methods**

Run the two Task 1 focused profile commands again.

Expected: `cloneValuePreserveRefsSeen` allocation drops by at least 10x on NAMS or NU. If it drops less, stop and inspect the remaining call site with `go tool pprof -list`.

---

### Task 4: Reduce Broad Alias Replacement Walks

**Files:**
- Modify: `internal/vm/method_dispatch.go`
- Modify: `internal/vm/runtime_state.go`
- Modify: `internal/vm/vm_benchmark_test.go`
- Modify: `internal/vm/method_test.go`

- [ ] **Step 1: Add a runtime alias index shape**

Add fields to `VM`:

```go
valueRefRoots map[uint64][]string
valueRefDirty bool
```

Use this index only for top-level `Globals` at first. Do not index every nested value until profiles prove it.

- [ ] **Step 2: Rebuild the top-level ref index lazily**

Add:

```go
func (vm *VM) rebuildValueRefRoots() {
	vm.valueRefRoots = make(map[uint64][]string, len(vm.Globals))
	for name, value := range vm.Globals {
		if value.Ref != 0 {
			vm.valueRefRoots[value.Ref] = append(vm.valueRefRoots[value.Ref], name)
		}
	}
	vm.valueRefDirty = false
}
```

- [ ] **Step 3: Use the index in `propagateValueMutationToScope`**

For `vm.Globals`, replace the full `for name, value := range scope` scan with indexed roots when the previous value has a ref. Keep the old full scan for non-global scopes.

Expected behavior:

- If no indexed roots reference the previous ref, return without walking every global.
- If indexed roots exist, replace only those roots.
- Preserve static propagation through the existing static field ref index.

- [ ] **Step 4: Mark the index dirty on global writes**

Update `storeReceiver`, assignment, method frame return writeback, and static/global mutation points that write top-level globals.

Use a small helper:

```go
func (vm *VM) setGlobal(name string, value Value) {
	vm.Globals[name] = value
	vm.valueRefDirty = true
}
```

Do not do a broad mechanical rewrite. Convert only writes that participate in local variable and receiver mutation paths.

- [ ] **Step 5: Run correctness and benchmarks**

Run:

```bash
go test ./internal/vm
go test -run '^$' -bench 'BenchmarkReplaceValueAliasLargeOrderGraph$' -benchmem ./internal/vm
```

Expected: alias correctness holds. `replaceValueAliasRef` cumulative CPU drops on both focused profiles.

---

### Task 5: Split Long Classes Without Raising Parallelism

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

### Task 6: Use Copy-On-Write Or Journal Isolation For Safe Method Runs

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

### Task 7: Read Duration History From Current Artifacts

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

### Task 8: Focused Validation And Sentinel Proof

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
- **Medium correctness risk:** method splitting within a class. Setup data, static state, async jobs, mocks, and limits must remain method-isolated.
- **Medium correctness risk:** copy-on-write test org isolation. Every write path must call `EnsureMutableObjectRecords` or equivalent before mutation.
- **Low correctness risk:** duration history loader for `outcomes[]`. It only affects ordering and sharding.

## Done Criteria

- [ ] Focused NAMS method no longer spends most time in `cloneValuePreserveRefsSeen`.
- [ ] Focused NU method stays green and materially faster.
- [ ] Slow-class focused runs show no new correctness failures.
- [ ] `src-nmb-nutpl-develop` and `sf-cred-pkg-develop` sentinels remain green with `--parallel 4`.
- [ ] Package tests pass for `internal/vm`, `internal/apextest`, `internal/storage`, `internal/dml`, and `internal/compat`.
- [ ] No project-specific behavior was added.
- [ ] No proprietary implementation source was used.

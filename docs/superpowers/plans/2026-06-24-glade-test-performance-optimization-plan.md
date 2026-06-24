# Glade Test Performance Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce DML-heavy focused test time and large-suite wall time with measured proof, while preserving Salesforce Apex, DML, SOQL, trigger, automation, rollback, and test-isolation behavior.

**Architecture:** Add low-overhead performance counters first, then optimize only the paths proven hot in real NAMS and corpus runs. The first implementation lead is temporary DML rollback journaling for bulk `allOrNone` operations that currently take full org snapshots in focused one-method runs. Suite-scale work improves duration history and scheduling without changing test semantics. VM static-alias work remains behind a proof gate and must fail open to the current recursive walk.

**Tech Stack:** Go, `internal/vm`, `internal/dml`, `internal/storage`, `internal/apextest`, `internal/gladecli`, `glade test`, `--perf-json`, pprof, external corpus projects under `/Users/matt/Dev/glade-corpus/private`.

---

## Evidence Already On Disk

The current handoff at `docs/superpowers/plans/2026-06-24-nams-membership-billing-performance-handoff.md` is the controlling prior report.

It found:

- `MembershipBillingSuite.simplestRenewal_bulk_200importedMemberships` passed in `388.89s` wall, with method duration `361115ms`.
- `MembershipBillingSuite.simplestRenewal_bulk_200purchasedMemberships` hit `context deadline exceeded` at `487.35s`.
- Startup and compile were not the issue.
- VM call/eval/execute dominated CPU.
- `propagateAliasSnapshotToStatics` cost about `103s` cumulative.
- `replaceAliasSnapshot` / `replaceValueAliasRef` cost about `93.5s` cumulative.
- Exact static alias paths, append-tail indexing, and direct-child pre-scan all failed the real NAMS timing gate.

Current saved perf JSON under `/tmp/glade-alias-path` confirms the same shape:

```text
baseline imported:
  durationMs 386882
  compileMs 9641
  apexPerf.runDurationMs 361115
  cloneRuntimeOrgCalls 1
  cloneRuntimeOrgMs 8
  cloneRuntimeMachineMs 295

baseline purchased:
  durationMs 487115
  compileMs 2311
  apexPerf.runDurationMs 481110
  error context deadline exceeded
```

Saved full-suite reports under `test-results/` show the large-run shape:

```text
nams.json       5723 tests   2918521ms
project-b.json 11526 tests  11587827ms
nutpl.json       761 tests     79169ms
project-c.json  4565 tests   2097744ms
```

Top full-suite classes from those reports:

```text
NAMS      MembershipBillingSuite       33 tests  549392ms
Project B TestOrderPaymentController  137 tests  355982ms
Project B CartSubmitterTest           163 tests  327420ms
Project C tst_dataMapper                4 tests   72106ms
```

The code already has good pieces:

- `internal/apextest/runner.go` compiles project and tests once per run, freezes class lookup, and reuses runtime cache.
- `internal/apextest/runner.go` can use `storage.IsolationJournal` for sequential methods in the same class.
- `internal/apextest/runner.go` can fan out methods inside one class when `ParallelMethods` is enabled.
- `internal/gladecli/test_command.go` supports `--parallelism`, `--duration-history`, `--write-class-shards`, `--perf-json`, CPU profiles, and memory profiles.
- `internal/vm/value_aliasing.go` keeps a coarse static ref index and falls back to recursive replacement for correctness.

The gaps are also clear:

- Perf JSON does not expose rollback snapshot counts.
- Perf JSON does not expose VM alias counters or DML phase counters.
- Focused one-method runs have no per-test journal, so bulk `allOrNone` DML rollback points fall back to full org snapshots.
- Duration history only feeds class-level scheduling and currently loads top slow classes, not all classes and not methods.
- A long class can start early with method parallelism `1`; idle workers cannot help it later because `methodParallelismForClassRun` is computed once before the class method loop starts.

## Non-Negotiable Rules

- Do not add NAMS-specific, NU-specific, package-specific, field-name-specific, or class-name-specific product code.
- Do not remove Salesforce-visible rollback behavior.
- Do not skip trigger, workflow, flow, summary, validation, mixed-DML, user-mode, async, or static-alias behavior for speed.
- Do not merge a synthetic benchmark win until the focused NAMS imported method improves on a same-load forced-build timing.
- Keep stale-index and full-walk fallbacks in VM alias propagation.
- Preserve `--no-parallel-methods`.
- Treat full external gates as the truth after focused proof.

## File Map

Modify:

- `internal/apextest/perf_counters.go`
  - Add storage clone stats and VM perf counters to the run snapshot.
  - Reset storage and VM counters with existing Apex test counters.
- `internal/apextest/runner.go`
  - Reset counters at run start.
  - Add method-duration history to options.
  - Sort method jobs by method duration when history exists.
  - Later: support adaptive method-worker borrowing.
- `internal/gladecli/test_command.go`
  - Add complete class and method duration maps to `--perf-json`.
  - Load class and method durations from existing perf JSON history.
  - Pass method durations to `apextest.Options`.
- `internal/vm/dml_runtime.go`
  - Add temporary DML rollback journals for rollback points when no class/test journal exists.
  - Keep `forceSnapshot` as an escape hatch.
  - Add lifecycle cleanup for temporary journals.
- `internal/vm/runtime_state.go`
  - Expose safe helpers for setting and restoring temporary journals only if needed.
- `internal/vm/alias_perf.go`
  - Create a small env/perf-gated VM counter file.
- `internal/vm/value_aliasing.go`
  - Record alias propagation counters only when VM perf counters are enabled.
  - Later: implement only the proven alias optimization.
- `internal/storage/clone_stats.go`
  - Reuse current `CloneStats`; no new storage behavior should be needed.

Test:

- `internal/apextest/perf_counters_test.go`
- `internal/apextest/runner_test.go`
- `internal/apextest/dispatcher_test.go`
- `internal/gladecli/test_command_selectors_test.go`
- `internal/vm/data_test.go`
- `internal/vm/method_test.go`
- `internal/vm/vm_benchmark_test.go`

## Task 1: Baseline Pack And Counter Surface

**Files:**

- Modify: `internal/apextest/perf_counters.go`
- Modify: `internal/apextest/runner.go`
- Modify: `internal/gladecli/test_command.go`
- Create: `internal/vm/alias_perf.go`
- Test: `internal/apextest/perf_counters_test.go`
- Test: `internal/gladecli/test_command_selectors_test.go`

- [ ] **Step 1: Build a forced baseline binary from current `main`**

Run:

```bash
mkdir -p /tmp/glade-test-perf
GOCACHE=/tmp/glade-go-build-test-perf-baseline \
GOMAXPROCS=4 \
go build -a -o /tmp/glade-test-perf/glade-baseline ./cmd/glade
```

Expected: exit code `0`.

- [ ] **Step 2: Capture focused baseline timings without new instrumentation**

Run:

```bash
/usr/bin/time -p /tmp/glade-test-perf/glade-baseline test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200importedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-test-perf/baseline-imported.perf.json \
  > /tmp/glade-test-perf/baseline-imported.result.json \
  2> /tmp/glade-test-perf/baseline-imported.stderr.log
```

Expected:

```bash
jq '.summary' /tmp/glade-test-perf/baseline-imported.result.json
```

shows `failed: 0`, `runtimeErrors: 0`, and `errors: 0`.

- [ ] **Step 3: Add storage clone stats to Apex perf counters**

Change `internal/apextest/perf_counters.go` so `PerfCounters` includes:

```go
StorageCloneStats storage.CloneStats `json:"storageCloneStats"`
```

Import `github.com/glade-sh/glade/internal/storage`.

Change `ResetPerfCounters`:

```go
func ResetPerfCounters() {
    storage.ResetCloneStats()
    vm.ResetPerfCounters()
    // existing resets stay below this line.
}
```

Change `SnapshotPerfCounters`:

```go
out := PerfCounters{
    CloneRuntimeOrgCalls:  perfCounters.cloneRuntimeOrg.Load(),
    JournalRollbacks:      perfCounters.journalRollbacks.Load(),
    CloneFallbacks:        perfCounters.cloneFallbacks.Load(),
    SetupDurationMS:       perfCounters.setupDurationMS.Load(),
    RunDurationMS:         perfCounters.runDurationMS.Load(),
    CloneRuntimeOrgMS:     perfCounters.cloneRuntimeOrgMS.Load(),
    CloneRuntimeMachineMS: perfCounters.cloneRuntimeMachineMS.Load(),
    StorageCloneStats:     storage.SnapshotCloneStats(),
    VMPerf:                vm.SnapshotPerfCounters(),
}
```

Add the second field:

```go
VMPerf vm.PerfCounters `json:"vmPerf,omitempty"`
```

- [ ] **Step 4: Add VM perf counters behind an explicit runtime flag**

Create `internal/vm/alias_perf.go`:

```go
package vm

import (
    "sort"
    "sync"
    "sync/atomic"
    "time"
)

type PerfCounters struct {
    Enabled                  bool                   `json:"enabled,omitempty"`
    StaticAlias              StaticAliasPerfCounters `json:"staticAlias,omitempty"`
    StaticAliasTopFields     []StaticAliasFieldPerf  `json:"staticAliasTopFields,omitempty"`
    DML                      DMLPerfCounters         `json:"dml,omitempty"`
}

type StaticAliasPerfCounters struct {
    Calls          uint64 `json:"calls,omitempty"`
    RefMisses      uint64 `json:"refMisses,omitempty"`
    LocationVisits uint64 `json:"locationVisits,omitempty"`
    Changed        uint64 `json:"changed,omitempty"`
    NoChange       uint64 `json:"noChange,omitempty"`
    DurationNS     int64  `json:"durationNs,omitempty"`
    CollectCalls   uint64 `json:"collectCalls,omitempty"`
    CollectNS      int64  `json:"collectNs,omitempty"`
}

type StaticAliasFieldPerf struct {
    Field       string `json:"field"`
    Visits      uint64 `json:"visits"`
    Kind        string `json:"kind,omitempty"`
    MaxChildren int    `json:"maxChildren,omitempty"`
}

type DMLPerfCounters struct {
    RollbackPoints          uint64 `json:"rollbackPoints,omitempty"`
    SnapshotRollbackPoints  uint64 `json:"snapshotRollbackPoints,omitempty"`
    JournalRollbackPoints   uint64 `json:"journalRollbackPoints,omitempty"`
    TemporaryJournalPoints  uint64 `json:"temporaryJournalPoints,omitempty"`
}

var perfCountersEnabled atomic.Bool

func SetPerfCountersEnabled(enabled bool) {
    perfCountersEnabled.Store(enabled)
}

func perfEnabled() bool {
    return perfCountersEnabled.Load()
}

func ResetPerfCounters() {
    perfCountersEnabled.Store(false)
    resetStaticAliasPerf()
    resetDMLPerf()
}

func SnapshotPerfCounters() PerfCounters {
    if !perfEnabled() {
        return PerfCounters{}
    }
    return PerfCounters{
        Enabled:              true,
        StaticAlias:          snapshotStaticAliasPerf(),
        StaticAliasTopFields: snapshotStaticAliasTopFields(20),
        DML:                  snapshotDMLPerf(),
    }
}
```

Keep the concrete atomic storage, record helpers, and snapshot helpers in the same file. Do not call `time.Now()` unless `perfEnabled()` is true.

- [ ] **Step 5: Enable VM perf counters only for perf runs**

In `internal/gladecli/test_command.go`, before calling `apextest.RunCasesContext`, pass this option:

```go
testOpts.PerfCounters = strings.TrimSpace(perfJSONPath) != ""
```

In `internal/apextest/runner.go`, add to `Options`:

```go
PerfCounters bool
```

At the start of `RunCasesContext`, after `ResetPerfCounters()`:

```go
vm.SetPerfCountersEnabled(opts.PerfCounters)
```

Do not defer-disable VM counters inside `RunCasesContext`. The CLI writes
`--perf-json` after the run returns, so the VM snapshot must remain readable
until `maybeWriteRunPerfJSON` calls `apextest.SnapshotPerfCounters()`. The next
run calls `ResetPerfCounters()` and clears the state.

- [ ] **Step 6: Write counter tests**

Add to `internal/apextest/perf_counters_test.go`:

```go
func TestPerfCountersIncludeStorageAndVMStats(t *testing.T) {
    ResetPerfCounters()
    t.Cleanup(ResetPerfCounters)

    storage.ResetCloneStats()
    _ = storage.SnapshotRuntimeOrg(&storage.OrgState{})
    vm.SetPerfCountersEnabled(true)

    stats := SnapshotPerfCounters()
    if stats.StorageCloneStats.CloneRollbackSnapshotCalls == 0 {
        t.Fatalf("storage clone stats missing rollback snapshot count: %#v", stats.StorageCloneStats)
    }
    if !stats.VMPerf.Enabled {
        t.Fatalf("vm perf counters not marked enabled: %#v", stats.VMPerf)
    }
}
```

- [ ] **Step 7: Verify the counter surface**

Run:

```bash
go test ./internal/apextest ./internal/gladecli ./internal/vm -run 'TestPerfCounters|TestRunTest' -count=1
```

Expected: `ok` for all three packages.

## Task 2: Temporary DML Rollback Journal

**Files:**

- Modify: `internal/vm/dml_runtime.go`
- Modify: `internal/vm/runtime_state.go`
- Test: `internal/vm/data_test.go`
- Test: `internal/dml/dml_test.go`

- [ ] **Step 1: Write a failing test for bulk DML rollback points**

Add to `internal/vm/data_test.go` near `TestNeedsEarlyDMLRollbackSnapshotKeepsRiskyCases`:

```go
func TestBulkAllOrNoneDMLRollbackPointUsesTemporaryJournal(t *testing.T) {
    storage.ResetCloneStats()
    t.Cleanup(storage.ResetCloneStats)

    program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{
  new Account(Name = 'One'),
  new Account(Name = 'Two')
};
insert accounts;
System.assertEquals(2, [SELECT COUNT() FROM Account]);
`)
    if err != nil {
        t.Fatal(err)
    }
    machine := New(nil)
    org := testDataOrg()
    machine.SetOrg(&org)
    machine.EnableTestContext()

    if _, err := machine.Execute(program); err != nil {
        t.Fatal(err)
    }
    if got := storage.SnapshotCloneStats().CloneRollbackSnapshotCalls; got != 0 {
        t.Fatalf("rollback snapshots = %d, want temporary journal path", got)
    }
}
```

Expected before implementation: this fails because bulk `allOrNone` insert creates a full rollback snapshot.

- [ ] **Step 2: Extend the rollback point state**

Change `vmDMLRollbackPoint` in `internal/vm/dml_runtime.go`:

```go
type vmDMLRollbackPoint struct {
    enabled          bool
    journal          bool
    temporaryJournal bool
    mark             storage.IsolationMark
    org              storage.OrgState
    previousJournal  *storage.IsolationJournal
}
```

- [ ] **Step 3: Build the temporary journal path**

Change `beginDMLRollbackPoint`:

```go
func (vm *VM) beginDMLRollbackPoint(enabled bool, forceSnapshot bool) vmDMLRollbackPoint {
    if !enabled {
        return vmDMLRollbackPoint{}
    }
    if !forceSnapshot && vm != nil && vm.Org != nil {
        if vm.isolationJournal != nil {
            return vmDMLRollbackPoint{enabled: true, journal: true, mark: vm.isolationJournal.Mark()}
        }
        journal := storage.NewIsolationJournal(vm.Org)
        vm.recordTemporaryDMLJournalPoint()
        previous := vm.isolationJournal
        vm.isolationJournal = journal
        return vmDMLRollbackPoint{
            enabled:          true,
            journal:          true,
            temporaryJournal: true,
            mark:             journal.Mark(),
            previousJournal:  previous,
        }
    }
    if vm == nil || vm.Org == nil {
        return vmDMLRollbackPoint{enabled: true}
    }
    vm.recordSnapshotDMLRollbackPoint()
    return vmDMLRollbackPoint{enabled: true, org: snapshotRuntimeOrgState(vm.Org)}
}
```

The `record...` helpers live in `internal/vm/alias_perf.go` and no-op when perf counters are off.

- [ ] **Step 4: Restore or finish the rollback point**

Add:

```go
func (vm *VM) finishDMLRollbackPoint(point vmDMLRollbackPoint) {
    if vm == nil || !point.enabled || !point.temporaryJournal {
        return
    }
    vm.isolationJournal = point.previousJournal
}
```

Change `restoreDMLRollbackPoint`:

```go
func (vm *VM) restoreDMLRollbackPoint(point vmDMLRollbackPoint) {
    if vm == nil || vm.Org == nil || !point.enabled {
        return
    }
    defer vm.finishDMLRollbackPoint(point)
    if point.journal && vm.isolationJournal != nil {
        _ = vm.isolationJournal.Rollback(point.mark)
        return
    }
    *vm.Org = point.org
}
```

In `applyDML` and `applyUpsertDML`, after the local `restoreRollback` closure:

```go
defer func() {
    if rollbackReady {
        vm.finishDMLRollbackPoint(rollback)
    }
}()
```

This keeps class-level journals in place and discards only the temporary DML journal.

- [ ] **Step 5: Add failure rollback tests**

Add a second test in `internal/vm/data_test.go`:

```go
func TestTemporaryDMLJournalRollsBackBulkTriggerFailure(t *testing.T) {
    storage.ResetCloneStats()
    t.Cleanup(storage.ResetCloneStats)

    triggerProgram, err := CompileTrigger(`
for (Account account : Trigger.new) {
  if (account.Name == 'Bad') {
    account.addError('blocked');
  }
}
`)
    if err != nil {
        t.Fatal(err)
    }
    program, err := CompileAnonymous(`
try {
  insert new List<Account>{
    new Account(Name = 'Good'),
    new Account(Name = 'Bad')
  };
  System.assert(false, 'expected DML failure');
} catch (DmlException e) {
  System.assert(e.getMessage().contains('blocked'), e.getMessage());
}
System.assertEquals(0, [SELECT COUNT() FROM Account]);
`)
    if err != nil {
        t.Fatal(err)
    }
    machine := New(nil)
    org := testDataOrg()
    machine.SetOrg(&org)
    machine.EnableTestContext()
    if err := machine.RegisterTrigger(Trigger{
        Name:      "AccountBeforeInsert",
        Object:    "Account",
        Timing:    triggerTimingBefore,
        Operation: "insert",
        Program:   triggerProgram,
    }); err != nil {
        t.Fatal(err)
    }
    if _, err := machine.Execute(program); err != nil {
        t.Fatal(err)
    }
    if got := storage.SnapshotCloneStats().CloneRollbackSnapshotCalls; got != 0 {
        t.Fatalf("rollback snapshots = %d, want temporary journal path", got)
    }
}
```

Add a third test for pre-existing journals:

```go
func TestTemporaryDMLJournalDoesNotReplaceClassJournal(t *testing.T) {
    machine := New(nil)
    org := testDataOrg()
    journal := storage.NewIsolationJournal(&org)
    machine.SetOrg(&org)
    machine.SetIsolationJournal(journal)

    point := machine.beginDMLRollbackPoint(true, false)
    machine.finishDMLRollbackPoint(point)

    if machine.isolationJournal != journal {
        t.Fatalf("class journal was replaced")
    }
}
```

- [ ] **Step 6: Verify focused packages**

Run:

```bash
go test ./internal/vm ./internal/dml ./internal/storage -run 'TemporaryDMLJournal|BulkAllOrNone|IsolationJournal|RollbackSnapshot' -count=1
go test ./internal/vm ./internal/dml ./internal/apextest -count=1
```

Expected: all packages pass.

- [ ] **Step 7: Measure the real NAMS gate**

Build candidate:

```bash
GOCACHE=/tmp/glade-go-build-test-perf-candidate \
GOMAXPROCS=4 \
go build -a -o /tmp/glade-test-perf/glade-candidate ./cmd/glade
```

Run:

```bash
/usr/bin/time -p /tmp/glade-test-perf/glade-candidate test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200importedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-test-perf/candidate-imported.perf.json \
  > /tmp/glade-test-perf/candidate-imported.result.json \
  2> /tmp/glade-test-perf/candidate-imported.stderr.log
```

Expected:

- JSON summary has no failures or errors.
- Same-load candidate improves by at least `15%` or `30s` over Step 2.
- `apexPerf.storageCloneStats.cloneRollbackSnapshotCalls` drops materially.
- `apexPerf.vmPerf.dml.temporaryJournalPoints` is greater than `0`.

Run purchased after imported passes:

```bash
/usr/bin/time -p /tmp/glade-test-perf/glade-candidate test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200purchasedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-test-perf/candidate-purchased.perf.json \
  > /tmp/glade-test-perf/candidate-purchased.result.json \
  2> /tmp/glade-test-perf/candidate-purchased.stderr.log
```

Expected: purchased does not regress materially against the same-load baseline. Passing under `8m` is a strong win, but not required for this single task if imported gives the proof.

## Task 3: Complete Duration History

**Files:**

- Modify: `internal/gladecli/test_command.go`
- Modify: `internal/apextest/runner.go`
- Test: `internal/gladecli/test_command_selectors_test.go`
- Test: `internal/apextest/runner_test.go`

- [ ] **Step 1: Add full duration maps to perf JSON**

Extend `runPerfSummary`:

```go
ClassDurations  map[string]int64 `json:"classDurations,omitempty"`
MethodDurations map[string]int64 `json:"methodDurations,omitempty"`
```

Implement:

```go
func runClassDurations(result testreport.Run) map[string]int64 {
    out := map[string]int64{}
    for _, suite := range result.Suites {
        for _, testCase := range suite.Cases {
            className := strings.TrimSpace(testCase.ClassName)
            if className == "" {
                className = strings.TrimSpace(suite.Name)
            }
            if className != "" {
                out[className] += testCase.DurationMS
            }
        }
    }
    if len(out) == 0 {
        return nil
    }
    return out
}

func runMethodDurations(result testreport.Run) map[string]int64 {
    out := map[string]int64{}
    for _, suite := range result.Suites {
        for _, testCase := range suite.Cases {
            className := strings.TrimSpace(testCase.ClassName)
            methodName := strings.TrimSpace(testCase.MethodName)
            if className == "" || methodName == "" {
                continue
            }
            out[className+"."+methodName] = testCase.DurationMS
        }
    }
    if len(out) == 0 {
        return nil
    }
    return out
}
```

In `maybeWriteRunPerfJSON`:

```go
perf.ClassDurations = runClassDurations(result)
perf.MethodDurations = runMethodDurations(result)
```

- [ ] **Step 2: Load full duration history**

Replace `loadCLIClassDurationHistory` with a history loader that returns both maps:

```go
type cliDurationHistory struct {
    Classes map[string]int64
    Methods map[string]int64
}
```

The loader must accept three shapes:

1. New perf JSON:

```json
{"classDurations":{"AccountTest":1000},"methodDurations":{"AccountTest.testOne":800}}
```

2. Old perf JSON:

```json
{"topSlowClasses":[{"class":"AccountTest","durationMs":1000}]}
```

3. Direct class map:

```json
{"AccountTest":1000}
```

Keep old `--duration-history` behavior working.

- [ ] **Step 3: Pass method history into the runner**

Add to `apextest.Options`:

```go
MethodDurationMS map[string]int64
```

In CLI setup:

```go
history, err := loadCLIDurationHistory(durationHistoryPath)
if err != nil {
    return testreport.Run{}, err
}
testOpts.ClassDurationMS = history.Classes
testOpts.MethodDurationMS = history.Methods
```

- [ ] **Step 4: Sort methods within a class**

Add helper in `internal/apextest/runner.go`:

```go
func sortMethodIndexes(indexes []int, planned []testCasePlan, methodDurationMS map[string]int64) {
    if len(indexes) <= 1 || len(methodDurationMS) == 0 {
        return
    }
    sort.SliceStable(indexes, func(i, j int) bool {
        left := planned[indexes[i]].TestCase
        right := planned[indexes[j]].TestCase
        leftMS := methodDurationMS[left.ClassName+"."+left.MethodName]
        rightMS := methodDurationMS[right.ClassName+"."+right.MethodName]
        if leftMS == rightMS {
            return left.MethodName < right.MethodName
        }
        return leftMS > rightMS
    })
}
```

Call this before serial per-class method loops and before `runClassMethodIndexes`.

- [ ] **Step 5: Add history loader and method ordering tests**

Add test cases:

```go
func TestLoadCLIDurationHistoryReadsClassAndMethodMaps(t *testing.T) {
    path := filepath.Join(t.TempDir(), "perf.json")
    data := `{
      "classDurations": {"SlowClass": 9000, "FastClass": 10},
      "methodDurations": {"SlowClass.slow": 8000, "SlowClass.fast": 20}
    }`
    if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
        t.Fatal(err)
    }
    history, err := loadCLIDurationHistory(path)
    if err != nil {
        t.Fatal(err)
    }
    if history.Classes["SlowClass"] != 9000 {
        t.Fatalf("class duration missing: %#v", history.Classes)
    }
    if history.Methods["SlowClass.slow"] != 8000 {
        t.Fatalf("method duration missing: %#v", history.Methods)
    }
}
```

Add runner helper test:

```go
func TestSortMethodIndexesUsesDurationHistory(t *testing.T) {
    planned := []testCasePlan{
        {TestCase: TestCase{ClassName: "SlowClass", MethodName: "fast"}},
        {TestCase: TestCase{ClassName: "SlowClass", MethodName: "slow"}},
    }
    indexes := []int{0, 1}
    sortMethodIndexes(indexes, planned, map[string]int64{
        "SlowClass.slow": 8000,
        "SlowClass.fast": 20,
    })
    if indexes[0] != 1 || indexes[1] != 0 {
        t.Fatalf("indexes = %#v, want slow method first", indexes)
    }
}
```

- [ ] **Step 6: Verify duration history**

Run:

```bash
go test ./internal/gladecli ./internal/apextest -run 'DurationHistory|SortMethodIndexes|ClassShard|MethodParallelism' -count=1
```

Expected: all tests pass.

## Task 4: Adaptive Method Work For Large Classes

**Files:**

- Modify: `internal/apextest/runner.go`
- Create: `internal/apextest/dispatcher_test.go`
- Test: `internal/apextest/runner_test.go`

This task comes after Task 3. Duration history must exist first.

- [ ] **Step 1: Add a scheduler test that proves the current tail**

Create `internal/apextest/dispatcher_test.go` with a pure scheduling test. Do not run Apex here.

```go
func TestAdaptiveMethodBudgetGivesDominantClassMoreWorkers(t *testing.T) {
    classes := []classScheduleInput{
        {ClassName: "BigClass", Methods: 40, DurationMS: 400_000},
        {ClassName: "SmallA", Methods: 2, DurationMS: 1_000},
        {ClassName: "SmallB", Methods: 2, DurationMS: 1_000},
        {ClassName: "SmallC", Methods: 2, DurationMS: 1_000},
    }
    budget := adaptiveClassMethodBudget(4, classes)
    if budget["BigClass"] < 2 {
        t.Fatalf("BigClass budget = %d, want at least 2", budget["BigClass"])
    }
}
```

- [ ] **Step 2: Implement a conservative budget helper**

Add:

```go
type classScheduleInput struct {
    ClassName  string
    Methods    int
    DurationMS int64
}

func adaptiveClassMethodBudget(totalParallelism int, classes []classScheduleInput) map[string]int {
    out := map[string]int{}
    if totalParallelism <= 1 || len(classes) == 0 {
        return out
    }
    sort.SliceStable(classes, func(i, j int) bool {
        return classes[i].DurationMS > classes[j].DurationMS
    })
    top := classes[0]
    if top.Methods <= 1 || top.DurationMS <= 0 {
        return out
    }
    var rest int64
    for _, item := range classes[1:] {
        rest += item.DurationMS
    }
    if rest == 0 || top.DurationMS < rest/2 {
        return out
    }
    reserve := totalParallelism / 2
    if reserve < 2 {
        reserve = 2
    }
    if reserve > top.Methods {
        reserve = top.Methods
    }
    out[top.ClassName] = reserve
    return out
}
```

This is intentionally narrow. It only helps a class that dominates the run. It does not change default behavior for balanced suites.

- [ ] **Step 3: Apply the budget only when history proves dominance**

In `runTestPlansWithSetups`, build `classScheduleInput` from `classOrder`, `classIndexes`, and `opts.ClassDurationMS`.

Before starting workers, reserve capacity for the dominant class:

```go
adaptiveBudgets := adaptiveClassMethodBudget(opts.Parallelism, scheduleInputs)
reservedMethodWorkers := 0
for _, budget := range adaptiveBudgets {
    if budget > 1 && budget-1 > reservedMethodWorkers {
        reservedMethodWorkers = budget - 1
    }
}
if reservedMethodWorkers > 0 && parallelism > 1 {
    parallelism -= reservedMethodWorkers
    if parallelism < 1 {
        parallelism = 1
    }
}
```

When running a class:

```go
methodParallelism := methodParallelismForClassRun(opts.Parallelism, parallelism, len(classIndexes[className]), dispatcher.unfinishedClassCount())
if extra := adaptiveBudgets[className]; extra > methodParallelism {
    methodParallelism = extra
}
```

This keeps total active test methods under `opts.Parallelism`: the dominant
class gets the reserved method workers, and the class-worker pool is smaller by
the same count.

- [ ] **Step 4: Preserve safety switches**

Make sure all of this stays behind:

```go
if opts.ParallelMethods && len(classIndexes[className]) > 1 {
    ...
}
```

`--no-parallel-methods` must keep serial behavior.

- [ ] **Step 5: Verify scheduler tests**

Run:

```bash
go test ./internal/apextest -run 'AdaptiveMethodBudget|MethodParallelism|RunUsesClassJournal' -count=1
```

Expected: all tests pass.

- [ ] **Step 6: Measure against saved suite history**

Use current saved full reports as history:

```bash
/tmp/glade-test-perf/glade-candidate test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --parallelism 4 \
  --test-timeout 8m \
  --duration-history /tmp/glade-test-perf/nams-history.perf.json \
  --progress \
  --json \
  --perf-json /tmp/glade-test-perf/nams-after-scheduler.perf.json \
  > /tmp/glade-test-perf/nams-after-scheduler.result.json \
  2> /tmp/glade-test-perf/nams-after-scheduler.stderr.log
```

Expected: full NAMS remains green. Wall time should improve over a same-load full baseline, with `MembershipBillingSuite` no longer sitting alone at the tail.

## Task 5: Static Alias Follow-Up, Only If Still Proven Hot

**Files:**

- Modify: `internal/vm/value_aliasing.go`
- Modify: `internal/vm/runtime_state.go`
- Test: `internal/vm/method_test.go`
- Test: `internal/vm/value_aliasing_test.go`
- Test: `internal/vm/vm_benchmark_test.go`

This task is gated. Do not start it until Task 2 and Task 3/4 measurements show VM static alias propagation still dominates focused NAMS time.

- [ ] **Step 1: Confirm alias is still the hot path**

Run a perf-enabled focused NAMS method after the DML journal candidate:

```bash
/usr/bin/time -p /tmp/glade-test-perf/glade-candidate test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200importedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --cpu-profile /tmp/glade-test-perf/imported.cpu.pprof \
  --mem-profile /tmp/glade-test-perf/imported.mem.pprof \
  --perf-json /tmp/glade-test-perf/imported-vm.perf.json \
  > /tmp/glade-test-perf/imported-vm.result.json \
  2> /tmp/glade-test-perf/imported-vm.stderr.log
```

Inspect:

```bash
jq '.apexPerf.vmPerf.staticAlias' /tmp/glade-test-perf/imported-vm.perf.json
go tool pprof -top -cum /tmp/glade-test-perf/glade-candidate /tmp/glade-test-perf/imported.cpu.pprof | sed -n '1,40p'
go tool pprof -top -alloc_space /tmp/glade-test-perf/glade-candidate /tmp/glade-test-perf/imported.mem.pprof | sed -n '1,40p'
```

Proceed only if static alias replacement remains above `25%` of CPU or `30s` wall-equivalent.

- [ ] **Step 2: Do not revive exact path indexing**

Delete or ignore the failed implementation worktree:

```text
/Users/matt/Dev/glade/.worktrees/vm-static-alias-path-index
```

The exact-path ledger failed real NAMS timings. This task must not maintain map/list/set path ledgers.

- [ ] **Step 3: Implement a no-change skip cache only if no-change visits are material**

Gate:

```text
staticAlias.noChange / staticAlias.locationVisits >= 0.15
```

If the gate passes, add a conservative cache:

```go
type staticAliasNoChangeKey struct {
    Ref        uint64
    Kind       ValueKind
    TypeName   string
    ClassName  string
    FieldName  string
    MutationSeq uint64
}
```

Add `aliasMutationSeq uint64` to `VM`. Increment it in every path that can mutate an aliased value:

- `propagateCollectionMutation`
- `propagateCollectionMutationFromSnapshot`
- `assignPath` when `propagate` runs
- method parameter/receiver mutation propagation
- SObject receiver mutation propagation

Skip only when the exact key was recorded as no-change under the same `aliasMutationSeq`. If any generation is stale or missing, run the current recursive walk.

- [ ] **Step 4: Add no-change cache tests**

Tests must prove:

- A repeated no-change static scan skips the second recursive walk.
- A collection mutation increments `aliasMutationSeq` and forces a new recursive walk.
- Object field assignment through `assignPath` increments `aliasMutationSeq` and forces a new recursive walk.
- `ResetStatics`, `ResetTestAsyncStaticCollections`, and `restoreStaticFieldSnapshot` invalidate the cache.

Run:

```bash
go test ./internal/vm -run 'StaticAliasNoChange|PropagateAliasSnapshotToStatics|ResetStatics|ResetTestAsyncStaticCollections' -count=1
```

- [ ] **Step 5: Consider subtree summaries only if changed visits dominate**

Gate:

```text
staticAlias.changed / staticAlias.locationVisits >= 0.70
```

If this gate passes, no-change caching is the wrong tool. Build a subtree summary that can have false positives but never false negatives.

Required shape:

```go
type aliasSubtreeSummary struct {
    Generation uint64
    Refs       map[uint64]ValueKind
    Overflow   bool
}
```

Rules:

- `Overflow` means run the old recursive walk.
- Missing summary means run the old recursive walk.
- Stale generation means rebuild or run the old recursive walk.
- A summary may say "could contain ref" when it does not.
- A summary must never say "cannot contain ref" when it can.

Do not attach this to `Value`. Use VM sidecar state keyed by `Value.Ref` or static field location so JSON shape and normal value copies do not change.

- [ ] **Step 6: Verify alias work**

Run:

```bash
go test ./internal/vm -run 'StaticValueRef|PropagateAliasSnapshot|ReplaceAliasSnapshot|NamespacedStaticSingleton|CollectionWriteback' -count=1
go test ./internal/vm -count=1
go test ./internal/vm ./internal/apextest ./internal/gladecli -count=1
go test ./...
git diff --check
```

Then rerun focused NAMS imported and purchased with forced candidate binary. Keep the work only if the real imported method beats same-load baseline by `15%` or `30s` and purchased does not regress.

## Task 6: External Proof And Closeout

**Files:**

- No code changes.

- [ ] **Step 1: Build final candidate**

Run:

```bash
GOCACHE=/tmp/glade-go-build-test-perf-final \
GOMAXPROCS=4 \
go build -a -o /tmp/glade-test-perf/glade-final ./cmd/glade
```

Expected: exit code `0`.

- [ ] **Step 2: Run focused gates**

Run imported and purchased:

```bash
/usr/bin/time -p /tmp/glade-test-perf/glade-final test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200importedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-test-perf/final-imported.perf.json \
  > /tmp/glade-test-perf/final-imported.result.json \
  2> /tmp/glade-test-perf/final-imported.stderr.log

/usr/bin/time -p /tmp/glade-test-perf/glade-final test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200purchasedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-test-perf/final-purchased.perf.json \
  > /tmp/glade-test-perf/final-purchased.result.json \
  2> /tmp/glade-test-perf/final-purchased.stderr.log
```

Expected:

- Imported passes.
- Imported improves by at least `15%` or `30s` against same-load baseline.
- Purchased does not regress materially.
- Perf JSON shows the intended counter movement.

- [ ] **Step 3: Run package tests**

Run:

```bash
go test ./internal/vm ./internal/dml ./internal/storage ./internal/apextest ./internal/gladecli -count=1
go test ./...
scripts/smoke.sh
git diff --check
```

Expected: all pass.

- [ ] **Step 4: Run external full gates**

Run with `--parallelism 4`:

```bash
/tmp/glade-test-perf/glade-final test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-test-perf/final-nams.perf.json \
  > /tmp/glade-test-perf/final-nams.result.json \
  2> /tmp/glade-test-perf/final-nams.stderr.log

/tmp/glade-test-perf/glade-final test \
  --project "$GLADE_CORPUS_PROJECT_B" \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-test-perf/final-nu.perf.json \
  > /tmp/glade-test-perf/final-nu.result.json \
  2> /tmp/glade-test-perf/final-nu.stderr.log

/tmp/glade-test-perf/glade-final test \
  --project "$GLADE_CORPUS_PROJECT_C" \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-test-perf/final-project-c.perf.json \
  > /tmp/glade-test-perf/final-project-c.result.json \
  2> /tmp/glade-test-perf/final-project-c.stderr.log
```

Expected:

```bash
jq '.summary' /tmp/glade-test-perf/final-nams.result.json
jq '.summary' /tmp/glade-test-perf/final-nu.result.json
jq '.summary' /tmp/glade-test-perf/final-project-c.result.json
```

shows `failed: 0`, `compileErrors: 0`, `runtimeErrors: 0`, and `errors: 0` for all three.

## Acceptance Bar

Merge only when all are true:

- Focused NAMS imported method passes.
- Focused imported method is at least `15%` or `30s` faster than same-load baseline.
- Purchased method does not regress materially.
- Full NAMS and two additional external corpus gates stay green.
- Touched Go packages pass.
- `go test ./...`, `scripts/smoke.sh`, and `git diff --check` pass.
- Perf JSON proves which counter moved.
- No product code contains project-specific exceptions.

## Stop Rules

Stop and write a handoff if any of these happen:

- A candidate helps a synthetic benchmark but not focused NAMS imported.
- A candidate changes Salesforce-visible behavior.
- A candidate needs package/class/field names from NAMS or NU.
- A candidate removes recursive alias fallback.
- Three implementation attempts fail the real timing gate.
- Full external gates fail in a way tied to runtime behavior.

## Recommended Order

1. Counter surface.
2. Temporary DML rollback journal.
3. Complete duration history.
4. Adaptive method work for dominant classes.
5. Static alias follow-up only if still proven hot.
6. Full external proof.

The first good stopping point is after Task 2. If temporary journals reduce rollback snapshots and focused NAMS imported improves, that is a useful mergeable slice without touching the fragile alias ledger.

## Final12 Implementation Proof

Status: focused gate passed on branch `codex/test-performance-optimization`.
The remaining risk before publishing is external full-suite breadth.

The mergeable cut is a VM static-alias direct-child index for large static maps.
When a static map has at least `64` immediate key/value children, Glade builds a
per-field index from direct alias ref to the immediate map key or value slot. A
hit validates that the current child still has the old ref and kind before
replacement. Duplicate refs, stale entries, nested refs, small maps, and
non-map shapes stay on the old recursive walk.

The final review added one correctness guard: direct map-key hits now respect
`mapKeyTypeCannotContainAlias`, so the fast path cannot mutate primitive-typed
map keys the recursive path would skip.

Focused proof used a forced binary:

```bash
go build -a -o /tmp/glade-test-perf/glade-final12 ./cmd/glade
```

Final12 focused imported result:

```text
baseline /tmp/glade-test-perf/baseline-imported-rerun.perf.json
  durationMs 206431

candidate /tmp/glade-test-perf/final12-direct-map-index-imported.perf.json
  durationMs 163435
  directChildHits 19347
  staticAlias.durationNs 73500044435

delta 42996ms
20.82 percent faster
```

Final12 no-perf imported result:

```text
baseline /tmp/glade-test-perf/baseline-imported-noperf.result.json
  durationMs 195222

candidate /tmp/glade-test-perf/final12-direct-map-index-imported-noperf.result.json
  durationMs 157972

delta 37250ms
19.08 percent faster
```

Final12 purchased guardrail:

```text
baseline /tmp/glade-test-perf/baseline-purchased-current.perf.json
  durationMs 269891

candidate /tmp/glade-test-perf/final12-direct-map-index-purchased.perf.json
  durationMs 213467
  directChildHits 27866
  staticAlias.durationNs 73286819582

delta 56424ms
20.90 percent faster
```

Rejected during implementation:

| Experiment | Artifact | Result | Reason |
| --- | --- | ---: | --- |
| Preserve unrelated child hints for scalar updates | `/tmp/glade-test-perf/final7-child-hint-preserve-imported.perf.json` | 208404ms | only `4` extra child-hint hits |
| Direct object child index | `/tmp/glade-test-perf/final10-direct-child-index-imported.perf.json` | 174293ms | fewer hits and slower than map-only |
| Earlier one-step direct static child index | `/tmp/glade-test-perf/static-direct-imported.perf.json` | 208073ms | more work than baseline |
| Earlier one-step subtree index | `/tmp/glade-test-perf/static-subtree-imported.perf.json` | 256444ms | `12721` hits but `166.3s` static-alias time |

Local verification after the final review guard:

```text
go test ./internal/vm -run 'TestDirectMapChildIndexRespectsPrimitiveMapKeyType|TestStaticCollectionWritebackInvalidatesDirectMapChildIndex|TestStaticAliasTopFieldsRankByDurationAndTrackOutcomes|TestPropagateAliasSnapshotToStatics(UsesDirectMapChildIndex|UsesLearnedChildHint|RemembersNestedRefsFromSameRefUpdate)|TestStaticValueRefCacheReplaceInvalidatesChildHints|TestReplaceAliasSnapshotWithStaticChildHintHandlesRootCycle' -count=1
ok github.com/glade-sh/glade/internal/vm 2.224s
```

Final review proof after the map-key type guard:

```text
imported candidate /tmp/glade-test-perf/final-review-direct-map-index-imported.perf.json
  durationMs 158069
  directChildHits 19347
  staticAlias.durationNs 70392965153
  delta vs 206431ms baseline: 48362ms, 23.42 percent faster

purchased candidate /tmp/glade-test-perf/final-review-direct-map-index-purchased.perf.json
  durationMs 216739
  directChildHits 27866
  staticAlias.durationNs 73994472275
  delta vs 269891ms baseline: 53152ms, 19.69 percent faster
```

Final review local gates:

```bash
go test ./internal/vm -count=1
go test ./internal/apextest ./internal/gladecli ./internal/vm ./internal/dml ./internal/storage -count=1
git diff --check
go test ./...
```

Result:

```text
go test ./internal/vm -count=1
ok github.com/glade-sh/glade/internal/vm 3.020s

go test ./internal/apextest ./internal/gladecli ./internal/vm ./internal/dml ./internal/storage -count=1
ok github.com/glade-sh/glade/internal/apextest 215.949s
ok github.com/glade-sh/glade/internal/gladecli 202.417s
ok github.com/glade-sh/glade/internal/vm 4.393s
ok github.com/glade-sh/glade/internal/dml 4.756s
ok github.com/glade-sh/glade/internal/storage 3.991s

git diff --check exited 0
go test ./... exited 0
```

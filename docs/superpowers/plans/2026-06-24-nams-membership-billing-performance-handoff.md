# NAMS Membership Billing Performance Handoff

Date: 2026-06-24

Status: investigated. No optimization is valid to merge.

## Problem

The slow tests are in:

```bash
/Users/matt/Dev/glade-corpus/private/nams-workspace
```

The visible long-running methods were:

```text
MembershipBillingSuite.simplestRenewal_bulk_200importedMemberships
MembershipBillingSuite.simplestRenewal_bulk_200purchasedMemberships
```

The original full command was:

```bash
glade test --project ./nams-workspace
```

## Current Evidence

Baseline forced binary from clean `main`:

```bash
GOCACHE=/tmp/glade-go-build-alias-path-baseline GOMAXPROCS=4 \
  go build -a -o /tmp/glade-alias-path-baseline ./cmd/glade
```

Focused imported baseline:

```bash
/usr/bin/time -p /tmp/glade-alias-path-baseline test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200importedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-alias-path/baseline-imported.perf.json \
  > /tmp/glade-alias-path/baseline-imported.result.json \
  2> /tmp/glade-alias-path/baseline-imported.stderr.log
```

Result under that machine load:

```text
PASS
real 388.89
user 278.07
sys 40.21
method durationMs 361115
total durationMs 386882
```

Focused purchased baseline:

```bash
/usr/bin/time -p /tmp/glade-alias-path-baseline test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200purchasedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-alias-path/baseline-purchased.perf.json \
  > /tmp/glade-alias-path/baseline-purchased.result.json \
  2> /tmp/glade-alias-path/baseline-purchased.stderr.log
```

Result under that machine load:

```text
FAIL: context deadline exceeded
real 487.35
user 344.78
sys 51.15
```

That means imported is the cleaner acceptance gate. Purchased already missed `8m`
under this load.

## Hot Area Found

The heavy cost sits in VM static alias propagation.

Observed from profiling and instrumentation:

```text
VM.call / eval / execute dominate CPU
propagateAliasSnapshotToStatics about 103s cumulative
replaceAliasSnapshot / replaceValueAliasRef about 93.5s cumulative
executeForEach about 161s cumulative
startup and compile are not the issue
```

Instrumentation run:

```text
staticSnapshotCalls=338384
staticSnapshotNs=122.5s
staticSnapshotRefMiss=290017
staticSnapshotLocationVisits=66327
staticSnapshotTopLevelLocations=16103
staticSnapshotDeepLocations=50214
staticSnapshotChanged=59070
staticSnapshotNoChange=7247
staticFieldCollectCalls=59742
staticFieldCollectNs=24.1s
updatedValueAliasesCalls=264716
updatedValueAliasesNs=32.6s
```

Hot static fields:

```text
PriceableOrder.requestByLine
  visits 29198
  kind map
  maxChildren 400

SObjectPersistenceService.cachedInstance
  visits 14897
  kind object
  maxChildren 5

R.API
  visits 5747
  kind map
  maxChildren 454
```

## Worktrees Used

First prototype worktree:

```text
/Users/matt/Dev/glade/.worktrees/nams-billing-speed
```

Second prototype worktree:

```text
/Users/matt/Dev/glade/.worktrees/nams-alias-prototypes
```

Implementation attempt worktree:

```text
/Users/matt/Dev/glade/.worktrees/vm-static-alias-path-index
branch codex/vm-static-alias-path-index
```

Do not merge the implementation attempt. It failed the real timing gate.

## Things Tried

### 1. Deferred Nested Static-Ref Indexing

Idea:

Build less nested static-reference index state up front.

Result:

Synthetic benchmark looked better. Real NAMS run timed out beyond `8m`.

Conclusion:

Invalid. It saved setup work but forced fallback scans in the workload that
matters.

Status:

Reverted in the old prototype worktree.

### 2. Append-Only List Tail Indexing

Idea:

When a list only appends, update alias/reference tracking for the new tail
instead of rescanning the whole list.

Result:

Synthetic benchmark improved. Real NAMS got worse:

```text
candidate 216.90s
baseline 211.07s
```

Those numbers came from an earlier, lighter-load run.

Conclusion:

Invalid. The narrow list append case was not the dominant real cost.

Status:

Reverted in the old prototype worktree.

### 3. Direct-Child Pre-Scan

Idea:

Before walking a large static field, cheaply check direct children to avoid
deep work when the target ref is absent.

Result:

Passed correctness checks, but real NAMS slowed badly:

```text
379.81s
```

Conclusion:

Invalid. The pre-scan added work to the hot loop and did not reject enough
real cases.

### 4. Alias-Update Assignment Counter

Idea:

Count whether `updatedValueAliases` work actually assigns values. If many walks
do no writes, prune them.

Observed:

```text
updatedValueAliasesWalks=228052
updatedValueAliasesAssignments=54143
```

Result:

The counter itself made wall time bad. The ratio still showed many walks do not
assign, but simple call-site pruning looked weak.

Conclusion:

Useful evidence, not a mergeable optimization.

### 5. Exact Static Alias Path Index

Idea:

For each static field, record exact paths to every referenced value:

```text
object field
map value
map key
list index
set index
```

Then replace aliases by exact path instead of recursively scanning the whole
static field.

Prototype files changed in the failed implementation:

```text
internal/vm/runtime_state.go
internal/vm/value_aliasing.go
internal/vm/method_test.go
internal/vm/vm_benchmark_test.go
```

Validation that passed in the prototype:

```bash
go test ./internal/vm -run 'TestStaticValueRef|TestPropagateAliasSnapshotToStatics|TestReplaceAliasSnapshotAtStaticPath|TestFindValueByRefFallsBackWhenStaticRefIndexIsStale' -count=1
go test ./internal/vm -run 'TestExecMethodParameterMapPropagatesNestedCollectionAliases|TestExecListMutationPropagatesThroughMapAlias|TestExecNamespacedStaticSingletonAliasesShareInstanceState' -count=1
go test ./internal/vm -count=1
go test ./internal/vm ./internal/apextest ./internal/gladecli -count=1
git diff --check
```

Synthetic benchmark after repair:

```text
BenchmarkPropagateAliasSnapshotToStaticsNestedMapPaths
settled around 134ms to 137ms/op after first setup-heavy iteration
```

Correctness hazards found and patched in prototype:

```text
same-ref static collection writeback can add a second path while the old path
still works

in-place exact-path replacement can corrupt shared static backing data between
two static fields
```

Added sentinel tests in prototype:

```text
TestStaticCollectionWritebackRefreshesAddedAliasPaths
TestPropagateAliasSnapshotToStaticsUpdatesSharedStaticFields
TestReplaceAliasSnapshotAtStaticPathReplacesMapKeyListAndSet
```

Real NAMS candidate results:

First path-index candidate:

```text
FAIL: context deadline exceeded
real 487.39
user 863.50
sys 13.33
```

Path-level ledger update candidate:

```text
interrupted after running past timeout
real 617.63
user 731.74
sys 14.44
result JSON empty due interrupt
```

Final copy-on-path plus path-level ledger candidate:

```text
interrupted after running past timeout
real 578.14
user 687.29
sys 13.00
result JSON empty due interrupt
```

Conclusion:

Invalid. Exact-path replacement reduces search but adds too much ledger
maintenance and path copying. The real workload gets worse.

## Things Not Tried Yet

### 1. Subtree Summaries

Likely next best lead.

Instead of exact paths, keep a cheap summary on static field subtrees:

```text
ref set or bloom-style ref membership
mutation generation
child count / shape generation
```

Goal:

Avoid recursive replacement when a subtree cannot contain the target ref, without
maintaining exact path entries for every nested child.

Risk:

The summary must update on map/list/set/object mutations. If stale, it must
fail open to the full recursive walk.

### 2. Dirty Static Field Generations

Track which static fields changed since the last alias propagation. For the
common miss path:

```text
staticSnapshotRefMiss=290017
```

avoid rebuilding or rechecking static fields that have not changed.

Risk:

Static field values often share backing collection data through aliases. The
generation must move when the backing collection mutates, not only when the
static field slot changes.

### 3. Type-Based Pruning

Use known type shape to skip map keys, list children, or fields that cannot hold
the target kind.

Existing helpers worth inspecting:

```text
valueCannotContainAliasRef
mapKeyTypeCannotContainAlias
listCannotContainAliasRef
```

Risk:

This was partly used already inside recursive replacement. A new layer must
prove it avoids enough outer work to matter.

### 4. Focus on `PriceableOrder.requestByLine`

This field dominates visits:

```text
29198 visits
map
maxChildren 400
```

Look for a product-general way to reduce repeated propagation across that map.
Do not add NAMS-specific exceptions.

Possible directions:

```text
batch static propagation during tight loops
per-ref last propagated generation for a static field
skip repeated no-change propagation for same previous snapshot and field generation
```

Risk:

Alias semantics are broad. Any skip must preserve mutations visible through
static singletons and cached maps.

## Must-Keep Correctness Gates

Focused VM gates:

```bash
go test ./internal/vm -count=1
go test ./internal/vm ./internal/apextest ./internal/gladecli -count=1
```

Focused NAMS imported gate:

```bash
mkdir -p /tmp/glade-alias-path
/usr/bin/time -p ./bin-or-temp-glade test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200importedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-alias-path/candidate-imported.perf.json \
  > /tmp/glade-alias-path/candidate-imported.result.json \
  2> /tmp/glade-alias-path/candidate-imported.stderr.log
```

Focused NAMS purchased gate:

```bash
mkdir -p /tmp/glade-alias-path
/usr/bin/time -p ./bin-or-temp-glade test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200purchasedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-alias-path/candidate-purchased.perf.json \
  > /tmp/glade-alias-path/candidate-purchased.result.json \
  2> /tmp/glade-alias-path/candidate-purchased.stderr.log
```

Full external gates from the original plan:

```bash
glade test --project /Users/matt/Dev/glade-corpus/private/nams-workspace
glade test --project /Users/matt/Dev/glade-corpus/private/external-project-b
glade test --project /Users/matt/Dev/glade-corpus/private/external-project-c
```

## Acceptance Bar

Do not merge unless:

```text
all local Go tests pass for touched packages
focused imported method passes
focused imported method is at least 15% faster or 30s faster than same-load baseline
purchased method does not regress materially
full external project gates remain passing
```

If a candidate only improves synthetic benchmarks, do not keep it.

## Advice For The Next Agent

Start with fresh timing. Machine load changed the absolute numbers a lot.

Build both binaries with forced rebuilds:

```bash
GOCACHE=/tmp/glade-go-build-baseline GOMAXPROCS=4 go build -a -o /tmp/glade-baseline ./cmd/glade
GOCACHE=/tmp/glade-go-build-candidate GOMAXPROCS=4 go build -a -o /tmp/glade-candidate ./cmd/glade
```

Run imported first. It is the cleaner gate.

Do not trust synthetic benchmarks until imported NAMS passes faster. The prior
prototypes all looked better in small benchmarks and lost on the real run.

Keep the stale-index fallback. Removing the full scan in `findValueByRef` or
static propagation is wrong unless every mutation path has a proven, tested
index update.

Watch these reset/restore paths:

```text
VM.ResetStatics
VM.ResetTestAsyncStaticCollections
restoreStaticFieldSnapshot
invalidateStaticValueRefs
```

Watch collection mutation shape changes:

```text
Map.put / putAll / remove / clear
List.add / remove / clear / sort / set
Set.add / remove / clear / removeAll / retainAll
object field assignment through assignPath
```

Set index paths are especially fragile after removal. Treat them as stale unless
the full field ledger is refreshed.

## Bottom Line

The hotspot is real.

The exact-path index was the wrong tool for this knot.

The next attempt should reduce repeated no-op work without maintaining exact
paths for every nested static value.

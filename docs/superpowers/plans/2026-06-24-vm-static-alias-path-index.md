# VM Static Alias Path Index Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace repeated full static-field alias walks with an exact per-ref path index while preserving existing VM alias behavior and keeping all Glade and external corpus tests passing.

**Architecture:** Keep the current `staticValueRefs` and `staticValueRefFields` broad index as the coarse guard, and add a precise `staticValueRefPaths` index keyed by ref. `propagateAliasSnapshotToStatics` should try indexed paths first, fall back to the existing recursive replacement if a path is stale, then repair the index for that static field. Correctness wins over speed; stale or unsupported paths must use the old walk.

**Tech Stack:** Go, `internal/vm`, `glade test`, pprof/perf JSON, external corpus projects under `/Users/matt/Dev/glade-corpus/private`.

---

## Evidence From Profiling

The current slow target is:

```bash
/tmp/glade-nams-slow-current test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200importedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json
```

Observed baseline from forced builds:

| Case | Result |
| --- | --- |
| Full method wall time | about 211s |
| `propagateAliasSnapshotToStatics` | about 115s to 125s |
| static ref recollection after writes | about 21s to 25s |
| `propagateUpdatedValueAliases` | about 31s |
| hottest static field | `PriceableOrder.requestByLine`, 29,198 visits, map size 400 |
| second static field | `SObjectPersistenceService.cachedInstance`, 14,897 visits |
| third static field | `R.API`, 5,747 visits, map size 454 |

Invalid prototype:

| Prototype | Result |
| --- | --- |
| direct-child scan before recursive walk | passed but slowed method to about 380s |

Valid lead:

| Prototype | Result |
| --- | --- |
| known exact path on synthetic 200-entry nested map | about 76,000 ns/op recursive walk to about 265 ns/op known path |

This makes exact-path indexing the only candidate worth implementing.

## Non-Negotiable Test Gates

Use a forced binary for timing. Do not judge a speed change with `go run`.

Final code must pass:

```bash
go test ./internal/vm
go test ./internal/vm ./internal/apextest ./internal/gladecli
go test ./...
```

Focused external gates must pass:

```bash
/tmp/glade-alias-path-candidate test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200importedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-alias-path/imported.perf.json \
  > /tmp/glade-alias-path/imported.result.json

/tmp/glade-alias-path-candidate test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --class MembershipBillingSuite \
  --method simplestRenewal_bulk_200purchasedMemberships \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-alias-path/purchased.perf.json \
  > /tmp/glade-alias-path/purchased.result.json
```

Full external project gates must pass before closeout:

```bash
/tmp/glade-alias-path-candidate test \
  --project /Users/matt/Dev/glade-corpus/private/nams-workspace \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-alias-path/nams-full.perf.json \
  > /tmp/glade-alias-path/nams-full.result.json

/tmp/glade-alias-path-candidate test \
  --project /Users/matt/Dev/glade-corpus/private/external-project-b \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-alias-path/external-b-full.perf.json \
  > /tmp/glade-alias-path/external-b-full.result.json

/tmp/glade-alias-path-candidate test \
  --project /Users/matt/Dev/glade-corpus/private/external-project-c \
  --parallelism 4 \
  --test-timeout 8m \
  --progress \
  --json \
  --perf-json /tmp/glade-alias-path/external-c-full.perf.json \
  > /tmp/glade-alias-path/external-c-full.result.json
```

Each JSON result must show:

```json
{
  "summary": {
    "failed": 0,
    "compileErrors": 0,
    "runtimeErrors": 0,
    "errors": 0
  }
}
```

Performance acceptance:

- Focused imported method should improve by at least 15 percent or 30 seconds against a fresh forced-build baseline.
- Focused purchased method must not regress by more than 5 percent.
- Full external project gates must stay green.
- If a candidate improves one focused method but regresses another gate, revert or narrow the implementation.

## File Map

Modify:

- `internal/vm/runtime_state.go`
  - Add `staticValueRefPaths map[uint64]staticFieldRefPathSet` beside the existing static ref fields.
- `internal/vm/value_aliasing.go`
  - Add path-step types.
  - Collect static ref paths.
  - Apply exact path replacements.
  - Repair or invalidate path entries when static fields change.
- `internal/vm/method_test.go`
  - Add tests for path indexing, stale path fallback, and static-field replacement behavior.
- `internal/vm/vm_benchmark_test.go`
  - Add or adapt a benchmark for nested static map propagation.

Optional during development only:

- `internal/vm/alias_perf.go`
  - Hidden counters behind `GLADE_ALIAS_PERF=1`.
- `internal/gladecli/test_command.go`
  - Add `aliasPerf` to `--perf-json` only if the team wants this hidden diagnostic surface to remain.
- `internal/apextest/runner.go`
  - Reset hidden alias counters at test-run start.

Do not add project-specific exceptions. No NAMS class names, package names, field names, or test-only heuristics belong in product code.

### Task 1: Create The Worktree And Baseline

**Files:**
- No code changes.

- [ ] **Step 1: Create an isolated worktree**

Run:

```bash
git worktree add -b codex/vm-static-alias-path-index \
  /Users/matt/Dev/glade/.worktrees/vm-static-alias-path-index \
  main
cd /Users/matt/Dev/glade/.worktrees/vm-static-alias-path-index
git status --short
```

Expected:

```text
```

- [ ] **Step 2: Build the baseline binary from unmodified `main`**

Run:

```bash
GOCACHE=/tmp/glade-go-build-alias-path-baseline \
GOMAXPROCS=4 \
go build -a -o /tmp/glade-alias-path-baseline ./cmd/glade
```

Expected: exit code `0`.

- [ ] **Step 3: Capture focused baseline timings**

Run:

```bash
mkdir -p /tmp/glade-alias-path
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

Expected: both commands exit `0`, both JSON summaries have no failures or errors.

- [ ] **Step 4: Record the baseline**

Run:

```bash
jq '.summary, .tests[0]' /tmp/glade-alias-path/baseline-imported.result.json
jq '.summary, .tests[0]' /tmp/glade-alias-path/baseline-purchased.result.json
tail -n 20 /tmp/glade-alias-path/baseline-imported.stderr.log
tail -n 20 /tmp/glade-alias-path/baseline-purchased.stderr.log
```

Expected: `failed`, `compileErrors`, `runtimeErrors`, and `errors` are all `0`.

### Task 2: Add Path Index Types

**Files:**
- Modify: `internal/vm/runtime_state.go`
- Modify: `internal/vm/value_aliasing.go`
- Test: `internal/vm/method_test.go`

- [ ] **Step 1: Write the failing type-level test**

Add this test near the existing static ref index tests in `internal/vm/method_test.go`:

```go
func TestStaticValueRefPathIndexTracksNestedMapObjectField(t *testing.T) {
	machine := New(nil)
	target := Object("Provider")
	holder := Map()
	key := mapKey(String("provider"))
	container := Object("Holder")
	container.Fields["value"] = target
	holder.Map[key] = container
	holder.MapKeys[key] = String("provider")
	machine.Classes["Registry"] = Class{
		Name: "Registry",
		StaticFields: map[string]Field{
			"values": {Name: "values", Type: "Map<String,Object>", Value: holder},
		},
	}

	machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()

	paths := machine.staticValueRefPaths[target.Ref].locations()
	if len(paths) != 1 {
		t.Fatalf("target ref paths = %#v, want one", paths)
	}
	if paths[0].Field.ClassName != "Registry" || paths[0].Field.FieldName != "values" {
		t.Fatalf("target ref field = %#v", paths[0].Field)
	}
	if len(paths[0].Path.steps) != 2 {
		t.Fatalf("target ref path = %#v, want map value then object field", paths[0].Path)
	}
}
```

- [ ] **Step 2: Run the test and verify it fails**

Run:

```bash
go test ./internal/vm -run TestStaticValueRefPathIndexTracksNestedMapObjectField -count=1
```

Expected: compile failure because `staticValueRefPaths`, `staticFieldRefPathSet`, or path types do not exist.

- [ ] **Step 3: Add VM field**

In `internal/vm/runtime_state.go`, extend the static ref group:

```go
// --- Static-field reference tracking (alias invalidation) ---
staticValueRefs      map[uint64]bool
staticValueRefFields map[uint64]staticFieldRefSet
staticValueRefPaths  map[uint64]staticFieldRefPathSet
```

- [ ] **Step 4: Add path types**

In `internal/vm/value_aliasing.go`, near `staticFieldRefSet`, add:

```go
type staticValuePathStepKind uint8

const (
	staticValuePathObjectField staticValuePathStepKind = iota + 1
	staticValuePathMapValue
	staticValuePathMapKey
	staticValuePathListIndex
	staticValuePathSetIndex
)

type staticValuePathStep struct {
	Kind  staticValuePathStepKind
	Name  string
	Key   string
	Index int
}

type staticValuePath struct {
	steps []staticValuePathStep
}

type staticFieldRefPath struct {
	Field staticFieldRef
	Path  staticValuePath
}

type staticFieldRefPathSet struct {
	values []staticFieldRefPath
}

func (s *staticFieldRefPathSet) add(location staticFieldRef, path staticValuePath) {
	next := staticFieldRefPath{Field: location, Path: cloneStaticValuePath(path)}
	for _, existing := range s.values {
		if sameStaticFieldRefPath(existing, next) {
			return
		}
	}
	s.values = append(s.values, next)
}

func (s staticFieldRefPathSet) empty() bool {
	return len(s.values) == 0
}

func (s staticFieldRefPathSet) forEach(fn func(staticFieldRefPath)) {
	for _, value := range s.values {
		fn(value)
	}
}

func (s staticFieldRefPathSet) locations() []staticFieldRefPath {
	return append([]staticFieldRefPath(nil), s.values...)
}

func cloneStaticValuePath(path staticValuePath) staticValuePath {
	return staticValuePath{steps: append([]staticValuePathStep(nil), path.steps...)}
}

func sameStaticFieldRefPath(left, right staticFieldRefPath) bool {
	if left.Field != right.Field || len(left.Path.steps) != len(right.Path.steps) {
		return false
	}
	for i := range left.Path.steps {
		if left.Path.steps[i] != right.Path.steps[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 5: Run the test**

Run:

```bash
go test ./internal/vm -run TestStaticValueRefPathIndexTracksNestedMapObjectField -count=1
```

Expected: still fails because paths are not collected yet.

### Task 3: Collect Static Ref Paths

**Files:**
- Modify: `internal/vm/value_aliasing.go`
- Test: `internal/vm/method_test.go`

- [ ] **Step 1: Change collection signatures**

Replace the current collection call shape with:

```go
func (vm *VM) collectStaticValueRefs() (map[uint64]bool, map[uint64]staticFieldRefSet) {
	refs := make(map[uint64]bool)
	fields := make(map[uint64]staticFieldRefSet)
	paths := make(map[uint64]staticFieldRefPathSet)
	seen := make(map[uint64]bool)
	for className, class := range vm.Classes {
		for fieldName, field := range class.StaticFields {
			clearRefSeen(seen)
			collectStaticFieldValueRefs(
				field.Value,
				refs,
				fields,
				paths,
				staticFieldRef{ClassName: className, FieldName: fieldName},
				staticValuePath{},
				seen,
			)
		}
	}
	vm.staticValueRefPaths = paths
	return refs, fields
}
```

This keeps the existing return shape and avoids a larger refactor. The new path index hangs off `vm`.

- [ ] **Step 2: Update `collectStaticFieldValueRefsInField`**

Use this body:

```go
func (vm *VM) collectStaticFieldValueRefsInField(value Value, location staticFieldRef) {
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	defer aliasRefSetPool.Put(seenPtr)
	if vm.staticValueRefPaths == nil {
		vm.staticValueRefPaths = make(map[uint64]staticFieldRefPathSet)
	}
	collectStaticFieldValueRefs(value, vm.staticValueRefs, vm.staticValueRefFields, vm.staticValueRefPaths, location, staticValuePath{}, seen)
}
```

- [ ] **Step 3: Replace recursive collector**

Update `collectStaticFieldValueRefs` to carry the path:

```go
func collectStaticFieldValueRefs(
	value Value,
	refs map[uint64]bool,
	fields map[uint64]staticFieldRefSet,
	paths map[uint64]staticFieldRefPathSet,
	location staticFieldRef,
	path staticValuePath,
	seen map[uint64]bool,
) {
	if value.Ref != 0 {
		if seen[value.Ref] {
			return
		}
		seen[value.Ref] = true
		refs[value.Ref] = true
		locations := fields[value.Ref]
		locations.add(location)
		fields[value.Ref] = locations
		pathSet := paths[value.Ref]
		pathSet.add(location, path)
		paths[value.Ref] = pathSet
	}
	switch value.Kind {
	case ValueObject:
		for name, child := range value.Fields {
			collectStaticFieldValueRefs(child, refs, fields, paths, location, appendStaticValuePath(path, staticValuePathStep{Kind: staticValuePathObjectField, Name: name}), seen)
		}
	case ValueMap:
		for key, child := range value.Map {
			collectStaticFieldValueRefs(child, refs, fields, paths, location, appendStaticValuePath(path, staticValuePathStep{Kind: staticValuePathMapValue, Key: key}), seen)
		}
		for key, child := range value.MapKeys {
			collectStaticFieldValueRefs(child, refs, fields, paths, location, appendStaticValuePath(path, staticValuePathStep{Kind: staticValuePathMapKey, Key: key}), seen)
		}
	case ValueList:
		for i, child := range value.List {
			collectStaticFieldValueRefs(child, refs, fields, paths, location, appendStaticValuePath(path, staticValuePathStep{Kind: staticValuePathListIndex, Index: i}), seen)
		}
	case ValueSet:
		for i, child := range value.Set {
			collectStaticFieldValueRefs(child, refs, fields, paths, location, appendStaticValuePath(path, staticValuePathStep{Kind: staticValuePathSetIndex, Index: i}), seen)
		}
	}
}

func appendStaticValuePath(path staticValuePath, step staticValuePathStep) staticValuePath {
	out := staticValuePath{steps: make([]staticValuePathStep, len(path.steps)+1)}
	copy(out.steps, path.steps)
	out.steps[len(path.steps)] = step
	return out
}
```

- [ ] **Step 4: Fix existing benchmark call sites**

Update `internal/vm/vm_benchmark_test.go` where it calls `collectStaticFieldValueRefs` directly:

```go
paths := make(map[uint64]staticFieldRefPathSet)
collectStaticFieldValueRefs(root, refs, fields, paths, location, staticValuePath{}, make(map[uint64]bool))
```

- [ ] **Step 5: Run path collection tests**

Run:

```bash
go test ./internal/vm -run 'TestStaticValueRef(Path|Index)|TestFindValueByRefFallsBackWhenStaticRefIndexIsStale' -count=1
```

Expected: tests pass.

- [ ] **Step 6: Commit**

Run:

```bash
git add internal/vm/runtime_state.go internal/vm/value_aliasing.go internal/vm/method_test.go internal/vm/vm_benchmark_test.go
git commit -m "test: track static alias paths"
```

### Task 4: Replace Aliases By Exact Static Path

**Files:**
- Modify: `internal/vm/value_aliasing.go`
- Test: `internal/vm/method_test.go`

- [ ] **Step 1: Write failing exact-path replacement test**

Add:

```go
func TestPropagateAliasSnapshotToStaticsUsesNestedMapPath(t *testing.T) {
	machine := New(nil)
	previous := Object("Provider")
	previous.Fields["Name"] = String("old")
	updated := previous
	updated.Fields["Name"] = String("new")

	holder := Map()
	key := mapKey(String("provider"))
	container := Object("Holder")
	container.Fields["value"] = previous
	holder.Map[key] = container
	holder.MapKeys[key] = String("provider")
	machine.Classes["Registry"] = Class{
		Name: "Registry",
		StaticFields: map[string]Field{
			"values": {Name: "values", Type: "Map<String,Object>", Value: holder},
		},
	}

	machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
	machine.propagateAliasSnapshotToStatics(snapshotAlias(previous), updated)

	got := machine.Classes["Registry"].StaticFields["values"].Value.Map[key].Fields["value"]
	if got.Fields["Name"].Text != "new" {
		t.Fatalf("nested static alias name = %q, want new", got.Fields["Name"].Text)
	}
}
```

- [ ] **Step 2: Run the test**

Run:

```bash
go test ./internal/vm -run TestPropagateAliasSnapshotToStaticsUsesNestedMapPath -count=1
```

Expected: pass on behavior already, but it still uses recursive fallback. The next step adds a helper and tests it directly.

- [ ] **Step 3: Write direct helper test**

Add:

```go
func TestReplaceAliasSnapshotAtStaticPathReplacesObjectField(t *testing.T) {
	previous := Object("Provider")
	updated := previous
	updated.Fields["Name"] = String("new")
	holder := Map()
	key := mapKey(String("provider"))
	container := Object("Holder")
	container.Fields["value"] = previous
	holder.Map[key] = container

	path := staticValuePath{steps: []staticValuePathStep{
		{Kind: staticValuePathMapValue, Key: key},
		{Kind: staticValuePathObjectField, Name: "value"},
	}}
	replaced, changed := replaceAliasSnapshotAtStaticPath(holder, path, snapshotAlias(previous), updated)
	if !changed {
		t.Fatal("indexed path did not replace alias")
	}
	got := replaced.Map[key].Fields["value"]
	if got.Fields["Name"].Text != "new" {
		t.Fatalf("replacement name = %q, want new", got.Fields["Name"].Text)
	}
}
```

- [ ] **Step 4: Run the helper test and verify it fails**

Run:

```bash
go test ./internal/vm -run TestReplaceAliasSnapshotAtStaticPathReplacesObjectField -count=1
```

Expected: compile failure because `replaceAliasSnapshotAtStaticPath` does not exist.

- [ ] **Step 5: Implement path replacement helper**

Add:

```go
func replaceAliasSnapshotAtStaticPath(value Value, path staticValuePath, previous aliasSnapshot, updated Value) (Value, bool) {
	if !previous.valid() {
		return value, false
	}
	return replaceAliasSnapshotAtStaticPathStep(value, path.steps, previous, updated)
}

func replaceAliasSnapshotAtStaticPathStep(value Value, steps []staticValuePathStep, previous aliasSnapshot, updated Value) (Value, bool) {
	if len(steps) == 0 {
		if value.Ref == previous.ref && value.Kind == previous.kind {
			return updated, true
		}
		return value, false
	}
	step := steps[0]
	switch step.Kind {
	case staticValuePathObjectField:
		if value.Kind != ValueObject || value.Fields == nil {
			return value, false
		}
		child, ok := value.Fields[step.Name]
		if !ok {
			return value, false
		}
		replaced, changed := replaceAliasSnapshotAtStaticPathStep(child, steps[1:], previous, updated)
		if !changed {
			return value, false
		}
		value.Fields[step.Name] = replaced
		return value, true
	case staticValuePathMapValue:
		if value.Kind != ValueMap || value.Map == nil {
			return value, false
		}
		child, ok := value.Map[step.Key]
		if !ok {
			return value, false
		}
		replaced, changed := replaceAliasSnapshotAtStaticPathStep(child, steps[1:], previous, updated)
		if !changed {
			return value, false
		}
		value.Map[step.Key] = replaced
		return value, true
	case staticValuePathMapKey:
		if value.Kind != ValueMap || value.MapKeys == nil {
			return value, false
		}
		child, ok := value.MapKeys[step.Key]
		if !ok {
			return value, false
		}
		replaced, changed := replaceAliasSnapshotAtStaticPathStep(child, steps[1:], previous, updated)
		if !changed {
			return value, false
		}
		value.MapKeys[step.Key] = replaced
		return value, true
	case staticValuePathListIndex:
		if value.Kind != ValueList || step.Index < 0 || step.Index >= len(value.List) {
			return value, false
		}
		replaced, changed := replaceAliasSnapshotAtStaticPathStep(value.List[step.Index], steps[1:], previous, updated)
		if !changed {
			return value, false
		}
		value.List[step.Index] = replaced
		return value, true
	case staticValuePathSetIndex:
		if value.Kind != ValueSet || step.Index < 0 || step.Index >= len(value.Set) {
			return value, false
		}
		replaced, changed := replaceAliasSnapshotAtStaticPathStep(value.Set[step.Index], steps[1:], previous, updated)
		if !changed {
			return value, false
		}
		value.Set[step.Index] = replaced
		return value, true
	default:
		return value, false
	}
}
```

- [ ] **Step 6: Run helper tests**

Run:

```bash
go test ./internal/vm -run 'TestReplaceAliasSnapshotAtStaticPath|TestPropagateAliasSnapshotToStaticsUsesNestedMapPath' -count=1
```

Expected: pass.

### Task 5: Use Path Index In Static Propagation

**Files:**
- Modify: `internal/vm/value_aliasing.go`
- Test: `internal/vm/method_test.go`

- [ ] **Step 1: Write stale-path fallback test**

Add:

```go
func TestPropagateAliasSnapshotToStaticsFallsBackWhenIndexedPathIsStale(t *testing.T) {
	machine := New(nil)
	previous := Object("Provider")
	previous.Fields["Name"] = String("old")
	updated := previous
	updated.Fields["Name"] = String("new")

	holder := Map()
	oldKey := mapKey(String("old"))
	newKey := mapKey(String("new"))
	holder.Map[oldKey] = Object("Other")
	container := Object("Holder")
	container.Fields["value"] = previous
	holder.Map[newKey] = container
	machine.Classes["Registry"] = Class{
		Name: "Registry",
		StaticFields: map[string]Field{
			"values": {Name: "values", Type: "Map<String,Object>", Value: holder},
		},
	}

	location := staticFieldRef{ClassName: "Registry", FieldName: "values"}
	machine.staticValueRefs = map[uint64]bool{previous.Ref: true}
	machine.staticValueRefFields = map[uint64]staticFieldRefSet{
		previous.Ref: {single: location, hasSingle: true},
	}
	stalePath := staticValuePath{steps: []staticValuePathStep{
		{Kind: staticValuePathMapValue, Key: oldKey},
		{Kind: staticValuePathObjectField, Name: "value"},
	}}
	pathSet := staticFieldRefPathSet{}
	pathSet.add(location, stalePath)
	machine.staticValueRefPaths = map[uint64]staticFieldRefPathSet{previous.Ref: pathSet}

	machine.propagateAliasSnapshotToStatics(snapshotAlias(previous), updated)

	got := machine.Classes["Registry"].StaticFields["values"].Value.Map[newKey].Fields["value"]
	if got.Fields["Name"].Text != "new" {
		t.Fatalf("fallback replacement name = %q, want new", got.Fields["Name"].Text)
	}
}
```

- [ ] **Step 2: Run stale-path test**

Run:

```bash
go test ./internal/vm -run TestPropagateAliasSnapshotToStaticsFallsBackWhenIndexedPathIsStale -count=1
```

Expected: pass before optimization because the old recursive path works. It must still pass after optimization.

- [ ] **Step 3: Add path-first propagation helper**

Add:

```go
func (vm *VM) propagateAliasSnapshotToStaticPath(previous aliasSnapshot, updated Value, indexed staticFieldRefPath) bool {
	class, ok := vm.Classes[indexed.Field.ClassName]
	if !ok || class.StaticFields == nil {
		return false
	}
	field, ok := class.StaticFields[indexed.Field.FieldName]
	if !ok {
		return false
	}
	replaced, changed := replaceAliasSnapshotAtStaticPath(field.Value, indexed.Path, previous, updated)
	if !changed {
		return false
	}
	field.Value = replaced
	class.StaticFields[indexed.Field.FieldName] = field
	vm.Classes[indexed.Field.ClassName] = class
	vm.rememberAdditionalStaticValueRefsInField(updated, indexed.Field)
	return true
}
```

- [ ] **Step 4: Update `propagateAliasSnapshotToStatics`**

At the start of the hit path, after `locations.empty()` check, try indexed paths:

```go
if vm.staticValueRefPaths != nil {
	if paths := vm.staticValueRefPaths[previous.ref]; !paths.empty() {
		handled := true
		paths.forEach(func(indexed staticFieldRefPath) {
			if !handled {
				return
			}
			if !vm.propagateAliasSnapshotToStaticPath(previous, updated, indexed) {
				handled = false
				return
			}
		})
		if handled {
			return
		}
	}
}
```

Then keep the existing recursive `locations.forEach` block unchanged as fallback. If the path index is stale, the old walk repairs behavior.

- [ ] **Step 5: Run focused static tests**

Run:

```bash
go test ./internal/vm -run 'TestStaticValueRef|TestPropagateAliasSnapshotToStatics|TestFindValueByRefFallsBackWhenStaticRefIndexIsStale' -count=1
```

Expected: pass.

- [ ] **Step 6: Run method alias tests**

Run:

```bash
go test ./internal/vm -run 'TestExecMethodParameterMapPropagatesNestedCollectionAliases|TestExecListMutationPropagatesThroughMapAlias|TestExecNamespacedStaticSingletonAliasesShareInstanceState' -count=1
```

Expected: pass.

### Task 6: Keep Path Index Correct When Fields Change

**Files:**
- Modify: `internal/vm/value_aliasing.go`
- Test: `internal/vm/method_test.go`

- [ ] **Step 1: Extend invalidation helpers**

Update:

```go
func (vm *VM) invalidateStaticValueRefs() {
	vm.staticValueRefs = nil
	vm.staticValueRefFields = nil
	vm.staticValueRefPaths = nil
}
```

- [ ] **Step 2: Add path removal helpers**

Add:

```go
func (vm *VM) forgetStaticValueRefPathInField(ref uint64, location staticFieldRef) {
	if vm.staticValueRefPaths == nil || ref == 0 {
		return
	}
	paths := vm.staticValueRefPaths[ref]
	if paths.empty() {
		return
	}
	filtered := paths.values[:0]
	for _, path := range paths.values {
		if path.Field != location {
			filtered = append(filtered, path)
		}
	}
	if len(filtered) == 0 {
		delete(vm.staticValueRefPaths, ref)
		return
	}
	paths.values = filtered
	vm.staticValueRefPaths[ref] = paths
}
```

Update `forgetStaticValueRefInField` so it also calls:

```go
vm.forgetStaticValueRefPathInField(ref, location)
```

- [ ] **Step 3: Ensure full static field replacement rebuilds paths**

`rememberStaticValueRefsInField`, `replaceStaticValueRefsInField`, and `forgetStaticValueRefsFromValue` must affect `staticValueRefPaths` through `forgetStaticValueRefInField` and `collectStaticFieldValueRefsInField`. Do not leave a path for a ref that no longer exists in the field.

- [ ] **Step 4: Write replacement cleanup test**

Add:

```go
func TestStaticValueRefPathIndexReplacesSingleStaticField(t *testing.T) {
	machine := New(nil)
	oldValue := Object("Old")
	newValue := Object("New")
	target := staticFieldRef{ClassName: "TrailRegistry", FieldName: "values"}
	machine.staticValueRefs = map[uint64]bool{oldValue.Ref: true}
	machine.staticValueRefFields = map[uint64]staticFieldRefSet{
		oldValue.Ref: {single: target, hasSingle: true},
	}
	pathSet := staticFieldRefPathSet{}
	pathSet.add(target, staticValuePath{})
	machine.staticValueRefPaths = map[uint64]staticFieldRefPathSet{oldValue.Ref: pathSet}

	machine.replaceStaticValueRefsInField(oldValue, newValue, target)

	if machine.staticValueRefPaths[oldValue.Ref].locations() != nil {
		t.Fatalf("old ref path still present: %#v", machine.staticValueRefPaths[oldValue.Ref].locations())
	}
	if got := machine.staticValueRefPaths[newValue.Ref].locations(); len(got) != 1 || got[0].Field != target {
		t.Fatalf("new ref paths = %#v, want target", got)
	}
}
```

- [ ] **Step 5: Run path-index tests**

Run:

```bash
go test ./internal/vm -run 'TestStaticValueRefPath|TestStaticValueRefCache|TestStaticValueRefIndex' -count=1
```

Expected: pass.

### Task 7: Add Benchmark And Measure Local Speed

**Files:**
- Modify: `internal/vm/vm_benchmark_test.go`

- [ ] **Step 1: Add benchmark**

Add:

```go
func BenchmarkPropagateAliasSnapshotToStaticsNestedMapPaths(b *testing.B) {
	const entries = 200
	machine := New(nil)
	holder := Map()
	targetKey := ""
	var previous Value
	for i := 0; i < entries; i++ {
		request := Object("ProductPricingRequest")
		request.Fields["Quantity"] = Int(int64(i))
		if i == entries/2 {
			previous = request
		}
		container := Object("PriceableLine")
		container.Fields["request"] = request
		key := mapKey(String(fmt.Sprintf("line-%d", i)))
		holder.Map[key] = container
		holder.MapKeys[key] = String(fmt.Sprintf("line-%d", i))
		if i == entries/2 {
			targetKey = key
		}
	}
	updated := previous
	updated.Fields["Quantity"] = Int(999)
	machine.Classes["PriceableOrder"] = Class{
		Name: "PriceableOrder",
		StaticFields: map[string]Field{
			"requestByLine": {Name: "requestByLine", Type: "Map<OrderLine,ProductPricingRequest>", Value: holder},
		},
	}
	machine.staticValueRefs, machine.staticValueRefFields = machine.collectStaticValueRefs()
	snapshot := snapshotAlias(previous)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		field := machine.Classes["PriceableOrder"].StaticFields["requestByLine"]
		container := field.Value.Map[targetKey]
		container.Fields["request"] = previous
		field.Value.Map[targetKey] = container
		class := machine.Classes["PriceableOrder"]
		class.StaticFields["requestByLine"] = field
		machine.Classes["PriceableOrder"] = class
		machine.propagateAliasSnapshotToStatics(snapshot, updated)
	}
}
```

Add `fmt` to the benchmark file imports if it is not already present.

- [ ] **Step 2: Run benchmark**

Run:

```bash
go test ./internal/vm -run '^$' -bench BenchmarkPropagateAliasSnapshotToStaticsNestedMapPaths -benchmem -count=5
```

Expected: benchmark passes. Compare against baseline from Task 1 or a temporary run before path-first propagation.

- [ ] **Step 3: Run VM test package**

Run:

```bash
go test ./internal/vm
```

Expected: pass.

- [ ] **Step 4: Commit**

Run:

```bash
git add internal/vm/runtime_state.go internal/vm/value_aliasing.go internal/vm/method_test.go internal/vm/vm_benchmark_test.go
git commit -m "perf: index static alias paths"
```

### Task 8: Optional Hidden Alias Perf Counters

**Files:**
- Optional create: `internal/vm/alias_perf.go`
- Optional modify: `internal/apextest/runner.go`
- Optional modify: `internal/gladecli/test_command.go`

Only do this task if the path index is still hard to validate from pprof and benchmark data. If implemented, it must stay behind `GLADE_ALIAS_PERF=1`.

- [ ] **Step 1: Add hidden counter package file**

Create `internal/vm/alias_perf.go` with counters for:

```go
type AliasPerfCounters struct {
	Enabled                         bool              `json:"enabled,omitempty"`
	StaticSnapshotCalls             uint64            `json:"staticSnapshotCalls,omitempty"`
	StaticSnapshotNs                int64             `json:"staticSnapshotNs,omitempty"`
	StaticSnapshotRefMiss           uint64            `json:"staticSnapshotRefMiss,omitempty"`
	StaticSnapshotLocationVisits    uint64            `json:"staticSnapshotLocationVisits,omitempty"`
	StaticSnapshotTopLevelLocations uint64            `json:"staticSnapshotTopLevelLocations,omitempty"`
	StaticSnapshotDeepLocations     uint64            `json:"staticSnapshotDeepLocations,omitempty"`
	StaticSnapshotPathHits          uint64            `json:"staticSnapshotPathHits,omitempty"`
	StaticSnapshotPathMisses        uint64            `json:"staticSnapshotPathMisses,omitempty"`
	StaticSnapshotFields            []AliasPerfField  `json:"staticSnapshotFields,omitempty"`
	UpdatedValueAliasesCalls        uint64            `json:"updatedValueAliasesCalls,omitempty"`
	UpdatedValueAliasesWalks        uint64            `json:"updatedValueAliasesWalks,omitempty"`
	UpdatedValueAliasesAssignments  uint64            `json:"updatedValueAliasesAssignments,omitempty"`
}
```

Use `os.Getenv("GLADE_ALIAS_PERF") != ""` as the only activation switch.

- [ ] **Step 2: Wire reset and snapshot**

In `internal/apextest/runner.go`, call:

```go
vm.ResetAliasPerfCounters()
```

right after `ResetPerfCounters()`.

In `internal/gladecli/test_command.go`, add:

```go
AliasPerf vm.AliasPerfCounters `json:"aliasPerf,omitempty"`
```

to `runPerfSummary`, and set:

```go
AliasPerf: vm.SnapshotAliasPerfCounters(),
```

in `maybeWriteRunPerfJSON`.

- [ ] **Step 3: Run focused CLI test**

Run:

```bash
go test ./internal/gladecli -run TestRunTestJSONAndJUnit -count=1
```

Expected: pass.

- [ ] **Step 4: Decide keep or remove**

If counters are not needed after proof, remove this task's code before final gates. If kept, include it in the commit and mention the hidden env var in the final handoff.

### Task 9: Forced-Build Candidate Timing

**Files:**
- No new source changes.

- [ ] **Step 1: Build candidate binary**

Run:

```bash
GOCACHE=/tmp/glade-go-build-alias-path-candidate \
GOMAXPROCS=4 \
go build -a -o /tmp/glade-alias-path-candidate ./cmd/glade
```

Expected: exit code `0`.

- [ ] **Step 2: Run focused NAMS methods**

Run the two focused commands from the Non-Negotiable Test Gates section.

Expected:

```bash
jq '.summary.failed, .summary.compileErrors, .summary.runtimeErrors, .summary.errors' \
  /tmp/glade-alias-path/imported.result.json \
  /tmp/glade-alias-path/purchased.result.json
```

prints only zeroes.

- [ ] **Step 3: Compare timing**

Run:

```bash
jq '.tests[0] | {name, status, durationMs}' \
  /tmp/glade-alias-path/baseline-imported.result.json \
  /tmp/glade-alias-path/imported.result.json \
  /tmp/glade-alias-path/baseline-purchased.result.json \
  /tmp/glade-alias-path/purchased.result.json
```

Expected: imported method is at least 15 percent or 30 seconds faster. Purchased method has no more than 5 percent regression.

- [ ] **Step 4: If timing fails, revert the optimization commit**

Run only if timing fails:

```bash
git revert --no-edit HEAD
go test ./internal/vm
```

Expected: revert succeeds and `internal/vm` passes.

### Task 10: Full Verification

**Files:**
- No source changes unless a failure is found.

- [ ] **Step 1: Run Go gates**

Run:

```bash
go test ./internal/vm
go test ./internal/vm ./internal/apextest ./internal/gladecli
go test ./...
```

Expected: all pass.

- [ ] **Step 2: Run full external project gates**

Run the three full external project commands from the Non-Negotiable Test Gates section.

Expected: all command exit codes are `0`.

- [ ] **Step 3: Verify JSON summaries**

Run:

```bash
jq '.summary | {total, passed, failed, compileErrors, runtimeErrors, unsupported, errors, durationMs}' \
  /tmp/glade-alias-path/nams-full.result.json \
  /tmp/glade-alias-path/external-b-full.result.json \
  /tmp/glade-alias-path/external-c-full.result.json
```

Expected: `failed`, `compileErrors`, `runtimeErrors`, and `errors` are `0` for every project. `passed` equals `total` unless the project already has explicit skipped tests; any skipped or unsupported count must be explained from current output.

- [ ] **Step 4: Check diff hygiene**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors. Status shows only intended files.

- [ ] **Step 5: Commit final proof update when files changed**

If benchmark or docs files were adjusted after the optimization commit:

```bash
git add internal/vm/runtime_state.go internal/vm/value_aliasing.go internal/vm/method_test.go internal/vm/vm_benchmark_test.go
git commit -m "test: cover static alias path index"
```

Expected: commit succeeds.

## Rollback Rules

Revert the path-index implementation if any of these happen:

- A focused alias/static test fails and the fix requires weakening assertions.
- Full NAMS or additional external corpus gates fail because of changed runtime behavior.
- The imported method does not improve by at least 15 percent or 30 seconds.
- The purchased method regresses by more than 5 percent.
- The implementation requires any NAMS-specific or package-specific branch in VM code.

## Review Checklist

Before calling this complete:

- [ ] No project-specific behavior was added.
- [ ] Stale indexed paths fall back to the old recursive walk.
- [ ] Static field writes update or invalidate `staticValueRefPaths`.
- [ ] Map values, map keys, object fields, list indexes, and set indexes have path tests.
- [ ] `go test ./...` passed.
- [ ] Focused imported and purchased NAMS methods passed.
- [ ] Full NAMS and additional external corpus gates passed.
- [ ] Final report includes before/after durations and the exact commands used.

# Glade Performance Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Cut cold command startup, Apex local-test wall time, sema/LSP allocation load, and enterprise-suite runtime without weakening Salesforce behavior or test isolation.

**Architecture:** Start with measured ratchets, then remove unnecessary work before changing language or runtime shape. Keep product runtime in Go. Permit only narrow native/Rust spikes after Go-side changes and only when the spike proves at least 10 percent end-to-end gain on an enterprise sentinel.

**Tech Stack:** Go 1.26, tree-sitter Apex via cgo, `pprof`, `/usr/bin/time -l`, `GODEBUG=inittrace=1`, existing `internal/apextest` perf counters, existing local-test JSON artifacts.

---

## Current Evidence

These numbers came from the June 12, 2026 deep dive in `/Users/matt/Dev/glade`.

- `go build -o /tmp/glade-perf-probe ./cmd/glade` succeeded.
- `/usr/bin/time -l /tmp/glade-perf-probe version`: `2.29 real`, `356417536` max RSS.
- `/usr/bin/time -l /tmp/glade-perf-probe doctor`: `2.21 real`, `360628224` max RSS.
- `GODEBUG=inittrace=1 /tmp/glade-perf-probe version`: `internal/vm` init took `1306 ms` and allocated `588388752` bytes.
- `BenchmarkRunTestSuiteWithClassSetup`: `9672380125 ns/op`, `3234657224 B/op`, `18263788 allocs/op`.
- `go tool pprof -top -alloc_space /tmp/glade-apextest.mem`: top allocation frames included `copyFieldMap`, `copyClassMapWithPlan`, `schemaCacheStampForOrg`, `io.ReadAll`, `CloneRuntimeFrozenShared`, `collectStaticFieldValueRefs`, and standard object merge.
- `BenchmarkAnalyzeIndex`: `693334042 ns/op`, `532808504 B/op`, `6008191 allocs/op`.
- `BenchmarkWorkspaceSymbols`: `669236458 ns/op`, `531833480 B/op`, `5989305 allocs/op`.
- Saved large-suite JSONs show the scale: `test-results/nu.json` has `11526` passing tests and `11587827` durationMs; `test-results/nams.json` has `5723` passing tests and `2918521` durationMs; `test-results/sf-cred.json` has `4565` passing tests and `2097744` durationMs.

## Rules

- Do not trade Salesforce behavior for speed.
- Do not weaken Apex test isolation.
- Do not add project-specific shortcuts.
- Do not infer field behavior from field names.
- Do not keep an optimization without before/after timing and correctness gates.
- Do not rewrite the VM in Rust or another language.
- A narrow native/Rust spike is allowed only after Go-side work, and only when it can prove a 10 percent or better end-to-end gain on `nu`, `nams`, or `sf-cred` sentinel targets.

## File Map

- `scripts/perf/glade-baseline.sh`: new repeatable baseline harness for cold startup, microbenchmarks, and saved-artifact summaries.
- `scripts/perf/localtest-targets.sh`: new helper that extracts slow classes and methods from saved local-test JSON artifacts.
- `internal/gladecli/startup_test.go`: new guard tests for light commands and package initialization.
- `internal/vm/platform_index.go`: lazy platform type and method index accessors.
- `internal/vm/vm.go`: remove eager platform index construction from `init`.
- `internal/vm/*`: replace direct generated index map reads with helper accessors.
- `internal/storage/runtime_template.go`: carry a safe schema stamp with runtime templates, if the measured stamp cache task earns its gate.
- `internal/storage/model.go`: preserve and clear schema-stamp metadata only through safe clone and mutation paths.
- `internal/vm/runtime_state.go`: avoid recomputing schema stamps when a trusted runtime-template stamp exists.
- `internal/apextest/perf_counters.go`: add timing counters for clone, setup, run, schema stamp, and VM clone phases.
- `internal/apextest/runner.go`: expose phase timings through existing perf JSON, then use them to target clone/setup work.
- `internal/sema/sema.go`: split full semantic analysis from lightweight symbol needs.
- `internal/lsp/handler.go`: avoid full sema work for workspace-symbol startup when diagnostics are not required.
- `docs/superpowers/plans/2026-06-12-glade-performance-optimization-plan.md`: this plan.

---

## Task 1: Establish Repeatable Performance Gates

**Files:**
- Create: `scripts/perf/glade-baseline.sh`
- Create: `scripts/perf/localtest-targets.sh`
- Modify: `.gitignore` only if new generated outputs land under the repo.

- [ ] **Step 1: Create the baseline harness**

Create `scripts/perf/glade-baseline.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

out_dir="${1:-/tmp/glade-perf-baseline}"
mkdir -p "$out_dir"

bin="$out_dir/glade"
echo "building $bin"
go build -o "$bin" ./cmd/glade

run_time() {
  local name="$1"
  shift
  echo "== $name =="
  /usr/bin/time -l "$@" >"$out_dir/$name.stdout" 2>"$out_dir/$name.time"
  cat "$out_dir/$name.time"
}

run_time version "$bin" version
run_time doctor "$bin" doctor

GODEBUG=inittrace=1 "$bin" version >"$out_dir/version.init.stdout" 2>"$out_dir/version.inittrace"

go test -run '^$' -bench=BenchmarkRunTestSuiteWithClassSetup -benchmem -benchtime=1x \
  -cpuprofile "$out_dir/apextest.cpu" -memprofile "$out_dir/apextest.mem" ./internal/apextest \
  | tee "$out_dir/apextest.bench"

go test -run '^$' -bench=BenchmarkAnalyzeIndex -benchmem -benchtime=1x \
  -cpuprofile "$out_dir/sema.cpu" -memprofile "$out_dir/sema.mem" ./internal/sema \
  | tee "$out_dir/sema.bench"

go test -run '^$' -bench=BenchmarkWorkspaceSymbols -benchmem -benchtime=1x \
  -cpuprofile "$out_dir/lsp.cpu" -memprofile "$out_dir/lsp.mem" ./internal/lsp \
  | tee "$out_dir/lsp.bench"

go tool pprof -top -alloc_space "$out_dir/apextest.mem" >"$out_dir/apextest.alloc.top"
go tool pprof -top -cum "$out_dir/apextest.cpu" >"$out_dir/apextest.cpu.cum"
go tool pprof -top -alloc_space "$out_dir/sema.mem" >"$out_dir/sema.alloc.top"
go tool pprof -top -alloc_space "$out_dir/lsp.mem" >"$out_dir/lsp.alloc.top"

echo "wrote $out_dir"
```

- [ ] **Step 2: Create the saved-artifact slow target extractor**

Create `scripts/perf/localtest-targets.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -lt 1 ]]; then
  echo "usage: $0 test-results/<run>.json [limit]" >&2
  exit 64
fi

file="$1"
limit="${2:-12}"

jq -r --argjson limit "$limit" '
  [.suites[].cases[]]
  | group_by(.className)
  | map({
      class: .[0].className,
      count: length,
      total: (map(.durationMs) | add),
      max: (map(.durationMs) | max),
      slowMethod: (max_by(.durationMs).methodName)
    })
  | sort_by(-.total)
  | .[:$limit][]
  | [.class, .count, .total, .max, .slowMethod]
  | @tsv
' "$file"
```

- [ ] **Step 3: Make scripts executable**

Run:

```bash
chmod +x scripts/perf/glade-baseline.sh scripts/perf/localtest-targets.sh
```

Expected: no output.

- [ ] **Step 4: Run the baseline**

Run:

```bash
scripts/perf/glade-baseline.sh /tmp/glade-perf-before
scripts/perf/localtest-targets.sh test-results/nu.json 10
scripts/perf/localtest-targets.sh test-results/nams.json 10
scripts/perf/localtest-targets.sh test-results/sf-cred.json 10
```

Expected:

- `/tmp/glade-perf-before/version.time` exists.
- `/tmp/glade-perf-before/version.inittrace` names `internal/vm`.
- Local-test target output includes the class, method count, class total ms, max method ms, and slowest method.

- [ ] **Step 5: Commit**

```bash
git add scripts/perf/glade-baseline.sh scripts/perf/localtest-targets.sh
git commit -m "chore: add glade performance baseline harness"
```

---

## Task 2: Stop Cold Commands From Building VM Platform Indexes

**Purpose:** `version`, `doctor`, help, completion, and other light commands should not pay for full platform runtime indexes.

**Files:**
- Modify: `internal/vm/vm.go`
- Modify: `internal/vm/platform_index.go`
- Modify direct generated-index readers in `internal/vm/*.go`
- Modify tests that monkey-patch `generatedPlatformMethodIndex` in `internal/vm/vm_test.go`
- Create: `internal/gladecli/startup_test.go`

- [ ] **Step 1: Add a cold-start regression test**

Create `internal/gladecli/startup_test.go`:

```go
package gladecli

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestLightCommandsReturnWithoutProjectRuntime(t *testing.T) {
	for _, args := range [][]string{
		{"version"},
		{"completion", "bash"},
		{"help"},
	} {
		t.Run(args[0], func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			start := time.Now()
			code := Run(context.Background(), args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("Run(%v) code = %d stderr=%s", args, code, stderr.String())
			}
			if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
				t.Fatalf("Run(%v) took %s, want under 500ms in-process", args, elapsed)
			}
		})
	}
}
```

Run:

```bash
go test -run TestLightCommandsReturnWithoutProjectRuntime ./internal/gladecli
```

Expected before the implementation may pass in-process because package init has already run before the test timer. Keep this test as a behavior guard. The real cold-start gate is the shell timing in Step 5.

- [ ] **Step 2: Replace eager VM init with `sync.Once` accessors**

In `internal/vm/vm.go`, replace the eager globals and init body with lazy storage:

```go
var standardSObjectPrefixes = map[string]string{
	"001": "Account",
	"003": "Contact",
	"005": "User",
	"006": "Opportunity",
	"00G": "Group",
	"00Q": "Lead",
	"00T": "Task",
	"00U": "Event",
	"00D": "Organization",
	"500": "Case",
	"701": "Campaign",
}
```

Then move the platform index globals out of `vm.go` and into `internal/vm/platform_index.go`.

- [ ] **Step 3: Add lazy accessors**

In `internal/vm/platform_index.go`, add this shape near the top:

```go
var platformIndexState struct {
	commonOnce sync.Once
	typeOnce   sync.Once
	methodOnce sync.Once

	commonNames []string
	types       map[string]generatedPlatformType
	methods     map[string]map[string][]Method
}

func CommonSObjectTypeNames() []string {
	platformIndexState.commonOnce.Do(func() {
		platformIndexState.commonNames = buildCommonSObjectTypeNames()
	})
	return platformIndexState.commonNames
}

func generatedPlatformTypes() map[string]generatedPlatformType {
	platformIndexState.typeOnce.Do(func() {
		platformIndexState.types = buildGeneratedPlatformTypeIndex()
	})
	return platformIndexState.types
}

func generatedPlatformMethods() map[string]map[string][]Method {
	platformIndexState.methodOnce.Do(func() {
		platformIndexState.methods = buildGeneratedPlatformMethodIndex()
	})
	return platformIndexState.methods
}
```

Add `sync` to the import list.

- [ ] **Step 4: Replace direct map reads**

Replace reads of:

```go
generatedPlatformTypeIndex
generatedPlatformMethodIndex
commonSObjectTypeNames
```

with:

```go
generatedPlatformTypes()
generatedPlatformMethods()
CommonSObjectTypeNames()
```

Do this in these files first:

- `internal/vm/vm.go`
- `internal/vm/class_lookup.go`
- `internal/vm/generated_platform_runtime.go`
- `internal/vm/json_runtime.go`
- `internal/vm/method_dispatch.go`
- `internal/vm/platform_metadata_reports.go`
- `internal/vm/platform_object_members.go`
- `internal/vm/platform_passive_members.go`

- [ ] **Step 5: Preserve test injection for generated method lookup**

`internal/vm/vm_test.go` currently swaps `generatedPlatformMethodIndex`. Replace that with a test helper:

```go
func replaceGeneratedPlatformMethodsForTest(t *testing.T, methods map[string]map[string][]Method) {
	t.Helper()
	original := platformIndexState.methods
	originalOnce := platformIndexState.methodOnce
	platformIndexState.methods = methods
	platformIndexState.methodOnce = sync.Once{}
	platformIndexState.methodOnce.Do(func() {})
	t.Cleanup(func() {
		platformIndexState.methods = original
		platformIndexState.methodOnce = originalOnce
	})
}
```

If `sync.Once` state proves awkward to restore, use an unexported package-level override:

```go
var generatedPlatformMethodsForTest map[string]map[string][]Method
```

and let `generatedPlatformMethods()` return it only when non-nil. Keep that path under tests only if possible.

- [ ] **Step 6: Run cold startup gate**

Run:

```bash
scripts/perf/glade-baseline.sh /tmp/glade-perf-lazy-platform
grep 'internal/vm' /tmp/glade-perf-lazy-platform/version.inittrace
```

Expected:

- `internal/vm` init no longer builds platform type and method indexes.
- `version` cold time drops at least 50 percent from `/tmp/glade-perf-before/version.time`.
- `version` max RSS drops at least 50 percent from `/tmp/glade-perf-before/version.time`.

If the 50 percent gate fails, inspect `version.inittrace` and keep cutting eager package init until the source is named.

- [ ] **Step 7: Run correctness gates**

Run:

```bash
go test ./internal/vm ./internal/typesys ./internal/sema ./internal/gladecli
scripts/smoke.sh
```

Expected: all pass. If `scripts/smoke.sh` is too broad for the branch, at minimum run the command surfaces that touch runtime:

```bash
go test ./internal/apextest ./internal/soql ./internal/dml ./internal/storage
```

- [ ] **Step 8: Commit**

```bash
git add internal/vm internal/gladecli/startup_test.go
git commit -m "perf: lazy-build VM platform indexes"
```

---

## Task 3: Add Apex Test Phase Timing So Hot Work Is Not Guessed

**Purpose:** Record clone/setup/run/stamp work per class without spamming normal output.

**Files:**
- Modify: `internal/apextest/perf_counters.go`
- Modify: `internal/apextest/runner.go`
- Modify: `internal/apextest/runner_test.go`
- Modify JSON output only if a current perf JSON field already exists.

- [ ] **Step 1: Extend perf counters**

In `internal/apextest/perf_counters.go`, add fields:

```go
SetupDurationMS       int64 `json:"setupDurationMs"`
RunDurationMS         int64 `json:"runDurationMs"`
CloneRuntimeOrgMS     int64 `json:"cloneRuntimeOrgMs"`
CloneRuntimeMachineMS int64 `json:"cloneRuntimeMachineMs"`
```

Add atomic counters beside the existing fields:

```go
setupDurationMS       atomic.Int64
runDurationMS         atomic.Int64
cloneRuntimeOrgMS     atomic.Int64
cloneRuntimeMachineMS atomic.Int64
```

Add recorder helpers:

```go
func recordSetupDuration(d time.Duration) {
	perfCounters.setupDurationMS.Add(d.Milliseconds())
}

func recordRunDuration(d time.Duration) {
	perfCounters.runDurationMS.Add(d.Milliseconds())
}

func recordCloneRuntimeOrgDuration(d time.Duration) {
	perfCounters.cloneRuntimeOrgMS.Add(d.Milliseconds())
}

func recordCloneRuntimeMachineDuration(d time.Duration) {
	perfCounters.cloneRuntimeMachineMS.Add(d.Milliseconds())
}
```

- [ ] **Step 2: Wrap clone and setup timing**

In `internal/apextest/runner.go`, wrap `cloneRuntimeOrgForClass`:

```go
func cloneRuntimeOrgForClass(org storage.OrgState, className, phase string) storage.OrgState {
	started := time.Now()
	defer func() { recordCloneRuntimeOrgDuration(time.Since(started)) }()
	recordCloneRuntimeOrg(className, phase)
	return org.CloneRuntimeFrozenShared()
}
```

In `prepareTestSetupOrg`, record the whole function duration:

```go
started := time.Now()
defer func() { recordSetupDuration(time.Since(started)) }()
```

In `runCase`, record the method run duration:

```go
started := time.Now()
defer func() {
	duration := time.Since(started)
	out.DurationMS = duration.Milliseconds()
	recordRunDuration(duration)
}()
```

In the two places that call `baseMachine.CloneRuntime(nil)`, wrap timing with `recordCloneRuntimeMachineDuration`.

- [ ] **Step 3: Reset and snapshot the new counters**

Update `ResetPerfCounters()` and `SnapshotPerfCounters()` so all four new counters reset and report.

- [ ] **Step 4: Test counter behavior**

Add or extend a focused test in `internal/apextest/runner_test.go`:

```go
func TestPerfCountersIncludePhaseDurations(t *testing.T) {
	ResetPerfCounters()
	t.Cleanup(ResetPerfCounters)

	root := t.TempDir()
	writeApexTestProject(t, root, map[string]string{
		"CounterTest.cls": `
@isTest
private class CounterTest {
  @TestSetup static void setupData() {
    insert new Account(Name = 'Acme');
  }
  @isTest static void runs() {
    System.assertEquals(1, [SELECT count() FROM Account]);
  }
}
`,
	})

	index := loadTestIndex(t, root)
	run := Run(index, Options{})
	if run.Summary().Passed != 1 {
		t.Fatalf("summary = %#v", run.Summary())
	}
	stats := SnapshotPerfCounters()
	if stats.CloneRuntimeOrgCalls == 0 {
		t.Fatalf("CloneRuntimeOrgCalls = 0")
	}
	if stats.SetupDurationMS == 0 {
		t.Fatalf("SetupDurationMS = 0")
	}
	if stats.RunDurationMS == 0 {
		t.Fatalf("RunDurationMS = 0")
	}
}
```

Use the existing project-writing helpers in `runner_test.go`. Do not invent a second fixture harness if one already exists.

- [ ] **Step 5: Run tests and baseline**

Run:

```bash
go test ./internal/apextest
scripts/perf/glade-baseline.sh /tmp/glade-perf-phase-counters
```

Expected: tests pass. Benchmark time should not move more than 2 percent compared with `/tmp/glade-perf-before`; if it does, reduce counter overhead.

- [ ] **Step 6: Commit**

```bash
git add internal/apextest
git commit -m "perf: expose Apex test phase counters"
```

---

## Task 4: Cut Repeated Schema Stamp Work With a Trusted Template Stamp

**Purpose:** `schemaCacheStampForOrg` is hot in setup and run. It sorts and hashes schema maps for every `SetOrg`. Use a trusted stamp only for frozen runtime-template clones.

**Files:**
- Modify: `internal/storage/runtime_template.go`
- Modify: `internal/storage/model.go`
- Modify: `internal/vm/runtime_state.go`
- Modify: `internal/storage/runtime_template_test.go`
- Modify: `internal/vm/platform_test.go` or a focused VM describe-cache test.

- [ ] **Step 1: Add a runtime schema stamp field**

In `internal/storage/model.go`, add an unexported field to `OrgState`:

```go
RuntimeSchemaStamp string
```

This field is intentionally not JSON-facing. It is trusted only when created by `RuntimeTemplate`.

- [ ] **Step 2: Let RuntimeTemplate carry the stamp**

In `internal/storage/runtime_template.go`, extend `RuntimeTemplate`:

```go
type RuntimeTemplate struct {
	Org               OrgState
	RuntimeSchemaStamp string
}
```

Add a constructor parameter through `NewRuntimeTemplate`:

```go
func NewRuntimeTemplate(org OrgState) RuntimeTemplate {
	return RuntimeTemplate{
		Org: org,
		RuntimeSchemaStamp: ComputeRuntimeSchemaStamp(org),
	}
}
```

Do not call into `internal/vm` from `internal/storage`. Put `ComputeRuntimeSchemaStamp` in `internal/storage` or a new neutral package only if the import graph stays clean. If that is too broad, use a VM-owned `TemplateSchemaStamp` map keyed by template pointer instead.

- [ ] **Step 3: Keep mutation safe**

Any path that mutates object definitions on a runtime org must clear `RuntimeSchemaStamp`.

Add this helper in `internal/storage/model.go`:

```go
func (o *OrgState) ClearRuntimeSchemaStamp() {
	if o != nil {
		o.RuntimeSchemaStamp = ""
	}
}
```

Call it from:

- `EnsureMutableObjectDefinition`
- `EnsureStandardObject` when it writes `state.Definition`
- project setup paths in `internal/apextest/runner.go` that assign `state.Definition`
- VM metadata paths in `internal/vm/platform_metadata_reports.go` that clone and mutate definitions

- [ ] **Step 4: Use trusted stamp in VM SetOrg**

In `internal/vm/runtime_state.go`, change `SetOrg` so it uses the trusted stamp when present:

```go
nextStamp := ""
if org != nil && strings.TrimSpace(org.RuntimeSchemaStamp) != "" {
	nextStamp = org.RuntimeSchemaStamp
} else {
	nextStamp = schemaCacheStampForOrg(org)
}
```

Keep the existing cache-clearing behavior when stamps differ.

- [ ] **Step 5: Add mutation safety tests**

In `internal/storage/runtime_template_test.go`, add:

```go
func TestRuntimeTemplateSchemaStampClearsOnMutableDefinition(t *testing.T) {
	org := benchmarkOrgState(2, 1)
	template := NewRuntimeTemplate(org)
	clone := template.CloneRuntimeOrg()
	if clone.RuntimeSchemaStamp == "" {
		t.Fatalf("RuntimeSchemaStamp is empty")
	}
	object, cloned := EnsureMutableObjectDefinition(&clone, "PerfObject0__c")
	if !cloned {
		t.Fatalf("expected mutable definition clone")
	}
	object.Definition.Fields["RuntimeOnly__c"] = Field{APIName: "RuntimeOnly__c", Type: FieldString}
	clone.Objects["PerfObject0__c"] = *object
	if clone.RuntimeSchemaStamp != "" {
		t.Fatalf("RuntimeSchemaStamp should clear after definition mutation")
	}
}
```

In a VM test, prove describe caches clear after metadata mutation:

```go
func TestSetOrgTrustedSchemaStampDoesNotHideDefinitionMutation(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	template := storage.NewRuntimeTemplate(org)
	clone := template.CloneRuntimeOrg()
	machine := New(nil)
	machine.SetOrg(&clone)

	object, _ := storage.EnsureMutableObjectDefinition(&clone, "Account")
	object.Definition.Fields["RuntimeOnly__c"] = storage.Field{APIName: "RuntimeOnly__c", Type: storage.FieldString}
	clone.Objects["Account"] = *object
	machine.SetOrg(&clone)

	describe := machine.describeSObjectValue("Account", clone.Objects["Account"].Definition)
	fields := describe.Fields["fields"].Fields["map"]
	if _, ok := fields.Map[mapKey(String("RuntimeOnly__c"))]; !ok {
		t.Fatalf("describe cache did not observe runtime field")
	}
}
```

Adjust helper names to match existing VM test conventions.

- [ ] **Step 6: Run focused performance gates**

Run:

```bash
go test ./internal/storage ./internal/vm ./internal/apextest
go test -run '^$' -bench=BenchmarkRunTestSuiteWithClassSetup -benchmem -benchtime=1x ./internal/apextest
scripts/perf/glade-baseline.sh /tmp/glade-perf-schema-stamp
```

Expected:

- `BenchmarkRunTestSuiteWithClassSetup` allocation drops at least 10 percent from `/tmp/glade-perf-before`.
- `schemaCacheStampForOrg` falls out of the top three allocation frames.
- No Salesforce describe semantics change.

- [ ] **Step 7: Commit only if the gate is met**

```bash
git add internal/storage internal/vm internal/apextest
git commit -m "perf: reuse trusted runtime schema stamps"
```

If the correctness work gets broad or brittle, stop and discard this task. A stale schema cache is worse than a slow one.

---

## Task 5: Reduce Apex Test Clone Cost Without Weakening Isolation

**Purpose:** The local test runner spends real time cloning orgs and VM class state. Keep isolation and reduce copied material.

**Files:**
- Modify: `internal/storage/model.go`
- Modify: `internal/storage/snapshot.go`
- Modify: `internal/apextest/runner.go`
- Modify: `internal/vm/runtime_state.go`
- Test: `internal/apextest/isolation_journal_test.go`
- Test: `internal/apextest/runner_test.go`
- Test: `internal/storage/snapshot_test.go`

- [ ] **Step 1: Read current counters before changing code**

Run:

```bash
scripts/perf/glade-baseline.sh /tmp/glade-perf-before-clone
grep -E 'CloneRuntimeOrg|CloneRuntimeMachine|SetupDuration|RunDuration' /tmp/glade-perf-before-clone/*.stdout /tmp/glade-perf-before-clone/*.bench || true
```

Expected: at least benchmark and pprof outputs exist. If Task 3 is complete, phase counters name clone/setup/run cost.

- [ ] **Step 2: Expand journal eligibility only with proof**

Inspect `classSupportsJournalIsolation` in `internal/apextest/runner.go`. Add table-driven tests for cases that must remain clone-backed:

```go
func TestClassSupportsJournalIsolationRejectsUnsafeMetadataMutation(t *testing.T) {
	// Use a fixture class that mutates setup metadata or schema shape.
	// Expected: classSupportsJournalIsolation returns false.
}
```

Add table-driven tests for cases that can use journal rollback:

```go
func TestClassSupportsJournalIsolationAllowsPlainDMLDataTests(t *testing.T) {
	// Use a fixture class with multiple methods that insert/update/delete business records only.
	// Expected: classSupportsJournalIsolation returns true.
}
```

Use actual helper functions from `runner.go`; do not introduce a separate classifier.

- [ ] **Step 3: Share immutable record maps for more setup objects**

Only extend `storage.IsImmutableMetadataObject` when the object is true Salesforce setup or compiled metadata and local tests cannot mutate it as business data.

Candidate additions must be proved by tests:

- `CustomPermission`
- `PermissionSetLicense`
- `UserPermissionAccess`
- `EntityParticle`

For each addition, add a copy-on-write test:

```go
func TestCloneRuntimeFrozenSharedCopyOnWritesNewImmutableObject(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "CustomPermission")
	state := org.Objects["CustomPermission"]
	state.Records["0CP000000000001"] = Record{
		ID: "0CP000000000001",
		Object: "CustomPermission",
		Fields: map[string]Value{"DeveloperName": StringValue("CanRun")},
	}
	org.Objects["CustomPermission"] = state

	clone := org.CloneRuntimeFrozenShared()
	if !clone.Objects["CustomPermission"].RecordsShared {
		t.Fatalf("CustomPermission records should be shared")
	}
	object, cloned := EnsureMutableObjectRecords(&clone, "CustomPermission")
	if !cloned {
		t.Fatalf("expected copy-on-write")
	}
	object.Records["0CP000000000001"] = Record{ID: "0CP000000000001", Object: "CustomPermission"}
	clone.Objects["CustomPermission"] = *object
	if org.Objects["CustomPermission"].Records["0CP000000000001"].Fields["DeveloperName"].String != "CanRun" {
		t.Fatalf("source records changed")
	}
}
```

- [ ] **Step 4: Reduce VM class clone work**

`copyClassMapWithPlan` still copies static field maps per clone. Add a benchmark that isolates static-heavy class clones:

```go
func BenchmarkCloneRuntimeStaticHeavyClasses(b *testing.B) {
	template := New(nil)
	for i := 0; i < 500; i++ {
		class := Class{Name: fmt.Sprintf("C%d", i), StaticFields: map[string]Value{
			"A": String("a"),
			"B": Integer(1),
		}}
		template.RegisterClass(class)
	}
	template.FreezeClassLookup()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		clone := template.CloneRuntime(nil)
		if len(clone.Classes) == 0 {
			b.Fatal("missing classes")
		}
	}
}
```

If this benchmark shows more than 5 percent of Apex test wall time, introduce copy-on-write static fields. If it does not, stop here.

- [ ] **Step 5: Run sentinel methods, not full suites**

Use exact slow methods from saved artifacts:

```bash
scripts/perf/localtest-targets.sh test-results/nu.json 6
scripts/perf/localtest-targets.sh test-results/nams.json 6
scripts/perf/localtest-targets.sh test-results/sf-cred.json 6
```

Run focused methods with the existing plugin path or wrapper used in this checkout. Use `--parallel 4`, `--class`, and `--method`. Do not use `--filter`.

Expected:

- At least three slow methods from `nu`.
- At least two slow methods from `nams`.
- At least one slow method from `sf-cred`.
- No result changes from pass to fail.

- [ ] **Step 6: Commit only with measured gain**

Required gain:

- `BenchmarkRunTestSuiteWithClassSetup` improves by at least 8 percent, or
- One enterprise slow-class set improves by at least 10 percent, and
- No focused correctness gate regresses.

Run:

```bash
go test ./internal/storage ./internal/vm ./internal/apextest ./internal/dml ./internal/soql
git diff --check
```

Commit:

```bash
git add internal/storage internal/vm internal/apextest
git commit -m "perf: reduce Apex test clone cost"
```

---

## Task 6: Target Alias and Static Field Tracking Hotspots

**Purpose:** Prior work named `collectStaticFieldValueRefs` and `replaceValueAliasRef` as remaining heavy frames in real `sf-cred` tests. Work only from profiles.

**Files:**
- Modify only after profile proof: `internal/vm/value_aliasing.go` or the file that currently contains these helpers.
- Test: `internal/vm/vm_benchmark_test.go`
- Test: `internal/vm/*_test.go` for alias semantics.

- [ ] **Step 1: Reproduce the current hot method**

Run:

```bash
scripts/perf/localtest-targets.sh test-results/sf-cred.json 3
```

Expected first row still names `tst_dataMapper` unless the artifact changed.

Run the exact focused class or method with CPU and memory profiles. Use the active compat plugin command in this checkout. The command shape is:

```bash
glade compat local-tests --project <sf-cred-project-root> --class tst_dataMapper --method tst_dataMapper --parallel 4 --progress --json --top-failures 10 --timeout 300000 --cpu-profile /tmp/sfcred-datamapper.cpu --mem-profile /tmp/sfcred-datamapper.mem --perf-json /tmp/sfcred-datamapper.perf.json
```

If the compat command is now plugin-hosted only, call the first-party compat plugin wrapper from `~/Dev/glade-tools`.

- [ ] **Step 2: Read allocation first**

Run:

```bash
go tool pprof -top -alloc_space /tmp/glade-localtest-current-forced /tmp/sfcred-datamapper.mem
go tool pprof -top -cum /tmp/glade-localtest-current-forced /tmp/sfcred-datamapper.cpu
```

Expected: keep this task only if alias/static tracking is still in the top frames.

- [ ] **Step 3: Add a focused benchmark before code changes**

Extend existing benchmarks in `internal/vm/vm_benchmark_test.go` for the exact shape from the profile:

```go
func BenchmarkCollectStaticFieldValueRefsRepeatedSharedGraph(b *testing.B) {
	root := benchmarkLargeOrderGraph(200)
	fields := map[string]Value{"Root": root}
	location := staticFieldLocation{ClassName: "Bench", FieldName: "Root"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		refs := make(staticFieldRefSet)
		collectStaticFieldValueRefs(root, refs, fields, location, make(map[uint64]bool))
		if len(refs) == 0 {
			b.Fatal("expected refs")
		}
	}
}
```

Adjust helper names to match current benchmark helpers. Do not create artificial value shapes that the profile did not show.

- [ ] **Step 4: Implement only one low-risk change**

Allowed changes:

- Reuse `seen` maps through a local pool when profiles show repeated map growth.
- Short-circuit immutable scalar and empty collection values before map lookup.
- Cache object graph alias fingerprints only when value identity and mutation rules stay intact.

Forbidden changes:

- Sharing mutable value graphs across test methods.
- Treating SObject records as immutable.
- Skipping alias propagation for static fields.

- [ ] **Step 5: Verify behavior and timing**

Run:

```bash
go test ./internal/vm ./internal/apextest
go test -run '^$' -bench='Benchmark(CollectStaticFieldValueRefs|ReplaceValueAlias|SameAliasRuntimeContent)' -benchmem ./internal/vm
```

Then rerun the focused `sf-cred` method profile.

Expected:

- No VM behavior regression.
- At least 10 percent improvement on `tst_dataMapper`, or discard the patch.

- [ ] **Step 6: Commit**

```bash
git add internal/vm
git commit -m "perf: reduce static alias tracking churn"
```

---

## Task 7: Make Sema and LSP Pay Only for Needed Work

**Purpose:** Workspace symbol startup pays for full sema diagnostics and standard member expansion. That is too much for symbol lookup.

**Files:**
- Modify: `internal/sema/sema.go`
- Modify: `internal/lsp/handler.go`
- Modify: `internal/lsp/handler_benchmark_test.go`
- Modify or add: `internal/sema/sema_benchmark_test.go`

- [ ] **Step 1: Split analysis options**

Add options in `internal/sema/sema.go`:

```go
type AnalyzeOptions struct {
	Diagnostics bool
	ExportTypes  bool
}

func Analyze(index typesys.Index) Result {
	return AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, ExportTypes: true})
}

func AnalyzeWithOptions(index typesys.Index, opts AnalyzeOptions) Result {
	a := NewAnalyzer()
	return a.AnalyzeWithOptions(index, opts)
}
```

Move the current `Analyze` body to `AnalyzeWithOptions`. When `Diagnostics` is false, skip:

- `checkTriggers`
- `checkMemberTypes`
- `checkMethodParameters`
- `checkAnnotations`
- `checkMethodBodies`
- `checkVisibility`
- `checkManagedPackageAccess`
- `checkInheritanceContracts`
- `checkSchemaReferences`

Keep known-type export when `ExportTypes` is true.

- [ ] **Step 2: Make LSP choose the lighter path only where safe**

In `internal/lsp/handler.go`, keep full sema for diagnostics:

```go
analysis: sema.Analyze(index),
```

Only if startup profile proves full sema hurts command launch, add a lazy analysis field:

```go
analysisOnce sync.Once
analysis sema.Result
```

Then compute diagnostics on first diagnostic-bearing request, not in `NewHandler`.

For `workspace/symbol`, use the index directly when possible. Do not use the light sema path if it changes symbol visibility or managed-package behavior.

- [ ] **Step 3: Add benchmarks for the split**

In `internal/sema/sema_benchmark_test.go`, add:

```go
func BenchmarkAnalyzeIndexLightTypesOnly(b *testing.B) {
	index := benchmarkIndex(200)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		result := AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: false, ExportTypes: true})
		if len(result.Types) == 0 {
			b.Fatalf("expected exported types")
		}
	}
}
```

In `internal/lsp/handler_benchmark_test.go`, keep the existing benchmark and add one for handler initialization if not present:

```go
func BenchmarkNewHandler(b *testing.B) {
	index := benchmarkLSPIndex(200)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		handler := NewHandler(index)
		if handler == nil {
			b.Fatal("nil handler")
		}
	}
}
```

- [ ] **Step 4: Run gates**

Run:

```bash
go test ./internal/sema ./internal/lsp ./internal/typesys
go test -run '^$' -bench='Benchmark(AnalyzeIndex|AnalyzeIndexLightTypesOnly|WorkspaceSymbols|NewHandler)' -benchmem ./internal/sema ./internal/lsp
```

Expected:

- Full sema benchmark does not regress.
- Light path is at least 50 percent cheaper than full sema.
- LSP behavior tests still pass.

- [ ] **Step 5: Commit**

```bash
git add internal/sema internal/lsp
git commit -m "perf: split lightweight sema from full diagnostics"
```

---

## Task 8: Compact Standard Symbol and Describe Data Without Changing Semantics

**Purpose:** Huge generated Go literals and gzip JSON inflate memory. Cut startup and sema allocation by changing representation, not semantics.

**Files:**
- Modify generator source in first-party tooling if it owns generated files.
- Modify generated outputs only through the generator.
- Modify: `internal/typesys/standard_symbols.go`
- Modify: `internal/storage/standard_describe_catalog.go`
- Test: `internal/typesys/standard_symbols_test.go`
- Test: `internal/storage/model_test.go`

- [ ] **Step 1: Do not hand-edit generated files**

Confirm generator ownership before changes:

```bash
rg -n "system_stub_symbols_generated|standard_sobject_stub_overlay_generated|standard_describe_catalog" scripts internal cmd ~/Dev/glade-tools
```

Expected: generator command or script is found. If not found, stop and write a separate generator discovery note.

- [ ] **Step 2: Measure generated-data load**

Run:

```bash
go test -run '^$' -bench=BenchmarkStandardPlatformSymbols -benchmem -benchtime=1x ./internal/typesys
go test -run '^$' -bench=BenchmarkEnsureStandardObjectFieldsFreshDefinition -benchmem -benchtime=100ms ./internal/storage
```

Expected: record allocations before changing representation.

- [ ] **Step 3: Prefer generated compact indexes over Rust**

First Go-side representation to try:

- Generate sorted string tables for repeated namespace, type, method, and property names.
- Store method/property specs as integer references into string tables.
- Build `TypeSymbol` only for requested namespace/type or during full sema.

Required API shape in `internal/typesys/standard_symbols.go`:

```go
func StandardPlatformSymbolView() []TypeSymbol {
	standardPlatformSymbolsOnce.Do(func() {
		standardPlatformSymbolsCache = buildStandardPlatformSymbols()
	})
	return standardPlatformSymbolsCache
}

func StandardPlatformSymbolByName(name string) (TypeSymbol, bool) {
	// Generated lookup should avoid building all symbols when possible.
}
```

- [ ] **Step 4: Preserve audit tests**

Run:

```bash
go test ./internal/typesys ./internal/storage
```

Expected: generated symbol audit tests pass with the same public shape.

- [ ] **Step 5: Keep only if it moves a real gate**

Required gain:

- `BenchmarkStandardPlatformSymbols` allocation drops at least 30 percent, and
- cold `version` RSS stays below the Task 2 ratchet, and
- `BenchmarkAnalyzeIndex` or `BenchmarkWorkspaceSymbols` allocation drops at least 10 percent.

Commit:

```bash
git add internal/typesys internal/storage
git commit -m "perf: compact generated platform metadata"
```

---

## Task 9: Native/Rust Spike Gate

**Purpose:** Answer the language question with proof, not taste. This is a spike only. No production native code lands from this task.

**Allowed candidate areas:**
- Apex source scanning before parse, only if profiles show regex/source scanning above 15 percent of end-to-end local-test wall time.
- Parser wrapper conversion from tree-sitter nodes to `apexast.File`, only if parser plus conversion exceeds 15 percent of enterprise run wall time.
- Standard symbol compact-table decoder, only if Go compact tables fail to cut sema/LSP allocation.

**Forbidden candidate areas:**
- VM execution engine.
- Apex value representation.
- Org state and storage model.
- DML/SOQL semantics.
- Test isolation machinery.

**Files:**
- Create spike under `/tmp/glade-native-spike` or a throwaway branch.
- Do not add production Rust files to `main` in this task.

- [ ] **Step 1: Prove the candidate is hot enough**

A candidate can enter the spike only if a current profile shows it is at least 15 percent of one of:

- `BenchmarkRunTestSuiteWithClassSetup`
- one focused `nu` slow class
- one focused `nams` slow class
- one focused `sf-cred` slow class

Run:

```bash
go tool pprof -top -cum <cpu-profile>
go tool pprof -top -alloc_space <mem-profile>
```

Expected: write the candidate, percentage, and profile path into `/tmp/glade-native-spike/decision.md`.

- [ ] **Step 2: Build the smallest native prototype**

The prototype must expose one function and one correctness fixture. Example shape:

```rust
#[no_mangle]
pub extern "C" fn glade_scan_apex_symbols(ptr: *const u8, len: usize) -> GladeScanResult {
    // Prototype only. No production landing.
}
```

The Go caller must include FFI overhead in the benchmark. No in-process fake timing.

- [ ] **Step 3: Compare against Go on the same input**

Run a benchmark that includes:

- Go current implementation.
- Native implementation with FFI crossing.
- Native implementation with result conversion into current Go structs.

Expected gate:

- At least 25 percent faster for the isolated function, and
- at least 10 percent faster end-to-end on one enterprise sentinel, and
- no semantic mismatch on all fixture comparisons.

- [ ] **Step 4: Decide**

If the gate fails, write:

```text
NO-GO: native spike failed end-to-end threshold.
```

If the gate passes, write a separate production integration plan. That plan must include:

- build portability for macOS, Linux, and CI;
- cgo/no-cgo behavior;
- release packaging;
- panic/error boundary;
- semantic fixture parity;
- rollback switch.

Do not merge native code from the spike directly.

---

## Task 10: Enterprise Sentinel Ratchet

**Purpose:** A 10 percent change matters only if it shows up on real suites.

**Files:**
- Create: `docs/perf/enterprise-sentinels.md`
- Use existing `test-results/*.json` as starting artifacts.

- [ ] **Step 1: Write the sentinel document**

Create `docs/perf/enterprise-sentinels.md`:

```markdown
# Enterprise Performance Sentinels

## Required Gates

- `apex-recipes`: small correctness and timing smoke.
- `sf-cred`: alias/static tracking and setup-heavy tests.
- `nu`: 11k+ enterprise suite and order/payment heavy classes.
- `nams`: imported membership and expression-heavy classes.
- `nutpl`: fast fflib/package sentinel.

## Rules

- Use `--parallel 4` for long local-test runs.
- Use `--class` and `--method`, not `--filter`.
- Prefer exact slow methods from saved JSON before full-suite runs.
- Full `nu` and `nams` runs require explicit approval unless a release gate needs them.

## Current Saved Artifacts

- `test-results/recipes.json`
- `test-results/sf-cred.json`
- `test-results/nu.json`
- `test-results/nams.json`
- `test-results/nutpl.json`
```

- [ ] **Step 2: Record the current top methods**

Append the output of:

```bash
scripts/perf/localtest-targets.sh test-results/nu.json 10
scripts/perf/localtest-targets.sh test-results/nams.json 10
scripts/perf/localtest-targets.sh test-results/sf-cred.json 10
scripts/perf/localtest-targets.sh test-results/nutpl.json 10
scripts/perf/localtest-targets.sh test-results/recipes.json 10
```

to the sentinel document.

- [ ] **Step 3: Define merge gates for this performance branch**

A branch can merge when:

- focused package tests pass;
- `scripts/perf/glade-baseline.sh` shows no cold-start regression;
- at least one enterprise sentinel target shows measured gain;
- no sentinel target changes pass/fail behavior;
- `git diff --check` passes.

- [ ] **Step 4: Commit**

```bash
git add docs/perf/enterprise-sentinels.md
git commit -m "docs: define enterprise performance sentinels"
```

---

## Execution Order

1. Task 1: Baseline harness.
2. Task 2: Lazy VM platform indexes. This should give the cleanest early win.
3. Task 3: Phase timing counters.
4. Task 4: Trusted template schema stamp, only if correctness stays tight.
5. Task 5: Clone cost cuts with isolation proof.
6. Task 6: Alias/static tracking profile work.
7. Task 7: Sema/LSP split.
8. Task 8: Compact generated metadata.
9. Task 9: Native/Rust spike, only if a hot function still qualifies.
10. Task 10: Enterprise sentinel ratchet.

## Expected Impact

- Cold simple commands: likely 50 percent or better after lazy VM platform indexes.
- Apex local tests: likely 10 to 25 percent across setup-heavy classes if schema stamp and clone work land.
- Sema/LSP: likely 30 to 50 percent allocation reduction for symbol-only paths if full diagnostics become lazy.
- Rust/native: no production work expected from current evidence. Keep it as a proof gate, not a plan of record.

## Stop Conditions

- Any change causes Salesforce-facing behavior to diverge.
- Any test isolation leak appears.
- A performance patch wins only on a synthetic benchmark and not on focused enterprise targets.
- A native spike cannot prove 10 percent end-to-end gain including FFI and conversion.
- The implementation requires a full VM rewrite.

## Verification Bundle

Run before final merge:

```bash
go test ./internal/storage ./internal/vm ./internal/apextest ./internal/soql ./internal/dml ./internal/typesys ./internal/sema ./internal/lsp ./internal/gladecli ./internal/repoguard
scripts/perf/glade-baseline.sh /tmp/glade-perf-final
git diff --check
```

Run broader checks when product surfaces move:

```bash
go test ./...
scripts/smoke.sh
```

If full enterprise suites are approved, use `--parallel 4` and record before/after JSON beside the saved artifacts. 

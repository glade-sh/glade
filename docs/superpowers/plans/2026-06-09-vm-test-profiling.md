# VM-Powered Test Performance Profiling

> **Prerequisite:** The base scanner (Tasks 1-10 in `docs/superpowers/plans/2026-06-09-salesforce-performance-scanner.md`) and Phase 1 (AST-based scanning) must be landed first.

**Goal:** Make `glade test` capture per-test-method governor limit consumption and support threshold gating so developers can enforce performance budgets.

**Architecture:** The VM already tracks 14 governor limits in `vm.Limits` and emits Chrome trace events. The gap is that `Test.stopTest()` restores parent limits before the `Result` is captured, so per-`startTest`/`stopTest` test-window limits are lost. Add a `TestWindowLimits` aggregator to `vm.Result`, surface it in `testreport.Case`, and add `--max-*` threshold flags to `glade test`.

---

## File Structure

- Modify `internal/vm/runtime_state.go`: add `TestLimits` and `TestLimitViolations` to `Result`.
- Modify `internal/vm/async_job_runtime.go`: snapshot limits in `testStop()` before parent restore.
- Modify `internal/apextest/runner.go`: copy `TestLimits` into `testreport.Case`.
- Modify `internal/testreport/model.go`: add `Limits` field to `Case`.
- Modify `internal/gladecli/test_command.go`: add `--max-*` threshold flags and gating logic.
- Modify `internal/gladecli/test_command_test.go`: threshold gating tests.
- Create `internal/testreport/limits_test.go`: test limit aggregation across `startTest`/`stopTest` windows.

---

## Task 1: Surface Test-Window Limits

### Step 1: Add TestLimits to vm.Result

In `internal/vm/runtime_state.go`, add after the `Limits` field in the `Result` struct:

```go
TestLimits          Limits           `json:"testLimits,omitempty"`
TestLimitViolations []LimitViolation `json:"testLimitViolations,omitempty"`
```

The `TestLimits` field aggregates limits consumed across all `Test.startTest()` / `Test.stopTest()` windows during one `execute()` call. It is zero when no test context is active or when `startTest()` was never called. `TestLimitViolations` aggregates violations that occurred during test windows.

### Step 2: Capture limits before parent restore

In `internal/vm/async_job_runtime.go`, modify `testStop()` to capture the test-window limits before restoring parent limits:

```go
func (vm *VM) testStop(result *Result) (Value, error) {
	if vm.testContext == nil {
		return Null, fmt.Errorf("Test.stopTest() called outside test context")
	}
	if !vm.testContext.Started {
		return Null, fmt.Errorf("Test.stopTest() called without calling Test.startTest()")
	}
	if vm.testContext.Stopped {
		return Null, fmt.Errorf("Test.stopTest() already called")
	}
	vm.testContext.Stopped = true
	err := vm.drainTestWork(result)
	if err != nil {
		return Null, err
	}
	// Snapshot test-window limits before restoring parent limits.
	// The vm.limits at this point reflect the startTest→stopTest window
	// plus any pre-startTest work that was zeroed by testStart.
	result.addTestLimits(vm.limits)
	for _, v := range vm.limitViolations {
		result.addTestLimitViolation(v)
	}
	vm.limits = vm.testContext.ParentLimits
	vm.limitViolations = append([]LimitViolation(nil), vm.testContext.ParentViolations...)
	return Null, nil
}
```

Add helper methods to `Result`:

```go
func (r *Result) addTestLimits(l Limits) {
	r.TestLimits.Queries += l.Queries
	r.TestLimits.QueryRows += l.QueryRows
	r.TestLimits.DMLStatements += l.DMLStatements
	r.TestLimits.DMLRows += l.DMLRows
	r.TestLimits.HeapSize = max(r.TestLimits.HeapSize, l.HeapSize)
	r.TestLimits.CPUTimeMS += l.CPUTimeMS
	r.TestLimits.Callouts += l.Callouts
	r.TestLimits.AsyncJobs += l.AsyncJobs
	r.TestLimits.FutureCalls += l.FutureCalls
	r.TestLimits.QueueableJobs += l.QueueableJobs
	r.TestLimits.BatchJobs += l.BatchJobs
	r.TestLimits.ScheduledJobs += l.ScheduledJobs
	r.TestLimits.EmailInvokes += l.EmailInvokes
	r.TestLimits.RunAs += l.RunAs
	r.TestLimits.Savepoints += l.Savepoints
}

func (r *Result) addTestLimitViolation(v LimitViolation) {
	for i, existing := range r.TestLimitViolations {
		if existing.Name == v.Name {
			r.TestLimitViolations[i].Used += v.Used
			return
		}
	}
	r.TestLimitViolations = append(r.TestLimitViolations, v)
}
```

Heap is tracked as max (peak) rather than sum because it is a ceiling limit, not a consumption limit.

### Step 3: Run VM tests and verify backward compat

```bash
go test ./internal/vm -count=1
```

Expected: PASS. Existing tests must not break. `TestLimits` stays zero when no `startTest()` is called.

---

## Task 2: Surface Limits In Test Report

### Step 1: Add Limits field to testreport.Case

In `internal/testreport/model.go`, add to the `Case` struct:

```go
Limits          *vm.Limits          `json:"limits,omitempty"`
LimitViolations []vm.LimitViolation `json:"limitViolations,omitempty"`
```

Add the import for `"github.com/glade-sh/glade/internal/vm"`.

### Step 2: Copy limits in the runner

In `internal/apextest/runner.go`, in the `runCase()` function, after `result, err := machine.ExecuteInClass(invokeProgram, testCase.ClassName)` and after `attachTraceProfile(&out, result, opts)`, add:

```go
out.Limits = result.TestLimits
out.LimitViolations = result.TestLimitViolations
```

Use `result.TestLimits` (not `result.Limits`) because `result.Limits` is the post-restore parent limits. `result.TestLimits` is the aggregated test-window consumption.

To avoid importing `vm` in `testreport`, copy the fields individually or define a shared `LimitsSnapshot` type. Use the approach that matches existing codebase patterns.

### Step 3: Write test for limits in report

Create `internal/testreport/limits_test.go`:

```go
package testreport

import (
	"testing"
)

func TestCaseJSONIncludesTestLimits(t *testing.T) {
	c := Case{
		Name:   "testMethod",
		Status: StatusPassed,
		Limits: &LimitsSnapshot{Queries: 8, DMLStatements: 3, CPUTimeMS: 45},
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"queries":8`, `"dmlStatements":3`, `"cpuTimeMs":45`} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("json missing %q: %s", want, string(data))
		}
	}
}
```

Use the actual shared type once defined in Step 2.

### Step 4: Run runner tests

```bash
go test ./internal/apextest ./internal/testreport -count=1
```

Expected: PASS.

---

## Task 3: Add Threshold Gating To `glade test`

### Step 1: Add CLI flags

In `internal/gladecli/test_command.go`, add threshold flags:

```go
maxQueries       int
maxQueryRows     int
maxDMLStatements int
maxDMLRows       int
maxCPUTimeMS     int
maxCallouts      int
maxAsyncJobs     int
maxFutureCalls   int
maxQueueableJobs int
maxEmailInvokes  int
```

Register them in the flag set for `glade test`. Default values: 0 (disabled — no threshold gating unless explicitly set).

### Step 2: Add threshold checking after test run

After all tests complete (in `runTest()` or the summary logic), iterate over report cases and check each against the thresholds:

```go
func checkThresholds(report testreport.RunReport, opts testOptions) []ThresholdViolation {
	var violations []ThresholdViolation
	for _, c := range report.Cases {
		if opts.maxQueries > 0 && c.Limits != nil && c.Limits.Queries > opts.maxQueries {
			violations = append(violations, ThresholdViolation{
				Case:    c.Name,
				Limit:   "queries",
				Used:    c.Limits.Queries,
				Max:     opts.maxQueries,
			})
		}
		// ... repeat for each threshold ...
	}
	return violations
}
```

### Step 3: Output and exit code

When thresholds are violated:
- Print a summary line: `FAIL: 2 test(s) exceeded performance thresholds`
- Print per-violation details: `  TestAccountTrigger queries=15 > max=10`
- Exit code: return 1 (test failure) when any threshold is violated

When no thresholds are set, behavior is unchanged.

### Step 4: Write CLI test

In `internal/gladecli/test_command_test.go`, add:

```go
func TestTestCommandRespectsMaxQueriesThreshold(t *testing.T) {
	root := writeTestProjectWithPerformanceRisks(t)
	var stdout, stderr strings.Builder
	code := Run(context.Background(), []string{"test", "--project", root, "--max-queries", "1", "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d. stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "threshold") {
		t.Fatalf("expected threshold violation in output: %s", stdout.String())
	}
}
```

### Step 5: Run CLI tests

```bash
go test ./internal/gladecli -run TestTestCommandRespectsMaxQueriesThreshold -count=1
```

Expected: PASS.

---

## Task 4: Final Validation

```bash
go test ./internal/vm ./internal/apextest ./internal/testreport ./internal/gladecli -count=1
```

Expected: all PASS.


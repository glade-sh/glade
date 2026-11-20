package scripts

import (
	"regexp"
	"strings"
	"testing"
)

// The nightly concurrent-tests job selects tests by name, one -run regex per
// package. One shared regex also carried the words Async, Queue, Stream,
// Signal and Lock, which in apextest and vm name Apex features (Queueable,
// @future, EventBus streams, Approval and FOR UPDATE locks) — around a
// hundred heavy conformance tests — while in server Serve matched the
// ServerRoutes HTTP route tests. The nightly apextest run then hit its 25m
// timeout inside TestCDCAsyncChainNativeConformance. Each package therefore
// gets its own regex over words that mark Go-level concurrency there, the
// job prints each selection's matched-test count so drift is visible, and an
// empty selection fails instead of silently passing.
func TestCIRaceNightlySelectionRegexesSelectGoConcurrencyOnly(t *testing.T) {
	concurrent := workflowJobBlocks(t, readWorkflowFile(t, "race.yml"))["concurrent-tests"]
	if concurrent == "" {
		t.Fatal("race.yml must keep the concurrent-tests nightly job")
	}

	// Every canary is a real test name: the Apex-feature conformance and
	// route tests the shared regex wrongly selected, and the Go-concurrency
	// tests each narrowed selection must keep.
	for _, selection := range []struct {
		env     string
		exclude []string
		include []string
	}{
		{
			env: "APEXTEST_CONCURRENCY_TEST_RE",
			exclude: []string{
				"TestCDCAsyncChainNativeConformance",
				"TestQueueableAccessLevelIdentityNativeConformance",
				"TestApprovalLockMutationNativeConformance",
			},
			include: []string{"TestConcurrentIdenticalRuntimeTransitionSingleflight"},
		},
		{
			env: "VM_CONCURRENCY_TEST_RE",
			exclude: []string{
				"TestDetachAccessLevelsKeepsAliasesWithinTheQueueable",
				"TestExecSOQLForUpdatePreservesExistingApprovalLock",
			},
			include: []string{"TestExecuteStopsWhenContextCanceled"},
		},
		{
			env: "SERVER_CONCURRENCY_TEST_RE",
			exclude: []string{
				"TestApprovalLockedMutationServerRoutes",
				"TestCartOrchestratorServerAndLightningCompileRoutes",
			},
			include: []string{"TestCase339FixtureAndResetReadOutsideOrgLock"},
		},
		{
			// gladecli keeps the broad set — its Daemon, Watch, GracefulStop,
			// Listener and Serve tests are Go-level — so it still selects the
			// words the other packages dropped.
			env:     "CONCURRENCY_TEST_RE",
			include: []string{"TestGracefulStopDAPStdioHelper"},
		},
	} {
		regex := workflowEnvValue(t, concurrent, selection.env)
		for _, name := range selection.exclude {
			if selectionMatches(t, regex, name) {
				t.Errorf("%s must not select the Apex-feature test %s", selection.env, name)
			}
		}
		for _, name := range selection.include {
			if !selectionMatches(t, regex, name) {
				t.Errorf("%s must keep selecting the Go-concurrency test %s", selection.env, name)
			}
		}
	}

	// lwc/compile and visualforce run the broad set as their nightly smoke,
	// so the words the narrowed packages dropped must stay in it.
	broad := workflowEnvValue(t, concurrent, "CONCURRENCY_TEST_RE")
	for _, word := range []string{"Async", "Queue", "Stream", "Signal", "Lock", "Serve"} {
		if !strings.Contains(broad, word) {
			t.Errorf("CONCURRENCY_TEST_RE must keep the broad word %q", word)
		}
	}

	// Each package keeps its own regex and timeout, prints its matched-test
	// count before running, and fails rather than silently running nothing.
	runStep := workflowStepBlock(t, concurrent, "go test -race")
	for _, marker := range []string{"go test -race -list", "grep -c '^Test'", "no tests match"} {
		if !strings.Contains(runStep, marker) {
			t.Errorf("concurrent-tests run step lacks %q", marker)
		}
	}
	for _, run := range []struct {
		pkg, regex, timeout string
	}{
		{"./internal/apextest", "$APEXTEST_CONCURRENCY_TEST_RE", "25m"},
		{"./internal/vm", "$VM_CONCURRENCY_TEST_RE", "25m"},
		{"./internal/gladecli", "$CONCURRENCY_TEST_RE", "25m"},
		{"./internal/server", "$SERVER_CONCURRENCY_TEST_RE", "25m"},
		{"./internal/lwc/compile", "$CONCURRENCY_TEST_RE", "10m"},
		{"./internal/visualforce", "$CONCURRENCY_TEST_RE", "10m"},
		{"./internal/repoguard", "'^TestPublicScan'", "10m"},
	} {
		line := workflowRunSelectionLine(t, runStep, run.pkg)
		if !strings.Contains(line, run.regex) {
			t.Errorf("%s selection must use %s", run.pkg, run.regex)
		}
		if !strings.Contains(line, run.timeout) {
			t.Errorf("%s selection must keep its %s timeout", run.pkg, run.timeout)
		}
	}
}

// workflowEnvValue returns the value of a single-quoted workflow-level env
// entry within a job block.
func workflowEnvValue(t *testing.T, jobBlock, name string) string {
	t.Helper()
	prefix := name + ": "
	for _, line := range strings.Split(jobBlock, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		value := strings.TrimPrefix(trimmed, prefix)
		if len(value) < 2 || value[0] != '\'' || !strings.HasSuffix(value, "'") {
			t.Fatalf("%s must be a single-quoted literal, got %q", name, value)
		}
		return value[1 : len(value)-1]
	}
	t.Fatalf("job block lacks the %s env entry", name)
	return ""
}

func selectionMatches(t *testing.T, regex, name string) bool {
	t.Helper()
	matched, err := regexp.MatchString(regex, name)
	if err != nil {
		t.Fatalf("%s is not a usable -run regex: %v", regex, err)
	}
	return matched
}

func workflowRunSelectionLine(t *testing.T, stepBlock, pkg string) string {
	t.Helper()
	for _, line := range strings.Split(stepBlock, "\n") {
		if strings.Contains(line, pkg) {
			return line
		}
	}
	t.Fatalf("run step lacks the %s selection", pkg)
	return ""
}

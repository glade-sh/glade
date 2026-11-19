//go:build race

package vm

import "testing"

// localListCallBudgetEnforced states whether
// TestLocalListCallArgumentAllocation enforces its per-call allocation
// budgets in this build. The race detector books extra memory on every
// allocation, and each eval in the AllocsPerRun loop pays instrumentation
// on the argument slice, the receiver alias view and the result value, so
// an instrumented run exceeds the normal-build budgets (the race lane run
// 44541836806 failed the test on its VM shard) without any product change.
// Under -race the test keeps measuring, keeps logging its allocation row
// and still enforces every result-shape assertion; only the numerical
// budgets are left to normal builds, which are the builds users run.
const localListCallBudgetEnforced = false

// The contract above is pinned per build: an instrumented run must keep
// observing the allocation counts and keep the result-shape assertions, but
// must not fail on the normal-build budgets.
func TestLocalListCallBudgetNotEnforcedUnderRace(t *testing.T) {
	if localListCallBudgetEnforced {
		t.Fatal("the local list call allocation budgets are a normal-build contract; race instrumentation inflates the counts and must not fail the run")
	}
}

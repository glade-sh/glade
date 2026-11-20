//go:build race

package visualforce

import "testing"

// fileDigestBudgetEnforced states whether
// TestFileContentDigestAllocationBudget enforces its 8 KiB per-file hashing
// budget in this build. The race detector books extra memory on every
// allocation, and the fixture's 200 fresh 4 KiB files each pay that
// instrumentation on the digest buffer, the path reads and the memo keys:
// the race lane measured 10,425 bytes per file against the 8,192-byte
// budget (run 44541836806, Visualforce shard 3) with no product change.
// Under -race the test keeps measuring, keeps hashing every file and keeps
// its digest-equality behavior assertions; only the numerical budget is
// left to normal builds, which are the builds users run.
const fileDigestBudgetEnforced = false

// The contract above is pinned per build: an instrumented run must keep
// hashing the fixture and keep the digest behavior assertions, but must not
// fail on the normal-build byte budget.
func TestFileDigestBudgetNotEnforcedUnderRace(t *testing.T) {
	if fileDigestBudgetEnforced {
		t.Fatal("the 8 KiB per-file hashing budget is a normal-build contract; race instrumentation inflates the measured bytes and must not fail the run")
	}
}

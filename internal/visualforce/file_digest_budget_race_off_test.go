//go:build !race

package visualforce

import "testing"

// fileDigestBudgetEnforced states whether
// TestFileContentDigestAllocationBudget enforces its 8 KiB per-file hashing
// budget in this build. Normal builds enforce it; the race build's
// counterpart explains why instrumented builds only observe the bytes.
const fileDigestBudgetEnforced = true

func TestFileDigestBudgetEnforcedInNormalBuilds(t *testing.T) {
	if !fileDigestBudgetEnforced {
		t.Fatal("normal builds must enforce the 8 KiB per-file hashing budget")
	}
}

//go:build !race

package vm

import "testing"

// localListCallBudgetEnforced states whether
// TestLocalListCallArgumentAllocation enforces its per-call allocation
// budgets in this build. Normal builds enforce them; the race build's
// counterpart explains why instrumented builds only observe the counts.
const localListCallBudgetEnforced = true

func TestLocalListCallBudgetEnforcedInNormalBuilds(t *testing.T) {
	if !localListCallBudgetEnforced {
		t.Fatal("normal builds must enforce the local list call allocation budgets")
	}
}

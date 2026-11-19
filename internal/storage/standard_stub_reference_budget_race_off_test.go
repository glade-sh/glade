//go:build !race

package storage

import "testing"

// stubReferenceFieldsBudgetEnforced states whether
// TestStandardStubReferenceFieldsAllocationBudget enforces its one-third
// allocation budget in this build. Normal builds enforce it; the race
// build's counterpart explains why instrumented builds only observe the
// ratio.
const stubReferenceFieldsBudgetEnforced = true

func TestStubReferenceFieldsBudgetEnforcedInNormalBuilds(t *testing.T) {
	if !stubReferenceFieldsBudgetEnforced {
		t.Fatal("normal builds must enforce the one-third reference-fields allocation budget")
	}
}

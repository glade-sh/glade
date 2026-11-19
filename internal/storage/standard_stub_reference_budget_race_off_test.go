//go:build !race

package storage

import "testing"

// stubReferenceFieldsBudgetEnforced states whether
// TestStandardStubReferenceFieldsAllocationBudget enforces its one-third
// allocation budget in this build.
const stubReferenceFieldsBudgetEnforced = true

func TestStubReferenceFieldsBudgetEnforcedInNormalBuilds(t *testing.T) {
	if !stubReferenceFieldsBudgetEnforced {
		t.Fatal("normal builds must enforce the one-third reference-fields allocation budget")
	}
}

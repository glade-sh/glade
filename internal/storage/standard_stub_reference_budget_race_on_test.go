//go:build race

package storage

import "testing"

// stubReferenceFieldsBudgetEnforced states whether
// TestStandardStubReferenceFieldsAllocationBudget enforces its one-third
// allocation budget in this build.
const stubReferenceFieldsBudgetEnforced = true

// The one-third budget is a normal-build performance contract: the race
// detector books extra memory on every allocation, which inflates the
// lookup's many small allocations far more than the full decode's large
// buffers, so an instrumented run can measure the ratio above one third
// without any product change. An instrumented run must keep observing the
// ratio and keep the no-full-decode behavior assertion, but must not fail
// on the normal-build budget.
func TestStubReferenceFieldsBudgetNotEnforcedUnderRace(t *testing.T) {
	if stubReferenceFieldsBudgetEnforced {
		t.Fatal("the one-third reference-fields budget is a normal-build contract; race instrumentation inflates the measured ratio and must not fail the run")
	}
}

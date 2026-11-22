package vm

import "testing"

// The one-item shape was not captured. Keep its origin/main outcome unchanged.
func TestComparableSortUncapturedSingletonNeighbor(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Plain"}); err != nil {
		t.Fatal(err)
	}
	err := machine.sortComparableValues([]Value{Object("Plain")}, &Result{})
	const want = `unsupported call "List.sort for non-primitive comparable values"`
	if err == nil || err.Error() != want {
		t.Fatalf("singleton sort error = %v, want %s", err, want)
	}
}

// Distinct unmeasured types keep the existing unsupported-call outcome.
func TestComparableSortUncapturedMixedNeighbor(t *testing.T) {
	machine := New(nil)
	for _, name := range []string{"PlainOne", "PlainTwo"} {
		if err := machine.RegisterClass(Class{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	err := machine.sortComparableValues([]Value{Object("PlainOne"), Object("PlainTwo")}, &Result{})
	const want = `unsupported call "List.sort for non-primitive comparable values"`
	if err == nil || err.Error() != want {
		t.Fatalf("mixed sort error = %v, want %s", err, want)
	}
}

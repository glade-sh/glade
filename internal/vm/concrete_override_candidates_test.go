package vm

import (
	"sort"
	"testing"
)

func TestConcreteOverrideCandidatesFollowClassGenerations(t *testing.T) {
	machine := New(nil)
	for _, class := range []Class{{Name: "Base", IsAbstract: true}, {Name: "Alpha", SuperClass: "Base"}, {Name: "Beta", SuperClass: "Base"}, {Name: "Other"}} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	machine.FreezeClassLookup()
	names := func(vm *VM) []string {
		got := append([]string(nil), vm.concreteOverrideCandidates("Base")...)
		sort.Strings(got)
		return got
	}
	want := []string{"Alpha", "Beta"}
	for pass := 0; pass < 2; pass++ {
		if got := names(machine); !equalStrings(got, want) {
			t.Fatalf("pass %d: candidates = %v, want %v", pass, got, want)
		}
	}
	if len(machine.concreteOverrideCandidateMemo) != 1 {
		t.Fatalf("memo holds %d lists after two calls", len(machine.concreteOverrideCandidateMemo))
	}
	// A registration after the freeze is not cached until the next freeze,
	// and the next freeze is a new generation.
	clone := machine.CloneRuntimeFrozenShared(nil)
	if err := clone.RegisterClass(Class{Name: "Gamma", SuperClass: "Base"}); err != nil {
		t.Fatal(err)
	}
	if got := names(clone); !equalStrings(got, []string{"Alpha", "Beta", "Gamma"}) {
		t.Fatalf("pending registration: candidates = %v", got)
	}
	clone.FreezeClassLookup()
	if got := names(clone); !equalStrings(got, []string{"Alpha", "Beta", "Gamma"}) {
		t.Fatalf("after freeze: candidates = %v", got)
	}
	if got := names(machine); !equalStrings(got, want) {
		t.Fatalf("the source machine saw the clone's class: %v", got)
	}
	// The calling class is part of the key: nested-name resolution reads it.
	machine.currentClass = "Other"
	if got := names(machine); !equalStrings(got, want) {
		t.Fatalf("other calling class: candidates = %v", got)
	}
	if len(machine.concreteOverrideCandidateMemo) != 2 {
		t.Fatalf("memo holds %d lists for two calling classes", len(machine.concreteOverrideCandidateMemo))
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

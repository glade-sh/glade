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

// An indirect implementation mixed with null was not captured; preserve it.
func TestComparableSortUncapturedIndirectNullNeighbor(t *testing.T) {
	program, err := CompileAnonymous(`return 0;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "ChildComparable", IsInterface: true, Interfaces: []string{"Comparable"}}); err != nil {
		t.Fatal(err)
	}
	method := Method{Name: "Rank.compareTo", ClassName: "Rank", ReturnType: "Integer", Params: []Param{{Name: "other", Type: "Object"}}, Program: program}
	if err := machine.RegisterClass(Class{Name: "Rank", Interfaces: []string{"ChildComparable"}, Methods: map[string]Method{"compareTo": method}}); err != nil {
		t.Fatal(err)
	}
	values := []Value{Object("Rank"), Null}
	if err := machine.sortComparableValues(values, &Result{}); err != nil {
		t.Fatalf("indirect and null sort: %v", err)
	}
	if values[0].Kind != ValueNull {
		t.Fatalf("first sorted value = %v, want null", values[0])
	}
}

// NATIVE homogeneous indirect sort: ../apextest/testdata/conformance/stdlib_comparable_interface/org.tsv:2-5.
func TestComparableSortIndirectDecisionIsPerCall(t *testing.T) {
	program, err := CompileAnonymous(`return 0;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "ChildComparable", IsInterface: true, Interfaces: []string{"Comparable"}}); err != nil {
		t.Fatal(err)
	}
	method := Method{Name: "Rank.compareTo", ClassName: "Rank", ReturnType: "Integer", Params: []Param{{Name: "other", Type: "Object"}}, Program: program}
	if err := machine.RegisterClass(Class{Name: "Rank", Interfaces: []string{"ChildComparable"}, Methods: map[string]Method{"compareTo": method}}); err != nil {
		t.Fatal(err)
	}
	rank := func() Value {
		value := Object("Rank")
		value.classInstance = true
		return value
	}
	mixed := []Value{rank(), Null}
	if err := machine.sortComparableValues(mixed, &Result{}); err != nil {
		t.Fatalf("mixed sort: %v", err)
	}
	homogeneous := []Value{rank(), rank()}
	err = machine.sortComparableValues(homogeneous, &Result{})
	thrown, ok := err.(*apexThrowError)
	if !ok || thrown.value.Type != "System.ListException" {
		t.Fatalf("homogeneous sort error = %v, want System.ListException", err)
	}
	mixed = []Value{Null, rank()}
	if err := machine.sortComparableValues(mixed, &Result{}); err != nil {
		t.Fatalf("second mixed sort: %v", err)
	}
}

// Object values without the captured class-instance marker keep the base outcome.
func TestComparableSortUncapturedObjectValueNeighbor(t *testing.T) {
	program, err := CompileAnonymous(`return 0;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "ChildComparable", IsInterface: true, Interfaces: []string{"Comparable"}}); err != nil {
		t.Fatal(err)
	}
	method := Method{Name: "Rank.compareTo", ClassName: "Rank", ReturnType: "Integer", Params: []Param{{Name: "other", Type: "Object"}}, Program: program}
	if err := machine.RegisterClass(Class{Name: "Rank", Interfaces: []string{"ChildComparable"}, Methods: map[string]Method{"compareTo": method}}); err != nil {
		t.Fatal(err)
	}
	values := []Value{Object("Rank"), Object("Rank")}
	if err := machine.sortComparableValues(values, &Result{}); err != nil {
		t.Fatalf("object-value sort: %v", err)
	}
}

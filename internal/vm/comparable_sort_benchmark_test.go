package vm

import "testing"

func BenchmarkComparableObjectSort(b *testing.B) {
	program, err := CompileAnonymous(`return this.n - ((Rank)other).n;`)
	if err != nil {
		b.Fatal(err)
	}
	machine := New(nil)
	method := Method{Name: "Rank.compareTo", ClassName: "Rank", ReturnType: "Integer", Params: []Param{{Name: "other", Type: "Object"}}, Program: program}
	if err := machine.RegisterClass(Class{Name: "Rank", Interfaces: []string{"Comparable"}, Fields: map[string]Field{"n": {Name: "n", Type: "Integer"}}, Methods: map[string]Method{"compareTo": method}}); err != nil {
		b.Fatal(err)
	}
	const size = 24
	input := make([]Value, size)
	for i := range input {
		input[i] = Object("Rank")
		input[i].classInstance = true
		input[i].Fields["n"] = Int(int64(size - i - 1))
	}
	values := make([]Value, size)
	result := &Result{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(values, input)
		if err := machine.sortComparableValues(values, result); err != nil {
			b.Fatal(err)
		}
		for j, value := range values {
			if value.Fields["n"].Kind != ValueInt || value.Fields["n"].Int != int64(j) {
				b.Fatalf("sorted position %d = %v", j, value.Fields["n"])
			}
		}
	}
}

func BenchmarkComparableIndirectNullSort(b *testing.B) {
	program, err := CompileAnonymous(`return 0;`)
	if err != nil {
		b.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "ChildComparable", IsInterface: true, Interfaces: []string{"Comparable"}}); err != nil {
		b.Fatal(err)
	}
	method := Method{Name: "Rank.compareTo", ClassName: "Rank", ReturnType: "Integer", Params: []Param{{Name: "other", Type: "Object"}}, Program: program}
	if err := machine.RegisterClass(Class{Name: "Rank", Interfaces: []string{"ChildComparable"}, Methods: map[string]Method{"compareTo": method}}); err != nil {
		b.Fatal(err)
	}
	const size = 128
	input := make([]Value, size)
	for i := 0; i < size-1; i++ {
		input[i] = Object("Rank")
		input[i].classInstance = true
	}
	input[size-1] = Null
	values := make([]Value, size)
	result := &Result{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		copy(values, input)
		if err := machine.sortComparableValues(values, result); err != nil {
			b.Fatal(err)
		}
		if values[0].Kind != ValueNull {
			b.Fatalf("first sorted value = %v, want null", values[0])
		}
		for j := 1; j < size; j++ {
			if values[j].Ref != input[j-1].Ref {
				b.Fatalf("stable position %d changed", j)
			}
		}
	}
}

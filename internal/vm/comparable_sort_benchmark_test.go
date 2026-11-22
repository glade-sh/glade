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

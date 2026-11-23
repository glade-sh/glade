package vm

import "testing"

func BenchmarkRecursiveMethodDepth(b *testing.B) {
	program, err := CompileAnonymous(`if (n == 0) return 0; return 1 + Depth.run(n - 1);`)
	if err != nil {
		b.Fatal(err)
	}
	method := Method{Name: "Depth.run", ClassName: "Depth", IsStatic: true, ReturnType: "Integer", Params: []Param{{Name: "n", Type: "Integer"}}, Program: program}
	machine := New(nil)
	if err := machine.RegisterMethod(method); err != nil {
		b.Fatal(err)
	}
	result := &Result{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, err := machine.callMethodWithReceiver(method, Null, []Value{Int(997)}, result)
		if err != nil || value.Kind != ValueInt || value.Int != 997 {
			b.Fatalf("recursive outcome = %v, %v; want 997", value, err)
		}
	}
}

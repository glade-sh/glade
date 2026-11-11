package vm

import (
	"fmt"
	"testing"
)

func strictHeapBenchMachine(b *testing.B, classes, statics int) *VM {
	machine := New(nil)
	machine.SetLimitMode(LimitModeStrict)
	for c := 0; c < classes; c++ {
		fields := map[string]Field{}
		for s := 0; s < statics; s++ {
			name := fmt.Sprintf("s%d", s)
			fields[name] = Field{Name: name, Static: true, Value: String("static-value-text")}
		}
		if err := machine.RegisterClass(Class{Name: fmt.Sprintf("Holder%d", c), StaticFields: fields}); err != nil {
			b.Fatal(err)
		}
	}
	return machine
}

func BenchmarkStrictHeapLoop(b *testing.B) {
	program, err := CompileAnonymous(`Integer total = 0; List<Integer> items = new List<Integer>(); for (Integer i = 0; i < 300; i++) { total += i; items.add(i); }`)
	if err != nil {
		b.Fatal(err)
	}
	machine := strictHeapBenchMachine(b, 100, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := machine.CloneRuntimeFrozenShared(nil)
		m.SetLimitMode(LimitModeStrict)
		if _, err := m.Execute(program); err != nil {
			b.Fatal(err)
		}
	}
}

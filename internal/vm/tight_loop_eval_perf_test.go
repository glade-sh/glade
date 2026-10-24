package vm

import "testing"

func TestPositionalCallAvoidsEmptyNamedArgumentAllocation(t *testing.T) {
	for _, source := range []string{"text.length();", "items.size();"} {
		t.Run(source, func(t *testing.T) {
			program, err := CompileAnonymous(source)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.Globals["text"] = String("abcd")
			machine.Globals["items"] = List(Int(1), Int(2), Int(3), Int(4))
			machine.VarTypes["text"] = "String"
			machine.VarTypes["items"] = "List<Integer>"
			result := &Result{}
			allocations := testing.AllocsPerRun(100, func() {
				got, err := machine.eval(program.Instructions[0].Expr, result)
				if err != nil || got.Kind != ValueInt || got.Int != 4 {
					t.Fatalf("call = %v, %v; want 4", got, err)
				}
			})
			t.Logf("ROW %s outcome=4 allocations=%.0f", source, allocations)
			if allocations != 0 {
				t.Fatalf("zero-argument primitive call allocated %.0f times; want 0", allocations)
			}
		})
	}
}

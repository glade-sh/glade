//go:build !race

package vm

import "testing"

func TestMethodReturnAliasBookkeepingAvoidsEagerSliceAllocation(t *testing.T) {
	noTargetProgram, err := CompileAnonymous(`Integer localValue = 1;`)
	if err != nil {
		t.Fatal(err)
	}
	noTargetMethod := Method{Name: "NoTarget.run", ReturnType: "void", Program: noTargetProgram}
	noTargetAllocs := testing.AllocsPerRun(100, func() {
		machine := New(nil)
		if _, err := machine.callMethodWithReceiver(noTargetMethod, Null, nil, &Result{}); err != nil {
			panic(err)
		}
	})
	if noTargetAllocs > 77 {
		t.Fatalf("no-target method allocated %.0f objects, want at most 77 without an alias target slice", noTargetAllocs)
	}

	oneTargetProgram, err := CompileAnonymous(`value.put('result', 'same-reference');`)
	if err != nil {
		t.Fatal(err)
	}
	oneTargetMethod := Method{
		Name:       "OneTarget.run",
		ReturnType: "void",
		Params:     []Param{{Name: "value", Type: "Map<String,String>"}},
		Program:    oneTargetProgram,
	}
	oneTargetAllocs := testing.AllocsPerRun(100, func() {
		machine := New(nil)
		value := Map()
		value.Type = "Map<String,String>"
		machine.Globals["alias"] = value
		if _, err := machine.callMethodWithReceiver(oneTargetMethod, Null, []Value{value}, &Result{}); err != nil {
			panic(err)
		}
	})
	if oneTargetAllocs > 115 {
		t.Fatalf("one-target method allocated %.0f objects, want at most 115 without an alias target slice", oneTargetAllocs)
	}
}

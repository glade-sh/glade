package vm

import (
	"testing"
)

func TestHeapSizeCountsAliasesOnceAcrossGlobalAndStaticRoots(t *testing.T) {
	object := Object("HeapValue")
	object.Fields["text"] = String("payload")
	mapping := Map()
	mapping.Map["text"] = String("payload")
	for _, value := range []Value{List(String("payload")), Set(String("payload")), mapping, object} {
		for _, referenced := range []bool{true, false} {
			t.Run(string(value.Kind)+"/referenced="+Bool(referenced).String(), func(t *testing.T) {
				if !referenced {
					value.Ref = 0
				}
				machine := New(nil)
				machine.Globals = map[string]Value{"active": value, "alias": value}
				machine.Classes["HeapRegistry"] = Class{Name: "HeapRegistry", StaticFields: map[string]Field{"held": {Name: "held", Static: true, Value: value}}}
				want := approxValueSize(value) + len("active") + len("alias") + len("held")
				if got := machine.currentHeapSize(); got != want {
					t.Fatalf("heap = %d, want one mutable value: %d", got, want)
				}
			})
		}
	}
}

func TestHeapSizeHandlesMutableCyclesAcrossGlobalAndStaticRoots(t *testing.T) {
	for _, referenced := range []bool{true, false} {
		t.Run("referenced="+Bool(referenced).String(), func(t *testing.T) {
			value := Map()
			if !referenced {
				value.Ref = 0
			}
			value.Map["self"] = value
			machine := New(nil)
			machine.Globals = map[string]Value{"active": value}
			machine.Classes["HeapRegistry"] = Class{Name: "HeapRegistry", StaticFields: map[string]Field{"held": {Name: "held", Static: true, Value: value}}}
			want := 24 + len("self") + len("active") + len("held")
			if got := machine.currentHeapSize(); got != want {
				t.Fatalf("cyclic heap = %d, want %d", got, want)
			}
		})
	}
}

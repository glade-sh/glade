package vm

import "testing"

// These are local characterization controls. The native positive capture has
// a strict preference in the typed argument; it does not decide mixed ties.
func TestOverloadBareNullKeepsUndecidedTypedTieBehavior(t *testing.T) {
	machine := New(nil)
	left := Method{Params: []Param{{Type: "Integer"}, {Type: "Object"}}}
	right := Method{Params: []Param{{Type: "Integer"}, {Type: "List<String>"}}}
	for _, args := range [][]Value{{Int(1), Null}, {Null, Null}} {
		if got := machine.compareMethodSpecificityForArgs(left, right, args); got != 1 {
			t.Fatalf("undecided typed arguments changed baseline preference: %d", got)
		}
	}
}

package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// The control executes the same declarations without a lexical scope. Its map
// cell allocations are not part of the saved-binding work being bounded.
func TestBlockScopeSavedBindingWorkBound(t *testing.T) {
	machine := New(nil)
	machine.SetTraceEnabled(false)
	block := ir.Instruction{Op: ir.OpFor, Then: []ir.Instruction{
		{Op: ir.OpDeclare, Name: "at", Type: "Integer"},
		{Op: ir.OpDeclare, Name: "value", Type: "Integer"},
	}}
	scoped := testing.AllocsPerRun(30, func() {
		if _, err := machine.executeBlockScope(ir.Program{}, block, nil); err != nil {
			t.Fatal(err)
		}
		if _, ok := machine.Globals["at"]; ok {
			t.Fatal("loop local escaped")
		}
		if _, ok := machine.VarTypes["value"]; ok {
			t.Fatal("loop type escaped")
		}
	})
	direct := testing.AllocsPerRun(30, func() {
		if _, err := machine.executeProgram(ir.Program{Instructions: block.Then}, nil); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"at", "value"} {
			delete(machine.Globals, name)
			delete(machine.VarTypes, name)
		}
	})
	t.Logf("scoped=%.0f direct=%.0f saved-binding allocations=%.0f", scoped, direct, scoped-direct)
	if scoped-direct > 1 {
		t.Fatalf("saved-binding allocation work = %.0f, want <= 1", scoped-direct)
	}
}

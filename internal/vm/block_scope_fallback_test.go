package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Supplemental coverage for scope shapes that use the general snapshot path.
func TestBlockScopeFallbackRestoresBindings(t *testing.T) {
	for _, names := range [][]string{{"alpha", "beta", "gamma"}, {"value", "VALUE"}, {"first", "first", "second", "third"}} {
		t.Run(names[0]+"/"+names[len(names)-1], func(t *testing.T) {
			machine := New(nil)
			machine.finalLocals = make(map[string]bool)
			block := ir.Instruction{Op: ir.OpBlock}
			for _, name := range names {
				machine.Globals[name] = Int(37)
				machine.VarTypes[name] = "Long"
				machine.finalLocals[name] = true
				block.Then = append(block.Then, ir.Instruction{Op: ir.OpDeclare, Name: name, Type: "Integer"})
			}
			if _, err := machine.executeBlockScope(ir.Program{}, block, nil); err != nil {
				t.Fatal(err)
			}
			for _, name := range names {
				if machine.Globals[name].Int != 37 || machine.VarTypes[name] != "Long" || !machine.finalLocals[name] {
					t.Fatalf("binding %q was not restored", name)
				}
			}
		})
	}
}

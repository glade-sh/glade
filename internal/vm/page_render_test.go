package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

func TestConstructControllerWithTraceEnabledDoesNotPanicOnNilResult(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:   "PageController",
		Access: "public",
		Constructors: []Method{{
			Name:          "PageController.<init>",
			ClassName:     "PageController",
			IsConstructor: true,
			Access:        "public",
			Program: ir.Program{Instructions: []ir.Instruction{{
				Op: ir.OpReturn,
			}}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ConstructController("PageController"); err != nil {
		t.Fatalf("ConstructController() err = %v", err)
	}
}

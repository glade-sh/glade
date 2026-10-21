package vm

import (
	"errors"
	"io"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

func TestFinalLocalDeclarationIR(t *testing.T) {
	program, err := CompileAnonymous("final Integer first=1; final Integer later; Integer mutable=2;")
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Instructions) != 3 || !program.Instructions[0].Final || !program.Instructions[1].Final || program.Instructions[2].Final {
		t.Fatalf("declaration final metadata = %#v", program.Instructions)
	}
	if program.Instructions[0].Expr.Kind == "" || program.Instructions[1].Expr.Kind != "" {
		t.Fatal("declaration lost the initialized versus uninitialized distinction")
	}
}

// State isolation checks supplement the captured API62/68 source rows; these
// inspect VM lifecycle invariants rather than claiming additional native parity.
func TestFinalLocalBindingFrameAndCloneIsolation(t *testing.T) {
	machine := New(io.Discard)
	if err := machine.RegisterClass(Class{Name: "Helper"}); err != nil {
		t.Fatal(err)
	}
	compile := func(source string) ir.Program {
		t.Helper()
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		return program
	}
	if _, err := machine.Execute(compile("final Integer x=1; final Integer later;")); err != nil {
		t.Fatal(err)
	}
	if !machine.finalLocals["x"] || machine.finalLocals["later"] {
		t.Fatal("incorrect final binding initialization state")
	}
	method := Method{Name: "Helper.step", ClassName: "Helper", ReturnType: "Integer", Params: []Param{{Name: "x", Type: "Integer"}},
		Program: compile("x+=1; final Integer local=4; return x;")}
	value, err := machine.callMethodWithReceiver(method, Object("Helper"), []Value{Int(2)}, &Result{})
	if err != nil || value.Int != 3 {
		t.Fatalf("mutable helper parameter = %#v, %v", value, err)
	}
	if len(machine.finalLocals) != 2 || !machine.finalLocals["x"] || machine.Globals["x"].Int != 1 {
		t.Fatal("helper frame leaked or lost caller final bindings")
	}
	trigger := Trigger{Name: "LocalScopeTrigger", Object: "Account", Operation: "insert", Timing: triggerTimingBefore,
		Program: compile("Integer x=2; x=3; final Integer triggerLocal=4;")}
	if _, err := machine.runTrigger(trigger, nil, nil, &Result{}); err != nil {
		t.Fatal(err)
	}
	if len(machine.finalLocals) != 2 || !machine.finalLocals["x"] || machine.Globals["x"].Int != 1 {
		t.Fatal("trigger frame leaked or lost caller final bindings")
	}
	machine.FreezeClassLookup()
	for _, clone := range []*VM{machine.CloneRuntime(io.Discard), machine.CloneRuntimeFrozenShared(io.Discard)} {
		if len(clone.finalLocals) != 0 {
			t.Fatal("runtime clone inherited request-local final bindings")
		}
		if _, err := clone.Execute(compile("Integer x=2; x=3;")); err != nil {
			t.Fatal(err)
		}
	}
	if err := machine.assign("later", Int(1)); err != nil {
		t.Fatalf("first late initialization: %v", err)
	}
	for _, name := range []string{"x", "later"} {
		err := machine.assign(name, Int(2))
		var runtimeErr *RuntimeError
		var caught *apexThrowError
		if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.FinalException" ||
			runtimeErr.Message != "Final variable has already been initialized" || errors.As(err, &caught) {
			t.Fatalf("final rebinding must remain a raw uncatchable error: %v", err)
		}
	}
}

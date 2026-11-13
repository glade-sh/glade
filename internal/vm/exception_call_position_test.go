package vm

import (
	"errors"
	"testing"
)

// NATIVE ../apextest/testdata/exception_implicit_call_frames/org.tsv:2-3.
// The direct VM route also retains the call-token line in RuntimeError.Stack.
func TestExceptionCallPositionAfterImplicitGetter(t *testing.T) {
	for _, argument := range []string{"1", "arg.Value"} {
		t.Run(argument, func(t *testing.T) {
			machine := New(nil)
			getter, err := CompileAnonymous("return 1;")
			if err != nil {
				t.Fatal(err)
			}
			fail, err := CompileAnonymous("throw new IllegalArgumentException('target');")
			if err != nil {
				t.Fatal(err)
			}
			caller, err := CompileAnonymous("Arg arg = new Arg();\nTarget target = new Target();\ntarget\n.fail(" + argument + ");")
			if err != nil {
				t.Fatal(err)
			}
			for _, class := range []Class{
				{Name: "Arg", Fields: map[string]Field{"Value": {Name: "Value", Type: "Integer", Property: true, Getter: &Method{Name: "Arg.Value.get", ClassName: "Arg", ReturnType: "Integer", Program: getter}}}},
				{Name: "Target", Methods: map[string]Method{"fail": {Name: "Target.fail", ClassName: "Target", Params: []Param{{Name: "value", Type: "Integer"}}, Program: fail}}},
				{Name: "Caller", Methods: map[string]Method{"run": {Name: "Caller.run", ClassName: "Caller", IsStatic: true, Program: caller}}},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			program, err := CompileAnonymous("Caller.run();")
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			var fault *RuntimeError
			if !errors.As(err, &fault) {
				t.Fatalf("expected runtime exception: %v", err)
			}
			if len(fault.Stack) < 2 || fault.Stack[1].Symbol != "Caller.run" || fault.Stack[1].Line != 4 {
				t.Fatalf("caller must retain call-token line 4: %#v", fault.Stack)
			}
		})
	}
}

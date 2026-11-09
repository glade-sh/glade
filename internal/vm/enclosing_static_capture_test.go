package vm

import "testing"

func TestCapturedEnclosingStaticDispatch(t *testing.T) {
	machine := New(nil)
	for _, class := range []Class{{Name: "Probe"}, {Name: "Probe.Adapter"}} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	program, err := CompileAnonymous("return true;")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Probe.ok", ClassName: "Probe", ReturnType: "Boolean", IsStatic: true, Program: program}); err != nil {
		t.Fatal(err)
	}
	if _, ok, ambiguous := machine.resolveStaticMethodForArgs("Probe.Adapter", "ok", nil); !ok || ambiguous {
		t.Fatalf("captured static call unresolved: ok=%t ambiguous=%t", ok, ambiguous)
	}
	if _, ok, _ := machine.resolveStaticMethodForArgs("Probe.Adapter", "missing", nil); ok {
		t.Fatal("missing method resolved")
	}
}

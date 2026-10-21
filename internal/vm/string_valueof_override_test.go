package vm

import "testing"

func TestExecStringValueOfNullableOverrideRunsOnce(t *testing.T) {
	// T05_VALUE supplies the null-result oracle. This counter is a local
	// single-dispatch control, not an additional native observation.
	body, err := CompileAnonymous("calls++; return null;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "framework_QualifiedMethod",
		StaticFields: map[string]Field{
			"calls": {Name: "calls", Type: "Integer", Static: true, Value: Int(0), InitialValue: Int(0)},
		},
		Methods: map[string]Method{
			"toString": {Name: "framework_QualifiedMethod.toString", ClassName: "framework_QualifiedMethod", ReturnType: "String", Program: body},
		},
	}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("Object value = new framework_QualifiedMethod(); System.assertEquals(null, String.valueOf(value)); System.assertEquals(1, framework_QualifiedMethod.calls);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

package vm

import "testing"

func TestInheritedTestReceiverControls(t *testing.T) {
	for _, mode := range []string{"declared-zero", "declared-argument", "local", "user-class"} {
		t.Run(mode, func(t *testing.T) {
			returned, err := CompileAnonymous("return false;")
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.EnableTestContext()
			harness := Class{Name: "PredicateHarness", Methods: map[string]Method{}}
			method := Method{Name: "PredicateHarness.isRunningTest", ClassName: "PredicateHarness", ReturnType: "Boolean", Program: returned}
			args := ""
			if mode == "declared-argument" {
				method.Params = []Param{{Name: "input", Type: "Boolean"}}
				args = "true"
			}
			harness.Methods["isRunningTest"] = method
			if err := machine.RegisterClass(harness); err != nil {
				t.Fatal(err)
			}
			value := Object("PredicateHarness")
			value.Fields["Database"] = String("marker")
			if err := machine.RegisterClass(Class{Name: "PredicateOwner", StaticFields: map[string]Field{
				"Test": {Name: "Test", Type: "PredicateHarness", Static: true, Property: true, HasGetter: true, Access: "public", Value: value},
			}}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterClass(Class{Name: "PredicateReader", SuperClass: "PredicateOwner"}); err != nil {
				t.Fatal(err)
			}
			prefix := ""
			if mode == "local" {
				prefix = "PredicateHarness test=new PredicateHarness();"
			}
			if mode == "user-class" {
				if err := machine.RegisterClass(Class{Name: "Test", Methods: map[string]Method{
					"isRunningTest": {Name: "Test.isRunningTest", ClassName: "Test", IsStatic: true, ReturnType: "Boolean", Program: returned},
				}}); err != nil {
					t.Fatal(err)
				}
			}
			program, err := CompileAnonymous(prefix + "return Test.isRunningTest(" + args + ");")
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{Name: "PredicateReader.read", ClassName: "PredicateReader", IsStatic: true, ReturnType: "Boolean", Program: program}); err != nil {
				t.Fatal(err)
			}
			control, err := CompileAnonymous("System.assertEquals(false,PredicateReader.read());System.assertEquals(true,System.Test.isRunningTest());System.assertEquals('marker',PredicateReader.Test.Database);")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(control); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInheritedTestReceiverInstanceFieldControl(t *testing.T) {
	machine := New(nil)
	machine.EnableTestContext()
	returned, err := CompileAnonymous("return false;")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "DeclaredPredicate", Methods: map[string]Method{
		"isRunningTest": {Name: "DeclaredPredicate.isRunningTest", ClassName: "DeclaredPredicate", ReturnType: "Boolean", Program: returned},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "EmptyPredicate"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "InstanceOwner", StaticFields: map[string]Field{
		"Test": {Name: "Test", Type: "EmptyPredicate", Static: true, Property: true, HasGetter: true, Value: Object("EmptyPredicate")},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "InstanceReader", SuperClass: "InstanceOwner", Fields: map[string]Field{
		"Test": {Name: "Test", Type: "DeclaredPredicate"},
	}}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("return Test.isRunningTest();")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "InstanceReader.read", ClassName: "InstanceReader", ReturnType: "Boolean", Program: program}); err != nil {
		t.Fatal(err)
	}
	receiver := Object("InstanceReader")
	receiver.Fields["Test"] = Object("DeclaredPredicate")
	machine.Globals["reader"] = receiver
	control, err := CompileAnonymous("System.assertEquals(false,reader.read());System.assertEquals(true,System.Test.isRunningTest());")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(control); err != nil {
		t.Fatal(err)
	}
}

package vm

import "testing"

// Owned synthetic declarations exercise an interface cast of this inside an
// inherited method, with an exact signature on the concrete nested receiver.
func TestInterfaceCastOfInheritedThisDispatchesConcreteBody(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, call := range []string{"action.run(records);", "((ActionContract.Apply)action).apply(records);"} {
			t.Run(api+"/"+call, func(t *testing.T) {
				machine := New(nil)
				machine.EnableTestContext()
				for _, class := range []Class{
					{Name: "ActionContract"},
					{Name: "ActionContract.Apply", IsInterface: true},
					{Name: "ActionBase"},
					{Name: "ActionProof", IsTest: true},
					{Name: "ActionProof.Concrete", SuperClass: "ActionBase", Interfaces: []string{"ActionContract.Apply"}, Fields: map[string]Field{"called": {Name: "called", Type: "Boolean"}}},
				} {
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
				}
				params := []Param{{Name: "records", Type: "List<SObject>"}}
				if err := machine.RegisterMethod(Method{Name: "ActionContract.Apply.apply", ClassName: "ActionContract.Apply", ReturnType: "void", Modifiers: []string{"abstract"}, Params: params}); err != nil {
					t.Fatal(err)
				}
				for _, method := range []struct{ owner, name, body string }{
					{"ActionBase", "run", "((ActionContract.Apply)this).apply(records);"},
					{"ActionProof.Concrete", "apply", "this.called = true;"},
				} {
					program, err := CompileAnonymousWithOptions(method.body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterMethod(Method{Name: method.owner + "." + method.name, ClassName: method.owner, ReturnType: "void", Params: params, Program: program}); err != nil {
						t.Fatal(err)
					}
				}
				program, err := CompileAnonymousWithOptions("ActionProof.Concrete action = new ActionProof.Concrete(); List<SObject> records = new List<SObject>(); "+call+" System.assertEquals(true, action.called);", CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// This is a PENDING-ROWS control of the existing record-list candidate. Public
// interface documentation alone does not settle narrowed list signatures.
func TestAbstractListInterfaceObjectCastCandidate(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, allocation := range []string{"new AccountAction()", "Type.forName('AccountAction').newInstance()"} {
			t.Run(api+"/"+allocation, func(t *testing.T) {
				machine := abstractListInterfaceMachine(t, api)
				// The shared helper declares a private top-level test class.
				// Reflection requires the concrete declaration to be public.
				concrete, _ := machine.lookupClass("AccountAction")
				concrete.Access = "public"
				if err := machine.RegisterClass(concrete); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				program, err := CompileAnonymousWithOptions(`((RecordAction)action).act(records, context);`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{
					Name: "ActionInvoker.dispatch", ClassName: "ActionInvoker", ReturnType: "void", IsStatic: true,
					Params: []Param{{Name: "action", Type: "Object"}, {Name: "records", Type: "List<SObject>"}, {Name: "context", Type: "ActionContext"}}, Program: program,
				}); err != nil {
					t.Fatal(err)
				}
				program, err = CompileAnonymousWithOptions(`Object action = `+allocation+`; List<SObject> records = new List<SObject>{new Account(Name='marker')}; ActionContext context = new ActionContext(); ActionInvoker.dispatch(action, records, context); System.assertEquals('marker', context.marker);`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

package vm

import "testing"

func TestNestedProtectedOverrideUsesDeclaredFieldRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"this.conductor", "conductor", "local"} {
			for _, view := range []string{"DispatchLeaf", "Object"} {
				t.Run(api+"/"+route+"/"+view, func(t *testing.T) {
					machine := New(nil)
					for _, class := range []Class{
						{Name: "DispatchRoot"},
						{Name: "DispatchRoot.Queued", SuperClass: "DispatchRoot"},
						{Name: "DispatchMiddle", SuperClass: "DispatchRoot.Queued"},
						{Name: "DispatchOuter", SuperClass: "DispatchMiddle"},
						{Name: "DispatchLeaf", SuperClass: "DispatchOuter"},
						{Name: "DispatchOuter.Completion", Fields: map[string]Field{"conductor": {Name: "conductor", Type: "DispatchOuter"}}},
					} {
						class.APIVersion = api
						if class.Name == "DispatchRoot.Queued" || class.Name == "DispatchOuter" || class.Name == "DispatchLeaf" {
							marker := map[string]string{"DispatchRoot.Queued": "queued", "DispatchOuter": "outer", "DispatchLeaf": "leaf"}[class.Name]
							program, err := CompileAnonymous("return '" + marker + "';")
							if err != nil {
								t.Fatal(err)
							}
							class.Methods = map[string]Method{"beginWork": {Name: class.Name + ".beginWork", ClassName: class.Name, Access: "protected", ReturnType: "String", APIVersion: api, Program: program}}
						}
						if err := machine.RegisterClass(class); err != nil {
							t.Fatal(err)
						}
					}
					machine.currentClass = "DispatchOuter.Completion"
					machine.currentMethod = Method{Name: "DispatchOuter.Completion.complete", ClassName: machine.currentClass}
					// The queueable/finalizer project regression produces this ancestor
					// view after enqueueing from the inherited nested superclass.
					receiver := Object(view)
					receiver.Runtime = "DispatchLeaf"
					receiver.Static = "DispatchRoot.Queued"
					if route == "local" {
						machine.VarTypes[route] = "DispatchOuter"
					}
					value, handled, err := machine.callValueMember(route, receiver, "beginWork", nil, &Result{})
					if err != nil || !handled || !value.Equal(String("leaf")) {
						t.Fatalf("native nested field dispatch returns leaf; value=%v handled=%v err=%v", value, handled, err)
					}
				})
			}
		}
	}
}

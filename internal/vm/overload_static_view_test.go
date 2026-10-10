package vm

import "testing"

func TestOverloadApplicabilityPreservesLocalTypeAfterAliasRefresh(t *testing.T) {
	for _, spelling := range []string{"local", "LOCAL"} {
		t.Run(spelling, func(t *testing.T) {
			machine := New(nil)
			for _, class := range []Class{
				{
					Name: "BaseValue",
					Fields: map[string]Field{
						"count": {Name: "count", Type: "Integer", InitialValue: Int(0)},
					},
				},
				{Name: "ChildValue", SuperClass: "BaseValue"},
				{Name: "Helper"},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			body, err := CompileAnonymous("return value.count;")
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{
				Name: "Helper.onlyChild", ClassName: "Helper", IsStatic: true,
				ReturnType: "Integer", Params: []Param{{Name: "value", Type: "ChildValue"}},
				Program: body,
			}); err != nil {
				t.Fatal(err)
			}
			setup, err := CompileAnonymous("ChildValue local = new ChildValue(); local.count = 1;")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(setup); err != nil {
				t.Fatal(err)
			}

			// A refreshed holder graph can carry its BaseValue field's view
			// of the same object into the caller's ChildValue local binding.
			updated := machine.Globals["local"]
			updated.Static = "BaseValue"
			updated.Fields = map[string]Value{"count": Int(2)}
			holder := Object("Holder")
			holder.Fields["stored"] = updated
			machine.propagateUpdatedValueAliases(machine.Globals, holder)

			program, err := CompileAnonymous("System.assertEquals(2, Helper.onlyChild(" + spelling + ")); System.assertEquals(2, " + spelling + ".count);")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

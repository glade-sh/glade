package vm

import (
	"fmt"
	"testing"
)

// collection_cast_reflection.json, PARAM_SOBJECT_VAR_OPPORTUNITY, records
// this empty-list parameter chain at API 62/67/68 in both execution contexts.
func TestListSObjectCastParameterReflection(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(fmt.Sprintf("%s/%s", api, route), func(t *testing.T) {
				inspect, err := CompileAnonymousWithOptions(`
String detail;
try { Account discarded = (Account)boxed; }
catch (TypeException failure) {
    System.assertEquals('System.TypeException', failure.getTypeName());
    detail = failure.getMessage();
}
System.assertEquals('Invalid conversion from runtime type List<Opportunity> to Account', detail);
String element = detail.substringBetween('List<', '>');
System.assertEquals('Opportunity', element);
System.assertEquals('Opportunity', Type.forName(element).getName());
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				view, err := CompileAnonymousWithOptions("CastNameProbe.inspect(input);", CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				program, err := CompileAnonymousWithOptions(`
List<SObject> input = new List<Opportunity>();
CastNameProbe.listView(input);
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				if err := machine.RegisterClass(Class{
					Name: "CastNameProbe",
					Methods: map[string]Method{
						"inspect":  {Name: "CastNameProbe.inspect", ClassName: "CastNameProbe", ReturnType: "void", Params: []Param{{Name: "boxed", Type: "Object"}}, IsStatic: true, APIVersion: api, Program: inspect},
						"listView": {Name: "CastNameProbe.listView", ClassName: "CastNameProbe", ReturnType: "void", Params: []Param{{Name: "input", Type: "List<Object>"}}, IsStatic: true, APIVersion: api, Program: view},
					},
				}); err != nil {
					t.Fatal(err)
				}
				if route == "isTest" {
					machine.EnableTestContext()
					_, err = machine.ExecuteInClass(program, "CastNameProbe")
				} else {
					_, err = machine.Execute(program)
				}
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

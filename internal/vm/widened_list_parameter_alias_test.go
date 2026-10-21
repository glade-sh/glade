package vm

import "testing"

// Owned API 62/67/68 anonymous and @IsTest observations are exported in
// internal/apextest/testdata/conformance/widened_list_alias_native.json.
func TestWidenedListParameterAliasMutation(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, body, call, checks string
		}{
			{
				name: "comparatorSort",
				body: `values.sort(order); return values;`,
				call: `WideListOps.apply(original, new AliasOrder());`,
				checks: `System.assertEquals(new List<Integer>{1, 2, 3}, original);
System.assertEquals(new List<Integer>{1, 2, 3}, alias);`,
			},
			{
				name: "sameLengthSet",
				body: `values.set(0, 8); return values;`,
				call: `WideListOps.apply(original, new AliasOrder());`,
				checks: `System.assertEquals(new List<Integer>{8, 1, 2}, original);
System.assertEquals(new List<Integer>{8, 1, 2}, alias);`,
			},
			{
				name: "unchangedView",
				body: `return values;`,
				call: `WideListOps.apply(original, new AliasOrder());`,
				checks: `System.assertEquals(new List<Integer>{3, 1, 2}, original);
System.assertEquals(new List<Integer>{3, 1, 2}, alias);`,
			},
			{
				name: "parameterRebinding",
				body: `values = new List<Object>{8, 9, 10}; return values;`,
				call: `WideListOps.apply(original, new AliasOrder());`,
				checks: `System.assertEquals(new List<Integer>{3, 1, 2}, original);
System.assertEquals(new List<Integer>{3, 1, 2}, alias);`,
			},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				machine := New(nil)
				for _, class := range []Class{
					{Name: "WideListOps"},
					{Name: "AliasOrder", Interfaces: []string{"Comparator"}},
				} {
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
				}
				for _, definition := range []struct {
					owner, member, result, body string
					params                      []Param
					static                      bool
				}{
					{
						owner: "AliasOrder", member: "compare", result: "Integer",
						params: []Param{{Name: "left", Type: "Object"}, {Name: "right", Type: "Object"}},
						body:   `return (Integer)left - (Integer)right;`,
					},
					{
						owner: "WideListOps", member: "apply", result: "List<Object>", static: true,
						params: []Param{{Name: "values", Type: "List<Object>"}, {Name: "order", Type: "Comparator"}},
						body:   tc.body,
					},
				} {
					program, err := CompileAnonymousWithOptions(definition.body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterMethod(Method{
						Name: definition.owner + "." + definition.member, ClassName: definition.owner,
						ReturnType: definition.result, Params: definition.params, IsStatic: definition.static,
						Program: program,
					}); err != nil {
						t.Fatal(err)
					}
				}
				source := `List<Integer> original = new List<Integer>{3, 1, 2};
List<Integer> alias = original;` + tc.call + tc.checks + `
System.assertEquals(3, original.size());
Object boxedOriginal = original;
Object boxedAlias = alias;
System.assert(boxedOriginal instanceof List<Integer>);
System.assert(boxedAlias instanceof List<Integer>);`
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
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

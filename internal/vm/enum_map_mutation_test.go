package vm

import (
	"strings"
	"testing"
)

func TestExecEnumMapKeyRoutes(t *testing.T) {
	routes := []struct {
		name      string
		namespace string
		keyType   string
		key       string
	}{
		{name: "values", keyType: "SignalHue", key: "SignalHue.values().get(0)"},
		{name: "namespacedValues", namespace: "orchard", keyType: "SignalHue", key: "SignalHue.values().get(0)"},
		{name: "namespacedParameter", namespace: "orchard", keyType: "orchard.SignalHue", key: "key"},
	}
	operations := []struct {
		name string
		body string
	}{
		{name: "get", body: `System.assertEquals(7, marks.get(lookup));`},
		{name: "containsKey", body: `System.assertEquals(true, marks.containsKey(lookup));`},
		{name: "put", body: `
System.assertEquals(7, marks.put(lookup, 11));
System.assertEquals(2, marks.size());
System.assertEquals(11, marks.get(SignalHue.Amber));
System.assertEquals(9, marks.get(SignalHue.Indigo));
`},
		{name: "remove", body: `
System.assertEquals(7, marks.remove(lookup));
System.assertEquals(1, marks.size());
System.assertEquals(false, marks.containsKey(SignalHue.Amber));
System.assertEquals(9, marks.get(SignalHue.Indigo));
`},
	}

	for _, route := range routes {
		for _, operation := range operations {
			t.Run(route.name+"/"+operation.name, func(t *testing.T) {
				program, err := CompileAnonymous(`
Map<` + route.keyType + `, Integer> marks = new Map<` + route.keyType + `, Integer>{
    SignalHue.Amber => 7, SignalHue.Indigo => 9
};
` + strings.ReplaceAll(operation.body, "lookup", route.key))
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				if err := machine.RegisterClass(Class{
					Name: "SignalHue", Namespace: route.namespace, EnumValues: []string{"Amber", "Indigo"},
				}); err != nil {
					t.Fatal(err)
				}
				if route.name != "namespacedParameter" {
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					return
				}
				if err := machine.RegisterClass(Class{
					Name: "SignalCheck", Namespace: route.namespace,
					Methods: map[string]Method{
						"run": {
							Name: "SignalCheck.run", ClassName: "SignalCheck", IsStatic: true, Access: "public",
							ReturnType: "void", Params: []Param{{Name: "key", Type: "SignalHue"}}, Program: program,
						},
					},
				}); err != nil {
					t.Fatal(err)
				}
				entry, err := CompileAnonymous("orchard.SignalCheck.run(orchard.SignalHue.Amber);")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.ExecuteInClass(entry, "orchard.SignalCheck"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// iterator_enum_native.json captures these overload selections at APIs
// 62/67/68 in anonymous Apex and @IsTest, including field-token collections.
func TestIteratorArgumentOverloadsPreserveCollectionBase(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct{ name, body string }{
			{"field-iterator", `Iterator<Schema.SObjectField> values = new List<Schema.SObjectField>{Account.Name}.iterator();
System.assertEquals('iterator', new TokenCollector().add(values).value());`},
			{"string-iterator", `Iterator<String> values = new List<String>{'Name'}.iterator();
System.assertEquals('iterator', new TokenCollector().add(values).value());`},
			{"object-iterator", `Iterator<Object> values = new List<Object>{'Name'}.iterator();
System.assertEquals('iterator', new TokenCollector().add(values).value());`},
			{"field-list", `System.assertEquals('field-list', new TokenCollector().add(new List<Schema.SObjectField>{Account.Name}).value());`},
			{"field-set", `System.assertEquals('field-set', new TokenCollector().add(new Set<Schema.SObjectField>{Account.Name}).value());`},
			{"string-list", `System.assertEquals('string-list', new TokenCollector().add(new List<String>{'Name'}).value());`},
			{"string-set", `System.assertEquals('string-set', new TokenCollector().add(new Set<String>{'Name'}).value());`},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				compile := func(source string) ir.Program {
					t.Helper()
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					return program
				}
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				if err := machine.RegisterClass(Class{
					Name: "TokenCollector",
					Fields: map[string]Field{
						"selected": {Name: "selected", Type: "String"},
					},
				}); err != nil {
					t.Fatal(err)
				}
				for _, overload := range []struct{ param, selected string }{
					{"List<String>", "string-list"},
					{"Set<String>", "string-set"},
					{"List<Schema.SObjectField>", "field-list"},
					{"Set<Schema.SObjectField>", "field-set"},
					{"Iterator<Object>", "iterator"},
				} {
					if err := machine.RegisterMethod(Method{
						Name: "TokenCollector.add", ClassName: "TokenCollector", ReturnType: "TokenCollector",
						Params:  []Param{{Name: "values", Type: overload.param}},
						Program: compile("this.selected = '" + overload.selected + "'; return this;"),
					}); err != nil {
						t.Fatal(err)
					}
				}
				if err := machine.RegisterMethod(Method{
					Name: "TokenCollector.value", ClassName: "TokenCollector", ReturnType: "String",
					Program: compile("return this.selected;"),
				}); err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(compile(tc.body)); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestIteratorFieldArgumentsKeepQualifiedAncestrySeparate(t *testing.T) {
	machine := New(nil)
	for _, class := range []Class{
		{Name: "sample.Root", Namespace: "sample"},
		{Name: "sample.Leaf", Namespace: "sample", SuperClass: "sample.Root"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{"List<Schema.SObjectField>", "Set<Schema.SObjectField>"} {
		if machine.typeAssignableTo("Iterator<Schema.SObjectField>", target) {
			t.Errorf("iterator field argument matched unrelated collection %s", target)
		}
	}
	for _, pair := range [][2]string{
		{"sample.Leaf", "sample.Root"},
		{"sample.Leaf", "Root"},
		{"List<Schema.SObjectField>", "List<Schema.SObjectField>"},
		{"Set<Schema.SObjectField>", "Set<Schema.SObjectField>"},
		{"Iterator<Schema.SObjectField>", "Iterator<Object>"},
		{"List<Schema.SObjectField>", "Iterable<Schema.SObjectField>"},
	} {
		if !machine.typeAssignableTo(pair[0], pair[1]) {
			t.Errorf("existing compatible type relation rejected %s -> %s", pair[0], pair[1])
		}
	}
}

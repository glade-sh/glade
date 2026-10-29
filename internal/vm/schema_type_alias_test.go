package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSchemaTypeAliasIdentitiesNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, name := range []string{"DescribeFieldResult", "DescribeSObjectResult", "SObjectType", "SObjectField"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				// schema_type_alias_boundaries/org.tsv:2-5 captures both
				// routes, including equality, display, sets and map keys.
				source := fmt.Sprintf(`
Type shortName = %s.class;
Type qualified = Schema.%s.class;
System.assertEquals('Schema.%s', shortName.getName());
System.assertEquals(qualified, shortName);
System.assert(shortName.equals(qualified));
System.assertEquals(1, new Set<Type>{shortName, qualified}.size());
Map<Type,String> keyed = new Map<Type,String>{shortName => 'retained'};
System.assertEquals('retained', keyed.get(qualified));
System.assertNotEquals(String.class, shortName);
System.assertNotEquals(null, shortName);
`, name, name, name)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				org := storage.NewOrgState()
				org.APIVersion = api
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestSchemaTypeFactoryShadowingNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, name := range []string{"DescribeFieldResult", "DescribeSObjectResult", "SObjectType", "SObjectField"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				// schema_type_factory_boundaries/org.tsv:2-5 distinguishes
				// global lookup from lexical literals and qualified source types.
				owner := "OwnedFactory" + name
				machine := New(nil)
				org := storage.NewOrgState()
				org.APIVersion = api
				machine.SetOrg(&org)
				for _, class := range []Class{
					{Name: owner, IsTest: true},
					{Name: owner + "." + name, Nested: true, Access: "public"},
				} {
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
				}
				source := fmt.Sprintf(`
Type platform = Schema.%[1]s.class;
System.assertEquals(platform, Type.forName('%[1]s'));
System.assertEquals(platform, Type.forName('Schema.%[1]s'));
System.assertNotEquals(platform, %[1]s.class);
Type nested = Type.forName('%[2]s.%[1]s');
System.assertNotEquals(null, nested);
System.assertNotEquals(platform, nested);
`, name, owner)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.ExecuteTestInClass(program, owner); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

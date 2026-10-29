package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Schema alias identity is captured at APIs 62 and 67 in
// ../apextest/testdata/conformance/schema_type_alias_boundaries/org.tsv:2-5.
// A stub callback must expose the same Type identity as a class literal.
func TestStubSchemaAliasSignatureIdentity(t *testing.T) {
	for _, api := range []string{"49.0", "62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named", "isTest", "namespacedIsTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				machine := New(nil)
				machine.EnableTestContext()
				compile := func(source string) ir.Program {
					t.Helper()
					p, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					return p
				}
				target := Class{Name: "AliasSignatureTarget", Methods: map[string]Method{
					"fetch":          {ReturnType: "List<SObject>", Params: []Param{{Name: "kind", Type: "sObjectType"}, {Name: "ids", Type: "Set<Id>"}, {Name: "fields", Type: "List<String>"}}},
					"field":          {ReturnType: "List<SObject>", Params: []Param{{Name: "token", Type: "SObjectField"}}},
					"describe":       {ReturnType: "List<SObject>", Params: []Param{{Name: "token", Type: "DescribeFieldResult"}}},
					"describeObject": {ReturnType: "List<SObject>", Params: []Param{{Name: "token", Type: "DescribeSObjectResult"}}},
				}}
				provider := Class{Name: "AliasSignatureProvider", Interfaces: []string{"StubProvider"}, Methods: map[string]Method{
					"handleMethodCall": {ReturnType: "Object", Params: []Param{{Name: "receiver", Type: "Object"}, {Name: "methodName", Type: "String"}, {Name: "returnType", Type: "Type"}, {Name: "parameterTypes", Type: "List<Type>"}, {Name: "parameterNames", Type: "List<String>"}, {Name: "arguments", Type: "List<Object>"}}, Program: compile(`
List<Type> expected;
if (methodName == 'fetch') expected = new List<Type>{SObjectType.class, Set<Id>.class, List<String>.class};
if (methodName == 'field') expected = new List<Type>{SObjectField.class};
if (methodName == 'describe') expected = new List<Type>{DescribeFieldResult.class};
if (methodName == 'describeObject') expected = new List<Type>{DescribeSObjectResult.class};
if (parameterTypes.equals(expected)) return new List<SObject>{new Account(Name='match')};
return null;
`)},
				}}
				for _, class := range []Class{{Name: "AliasSignatureTest", IsTest: route != "named"}, target, provider} {
					if route == "namespacedIsTest" {
						class.Namespace = "sample"
					}
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
				}
				p := compile(`
AliasSignatureTarget stub = (AliasSignatureTarget) Test.createStub(AliasSignatureTarget.class, new AliasSignatureProvider());
System.assertEquals('match', stub.fetch(null, null, null)[0].get('Name'));
System.assertEquals('match', stub.field(null)[0].get('Name'));
System.assertEquals('match', stub.describe(null)[0].get('Name'));
System.assertEquals('match', stub.describeObject(null)[0].get('Name'));
`)
				var err error
				if route == "isTest" || route == "namespacedIsTest" {
					caller := "AliasSignatureTest"
					if route == "namespacedIsTest" {
						caller = "sample.AliasSignatureTest"
					}
					_, err = machine.ExecuteTestInClass(p, caller)
				} else if route == "named" {
					_, err = machine.ExecuteInClass(p, "AliasSignatureTest")
				} else {
					_, err = machine.Execute(p)
				}
				if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

package vm

import (
	"github.com/glade-sh/glade/internal/ir"
	"testing"
)

// These controls retain the behavior before the compatibility repair; the
// native schema alias rows do not capture callback return or generic metadata.
func TestStubAliasScopeControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				machine := New(nil)
				machine.EnableTestContext()
				compile := func(source string) ir.Program {
					t.Helper()
					p, e := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if e != nil {
						t.Fatal(e)
					}
					return p
				}
				target := Class{Name: "ScopeTarget", Methods: map[string]Method{
					"scalar":  {ReturnType: "Object", Params: []Param{{Name: "x", Type: "sObjectType"}}},
					"generic": {ReturnType: "Object", Params: []Param{{Name: "x", Type: "List<sObjectType>"}}},
					"other":   {ReturnType: "Object", Params: []Param{{Name: "x", Type: "sObjectField"}}},
					"result":  {ReturnType: "sObjectType"},
				}}
				provider := Class{Name: "ScopeProvider", Interfaces: []string{"StubProvider"}, Methods: map[string]Method{
					"handleMethodCall": {ReturnType: "Object", Params: []Param{{Name: "receiver", Type: "Object"}, {Name: "methodName", Type: "String"}, {Name: "returnType", Type: "Type"}, {Name: "parameterTypes", Type: "List<Type>"}, {Name: "parameterNames", Type: "List<String>"}, {Name: "arguments", Type: "List<Object>"}}, Program: compile(`
if(methodName=='scalar') System.assert(parameterTypes[0].equals(Schema.SObjectType.class));
if(methodName=='generic') System.assertEquals('List<sObjectType>', parameterTypes[0].getName());
if(methodName=='other') System.assertEquals('sObjectField', parameterTypes[0].getName());
if(methodName=='result') System.assertEquals('sObjectType', returnType.getName());
if(methodName=='shadow') System.assertNotEquals(Schema.SObjectType.class, parameterTypes[0]);
if(methodName=='missing') System.assertEquals(String.class, parameterTypes[0]);
return null;
`)},
				}}
				for _, class := range []Class{{Name: "ScopeCaller", IsTest: route == "isTest"}, target, {Name: "ShadowTarget", Methods: map[string]Method{"shadow": {ReturnType: "Object", Params: []Param{{Name: "x", Type: "sObjectType"}}}}}, {Name: "ShadowTarget.sObjectType", Nested: true, Access: "public"}, provider} {
					if e := machine.RegisterClass(class); e != nil {
						t.Fatal(e)
					}
				}
				p := compile(`
ScopeTarget s=(ScopeTarget)Test.createStub(ScopeTarget.class,new ScopeProvider());
s.scalar(null); s.generic(null); s.other(null); s.result(); s.missing('dynamic');
ShadowTarget shadow=(ShadowTarget)Test.createStub(ShadowTarget.class,new ScopeProvider()); shadow.shadow(null);
`)
				var e error
				switch route {
				case "anonymous":
					_, e = machine.Execute(p)
				case "named":
					_, e = machine.ExecuteInClass(p, "ScopeCaller")
				default:
					_, e = machine.ExecuteTestInClass(p, "ScopeCaller")
				}
				if e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}

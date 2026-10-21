package vm

import "testing"

// The Stub API excludes properties. An accessor executes its body, and a
// mockable method called by that body is the provider's sole invocation.
// https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_testing_stub_api.htm
func TestExecStubExplicitGetterInterceptsOnlyVirtualHelper(t *testing.T) {
	for _, propertyResult := range []string{"null", "'unexpected'"} {
		t.Run(propertyResult, func(t *testing.T) {
			machine := New(nil)
			machine.EnableTestContext()
			for _, class := range []Class{
				{Name: "PropertySource", Fields: map[string]Field{
					"Label": {Name: "Label", Type: "String", Getter: &Method{
						Name: "PropertySource.Label.get", ClassName: "PropertySource", ReturnType: "String",
						Program: collisionProgram(t, "return readValue();"),
					}},
				}, Methods: map[string]Method{
					"readValue": {Access: "public", Modifiers: []string{"virtual"}, ReturnType: "String", Program: collisionProgram(t, "return 'real method';")},
				}},
				{Name: "BoundaryProvider", Interfaces: []string{"StubProvider"}, Fields: map[string]Field{
					"calls":          {Type: "List<String>", InitialValue: typedList("List<String>")},
					"propertyResult": {Type: "String"},
				}, Methods: map[string]Method{
					"handleMethodCall": {Access: "public", ReturnType: "Object",
						Params:  []Param{{Name: "stub", Type: "Object"}, {Name: "member", Type: "String"}, {Name: "returnType", Type: "Type"}, {Name: "types", Type: "List<Type>"}, {Name: "names", Type: "List<String>"}, {Name: "args", Type: "List<Object>"}},
						Program: collisionProgram(t, "calls.add(member); return member=='readValue' ? 'ready' : propertyResult;"),
					},
				}},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := machine.Execute(collisionProgram(t, `
BoundaryProvider provider=new BoundaryProvider();
provider.propertyResult=`+propertyResult+`;
PropertySource proxy=(PropertySource)Test.createStub(PropertySource.class,provider);
System.assertEquals('ready',proxy.Label);
System.assertEquals(new List<String>{'readValue'},provider.calls);`)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

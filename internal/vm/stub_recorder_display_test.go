package vm

import (
	"strings"
	"testing"
)

func TestStubRecorderExpressionDisplayPreservesQualifiedType(t *testing.T) {
	machine := newStubNamespaceRecorderMachine(t, false)
	provider, ok := machine.lookupClass("ns.Provider")
	if !ok {
		t.Fatal("missing provider")
	}
	provider.Fields = map[string]Field{
		"methodName":    {Name: "methodName", Type: "String", Access: "public"},
		"argumentTypes": {Name: "argumentTypes", Type: "List<Type>", Access: "public"},
		"declaredType":  {Name: "declaredType", Type: "Type", Access: "public"},
		"calls":         {Name: "calls", Type: "Integer", Access: "private", InitialValue: Int(0)},
		"cursor":        {Name: "cursor", Type: "Integer", Access: "private", InitialValue: Int(0)},
	}
	if err := machine.RegisterClass(provider); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Holder", Namespace: "ns", Access: "public", Fields: map[string]Field{"held": {Name: "held", Type: "Object", Access: "public"}}}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Provider provider=new Provider();
provider.declaredType=Foo.class;
Object mockInstance=Test.createStub(Foo.class,provider);
String expected='Foo__sfdc_ApexStub:[stubProvider=Provider:[argumentTypes=null, calls=0, cursor=0, declaredType=ns.Foo, methodName=null]]';
System.assertEquals(expected,String.valueOf(mockInstance));
System.assertEquals(expected,mockInstance.toString());
System.assertEquals(expected,''+mockInstance);
System.assertEquals(expected,String.format('{0}',new List<Object>{mockInstance}));
System.assertEquals('('+expected+')',String.valueOf(new List<Object>{mockInstance}));
System.assertEquals('{'+expected+'=sample}',String.valueOf(new Map<Object,String>{mockInstance=>'sample'}));
System.assertEquals('{sample='+expected+'}',String.valueOf(new Map<String,Object>{'sample'=>mockInstance}));
Holder holder=new Holder(); holder.held=mockInstance;
System.assertEquals('Holder:[held='+expected+']',String.valueOf(holder));
String key=String.valueOf(mockInstance).split(':').get(0)+'.add(String)';
System.assertEquals('METHOD: Foo__sfdc_ApexStub.add(String)','METHOD: '+key);
System.assertEquals('No more interactions were expected after the Foo__sfdc_ApexStub.add(String)',
 'No more interactions were expected after the '+key);
System.assertEquals('ns.Foo',provider.declaredType.getName());
// Generic debug and assertion display use the owning-org generated name.
System.debug(mockInstance);
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.ExecuteInClass(program, "ns.Recorder")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Debug) != 1 || !strings.HasPrefix(result.Debug[0], "Foo__sfdc_ApexStub:") {
		t.Fatalf("generic debug text = %v", result.Debug)
	}
	assertStubRecorderGenericAssertionText(t, machine, "Foo__sfdc_ApexStub:")
}

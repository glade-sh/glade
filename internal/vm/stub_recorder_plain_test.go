package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestStubRecorderPlainConversionsUseProviderState(t *testing.T) {
	machine := New(nil)
	org := storage.NewOrgState()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Foo", Access: "public", Modifiers: []string{"virtual"},
		Methods: map[string]Method{"act": {Name: "Foo.act", ClassName: "Foo", Access: "public", SourceContextBound: true, Modifiers: []string{"virtual"}, ReturnType: "void", Params: []Param{{Name: "value", Type: "String"}}}},
	}); err != nil {
		t.Fatal(err)
	}
	handler, err := CompileAnonymous(`
this.methodName=methodName;this.argumentTypes=paramTypes;this.calls++;
System.assertEquals(String.valueOf(stub),stub.toString());
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Provider", Access: "public", Interfaces: []string{"StubProvider"},
		Fields: map[string]Field{
			"methodName":    {Name: "methodName", Type: "String", Access: "public"},
			"argumentTypes": {Name: "argumentTypes", Type: "List<Type>", Access: "public"},
			"declaredType":  {Name: "declaredType", Type: "Type", Access: "public"},
			"calls":         {Name: "calls", Type: "Integer", Access: "private", InitialValue: Int(0)},
			"cursor":        {Name: "cursor", Type: "Integer", Access: "private", InitialValue: Int(0)},
		},
		Methods: map[string]Method{"handleMethodCall": {
			Name: "Provider.handleMethodCall", ClassName: "Provider", Access: "public", SourceContextBound: true, ReturnType: "Object", Program: handler,
			Params: []Param{{Name: "stub", Type: "Object"}, {Name: "methodName", Type: "String"}, {Name: "returnType", Type: "Type"}, {Name: "paramTypes", Type: "List<Type>"}, {Name: "paramNames", Type: "List<String>"}, {Name: "args", Type: "List<Object>"}},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	for _, class := range []Class{
		{Name: "Holder", Access: "public", Fields: map[string]Field{"held": {Name: "held", Type: "Object", Access: "public"}}},
		{Name: "Caller", Access: "public"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	program, err := CompileAnonymous(`
Provider provider=new Provider();provider.declaredType=Foo.class;
Object mockInstance=Test.createStub(Foo.class,provider);
String expected='Foo__sfdc_ApexStub:[stubProvider=Provider:[argumentTypes=null, calls=0, cursor=0, declaredType=Foo, methodName=null]]';
System.assertEquals(expected,String.valueOf(mockInstance));
System.assertEquals(expected,mockInstance.toString());
System.assertEquals(expected,''+mockInstance);
System.assertEquals(expected,String.format('{0}',new List<Object>{mockInstance}));
System.assertEquals('('+expected+')',String.valueOf(new List<Object>{mockInstance}));
System.assertEquals('{'+expected+'=sample}',String.valueOf(new Map<Object,String>{mockInstance=>'sample'}));
System.assertEquals('{sample='+expected+'}',String.valueOf(new Map<String,Object>{'sample'=>mockInstance}));
Holder holder=new Holder();holder.held=mockInstance;
System.assertEquals('Holder:[held='+expected+']',String.valueOf(holder));
((Foo)mockInstance).act('sample');
String after='Foo__sfdc_ApexStub:[stubProvider=Provider:[argumentTypes=(String), calls=1, cursor=0, declaredType=Foo, methodName=act]]';
System.assertEquals(after,String.valueOf(mockInstance));
System.assertEquals(after,mockInstance.toString());
System.assertEquals('Foo',provider.declaredType.getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "Caller"); err != nil {
		t.Fatal(err)
	}
}

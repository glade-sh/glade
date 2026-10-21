package vm

import "testing"

func TestStubRecorderGlobalHelperPreservesProviderState(t *testing.T) {
	machine := newStubNamespaceRecorderMachine(t, false)
	if err := machine.RegisterClass(Class{
		Name: "GlobalFoo", Namespace: "ns", Access: "global", Modifiers: []string{"virtual"},
		Methods: map[string]Method{"act": {Name: "GlobalFoo.act", ClassName: "GlobalFoo", Namespace: "ns", SourceContextBound: true, Access: "global", Modifiers: []string{"virtual"}, ReturnType: "void", Params: []Param{{Name: "value", Type: "String"}}}},
	}); err != nil {
		t.Fatal(err)
	}
	makeStub, err := CompileAnonymous("return Test.createStub(type,provider);")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Factory", Namespace: "ns", Access: "public",
		Methods: map[string]Method{"make": {Name: "Factory.make", ClassName: "Factory", Namespace: "ns", SourceContextBound: true, Access: "public", IsStatic: true, ReturnType: "Object", Params: []Param{{Name: "type", Type: "Type"}, {Name: "provider", Type: "Provider"}}, Program: makeStub}},
	}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Provider provider=new Provider();
GlobalFoo mockInstance=(GlobalFoo)Factory.make(GlobalFoo.class,provider);
mockInstance.act('sample');
System.assertEquals('GlobalFoo__sfdc_ApexStub',String.valueOf(mockInstance).split(':').get(0));
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.ExecuteInClass(program, "ns.Recorder")
	if err != nil {
		t.Fatal(err)
	}
	provider, ok := result.Vars["provider"]
	if !ok || len(provider.Fields) != 0 {
		t.Fatalf("undeclared provider state = %v", provider.Fields)
	}
}

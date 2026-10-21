package vm

import "testing"

func newStubIdentityHashVM(t *testing.T) *VM {
	t.Helper()
	machine := New(nil)
	machine.EnableTestContext()
	register := func(class Class) {
		t.Helper()
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	register(Class{Name: "EchoService", Methods: map[string]Method{
		"touch": {Access: "public", ReturnType: "void"},
	}})
	register(Class{Name: "CountingProvider", Interfaces: []string{"StubProvider"},
		Fields: map[string]Field{
			"calls":   {Type: "Integer", InitialValue: Int(0)},
			"members": {Type: "List<String>", InitialValue: typedList("List<String>")},
			"history": {Type: "List<String>", InitialValue: typedList("List<String>")},
		},
		Methods: map[string]Method{"handleMethodCall": {
			Access: "public", ReturnType: "Object",
			Params:  []Param{{Name: "stub", Type: "Object"}, {Name: "member", Type: "String"}, {Name: "returnType", Type: "Type"}, {Name: "types", Type: "List<Type>"}, {Name: "names", Type: "List<String>"}, {Name: "args", Type: "List<Object>"}},
			Program: collisionProgram(t, "members.add(member); history.add(member); calls++; return null;"),
		}},
	})
	register(Class{Name: "InvocationKey", Fields: map[string]Field{
		"receiver": {Type: "Object"}, "member": {Type: "String"},
	}, Constructors: []Method{{IsConstructor: true, Params: []Param{{Name: "receiver", Type: "Object"}, {Name: "member", Type: "String"}},
		Program: collisionProgram(t, "this.receiver=receiver; this.member=member;")}},
		Methods: map[string]Method{
			"hashCode": {ReturnType: "Integer", Program: collisionProgram(t, "return receiver.hashCode() ^ member.hashCode();")},
			"equals":   {ReturnType: "Boolean", Params: []Param{{Name: "other", Type: "Object"}}, Program: collisionProgram(t, "if(!(other instanceof InvocationKey)) return false; InvocationKey key=(InvocationKey)other; return receiver===key.receiver && member==key.member;")},
		},
	})
	return machine
}

func TestExecStubIdentityHashSurvivesProviderMutation(t *testing.T) {
	for _, shape := range []string{"hash", "customKey"} {
		t.Run(shape, func(t *testing.T) {
			machine := newStubIdentityHashVM(t)
			code := `CountingProvider provider=new CountingProvider();
EchoService service=(EchoService)Test.createStub(EchoService.class,provider);
Object original=service;
Integer before=service.hashCode();
InvocationKey key=new InvocationKey(service,'touch');
Map<InvocationKey,String> values=new Map<InvocationKey,String>{key=>'retained'};
service.touch();
System.assertEquals(1,provider.calls);
System.assert(original===service,'provider mutation changed stub identity');
`
			if shape == "hash" {
				code += `System.assertEquals(before,service.hashCode(),'same stub hash after callback');`
			} else {
				code += `InvocationKey lookup=new InvocationKey(service,'touch');
System.assert(key.equals(lookup),'same invocation key');
System.assertEquals('retained',values.get(lookup),'custom key after callback');`
			}
			if _, err := machine.Execute(collisionProgram(t, code)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Object.hashCode must stay consistent when equality is unchanged:
// https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/langCon_apex_collections_maps_keys_userdefined.htm
// A VM alias snapshot changes backing storage, not the Apex object's identity.
func TestExecObjectIdentityHashSurvivesAliasSnapshot(t *testing.T) {
	for _, receiver := range []string{"plain", "stub"} {
		for _, check := range []string{"hash", "customKey"} {
			t.Run(receiver+"/"+check, func(t *testing.T) {
				machine := newStubIdentityHashVM(t)
				if err := machine.RegisterClass(Class{Name: "PlainReceiver"}); err != nil {
					t.Fatal(err)
				}
				setup := "Object subject=new PlainReceiver();"
				if receiver == "stub" {
					setup = "Object subject=Test.createStub(EchoService.class,new CountingProvider());"
				}
				setup += `Object alias=subject;
Integer before=subject.hashCode();
InvocationKey key=new InvocationKey(subject,'touch');
Map<InvocationKey,String> values=new Map<InvocationKey,String>{key=>'retained'};`
				if _, err := machine.Execute(collisionProgram(t, setup)); err != nil {
					t.Fatal(err)
				}
				// Detach every stored view so another alias cannot hide a hash
				// that accidentally depends on a particular backing map.
				for name, value := range machine.Globals {
					machine.Globals[name] = cloneValuePreserveRefs(value)
				}
				code := `System.assert(subject===alias,'snapshot changed object identity');`
				if check == "hash" {
					code += `System.assertEquals(before,subject.hashCode(),'identity hash after snapshot');`
				} else {
					code += `InvocationKey lookup=new InvocationKey(subject,'touch');
System.assert(key.equals(lookup),'equal keys after snapshot');
System.assertEquals('retained',values.get(lookup),'custom key after snapshot');`
				}
				if _, err := machine.Execute(collisionProgram(t, code)); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// The provider receives each original argument. A list containing an SObject
// remains a typed list; it is not a scalar SObject argument.
func TestExecStubProviderReceivesWholeListArgument(t *testing.T) {
	machine := newStubIdentityHashVM(t)
	for _, class := range []Class{
		{Name: "ArgumentService", Methods: map[string]Method{
			"inspect": {Access: "public", ReturnType: "Boolean", Params: []Param{{Name: "value", Type: "Object"}}},
		}},
		{Name: "ArgumentProvider", Interfaces: []string{"StubProvider"}, Methods: map[string]Method{
			"handleMethodCall": {Access: "public", ReturnType: "Object",
				Params:  []Param{{Name: "stub", Type: "Object"}, {Name: "member", Type: "String"}, {Name: "returnType", Type: "Type"}, {Name: "types", Type: "List<Type>"}, {Name: "names", Type: "List<String>"}, {Name: "args", Type: "List<Object>"}},
				Program: collisionProgram(t, "return args[0] instanceof List<SObject>;"),
			},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(collisionProgram(t, `
ArgumentService service=(ArgumentService)Test.createStub(ArgumentService.class,new ArgumentProvider());
Account record=new Account(Name='Sample');
System.assertEquals(false,service.inspect(record));
System.assertEquals(true,service.inspect(new List<SObject>{record}));
System.assertEquals(true,service.inspect(new List<SObject>()));
System.assertEquals(false,service.inspect(null));`)); err != nil {
		t.Fatal(err)
	}
}

func TestExecStubProviderKeepsOverloadResultsSeparate(t *testing.T) {
	machine := newStubIdentityHashVM(t)
	if err := machine.RegisterClass(Class{Name: "OverloadFactory"}); err != nil {
		t.Fatal(err)
	}
	for _, params := range [][]Param{nil, {{Name: "types", Type: "List<Type>"}}} {
		if err := machine.RegisterMethod(Method{Name: "OverloadFactory.acquire", ClassName: "OverloadFactory", Access: "public", ReturnType: "Object", Params: params}); err != nil {
			t.Fatal(err)
		}
	}
	if err := machine.RegisterClass(Class{Name: "FactoryProvider", Interfaces: []string{"StubProvider"},
		Fields: map[string]Field{"configured": {Type: "Object"}},
		Methods: map[string]Method{"handleMethodCall": {
			Access: "public", ReturnType: "Object",
			Params:  []Param{{Name: "stub", Type: "Object"}, {Name: "member", Type: "String"}, {Name: "returnType", Type: "Type"}, {Name: "types", Type: "List<Type>"}, {Name: "names", Type: "List<String>"}, {Name: "args", Type: "List<Object>"}},
			Program: collisionProgram(t, "return args.isEmpty() ? configured : null;"),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(collisionProgram(t, `
FactoryProvider provider=new FactoryProvider();
provider.configured=new CountingProvider();
OverloadFactory factory=(OverloadFactory)Test.createStub(OverloadFactory.class,provider);
System.assert(provider.configured===factory.acquire());
System.assertEquals(null,factory.acquire(new List<Type>{Account.class}));
System.assert(provider.configured===factory.acquire());`)); err != nil {
		t.Fatal(err)
	}
}

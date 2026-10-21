package vm

import "testing"

func TestTypeForNameRejectsExternalPrivateNestedClasses(t *testing.T) {
	for _, lookup := range []struct {
		name, body string
	}{
		{"single argument", "return Type.forName(name);"},
		{"empty namespace", "return Type.forName('', name);"},
	} {
		for _, name := range []string{"ReflectionOwner.PrivateItem", "ReflectionOwnerTest.PrivateItem"} {
			t.Run(lookup.name+"/"+name, func(t *testing.T) {
				machine := privateNestedLookupMachine(t, lookup.body)
				program, err := CompileAnonymous("System.assertEquals(null, ExternalTypeLookup.resolve('" + name + "'));")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestTypeForNamePreservesExternalPublicClasses(t *testing.T) {
	for _, lookup := range []struct {
		name, body string
	}{
		{"single argument", "return Type.forName(name);"},
		{"empty namespace", "return Type.forName('', name);"},
	} {
		for _, name := range []string{"ReflectionOwner.PublicItem", "PublicReflectionItem"} {
			t.Run(lookup.name+"/"+name, func(t *testing.T) {
				machine := privateNestedLookupMachine(t, lookup.body)
				program, err := CompileAnonymous(`
Type reflected = ExternalTypeLookup.resolve('` + name + `');
System.assertNotEquals(null, reflected);
System.assertEquals('` + name + `', reflected.getName());
System.assert(reflected.newInstance() instanceof ` + name + `);
System.assert(JSON.deserialize('{}', reflected) instanceof ` + name + `);
`)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestPrivateNestedTypeLiteralRemainsUsableWithinOwner(t *testing.T) {
	machine := privateNestedReflectionMachine(t)
	verify, err := CompileAnonymous(`
ReflectionOwner.PrivateItem constructed = new ReflectionOwner.PrivateItem();
System.assert(constructed instanceof ReflectionOwner.PrivateItem);
Type literal = ReflectionOwner.PrivateItem.class;
System.assertEquals('ReflectionOwner.PrivateItem', literal.getName());
ReflectionOwner.PrivateItem decoded = (ReflectionOwner.PrivateItem)JSON.deserialize('{"label":"retained"}', literal);
System.assertEquals('retained', decoded.label);
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name: "ReflectionOwner.verify", ClassName: "ReflectionOwner",
		ReturnType: "void", IsStatic: true, Access: "public", Program: verify,
	}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous("ReflectionOwner.verify();")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func privateNestedReflectionMachine(t *testing.T) *VM {
	t.Helper()
	machine := New(nil)
	machine.EnableTestContext()
	for _, class := range []Class{
		{Name: "ReflectionBase", Access: "global"},
		{Name: "ReflectionOwner", Access: "public"},
		{Name: "ReflectionOwnerTest", Access: "private", IsTest: true},
		{
			Name: "ReflectionOwner.PrivateItem", Access: "private", SuperClass: "ReflectionBase",
			Fields: map[string]Field{"label": {Name: "label", Type: "String", Access: "public"}},
		},
		{Name: "ReflectionOwnerTest.PrivateItem", Access: "private", SuperClass: "ReflectionBase"},
		{Name: "ReflectionOwner.PublicItem", Access: "public", SuperClass: "ReflectionBase"},
		{Name: "PublicReflectionItem", Access: "public", SuperClass: "ReflectionBase"},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

func privateNestedLookupMachine(t *testing.T, body string) *VM {
	t.Helper()
	resolve, err := CompileAnonymous(body)
	if err != nil {
		t.Fatal(err)
	}
	machine := privateNestedReflectionMachine(t)
	if err := machine.RegisterClass(Class{
		Name: "ExternalTypeLookup", Access: "public",
		Methods: map[string]Method{
			"resolve": {
				Name: "ExternalTypeLookup.resolve", ClassName: "ExternalTypeLookup",
				ReturnType: "Type", IsStatic: true, Access: "public",
				Params: []Param{{Name: "name", Type: "String"}}, Program: resolve,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	return machine
}

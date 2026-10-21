package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func TestStubCallbackUsesSchemaListTypeIdentity(t *testing.T) {
	for _, api := range []string{"49.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine, compile := stubCallbackIdentityFixture(t, api)
			program := compile(`
IdentityEchoTarget target = (IdentityEchoTarget)Test.createStub(IdentityEchoTarget.class, new IdentityEchoProvider());
List<Account> records = new List<Account>{new Account(Name='Echo')};
List<Account> echoed = target.echo(records);
System.assertEquals('Echo', echoed[0].Name);
`)
			if _, err := machine.ExecuteTestInClass(program, "IdentityEchoTest"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStubCallbackPrimitiveTypeIdentityControls(t *testing.T) {
	for _, api := range []string{"49.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			machine, compile := stubCallbackIdentityFixture(t, api)
			program := compile(`
IdentityEchoTarget target = (IdentityEchoTarget)Test.createStub(IdentityEchoTarget.class, new IdentityEchoProvider());
System.assertEquals('stable', target.text('stable'));
System.assertEquals(7, target.number(7));
`)
			if _, err := machine.ExecuteTestInClass(program, "IdentityEchoTest"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func stubCallbackIdentityFixture(t *testing.T, api string) (*VM, func(string) ir.Program) {
	t.Helper()
	machine := New(nil)
	org := storage.NewOrgState()
	org.APIVersion = api
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	compile := func(source string) ir.Program {
		t.Helper()
		program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		return program
	}
	classes := []Class{
		{Name: "IdentityEchoTest", IsTest: true},
		{Name: "IdentityEchoTarget", Methods: map[string]Method{
			"echo":   {ReturnType: "List<Account>", Params: []Param{{Name: "records", Type: "List<Account>"}}, Program: compile("return records;")},
			"text":   {ReturnType: "String", Params: []Param{{Name: "value", Type: "String"}}, Program: compile("return value;")},
			"number": {ReturnType: "Integer", Params: []Param{{Name: "value", Type: "Integer"}}, Program: compile("return value;")},
		}},
		{Name: "IdentityEchoProvider", Interfaces: []string{"StubProvider"}, Methods: map[string]Method{
			"handleMethodCall": {ReturnType: "Object", Params: []Param{
				{Name: "receiver", Type: "Object"}, {Name: "methodName", Type: "String"},
				{Name: "returnType", Type: "Type"}, {Name: "parameterTypes", Type: "List<Type>"},
				{Name: "parameterNames", Type: "List<String>"}, {Name: "arguments", Type: "List<Object>"},
			}, Program: compile(`
Type expected;
if (methodName == 'echo') expected = List<Account>.class;
if (methodName == 'text') expected = String.class;
if (methodName == 'number') expected = Integer.class;
System.assert(returnType.equals(expected), 'callback return type identity');
System.assert(parameterTypes[0].equals(expected), 'callback parameter type identity');
return arguments[0];
`)},
		}},
	}
	for _, class := range classes {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	return machine, compile
}

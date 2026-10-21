package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Empty providers have captured local conversion names. The Boolean-state
// provider differs from the captured recorder fields and uses the generic
// owning-org display, which agrees on the local generated name.
func TestStubNamespaceRecorderKeyMatchesProviderShape(t *testing.T) {
	for _, tc := range []struct {
		name     string
		stateful bool
		key      string
	}{
		{name: "empty_provider", key: "Foo__sfdc_ApexStub.add(String)"},
		{name: "stateful_provider", stateful: true, key: "Foo__sfdc_ApexStub.add(String)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := newStubNamespaceRecorderMachine(t, tc.stateful)
			program, err := CompileAnonymous(`
Object mockInstance=Test.createStub(Foo.class,new Provider());
Recorder recorded=new Recorder();
recorded.typeName=String.valueOf(mockInstance).split(':').get(0);
recorded.methodName='add';
recorded.argumentTypes=new List<Type>{String.class};
System.assertEquals('` + tc.key + `',String.valueOf(recorded));
System.assertEquals('` + tc.key + `',recorded.toString());
System.assertEquals(recorded.typeName,mockInstance.toString().split(':').get(0));
System.assertEquals('ns.Foo',Foo.class.getName());
`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(program, "ns.Recorder"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func newStubNamespaceRecorderMachine(t *testing.T, stateful bool) *VM {
	t.Helper()
	machine := New(nil)
	org := storage.NewOrgState()
	org.Namespace = "ns"
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if err := machine.RegisterClass(Class{
		Name: "Foo", Namespace: "ns", Access: "public", Modifiers: []string{"virtual"},
		Methods: map[string]Method{
			"add": {Name: "Foo.add", ClassName: "Foo", Namespace: "ns", SourceContextBound: true, Access: "public", Modifiers: []string{"virtual"}, ReturnType: "void", Params: []Param{{Name: "value", Type: "String"}}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	handler, err := CompileAnonymous("return null;")
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]Field{}
	if stateful {
		fields["state"] = Field{Name: "state", Type: "Boolean", InitialValue: Bool(false)}
	}
	if err := machine.RegisterClass(Class{
		Name: "Provider", Namespace: "ns", Access: "public", Interfaces: []string{"StubProvider"}, Fields: fields,
		Methods: map[string]Method{
			"handleMethodCall": {
				Name: "Provider.handleMethodCall", ClassName: "Provider", Namespace: "ns", SourceContextBound: true, Access: "public", ReturnType: "Object",
				Params:  []Param{{Name: "stub", Type: "Object"}, {Name: "methodName", Type: "String"}, {Name: "returnType", Type: "Type"}, {Name: "paramTypes", Type: "List<Type>"}, {Name: "paramNames", Type: "List<String>"}, {Name: "args", Type: "List<Object>"}},
				Program: handler,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	recorderText, err := CompileAnonymous("return typeName + '.' + methodName + argumentTypes;")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name: "Recorder", Namespace: "ns", Access: "public",
		Fields: map[string]Field{
			"typeName":      {Name: "typeName", Type: "String", Access: "public"},
			"methodName":    {Name: "methodName", Type: "String", Access: "public"},
			"argumentTypes": {Name: "argumentTypes", Type: "List<Type>", Access: "public"},
		},
		Methods: map[string]Method{
			"toString": {Name: "Recorder.toString", ClassName: "Recorder", Namespace: "ns", SourceContextBound: true, Access: "public", ReturnType: "String", Program: recorderText},
		},
	}); err != nil {
		t.Fatal(err)
	}

	return machine
}

// This is the captured formatting expression with an empty provider. It does
// not claim coverage of a stateful provider's verification-message route.
func TestStubNamespaceVerifyMessageUsesCapturedValueOfExpression(t *testing.T) {
	machine := newStubNamespaceRecorderMachine(t, false)
	program, err := CompileAnonymous(`
Object mockInstance=Test.createStub(Foo.class,new Provider());
String key=String.valueOf(mockInstance).split(':').get(0)+'.add'+new List<Type>{String.class};
System.assertEquals('METHOD: Foo__sfdc_ApexStub.add(String)',String.format('METHOD: {0}',new List<String>{key}));
String implicitText=''+mockInstance;
System.assert(implicitText.startsWith('Foo__sfdc_ApexStub:'));
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

// The uncaptured Boolean-state provider records the generic owning-org name.
func TestStubNamespaceRecordUsesOwningOrgDisplayExpression(t *testing.T) {
	machine := newStubNamespaceRecorderMachine(t, true)
	program, err := CompileAnonymous(`
Object mockInstance=Test.createStub(Foo.class,new Provider());
Recorder recorded=new Recorder();
recorded.typeName=String.valueOf(mockInstance).split(':').get(0);
recorded.methodName='add';
recorded.argumentTypes=new List<Type>{String.class};
System.assertEquals('Foo__sfdc_ApexStub.add(String)',recorded.toString());
System.assertEquals('Foo__sfdc_ApexStub.add(String)',String.valueOf(recorded));
System.assertEquals('No more interactions were expected after the Foo__sfdc_ApexStub.add(String)',
 'No more interactions were expected after the '+recorded);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.ExecuteInClass(program, "ns.Recorder"); err != nil {
		t.Fatal(err)
	}
}

// Assertions escape the VM's Apex catch path. Inspect that existing boundary
// from Go instead of assuming a catchable assertion inside the probe program.
func assertStubRecorderGenericAssertionText(t *testing.T, machine *VM, prefix string) {
	t.Helper()
	program, err := CompileAnonymous("System.assertEquals(null,mockInstance);")
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.ExecuteInClass(program, "ns.Recorder")
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" || !strings.Contains(runtimeErr.ExceptionMessage(), "Actual: "+prefix) {
		t.Fatalf("generic object assertion = %v", err)
	}
}

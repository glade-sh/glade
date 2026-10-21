package vm

import "testing"

// Owned named source52 observations retain the concrete receiver across
// Type.forName, newInstance and an interface cast. The static and instance
// implementations have identical interface signatures and no instance fields.
func TestStaticInterfaceReflectionNative52(t *testing.T) {
	for _, selected := range []struct{ name, other, direct string }{
		{"OwnedStaticAction", "OwnedInstanceAction", "OwnedStaticAction.run();"},
		{"OwnedInstanceAction", "OwnedStaticAction", "new OwnedInstanceAction().run();"},
	} {
		t.Run(selected.name, func(t *testing.T) {
			machine := staticInterfaceNative52Machine(t)
			source := selected.direct + `
System.assertEquals(1, ` + selected.name + `.calls);
System.assertEquals(0, ` + selected.other + `.calls);
Type token = Type.forName('` + selected.name + `');
System.assertNotEquals(null, token);
Object value = token.newInstance();
System.assertNotEquals(null, value);
OwnedAction action = (OwnedAction)value;
System.assertNotEquals(null, action);
action.run();
System.assertEquals(2, ` + selected.name + `.calls);
System.assertEquals(0, ` + selected.other + `.calls);
`
			runStaticInterfaceNative52(t, machine, source)
		})
	}
}

func TestStaticInterfaceDirectNative52Controls(t *testing.T) {
	runStaticInterfaceNative52(t, staticInterfaceNative52Machine(t), `
OwnedStaticAction.run();
System.assertEquals(1, OwnedStaticAction.calls);
System.assertEquals(0, OwnedInstanceAction.calls);
new OwnedInstanceAction().run();
System.assertEquals(1, OwnedStaticAction.calls);
System.assertEquals(1, OwnedInstanceAction.calls);
`)
}

func TestStaticInterfaceReceiverIdentityNative52(t *testing.T) {
	for _, selected := range []string{"OwnedStaticAction", "OwnedOtherStaticAction", "OwnedInstanceAction"} {
		t.Run(selected, func(t *testing.T) {
			machine := staticInterfaceNative52Machine(t)
			addStaticInterfaceNative52Class(t, machine, "OwnedOtherStaticAction", true)
			source := `Type token=Type.forName('` + selected + `');
Object value=token.newInstance();
OwnedAction action=(OwnedAction)value;
action.run();
`
			for _, owner := range []string{"OwnedStaticAction", "OwnedOtherStaticAction", "OwnedInstanceAction"} {
				expected := "0"
				if owner == selected {
					expected = "1"
				}
				source += "System.assertEquals(" + expected + ", " + owner + ".calls);\n"
			}
			runStaticInterfaceNative52(t, machine, source)
		})
	}
}

func TestStaticInterfaceCastAndNullNative52Controls(t *testing.T) {
	machine := staticInterfaceNative52Machine(t)
	addStaticInterfaceNative52Class(t, machine, "OwnedOtherStaticAction", true)
	addStaticInterfaceNative52Class(t, machine, "OwnedUnrelatedAction", false)
	runStaticInterfaceNative52(t, machine, `
OwnedOtherStaticAction.run();
System.assertEquals(1, OwnedOtherStaticAction.calls);
System.assertEquals(0, OwnedStaticAction.calls);
try {
    Object value=Type.forName('OwnedUnrelatedAction').newInstance();
    OwnedAction action=(OwnedAction)value;
    System.assert(false, 'unrelated cast returned');
} catch(TypeException issue) {
    System.assertEquals('Invalid conversion from runtime type OwnedUnrelatedAction to OwnedAction', issue.getMessage());
}
OwnedAction missing=(OwnedAction)null;
System.assertEquals(null, missing);
try {
    missing.run();
    System.assert(false, 'null interface call returned');
} catch(NullPointerException issue) {
    System.assertEquals('Attempt to de-reference a null object', issue.getMessage());
}
System.assertEquals(0, OwnedStaticAction.calls);
System.assertEquals(1, OwnedOtherStaticAction.calls);
System.assertEquals(0, OwnedInstanceAction.calls);
System.assertEquals(0, OwnedUnrelatedAction.calls);
`)
}

func addStaticInterfaceNative52Class(t *testing.T, machine *VM, name string, implements bool) {
	t.Helper()
	class := Class{Name: name, APIVersion: "52.0", Access: "public", StaticFields: map[string]Field{
		"calls": {Name: "calls", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)},
	}}
	if implements {
		class.Interfaces = []string{"OwnedAction"}
	}
	if err := machine.RegisterClass(class); err != nil {
		t.Fatal(err)
	}
	body, err := CompileAnonymousWithOptions("calls++;", CompileOptions{APIVersion: "52.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name: name + ".run", ClassName: name, APIVersion: "52.0", Access: "public", ReturnType: "void", IsStatic: true, Program: body,
	}); err != nil {
		t.Fatal(err)
	}
}

func staticInterfaceNative52Machine(t *testing.T) *VM {
	t.Helper()
	machine := New(nil)
	for _, class := range []Class{
		{Name: "OwnedAction", APIVersion: "52.0", IsInterface: true, Access: "public"},
		{Name: "OwnedStaticAction", APIVersion: "52.0", Access: "public", Interfaces: []string{"OwnedAction"}, StaticFields: map[string]Field{
			"calls": {Name: "calls", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)},
		}},
		{Name: "OwnedInstanceAction", APIVersion: "52.0", Access: "public", Interfaces: []string{"OwnedAction"}, StaticFields: map[string]Field{
			"calls": {Name: "calls", Type: "Integer", Static: true, Access: "public", Value: Int(0), InitialValue: Int(0)},
		}},
		{Name: "OwnedDispatchCheck", APIVersion: "52.0", IsTest: true},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	if err := machine.RegisterMethod(Method{
		Name: "OwnedAction.run", ClassName: "OwnedAction", APIVersion: "52.0", Access: "public", ReturnType: "void", Modifiers: []string{"abstract"},
	}); err != nil {
		t.Fatal(err)
	}
	for _, implementation := range []struct {
		name   string
		static bool
	}{
		{"OwnedStaticAction", true}, {"OwnedInstanceAction", false},
	} {
		body, err := CompileAnonymousWithOptions("calls++;", CompileOptions{APIVersion: "52.0"})
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{
			Name: implementation.name + ".run", ClassName: implementation.name, APIVersion: "52.0", Access: "public", ReturnType: "void", IsStatic: implementation.static, Program: body,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

func runStaticInterfaceNative52(t *testing.T, machine *VM, source string) {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "52.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	if _, err := machine.ExecuteInClass(program, "OwnedDispatchCheck"); err != nil {
		t.Fatal(err)
	}
}

// These controls preserve the local resolution boundary for shapes not
// established by the owned named zero-argument void observations.
func TestStaticInterfaceImplementationScopeControls(t *testing.T) {
	for _, cell := range []struct {
		name   string
		modify func(*VM, *Value, *Method, *Method)
	}{
		{"instanceMethod", func(_ *VM, _ *Value, _ *Method, candidate *Method) { candidate.IsStatic = false }},
		{"privateMethod", func(_ *VM, _ *Value, _ *Method, candidate *Method) { candidate.Access = "private" }},
		{"protectedMethod", func(_ *VM, _ *Value, _ *Method, candidate *Method) { candidate.Access = "protected" }},
		{"globalMethod", func(_ *VM, _ *Value, _ *Method, candidate *Method) { candidate.Access = "global" }},
		{"constructor", func(_ *VM, _ *Value, _ *Method, candidate *Method) { candidate.IsConstructor = true }},
		{"abstractMethod", func(_ *VM, _ *Value, _ *Method, candidate *Method) { candidate.Modifiers = []string{"abstract"} }},
		{"futureMethod", func(_ *VM, _ *Value, _ *Method, candidate *Method) { candidate.Modifiers = []string{"future"} }},
		{"nonAbstractSurface", func(_ *VM, _ *Value, surface *Method, _ *Method) { surface.Modifiers = nil }},
		{"parameter", func(_ *VM, _ *Value, surface *Method, candidate *Method) {
			surface.Params = []Param{{Name: "value", Type: "String"}}
			candidate.Params = []Param{{Name: "value", Type: "String"}}
		}},
		{"nonVoid", func(_ *VM, _ *Value, surface *Method, candidate *Method) {
			surface.ReturnType, candidate.ReturnType = "String", "String"
		}},
		{"abstractBase", func(machine *VM, _ *Value, _ *Method, _ *Method) {
			class := machine.Classes["OwnedAction"]
			class.IsInterface, class.IsAbstract = false, true
			machine.Classes["OwnedAction"] = class
		}},
		{"platformInterface", func(machine *VM, _ *Value, _ *Method, _ *Method) {
			class := machine.Classes["OwnedAction"]
			class.PlatformType = true
			machine.Classes["OwnedAction"] = class
		}},
		{"inheritedStatic", func(_ *VM, _ *Value, _ *Method, candidate *Method) { candidate.ClassName = "OwnedStaticBase" }},
		{"wrongRuntimeClass", func(_ *VM, receiver *Value, _ *Method, _ *Method) { *receiver = Object("OwnedInstanceAction") }},
		{"unrelatedRuntimeClass", func(_ *VM, receiver *Value, _ *Method, candidate *Method) {
			*receiver = Object("OwnedDispatchCheck")
			candidate.ClassName = "OwnedDispatchCheck"
		}},
		{"source51", func(machine *VM, _ *Value, _ *Method, _ *Method) { machine.currentMethod.APIVersion = "51.0" }},
		{"source68", func(machine *VM, _ *Value, _ *Method, _ *Method) { machine.currentMethod.APIVersion = "68.0" }},
		{"unknownSource", func(machine *VM, _ *Value, _ *Method, _ *Method) { machine.currentMethod.APIVersion = "" }},
		{"unknownCandidateVersion", func(_ *VM, _ *Value, _ *Method, candidate *Method) { candidate.APIVersion = "" }},
		{"unknownSurfaceVersion", func(_ *VM, _ *Value, surface *Method, _ *Method) { surface.APIVersion = "" }},
		{"unknownContractVersion", func(machine *VM, _ *Value, _ *Method, _ *Method) {
			class := machine.Classes["OwnedAction"]
			class.APIVersion = ""
			machine.Classes["OwnedAction"] = class
		}},
		{"unknownImplementationVersion", func(machine *VM, _ *Value, _ *Method, _ *Method) {
			class := machine.Classes["OwnedStaticAction"]
			class.APIVersion = ""
			machine.Classes["OwnedStaticAction"] = class
		}},
		{"mixedCandidateVersion", func(_ *VM, _ *Value, _ *Method, candidate *Method) { candidate.APIVersion = "62.0" }},
		{"mixedSurfaceVersion", func(_ *VM, _ *Value, surface *Method, _ *Method) { surface.APIVersion = "62.0" }},
		{"callerNamespace", func(machine *VM, _ *Value, _ *Method, _ *Method) {
			machine.currentMethod.SourceContextBound = true
			machine.currentMethod.Namespace = "owned"
		}},
		{"orgNamespace", func(machine *VM, _ *Value, _ *Method, _ *Method) {
			org := testDataOrg()
			org.Namespace = "owned"
			machine.SetOrg(&org)
		}},
		{"contractNamespace", func(machine *VM, _ *Value, _ *Method, _ *Method) {
			class := machine.Classes["OwnedAction"]
			class.Namespace = "owned"
			machine.Classes["OwnedAction"] = class
		}},
		{"implementationNamespace", func(machine *VM, _ *Value, _ *Method, _ *Method) {
			class := machine.Classes["OwnedStaticAction"]
			class.Namespace = "owned"
			machine.Classes["OwnedStaticAction"] = class
		}},
		{"candidateNamespace", func(_ *VM, _ *Value, _ *Method, candidate *Method) { candidate.Namespace = "owned" }},
		{"surfaceNamespace", func(_ *VM, _ *Value, surface *Method, _ *Method) { surface.Namespace = "owned" }},
	} {
		t.Run(cell.name, func(t *testing.T) {
			machine := staticInterfaceNative52Machine(t)
			machine.currentMethod.APIVersion = "52.0"
			receiver := Object("OwnedStaticAction")
			surface, _ := machine.registeredMethod("OwnedAction.run")
			candidate, _ := machine.registeredMethod("OwnedStaticAction.run")
			if !machine.observedStaticInterfaceImplementation(receiver, surface, candidate) {
				t.Fatal("observed static implementation excluded before control mutation")
			}
			cell.modify(machine, &receiver, &surface, &candidate)
			if machine.observedStaticInterfaceImplementation(receiver, surface, candidate) {
				t.Fatal("unobserved implementation crossed static interface boundary")
			}
		})
	}
}

func TestStaticInterfaceDeclaredConcreteKeepsResolution(t *testing.T) {
	machine := staticInterfaceNative52Machine(t)
	machine.currentMethod.APIVersion = "52.0"
	receiver := Object("OwnedStaticAction")
	receiver.Static = "OwnedStaticAction"
	target, found, ambiguous := machine.resolveValueInstanceMethodForArgs("", receiver, "run", nil)
	if ambiguous || !found || target.IsStatic || !methodHasModifier(target.Modifiers, "abstract") {
		t.Fatalf("declared concrete receiver changed resolution: method=%+v found=%v ambiguous=%v", target, found, ambiguous)
	}
}

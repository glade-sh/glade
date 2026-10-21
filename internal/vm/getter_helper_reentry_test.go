package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Owned vm-boundaries and accessor-constructor API64 named-test observations
// distinguish helper reentry from direct lexical property storage. The helper
// reads the getter twice and mutates its separately stored Set.
func TestGetterHelperReentryInheritedSet(t *testing.T) {
	testGetterHelperReentrySet(t, "InheritedGetterReentryProbe.Base", "InheritedGetterReentryProbe", "expand")
}

func TestGetterHelperReentryPlainSet(t *testing.T) {
	testGetterHelperReentrySet(t, "GetterReentryProbe", "GetterReentryProbe", "addThroughProperty")
}

func testGetterHelperReentrySet(t *testing.T, owner, instance, helper string) {
	t.Helper()
	machine := New(nil)
	getter := getterHelperProgram(t, "reads++; resolve(); return storage;")
	setter := getterHelperProgram(t, "storage = value;")
	resolve := getterHelperProgram(t, "if (pending) { pending = false; "+helper+"(); }")
	expand := getterHelperProgram(t, "values.add('expanded');")
	if err := machine.RegisterClass(Class{
		Name:       owner,
		APIVersion: "64.0",
		Fields: map[string]Field{
			"storage": {Name: "storage", Type: "Set<String>", Access: "public"},
			"pending": {Name: "pending", Type: "Boolean", Access: "public"},
			"reads":   {Name: "reads", Type: "Integer", Access: "public"},
			"values": {
				Name: "values", Type: "Set<String>", Access: "public", Property: true,
				Getter: &Method{Name: owner + ".values.get", ClassName: owner, ReturnType: "Set<String>", APIVersion: "64.0", Program: getter},
				Setter: &Method{Name: owner + ".values.set", ClassName: owner, APIVersion: "64.0", Params: []Param{{Name: "value", Type: "Set<String>"}}, Program: setter},
			},
		},
		Methods: map[string]Method{
			"resolve": {Name: owner + ".resolve", ClassName: owner, Access: "private", APIVersion: "64.0", Program: resolve},
			helper:    {Name: owner + "." + helper, ClassName: owner, Access: "private", APIVersion: "64.0", Program: expand},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if instance != owner {
		if err := machine.RegisterClass(Class{Name: instance, SuperClass: owner, APIVersion: "64.0"}); err != nil {
			t.Fatal(err)
		}
	}
	program := getterHelperProgram(t, fmt.Sprintf(`
%s p = new %s();
p.storage = new Set<String>{'seed'};
p.pending = true;
p.reads = 0;
Set<String> result = p.values;
System.assertEquals(2, p.reads);
System.assertEquals(2, result.size());
System.assert(result === p.storage);
`, instance, instance))
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestGetterHelperReentryDirectSelfStorageControl(t *testing.T) {
	machine := New(nil)
	const owner = "GetterReentryProbe"
	getter := getterHelperProgram(t, "return direct;")
	setter := getterHelperProgram(t, "direct = value;")
	if err := machine.RegisterClass(Class{
		Name:       owner,
		APIVersion: "64.0",
		Fields: map[string]Field{
			"direct": {
				Name: "direct", Type: "Set<String>", Access: "public", Property: true,
				Getter: &Method{Name: owner + ".direct.get", ClassName: owner, ReturnType: "Set<String>", APIVersion: "64.0", Program: getter},
				Setter: &Method{Name: owner + ".direct.set", ClassName: owner, APIVersion: "64.0", Params: []Param{{Name: "value", Type: "Set<String>"}}, Program: setter},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	program := getterHelperProgram(t, `
GetterReentryProbe p = new GetterReentryProbe();
Set<String> seed = new Set<String>{'direct'};
p.direct = seed;
Set<String> result = p.direct;
System.assertEquals(1, result.size());
System.assert(result === seed);
`)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func getterHelperProgram(t *testing.T, source string) ir.Program {
	t.Helper()
	program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "64.0"})
	if err != nil {
		t.Fatal(err)
	}
	return program
}

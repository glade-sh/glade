package vm

import (
	"strings"
	"testing"
)

// Named helper/caller API63 observations are owned in the primitive reflection
// fixture. Literal and forName tokens report different constructor diagnostics.
func TestIntegerTypeNewInstanceNative63(t *testing.T) {
	for _, cell := range []struct {
		name, expression, message string
	}{
		{"literal", "Integer.class", "Integer does not have a no-arg constructor"},
		{"lookup", "Type.forName('Integer')", "Integer cannot be constructed"},
	} {
		t.Run(cell.name, func(t *testing.T) {
			machine := primitiveConstructorNativeVM63(t)
			program, err := CompileAnonymousWithOptions(`
System.assertEquals('System.TypeException|`+cell.message+`', OwnedPrimitiveReflection.observe(`+cell.expression+`));
`, CompileOptions{APIVersion: "63.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Interface and abstract constructor messages remain native-known carries.
// These unchanged controls assert their observed catchable exception type only.
func TestPrimitiveTypeNewInstanceNativeControls63(t *testing.T) {
	machine := primitiveConstructorNativeVM63(t)
	program, err := CompileAnonymousWithOptions(`
System.assertEquals('', (String)String.class.newInstance());
System.assertEquals(0, ((List<String>)List<String>.class.newInstance()).size());
Object emptyString = '';
System.assert(emptyString instanceof String);
System.assert(OwnedPrimitiveReflection.ConcreteValue.class.newInstance() instanceof OwnedPrimitiveReflection.ConcreteValue);
System.assert(OwnedPrimitiveReflection.observe(OwnedPrimitiveReflection.Contract.class).startsWith('System.TypeException|'));
System.assert(OwnedPrimitiveReflection.observe(OwnedPrimitiveReflection.AbstractValue.class).startsWith('System.TypeException|'));
`, CompileOptions{APIVersion: "63.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func primitiveConstructorNativeVM63(t *testing.T) *VM {
	t.Helper()
	return primitiveConstructorNativeVM(t, "63.0")
}

func primitiveConstructorNativeVM(t *testing.T, api string) *VM {
	t.Helper()
	machine := New(nil)
	for _, class := range []Class{
		{Name: "OwnedPrimitiveReflection", APIVersion: api},
		{Name: "OwnedPrimitiveReflection.Contract", APIVersion: api, IsInterface: true},
		{Name: "OwnedPrimitiveReflection.AbstractValue", APIVersion: api, IsAbstract: true},
		{Name: "OwnedPrimitiveReflection.ConcreteValue", APIVersion: api},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	body, err := CompileAnonymousWithOptions(`
try {
    target.newInstance();
    return 'returned';
} catch (Exception problem) {
    return problem.getTypeName()+'|'+problem.getMessage();
}
`, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name: "OwnedPrimitiveReflection.observe", ClassName: "OwnedPrimitiveReflection",
		APIVersion: api, Access: "public", IsStatic: true, ReturnType: "String",
		Params: []Param{{Name: "target", Type: "Type"}}, Program: body,
	}); err != nil {
		t.Fatal(err)
	}
	return machine
}

// This local trust control preserves the source-class exclusion; it makes no
// native claim about classes with primitive spellings.
func TestIntegerTypeNewInstanceKeepsSourceClassShadow(t *testing.T) {
	machine := primitiveConstructorNativeVM63(t)
	if err := machine.RegisterClass(Class{Name: "Integer", APIVersion: "63.0"}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymousWithOptions("OwnedPrimitiveReflection.observe(Integer.class);", CompileOptions{APIVersion: "63.0"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "Type.newInstance uninstantiable built-in Integer") {
		t.Fatalf("source-class exclusion changed: %v", err)
	}
}

// Unobserved versions retain the existing unsupported boundary. These local
// scope controls do not assert native constructor behavior at those versions.
func TestIntegerTypeNewInstanceKeepsUnobservedSourceVersions(t *testing.T) {
	for _, api := range []string{"61.0", "68.0"} {
		for _, expression := range []string{"Integer.class", "Type.forName('Integer')"} {
			t.Run(api+"/"+expression, func(t *testing.T) {
				machine := primitiveConstructorNativeVM(t, api)
				program, err := CompileAnonymousWithOptions("OwnedPrimitiveReflection.observe("+expression+");", CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.Execute(program)
				if err == nil || !strings.Contains(err.Error(), "Type.newInstance uninstantiable built-in Integer") {
					t.Fatalf("unobserved source boundary changed: %v", err)
				}
			})
		}
	}
}

package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// The API64 named-test observations in native-capture cover this source
// layout: a virtual product extending its own inner base, an explicit protected
// no-argument constructor, and reflection from an unrelated nested factory.
func TestTypeNewInstanceProtectedConstructorExternalFactory(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"rejectProtectedZeroArgument", `
Boolean caught = false;
try {
  new ReflectionLayoutFactory.Reader().build('InheritedReflectProduct');
} catch (TypeException issue) {
  caught = true;
  System.assertEquals('System.TypeException', issue.getTypeName());
  System.assertEquals('InheritedReflectProduct does not have a no-arg constructor', issue.getMessage());
}

System.assert(caught, 'External reflection must reject the protected constructor');`},
		{"retainPublicArgument", `
InheritedReflectProduct product = new InheritedReflectProduct('ordinary');
System.assertEquals('ordinary', product.marker);`},
	} {
		t.Run(test.name, func(t *testing.T) {
			compile := func(source string) ir.Program {
				t.Helper()
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "64.0"})
				if err != nil {
					t.Fatal(err)
				}
				return program
			}
			machine := New(nil)
			machine.EnableTestContext()
			classes := []Class{
				{Name: "InheritedReflectProduct.Base", APIVersion: "64.0", Access: "global", IsAbstract: true,
					Constructors: []Method{{Name: "InheritedReflectProduct.Base.<init>", ClassName: "InheritedReflectProduct.Base", IsConstructor: true, Access: "protected", APIVersion: "64.0", Program: compile("")}}},
				{Name: "InheritedReflectProduct", SuperClass: "InheritedReflectProduct.Base", APIVersion: "64.0", Access: "global", Modifiers: []string{"virtual"},
					Fields: map[string]Field{"marker": {Name: "marker", Type: "String"}},
					Constructors: []Method{
						{Name: "InheritedReflectProduct.<init>", ClassName: "InheritedReflectProduct", IsConstructor: true, Access: "protected", APIVersion: "64.0", Program: compile("super(); marker = 'zero';")},
						{Name: "InheritedReflectProduct.<init>", ClassName: "InheritedReflectProduct", IsConstructor: true, Access: "public", APIVersion: "64.0", Params: []Param{{Name: "value", Type: "String"}}, Program: compile("super(); marker = value;")},
						{Name: "InheritedReflectProduct.<init>", ClassName: "InheritedReflectProduct", IsConstructor: true, Access: "public", APIVersion: "64.0", Params: []Param{{Name: "factory", Type: "ReflectionLayoutFactory"}}, Program: compile("this();")},
					}},
				{Name: "ReflectionLayoutFactory", APIVersion: "64.0", Access: "public"},
				{Name: "ReflectionLayoutFactory.Reader", APIVersion: "64.0", Access: "public",
					Methods: map[string]Method{"build": {Name: "ReflectionLayoutFactory.Reader.build", ClassName: "ReflectionLayoutFactory.Reader", ReturnType: "Object", Access: "public", APIVersion: "64.0", Params: []Param{{Name: "name", Type: "String"}}, Program: compile("return Type.forName(name)?.newInstance();")}}},
			}
			for _, class := range classes {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := machine.Execute(compile(test.source)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The API64 named-test rows in native-capture distinguish
// reflective constructor admission from ordinary protected member access.
func TestTypeNewInstanceProtectedConstructorCallerIndependent(t *testing.T) {
	for _, test := range []struct{ name, call string }{
		{"external", "new OwnedReflectionReader64.Reader().build()"},
		{"owner", "OwnedProtectedCtor64.ownerBuild()"},
		{"subclass", "OwnedProtectedSubclass64.subclassBuild()"},
	} {
		t.Run(test.name, func(t *testing.T) {
			compile := func(source string) ir.Program {
				t.Helper()
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "64.0"})
				if err != nil {
					t.Fatal(err)
				}
				return program
			}
			machine := New(nil)
			machine.EnableTestContext()
			reflectBody := compile("return Type.forName('OwnedProtectedCtor64').newInstance();")
			for _, class := range []Class{
				{Name: "OwnedProtectedCtor64", APIVersion: "64.0", Access: "global", Modifiers: []string{"virtual"},
					Fields: map[string]Field{"marker": {Name: "marker", Type: "String"}},
					Constructors: []Method{
						{Name: "OwnedProtectedCtor64.<init>", ClassName: "OwnedProtectedCtor64", IsConstructor: true, Access: "protected", APIVersion: "64.0", Program: compile("marker = 'zero';")},
						{Name: "OwnedProtectedCtor64.<init>", ClassName: "OwnedProtectedCtor64", IsConstructor: true, Access: "public", APIVersion: "64.0", Params: []Param{{Name: "text", Type: "String"}}, Program: compile("marker = text;")},
					},
					Methods: map[string]Method{"ownerBuild": {Name: "OwnedProtectedCtor64.ownerBuild", ClassName: "OwnedProtectedCtor64", IsStatic: true, ReturnType: "Object", Access: "public", APIVersion: "64.0", Program: reflectBody}}},
				{Name: "OwnedProtectedSubclass64", SuperClass: "OwnedProtectedCtor64", APIVersion: "64.0", Access: "public",
					Methods: map[string]Method{"subclassBuild": {Name: "OwnedProtectedSubclass64.subclassBuild", ClassName: "OwnedProtectedSubclass64", IsStatic: true, ReturnType: "Object", Access: "public", APIVersion: "64.0", Program: reflectBody}}},
				{Name: "OwnedReflectionReader64", APIVersion: "64.0", Access: "public"},
				{Name: "OwnedReflectionReader64.Reader", APIVersion: "64.0", Access: "public",
					Methods: map[string]Method{"build": {Name: "OwnedReflectionReader64.Reader.build", ClassName: "OwnedReflectionReader64.Reader", ReturnType: "Object", Access: "public", APIVersion: "64.0", Program: reflectBody}}},
			} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			program := compile(fmt.Sprintf(`
Boolean caught = false;
try { %s; }
catch (TypeException issue) {
  caught = true;
  System.assertEquals('System.TypeException', issue.getTypeName());
  System.assertEquals('OwnedProtectedCtor64 does not have a no-arg constructor', issue.getMessage());
}
System.assert(caught, 'Reflection must reject the protected constructor regardless of caller');`, test.call))
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

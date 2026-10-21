package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

func collisionProgram(t *testing.T, source string) ir.Program {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func collisionClass(t *testing.T, machine *VM, class Class) {
	t.Helper()
	if err := machine.RegisterClass(class); err != nil {
		t.Fatal(err)
	}
}

func TestImplicitNamespaceCollisionLocalPrivateMember(t *testing.T) {
	machine := New(nil)
	collisionClass(t, machine, Class{Name: "CartLike", Methods: map[string]Method{
		"a": {ReturnType: "Id", Access: "public", Program: collisionProgram(t, "return b();")},
		"b": {ReturnType: "Id", Access: "private", Program: collisionProgram(t, "return null;")},
	}})
	// A separate source owner references the absent package and its nested type.
	collisionClass(t, machine, Class{Name: "PackageReference", Fields: map[string]Field{
		"options": {Type: "missingns.CartLike.SubmitOptions", Access: "public"},
	}, Methods: map[string]Method{
		"save": {IsStatic: true, Program: collisionProgram(t, "missingns.CartLike.Instance.save();")},
	}})
	if _, err := machine.Execute(collisionProgram(t, "System.assertEquals(null, new CartLike().a());")); err != nil {
		t.Fatal(err)
	}
}

func TestImplicitNamespaceCollisionLexicalMemberBeforeFallback(t *testing.T) {
	for _, shape := range []string{"ambientNamespace", "passiveReceiver", "inherited", "outer"} {
		t.Run(shape, func(t *testing.T) {
			machine := New(nil)
			access := "private"
			if shape == "inherited" {
				access = "protected"
			}
			collisionClass(t, machine, Class{Name: "CartLike", Methods: map[string]Method{
				"b": {ReturnType: "String", Access: access, IsStatic: shape == "outer", Program: collisionProgram(t, "return 'local';")},
			}})
			machine.currentClass = "CartLike"
			machine.currentMethod = Method{ClassName: "CartLike"}
			machine.currentNamespace = "missingns"
			machine.Globals["this"] = Object("CartLike")
			// A competing qualified static candidate must not win over the
			// enclosing, inherited or outer declaration. A passive receiver
			// tests the enclosing identity independently of this.Type.
			collisionClass(t, machine, Class{Name: "CartLike", Namespace: "missingns", Dependency: true, Methods: map[string]Method{
				"b": {IsStatic: true, ReturnType: "String", Access: "public", Program: collisionProgram(t, "return 'package';")},
			}})
			switch shape {
			case "passiveReceiver":
				machine.Globals["this"] = Object("absentns.CartLike")
			case "inherited":
				collisionClass(t, machine, Class{Name: "ChildCart", SuperClass: "CartLike"})
				machine.currentClass = "ChildCart"
				machine.currentMethod.ClassName = "ChildCart"
				machine.Globals["this"] = Object("ChildCart")
				collisionClass(t, machine, Class{Name: "ChildCart", Namespace: "missingns", Dependency: true, Methods: map[string]Method{
					"b": {IsStatic: true, ReturnType: "String", Access: "public", Program: collisionProgram(t, "return 'package';")},
				}})
			case "outer":
				collisionClass(t, machine, Class{Name: "CartLike.Inner"})
				collisionClass(t, machine, Class{Name: "CartLike.Inner", Namespace: "missingns", Dependency: true, Methods: map[string]Method{
					"b": {IsStatic: true, ReturnType: "String", Access: "public", Program: collisionProgram(t, "return 'package';")},
				}})
				machine.currentClass = "CartLike.Inner"
				machine.currentMethod.ClassName = "CartLike.Inner"
				machine.Globals["this"] = Object("CartLike.Inner")
			}
			value, err := machine.call("b", nil, nil, &Result{})
			if err != nil || value.Kind != ValueString || value.Text != "local" {
				t.Fatalf("implicit b() = %#v, %v; want local", value, err)
			}
		})
	}
}

func TestImplicitNamespaceCollisionQualifiedAndMissingOnly(t *testing.T) {
	for _, local := range []bool{false, true} {
		name := "missingOnly"
		if local {
			name = "localAndMissing"
		}
		t.Run(name, func(t *testing.T) {
			machine := New(nil)
			if local {
				collisionClass(t, machine, Class{Name: "CartLike", Methods: map[string]Method{
					"enter": {IsStatic: true, ReturnType: "String", Access: "public", Program: collisionProgram(t, "return 'local';")},
					"save":  {IsStatic: true, ReturnType: "String", Access: "public", Program: collisionProgram(t, "return 'local';")},
				}})
				machine.currentClass = "CartLike"
			}
			value, err := machine.call("missingns.CartLike.Instance.enter", nil, nil, &Result{})
			if err != nil || value.Kind != ValueNull {
				t.Fatalf("qualified passive enter = %#v, %v; want null", value, err)
			}
			value, err = machine.call("new:missingns.CartLike.SubmitOptions", nil, nil, &Result{})
			if err != nil || value.Kind != ValueObject || value.Type != "missingns.CartLike.SubmitOptions" {
				t.Fatalf("qualified options = %#v, %v", value, err)
			}
			// save has no passive implementation today. Preserve that rejection
			// rather than redirect a qualified call to the local source method.
			_, err = machine.call("missingns.CartLike.Instance.save", nil, nil, &Result{})
			runtimeErr, ok := err.(*RuntimeError)
			if !ok || runtimeErr.Type != "UnsupportedFeature" || runtimeErr.Message != `unsupported call "missingns.CartLike.Instance.save"` {
				t.Fatalf("qualified save = %v; want existing unsupported boundary", err)
			}
		})
	}
}

func TestImplicitNamespaceCollisionPreservesOverrideAndPrivateBinding(t *testing.T) {
	for _, private := range []bool{false, true} {
		name, access, want := "override", "protected", "derived"
		if private {
			name, access, want = "private", "private", "base"
		}
		t.Run(name, func(t *testing.T) {
			machine := New(nil)
			var baseModifiers, derivedModifiers []string
			if !private {
				baseModifiers = []string{"virtual"}
				derivedModifiers = []string{"override"}
			}
			collisionClass(t, machine, Class{Name: "CartLike", Methods: map[string]Method{
				"a": {ReturnType: "String", Access: "public", Program: collisionProgram(t, "return b();")},
				"b": {ReturnType: "String", Access: access, Modifiers: baseModifiers, Program: collisionProgram(t, "return 'base';")},
			}})
			collisionClass(t, machine, Class{Name: "DerivedCart", SuperClass: "CartLike", Methods: map[string]Method{
				"b": {ReturnType: "String", Access: "public", Modifiers: derivedModifiers, Program: collisionProgram(t, "return 'derived';")},
			}})
			if _, err := machine.Execute(collisionProgram(t, "System.assertEquals('"+want+"', new DerivedCart().a());")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestImplicitNamespaceCollisionPreservesSelectedSignature(t *testing.T) {
	machine := New(nil)
	collisionClass(t, machine, Class{Name: "CartLike", Methods: map[string]Method{
		"a": {ReturnType: "String", Access: "public", Program: collisionProgram(t, "return b('value');")},
		"b": {ReturnType: "String", Access: "protected", Params: []Param{{Name: "value", Type: "Object"}}, Program: collisionProgram(t, "return 'base';")},
	}})
	collisionClass(t, machine, Class{Name: "DerivedCart", SuperClass: "CartLike", Methods: map[string]Method{
		"b": {ReturnType: "String", Access: "public", Params: []Param{{Name: "value", Type: "String"}}, Program: collisionProgram(t, "return 'overload';")},
	}})
	if _, err := machine.Execute(collisionProgram(t, "System.assertEquals('base', new DerivedCart().a());")); err != nil {
		t.Fatal(err)
	}
}

func TestImplicitNamespaceCollisionKeepsStubProviderInterception(t *testing.T) {
	machine := New(nil)
	machine.EnableTestContext()
	collisionClass(t, machine, Class{Name: "CartLike", Methods: map[string]Method{
		"b": {ReturnType: "String", Access: "public", Program: collisionProgram(t, "return 'body';")},
	}})
	collisionClass(t, machine, Class{Name: "CollisionProvider", Interfaces: []string{"System.StubProvider"}, Methods: map[string]Method{
		"handleMethodCall": {ReturnType: "Object", Access: "public", Params: []Param{
			{Name: "receiver", Type: "Object"}, {Name: "method", Type: "String"},
			{Name: "returnType", Type: "Type"}, {Name: "parameterTypes", Type: "List<Type>"},
			{Name: "parameterNames", Type: "List<String>"}, {Name: "arguments", Type: "List<Object>"},
		}, Program: collisionProgram(t, "return 'provider';")},
	}})
	result, err := machine.Execute(collisionProgram(t, "CartLike proxy = (CartLike)Test.createStub(CartLike.class, new CollisionProvider());"))
	if err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "CartLike"
	machine.currentMethod = Method{ClassName: "CartLike"}
	machine.Globals["this"] = result.Vars["proxy"]
	value, err := machine.call("b", nil, nil, &Result{})
	if err != nil || value.Kind != ValueString || value.Text != "provider" {
		t.Fatalf("implicit proxy b() = %#v, %v; want provider", value, err)
	}
}

func TestImplicitNamespaceCollisionSkipsBareDependencyMethod(t *testing.T) {
	machine := New(nil)
	collisionClass(t, machine, Class{Name: "BaseLike", Methods: map[string]Method{
		"b": {ReturnType: "String", Access: "protected", Program: collisionProgram(t, "return 'local';")},
	}})
	collisionClass(t, machine, Class{Name: "CartLike", SuperClass: "BaseLike", Methods: map[string]Method{
		"a": {ReturnType: "String", Access: "public", Program: collisionProgram(t, "return b();")},
	}})
	pkg := Method{ReturnType: "String", Access: "global", Program: collisionProgram(t, "return 'package';")}
	collisionClass(t, machine, Class{Name: "CartLike", Namespace: "pkgns", Dependency: true, Methods: map[string]Method{"b": pkg}})
	// Project loaders also register package methods under the bare class name.
	pkg.Name, pkg.ClassName, pkg.Dependency = "CartLike.b", "CartLike", true
	if err := machine.RegisterMethod(pkg); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(collisionProgram(t, "System.assertEquals('local', new CartLike().a());")); err != nil {
		t.Fatal(err)
	}
}

package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Typed-expression dispatch is backed by the owned the captured rows and
// The captured rows rows and typed_null_n.json N01-N09/N14-N24/N31. The latter
// includes automatic-property hydration and lazy user-list serializer twins
// at APIs 62/67/68 through anonymous and named test callers.
func nullChoiceMachine(t *testing.T) *VM {
	t.Helper()
	machine := New(nil)
	for _, class := range []Class{
		{Name: "NullChoiceLab"},
		{Name: "NullChoiceLab.Pebble"},
		{Name: "NullChoiceLab.Parcel", Fields: map[string]Field{
			"items": {Name: "items", Type: "List<NullChoiceLab.Pebble>", Property: true, HasGetter: true, HasSetter: true},
		}},
	} {
		if err := machine.RegisterClass(class); err != nil {
			t.Fatal(err)
		}
	}
	for _, spec := range []struct{ name, param, label string }{
		{"inspect", "List<String>", "strings"},
		{"inspect", "List<Object>", "objects"},
		{"inspect", "List<Id>", "ids"},
		{"describe", "String", "text"},
		{"describe", "List<NullChoiceLab.Pebble>", "pebbles"},
	} {
		body, err := CompileAnonymous("return '" + spec.label + "';")
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{Name: "NullChoiceLab." + spec.name, ClassName: "NullChoiceLab", IsStatic: true, ReturnType: "String", Params: []Param{{Name: "v", Type: spec.param}}, Program: body}); err != nil {
			t.Fatal(err)
		}
	}
	return machine
}

func TestJSONNullCollectionPreservesDeclaredType(t *testing.T) {
	for _, strict := range []bool{false, true} {
		machine := nullChoiceMachine(t)
		parcel, err := machine.typedValueFromJSON("NullChoiceLab.Parcel", map[string]any{"items": nil}, strict)
		if err != nil {
			t.Fatal(err)
		}
		value := parcel.Fields["items"]
		if value.Kind != ValueNull || value.Type != "List<NullChoiceLab.Pebble>" || value.Static != value.Type {
			t.Fatalf("hydrated field lost declared type: %#v", value)
		}
	}
	program, err := CompileAnonymous(`
NullChoiceLab.Parcel p = (NullChoiceLab.Parcel)JSON.deserialize('{"items":null}', NullChoiceLab.Parcel.class);
System.assertEquals(null, p.items);
System.assertEquals('objects', NullChoiceLab.inspect(p.items));
p = (NullChoiceLab.Parcel)JSON.deserialize('{}', NullChoiceLab.Parcel.class);
System.assertEquals('objects', NullChoiceLab.inspect(p.items));
p = (NullChoiceLab.Parcel)JSON.deserialize('{"items":[]}', NullChoiceLab.Parcel.class);
System.assertEquals(0, p.items.size());
System.assertEquals('objects', NullChoiceLab.inspect(p.items));
p = new NullChoiceLab.Parcel();
System.assertEquals('objects', NullChoiceLab.inspect(p.items));
p.items = null;
System.assertEquals('objects', NullChoiceLab.inspect(p.items));
System.assertEquals(null, JSON.deserializeUntyped('null'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nullChoiceMachine(t).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestNullTernaryUsesDeclaredReturnTypeLazily(t *testing.T) {
	for _, expression := range []string{
		"pebbles == null ? null : JSON.serialize(pebbles)",
		"pebbles != null ? JSON.serialize(pebbles) : null",
		"true ? null : JSON.serialize(new List<NullChoiceLab.Pebble>())",
		"true ? null : NullChoiceLab.render(pebbles)",
		"false ? NullChoiceLab.render(pebbles) : null",
		"true ? null : (false ? null : NullChoiceLab.render(pebbles))",
		"true ? null : (String)null",
		"true ? null : 'literal'",
	} {
		t.Run(expression, func(t *testing.T) {
			machine := nullChoiceMachine(t)
			body, err := CompileAnonymous("System.assert(false, 'skipped branch executed'); return JSON.serialize(v);")
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{Name: "NullChoiceLab.render", ClassName: "NullChoiceLab", IsStatic: true, ReturnType: "String", Params: []Param{{Name: "v", Type: "List<NullChoiceLab.Pebble>"}}, Program: body}); err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymous("List<NullChoiceLab.Pebble> pebbles = null; System.assertEquals('text', NullChoiceLab.describe(" + expression + ")); System.assertEquals(null, " + expression + ");")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNullExpressionProvenanceKeepsControls(t *testing.T) {
	for _, call := range []string{
		"NullChoiceLab.inspect(null)",
		"NullChoiceLab.describe(null)",
		"NullChoiceLab.describe(true ? null : null)",
	} {
		program, err := CompileAnonymous(call + ";")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := nullChoiceMachine(t).Execute(program); err == nil || !strings.Contains(err.Error(), "ambiguous overload") {
			t.Fatalf("bare-null control %s: %v", call, err)
		}
	}
	program, err := CompileAnonymous(`
List<NullChoiceLab.Pebble> pebbles = new List<NullChoiceLab.Pebble>();
System.assertEquals('text', NullChoiceLab.describe(pebbles == null ? null : JSON.serialize(pebbles)));
System.assertEquals('[]', pebbles == null ? null : JSON.serialize(pebbles));
System.assertEquals('pebbles', NullChoiceLab.describe(true ? null : pebbles));
System.assertEquals('text', NullChoiceLab.describe((String)null));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nullChoiceMachine(t).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestNullTernaryResolvesInstanceOverloadsAndProjectShadows(t *testing.T) {
	machine := nullChoiceMachine(t)
	for _, name := range []string{"Relay", "JSON"} {
		if err := machine.RegisterClass(Class{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	if err := machine.RegisterClass(Class{Name: "RelaySurface", IsInterface: true}); err != nil {
		t.Fatal(err)
	}
	body, err := CompileAnonymous("System.assert(false, 'skipped branch executed'); return null;")
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []Method{
		{Name: "Relay.render", ClassName: "Relay", ReturnType: "String", Params: []Param{{Name: "v", Type: "List<NullChoiceLab.Pebble>"}}, Program: body},
		{Name: "Relay.render", ClassName: "Relay", ReturnType: "List<NullChoiceLab.Pebble>", Params: []Param{{Name: "v", Type: "String"}}, Program: body},
		{Name: "Relay.render", ClassName: "Relay", ReturnType: "String", Params: []Param{{Name: "v", Type: "Object"}}, Program: body},
		{Name: "RelaySurface.render", ClassName: "RelaySurface", ReturnType: "String", Params: []Param{{Name: "v", Type: "Object"}}, Modifiers: []string{"abstract"}},
		{Name: "JSON.serialize", ClassName: "JSON", IsStatic: true, ReturnType: "List<NullChoiceLab.Pebble>", Params: []Param{{Name: "v", Type: "List<NullChoiceLab.Pebble>"}}, Program: body},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	program, err := CompileAnonymous(`
Relay relay = new Relay();
List<NullChoiceLab.Pebble> pebbles = null;
Object held = 'runtime string';
RelaySurface surface = null;
System.assertEquals('text', NullChoiceLab.describe(true ? null : relay.render(pebbles)));
System.assertEquals('text', NullChoiceLab.describe(true ? null : (relay).render(pebbles)));
System.assertEquals('pebbles', NullChoiceLab.describe(true ? null : relay.render((String)null)));
System.assertEquals('pebbles', NullChoiceLab.describe(true ? null : JSON.serialize(pebbles)));
System.assertEquals('text', NullChoiceLab.describe(true ? null : relay.render(held)));
System.assertEquals('text', NullChoiceLab.describe(true ? null : surface.render(pebbles)));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestNullTernaryStaticFieldTypeUsesDeclaringOwner(t *testing.T) {
	machine := nullChoiceMachine(t)
	if err := machine.RegisterClass(Class{Name: "NullChoiceLab", StaticFields: map[string]Field{
		"item": {Name: "item", Type: "Pebble", Static: true},
	}}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"OtherChoiceLab", "OtherChoiceLab.Pebble"} {
		if err := machine.RegisterClass(Class{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	machine.currentClass = "OtherChoiceLab"
	other := ir.Expr{Kind: ir.ExprVariable, Name: "NullChoiceLab.item"}
	value := machine.typedNullTernaryResult(ir.Expr{}, ir.Expr{Kind: ir.ExprLiteral, Value: "null"}, other, Null)
	if value.Type != "NullChoiceLab.Pebble" || value.Static != value.Type {
		t.Fatalf("static field type resolved against caller: %#v", value)
	}
}

package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRuntimeExpressionTypeResolverUsesSemanticTernaryType(t *testing.T) {
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{Name: "NullChoiceLab", Kind: apexast.DeclarationClass, EffectiveAPIVersion: "62.0", Members: []typesys.MemberSymbol{
			{Name: "item", Kind: apexast.DeclarationField, Type: "Pebble", Modifiers: []string{"public", "static"}},
		}},
		{Name: "NullChoiceLab.Pebble", Kind: apexast.DeclarationClass, EffectiveAPIVersion: "62.0"},
		{Name: "OtherChoiceLab", Kind: apexast.DeclarationClass, EffectiveAPIVersion: "62.0"},
		{Name: "OtherChoiceLab.Pebble", Kind: apexast.DeclarationClass, EffectiveAPIVersion: "62.0"},
	}}
	resolve := RuntimeExpressionTypeResolver(index)
	variables := map[string]string{"pebbles": "List<NullChoiceLab.Pebble>", "texts": "List<String>"}
	for _, tc := range []struct{ expr, want string }{
		{"true ? null : JSON.serialize(pebbles)", "String"},
		{"false ? JSON.serialize(pebbles) : null", "String"},
		{"true ? null : ('a' + 'b')", "String"},
		{"true ? null : texts.get(0)", "String"},
		{"true ? null : pebbles", "List<NullChoiceLab.Pebble>"},
		{"true ? null : null", "null"},
	} {
		program, err := vm.CompileAnonymous("return " + tc.expr + ";")
		if err != nil {
			t.Fatal(err)
		}
		if got := resolve(program.Instructions[0].Expr, variables, "NullChoiceLab"); got != tc.want {
			t.Errorf("%s: resolved type %q, want %q", tc.expr, got, tc.want)
		}
	}
	ownerProgram, err := vm.CompileAnonymous("return true ? null : NullChoiceLab.item;")
	if err != nil {
		t.Fatal(err)
	}
	if got := resolve(ownerProgram.Instructions[0].Expr, nil, "OtherChoiceLab"); got != "NullChoiceLab.Pebble" {
		t.Fatalf("semantic static field used caller owner: %q", got)
	}

	machine := vm.New(nil)
	machine.SetExpressionTypeResolver(resolve)
	// Anonymous-only declarations must not replace the project resolver.
	machine.AddExpressionTypeResolver(RuntimeExpressionTypeResolver(typesys.Index{Types: []typesys.TypeSymbol{
		{Name: "OtherChoiceLab", Kind: apexast.DeclarationClass},
	}}))
	if err := machine.RegisterClass(vm.Class{Name: "NullChoiceLab"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(vm.Class{Name: "NullChoiceLab.Pebble"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ param, label string }{{"String", "text"}, {"List<NullChoiceLab.Pebble>", "pebbles"}} {
		body, err := vm.CompileAnonymous("return '" + tc.label + "';")
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(vm.Method{Name: "NullChoiceLab.describe", ClassName: "NullChoiceLab", IsStatic: true, ReturnType: "String", Params: []vm.Param{{Name: "v", Type: tc.param}}, Program: body}); err != nil {
			t.Fatal(err)
		}
	}
	body, err := vm.CompileAnonymous(`
List<NullChoiceLab.Pebble> pebbles = null;
List<String> texts = null;
System.assertEquals('text', NullChoiceLab.describe(true ? null : JSON.serialize(pebbles)));
System.assertEquals('text', NullChoiceLab.describe(true ? null : ('a' + 'b')));
System.assertEquals('text', NullChoiceLab.describe(true ? null : texts.get(0)));
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(vm.Method{Name: "NullChoiceLab.probe", ClassName: "NullChoiceLab", IsStatic: true, ReturnType: "void", Program: body}); err != nil {
		t.Fatal(err)
	}
	invoke, err := vm.CompileAnonymous("NullChoiceLab.probe();")
	if err != nil {
		t.Fatal(err)
	}
	// The skipped get() would dereference null. Binary/get return typing also
	// requires the semantic bridge, so this checks that Clone retains it.
	if _, err := machine.CloneRuntime(nil).Execute(invoke); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeExpressionTypeResolverRejectsAmbiguousBareOwner(t *testing.T) {
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{Name: "NullChoiceLab", Kind: apexast.DeclarationClass},
		{Name: "NullChoiceLab", Namespace: "pkg", Dependency: true, Kind: apexast.DeclarationClass},
	}}
	program, err := vm.CompileAnonymous("return true ? null : 'text';")
	if err != nil {
		t.Fatal(err)
	}
	resolve := RuntimeExpressionTypeResolver(index)
	if got := resolve(program.Instructions[0].Expr, nil, "NullChoiceLab"); got != "" {
		t.Fatalf("ambiguous bare owner must use runtime declaration fallback: %q", got)
	}
	if got := resolve(program.Instructions[0].Expr, nil, "pkg.NullChoiceLab"); got != "String" {
		t.Fatalf("qualified owner lost its semantic view: %q", got)
	}
}

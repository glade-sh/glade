package sema

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestFieldScopeInheritancePrecedenceAndCycles(t *testing.T) {
	field := func(typ string, modifiers ...string) typesys.MemberSymbol {
		return typesys.MemberSymbol{Type: typ, Modifiers: modifiers}
	}
	model := semaTypeMemberViewFromMembers(map[string]typeMembers{
		"outer":       {name: "Outer", fields: map[string]typesys.MemberSymbol{"shared": field("Outer", "static"), "outeronly": field("Boolean", "static"), "instanceonly": field("Long")}},
		"outer.child": {name: "Outer.Child", superClass: "Parent", fields: map[string]typesys.MemberSymbol{"shared": field("Child"), "childonly": field("Integer")}},
		"parent":      {name: "Parent", superClass: "Grand", fields: map[string]typesys.MemberSymbol{"shared": field("Parent"), "parentonly": field("Decimal")}},
		"grand":       {name: "Grand", superClass: "Outer.Child", fields: map[string]typesys.MemberSymbol{"grandonly": field("Date")}},
	})
	want := map[string]string{"shared": "Child", "outeronly": "Boolean", "childonly": "Integer", "parentonly": "Decimal", "grandonly": "Date"}
	if got := semaFieldScope(model, "Outer.Child", make(map[string]bool)); !reflect.DeepEqual(got, want) {
		t.Fatalf("field scope = %#v; want %#v", got, want)
	}
	// The caller's visited set cuts off the parent chain without losing the child's fields.
	got := semaFieldScope(model, "Outer.Child", map[string]bool{"parent": true})
	want = map[string]string{"shared": "Child", "outeronly": "Boolean", "childonly": "Integer"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("visited field scope = %#v; want %#v", got, want)
	}
}

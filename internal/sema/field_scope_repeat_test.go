package sema

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Repeated method scopes must remain independent while avoiding repeated
// inherited-field traversal for a stable view.
func TestRepeatedBodyFieldScopesPreserveIsolationAndBoundAllocations(t *testing.T) {
	members := make(map[string]typeMembers)
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("ScopeLevel%d", i)
		parent := ""
		if i > 0 {
			parent = fmt.Sprintf("ScopeLevel%d", i-1)
		}
		members[normalizeName(name)] = typeMembers{name: name, superClass: parent, fields: map[string]typesys.MemberSymbol{fmt.Sprintf("field%d", i): {Type: "Integer"}}}
	}
	members["other"] = typeMembers{name: "Other", fields: map[string]typesys.MemberSymbol{"field0": {Type: "String"}}}
	model := semaTypeMemberViewFromMembers(members)
	typ := typesys.TypeSymbol{Name: "ScopeLevel19"}
	member := typesys.MemberSymbol{}
	first := semaBodyBaseScope(typ, member, model)
	if len(first) != 22 || first["field0"] != "Integer" {
		t.Fatalf("first scope = %#v", first)
	}
	first["field0"] = "Changed"
	if got := semaBodyBaseScope(typ, member, model)["field0"]; got != "Integer" {
		t.Fatalf("mutating returned scope affected next call: %q", got)
	}
	if got := semaBodyBaseScope(typesys.TypeSymbol{Name: "Other"}, member, model)["field0"]; got != "String" {
		t.Fatalf("other type field = %q, want String", got)
	}
	_ = semaBodyBaseScope(typ, member, model)
	allocs := testing.AllocsPerRun(30, func() { _ = semaBodyBaseScope(typ, member, model) })
	if allocs > 15 {
		t.Fatalf("repeated deep body scope used %.1f allocations; want at most 15", allocs)
	}
	updated := members["scopelevel0"]
	updated.fields = map[string]typesys.MemberSymbol{"field0": {Type: "Boolean"}}
	members["scopelevel0"] = updated
	model.changedHydration()
	if got := semaBodyBaseScope(typ, member, model)["field0"]; got != "Boolean" {
		t.Fatalf("scope after generation change = %q, want Boolean", got)
	}
}

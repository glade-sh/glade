package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestMethodFamilyMemoPreservesViewAndHydration(t *testing.T) {
	member := func(name string) typesys.MemberSymbol { return typesys.MemberSymbol{Name: name, Type: "String"} }
	view := semaTypeMemberViewFromMembers(map[string]typeMembers{
		"child":  {name: "Child", superClass: "Parent", methods: map[string][]typesys.MemberSymbol{"run": {member("run")}}},
		"parent": {name: "Parent", methods: map[string][]typesys.MemberSymbol{"other": {member("other")}}},
	})
	// Repeated inherited family expansion must avoid rebuilding intermediate maps.
	resolveMemberMethods(view, "Child", "run")
	allocs := testing.AllocsPerRun(100, func() { _ = resolveMemberMethods(view, "Child", "run") })
	if allocs > 3 {
		t.Errorf("warm inherited family used %.1f allocations per lookup; want at most 3", allocs)
	}
	first := resolveMemberMethods(view, "Child", "run")
	if len(first) != 1 || first[0].owner != "Child" {
		t.Fatalf("initial family: %#v", first)
	}
	// A returned slice is owned by the caller, not by the warm cache.
	first[0].owner = "changed"
	if again := resolveMemberMethods(view, "Child", "run"); len(again) != 1 || again[0].owner != "Child" {
		t.Fatalf("mutated cached family: %#v", again)
	}
	// An overlay shares hydrated members but not its source-local current type.
	sibling := semaModelWithCurrentType(view, typesys.TypeSymbol{Name: "Sibling"})
	sibling.storeHydrated("parent", typeMembers{name: "Parent", methods: map[string][]typesys.MemberSymbol{"run": {{Name: "run", Type: "Integer", Parameters: nil}}}})
	// Same-signature parent method is hidden by Child, but another receiver sees it.
	if got := resolveMemberMethods(view, "Parent", "run"); len(got) != 1 || got[0].member.Type != "Integer" {
		t.Fatalf("hydrated family: %#v", got)
	}
	sibling.storeHydrated("parent", typeMembers{name: "Parent", methods: map[string][]typesys.MemberSymbol{"run": {{Name: "run", Type: "Boolean"}}}})
	if got := resolveMemberMethods(view, "Parent", "run"); len(got) != 1 || got[0].member.Type != "Boolean" {
		t.Fatalf("stale shared hydration: %#v", got)
	}
}

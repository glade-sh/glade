package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestOverloadRepeatedRelationSelection(t *testing.T) {
	view := semaTypeMemberViewFromMembers(map[string]typeMembers{
		"base":   {name: "Base"},
		"middle": {name: "Middle", superClass: "Base"},
		"leaf":   {name: "Leaf", superClass: "Middle"},
	})
	params := func(tail string) []apexast.Parameter {
		return []apexast.Parameter{{Type: "Base"}, {Type: "Base"}, {Type: "Base"}, {Type: tail}}
	}
	candidates := []resolvedMember{
		{owner: "Owner", member: typesys.MemberSymbol{Name: "choose", Parameters: params("String")}},
		{owner: "Owner", member: typesys.MemberSymbol{Name: "choose", Parameters: params("Object")}},
	}
	args := []string{"Leaf", "Leaf", "Leaf", "String"}
	check := func() {
		got, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, args, view)
		if !ok || ambiguous || got.member.Parameters[3].Type != "String" {
			t.Fatalf("overload: %#v, ok=%v ambiguous=%v", got, ok, ambiguous)
		}
	}
	check()
	allocs := testing.AllocsPerRun(20, check)
	t.Logf("overload selection allocations: %.1f", allocs)
}

package sema

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestMethodFamilyUniqueLookupsDoNotRetainExcessAllocations(t *testing.T) {
	members := map[string]typeMembers{"parent": {name: "Parent", methods: map[string][]typesys.MemberSymbol{"run": {{Name: "run", Type: "String"}}}}}
	names := make([]string, 80)
	for i := range names {
		names[i] = fmt.Sprintf("Child%d", i)
		members[normalizeName(names[i])] = typeMembers{name: names[i], superClass: "Parent"}
	}
	state := semaTypeMemberViewFromMembers(members).state
	cached := testing.AllocsPerRun(10, func() {
		view := state.view()
		for _, name := range names {
			got := resolveMemberMethods(view, name, "run")
			if len(got) != 1 || got[0].owner != "Parent" {
				t.Fatalf("%s family = %#v", name, got)
			}
		}
	})
	reference := testing.AllocsPerRun(10, func() {
		view := state.view()
		for _, name := range names {
			got := resolveMemberMethodsSeen(view, name, "run", make(map[string]bool))
			if len(got) != 1 || got[0].owner != "Parent" {
				t.Fatalf("%s reference = %#v", name, got)
			}
		}
	})
	if cached > reference+20 {
		t.Fatalf("unique inherited families used %.0f allocations vs %.0f uncached; want overhead at most 20", cached, reference)
	}
}

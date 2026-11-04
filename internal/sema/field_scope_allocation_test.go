package sema

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestFieldScopeDeepInheritanceAllocation(t *testing.T) {
	members := make(map[string]typeMembers)
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("Level%d", i)
		parent := ""
		if i > 0 {
			parent = fmt.Sprintf("Level%d", i-1)
		}
		members[normalizeName(name)] = typeMembers{name: name, superClass: parent, fields: map[string]typesys.MemberSymbol{fmt.Sprintf("field%d", i): {Type: "Integer"}}}
	}
	model := semaTypeMemberViewFromMembers(members)
	got := semaFieldScope(model, "Level19", make(map[string]bool))
	if len(got) != 20 {
		t.Fatalf("field count = %d, want 20", len(got))
	}
	allocs := testing.AllocsPerRun(30, func() { _ = semaFieldScope(model, "Level19", make(map[string]bool)) })
	if allocs > 60 {
		t.Fatalf("deep inherited scope used %.1f allocations; want at most 60", allocs)
	}
}

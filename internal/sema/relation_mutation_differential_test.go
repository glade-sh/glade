package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

// Exercise the same selection with and without the operation-local memo while
// another view replaces a hydrated relation between candidate callbacks.
func TestRelationMemoSelectionAfterSharedHydrationReplacement(t *testing.T) {
	method := func(parameters ...string) typesys.MemberSymbol {
		var m typesys.MemberSymbol
		for _, p := range parameters {
			m.Parameters = append(m.Parameters, apexast.Parameter{Type: p})
		}
		return m
	}
	candidates := []typesys.MemberSymbol{
		method("Base", "String", "Integer"), method("Base", "String", "Integer"), method("Child", "Object", "Object"),
	}
	args := []string{"Child", "String", "Integer"}
	type outcome struct {
		index         int
		ok            bool
		before, after int
	}
	run := func(initial bool, memoized bool) outcome {
		view := newSemaTypeMemberStateWithPlatform(&semaTypeMemberModel{members: map[string]typeMembers{
			"child": {name: "Child", superClass: "Base", kind: apexast.DeclarationClass},
			"base":  {name: "Base", kind: apexast.DeclarationClass},
		}}, nil).view()
		child := view.state.base.members["child"]
		if !initial {
			child.superClass = "Other"
		}
		view.storeHydrated("child", child)
		sibling := *view
		var out outcome
		at := func(i int) typesys.MemberSymbol {
			if i == 0 {
				out.before = semaConversionScore("Base", "Child", view)
			}
			if i == 1 {
				next := child
				if initial {
					next.superClass = "Other"
				} else {
					next.superClass = "Base"
				}
				sibling.storeHydrated("child", next)
				out.after = semaConversionScore("Base", "Child", view)
			}
			return candidates[i]
		}
		if memoized {
			var memo semaRelationMemo
			out.index, out.ok = bestMemberConversionScoreIndex(len(candidates), args, view, at, &memo)
		} else {
			out.index, out.ok = bestMemberConversionScoreIndex(len(candidates), args, view, at)
		}
		return out
	}
	for _, initial := range []bool{false, true} {
		want, got := run(initial, false), run(initial, true)
		expected := 1
		if initial {
			expected = 0
		}
		if !want.ok || want.index != expected || (initial && !(want.before >= 0 && want.after < 0)) || (!initial && !(want.before < 0 && want.after >= 0)) {
			t.Fatalf("initial=%t: non-discriminating reference: %+v", initial, want)
		}
		if got != want {
			t.Fatalf("initial=%t: memoized %+v; reference %+v", initial, got, want)
		}
	}
}

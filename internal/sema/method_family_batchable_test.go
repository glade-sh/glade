package sema

import "testing"

func TestMethodFamilyBatchableParametersAreIndependent(t *testing.T) {
	view := methodsAllocationTestView()
	for _, receiver := range []string{"Database.Batchable<Account>", "Database.Batchable<Contact>"} {
		first := resolveMemberMethods(view, receiver, "execute")
		if len(first) != 1 || len(first[0].member.Parameters) != 2 {
			t.Fatalf("%s family: %#v", receiver, first)
		}
		want := first[0].member.Parameters[1].Type
		first[0].member.Parameters[1].Type = "Mutated"
		again := resolveMemberMethods(view, receiver, "execute")
		if len(again) != 1 || again[0].member.Parameters[1].Type != want {
			t.Fatalf("%s cached parameter changed: %#v", receiver, again)
		}
		again[0].member.Parameters[1].Type = "MutatedAgain"
		third := resolveMemberMethods(view, receiver, "execute")
		if third[0].member.Parameters[1].Type != want {
			t.Fatalf("%s returned shared parameter: %#v", receiver, third)
		}
	}
}

package sema

import "testing"

func TestMethodFamilyEmptyAndMissingRemainDistinct(t *testing.T) {
	model := semaTypeMemberViewFromMembers(map[string]typeMembers{"present": {name: "Present"}})
	for i := 0; i < 2; i++ {
		if got := resolveMemberMethods(model, "Present", "missing"); got == nil || len(got) != 0 {
			t.Fatalf("present receiver family = %#v, want non-nil empty", got)
		}
		if got := resolveMemberMethods(model, "Missing", "missing"); got != nil {
			t.Fatalf("missing receiver family = %#v, want nil", got)
		}
	}
}

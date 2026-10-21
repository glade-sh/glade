package vm

import "testing"

func TestSObjectNullIdentityProjection(t *testing.T) {
	left := Object("Account")
	left.Fields["Name"] = String("sample")
	right := cloneValue(left)
	right.Fields["Id"] = Null
	markExplicitSObjectField(&right, "Id")
	if left.Equal(right) && sObjectHashCode(left, map[uint64]bool{}) != sObjectHashCode(right, map[uint64]bool{}) {
		t.Fatal("equal values must share their hash")
	}
	if _, present := right.Fields["Id"]; !present || !isExplicitSObjectField(right, "Id") {
		t.Fatal("equality projection removed a caller's explicit field")
	}
	// Native omitted/null equality outcomes remain in the pending probe packet.
}

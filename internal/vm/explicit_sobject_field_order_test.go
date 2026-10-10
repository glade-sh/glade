package vm

import "testing"

func TestMarkExplicitSObjectFieldKeepsOrdinaryMarkerOrder(t *testing.T) {
	record := Object("Account")
	markExplicitSObjectField(&record, "Name")
	markExplicitSObjectField(&record, "Phone")
	markExplicitSObjectField(&record, "Name")
	got := explicitSObjectFieldNames(record)
	want := []string{"name", "phone"}
	if len(got) != len(want) {
		t.Fatalf("ordinary marker order = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ordinary marker order = %#v, want %#v", got, want)
		}
	}
}

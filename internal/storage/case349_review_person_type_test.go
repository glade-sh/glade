package storage

import "testing"

func TestCase349RecordTypeFamilyUsesMetadata(t *testing.T) {
	for _, tc := range []struct {
		name   string
		person bool
	}{
		{"Person_Commerce", false}, {"Individual_Business", false}, {"Patient", true},
	} {
		info := RecordTypeInfo{DeveloperName: tc.name, Name: tc.name, PersonAccount: tc.person}
		if got := recordTypeIsPersonType("Account", info); got != tc.person {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.person)
		}
	}
}

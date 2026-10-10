package storage

import "testing"

func TestStandardFieldAssignmentReadOnly(t *testing.T) {
	for _, test := range []struct {
		object, field string
		readOnly      bool
	}{
		{"Contact", "CreatedDate", true}, {"contact", "createddate", true},
		{"Contact", "Id", false}, {"Contact", "LastName", false},
		{"PermissionSetAssignment", "AssigneeId", false},
		{"PermissionSetAssignment", "PermissionSetId", false},
		{"PermissionSetAssignment", "SystemModstamp", true},
		{"Owned", "CreatedDate", false}, {"Contact", "OwnedUnknown", false},
	} {
		if got := StandardFieldAssignmentReadOnly(test.object, test.field); got != test.readOnly {
			t.Errorf("%s.%s readonly=%v", test.object, test.field, got)
		}
	}
}

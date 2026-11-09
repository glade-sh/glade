package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Supplemental org.tsv rows 4-5: a group grant changes a user's validation
// result, while an otherwise identical user without the assignment is denied.
func TestFormulaPermissionGroupUserContext(t *testing.T) {
	const group storage.ID = "0PG000000000001"
	const other storage.ID = "005000000000002"
	org := permissionRegexOrg(false, "Bypass")
	org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000002": {Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(permTestUser), "PermissionSetGroupId": storage.IDValue(group)}}}}
	org.Objects["PermissionSetGroupComponent"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0PG000000000002": {Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue(group), "PermissionSetId": storage.IDValue(permTestSet)}}}}
	def := storage.ObjectDefinition{APIName: "Probe__c", ValidationRules: []storage.ValidationRule{{Name: "Guarded", Active: true, ErrorConditionFormula: "NOT($Permission.Bypass)", ErrorMessage: "no grant"}}}
	for _, tc := range []struct {
		user   storage.ID
		denied bool
	}{{permTestUser, false}, {other, true}} {
		engine := NewEngine(&org)
		engine.UserID = tc.user
		record := storage.Record{Object: def.APIName, Fields: map[string]storage.Value{}}
		err := engine.validateValidationRules(def.APIName, def, record, &record, true)
		if (err != nil) != tc.denied {
			t.Errorf("user %s: error=%v want denied=%v", tc.user, err, tc.denied)
		}
	}
}

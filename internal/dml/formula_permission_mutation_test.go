package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Repeated formula evaluation must reflect permission assignment changes.
// The native group rows in supplemental/org.tsv show the same user before and
// after an assignment, including the rejection twin.
func TestFormulaPermissionAssignmentChangesRemainVisible(t *testing.T) {
	org := permissionRegexOrg(false, "Bypass")
	engine := NewEngine(&org)
	engine.UserID = permTestUser
	def := storage.ObjectDefinition{APIName: "Probe__c", ValidationRules: []storage.ValidationRule{{Name: "Guard", Active: true, ErrorConditionFormula: "NOT($Permission.Bypass)", ErrorMessage: "denied"}}}
	record := storage.Record{Object: def.APIName, Fields: map[string]storage.Value{}}
	check := func(denied bool) {
		t.Helper()
		err := engine.validateValidationRules(def.APIName, def, record, &record, true)
		if (err != nil) != denied {
			t.Fatalf("denied=%v error=%v", denied, err)
		}
	}
	check(true)
	state := org.Objects["PermissionSetAssignment"]
	state.Records[permTestAssign] = storage.Record{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(permTestUser), "PermissionSetId": storage.IDValue(permTestSet)}}
	org.Objects["PermissionSetAssignment"] = state
	check(false)
	delete(state.Records, permTestAssign)
	check(true)
}

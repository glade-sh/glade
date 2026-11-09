package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const (
	permTestUser      storage.ID = "005000000000001AAA"
	permTestSet       storage.ID = "0PS000000000001AAA"
	permTestEntityID  storage.ID = "0CP000000000001AAA"
	permTestOtherEnt  storage.ID = "0CP000000000002AAA"
	permTestAssign    storage.ID = "0Pa000000000001AAA"
	permTestEntityRow storage.ID = "0Pa000000000002AAA"
)

func permissionRegexOrg(assigned bool, granted string) storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		permTestUser: {Fields: map[string]storage.Value{}},
	}}
	org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		permTestSet: {Fields: map[string]storage.Value{"Name": storage.StringValue("Bypass_Set")}},
	}}
	org.Objects["CustomPermission"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		permTestEntityID: {Fields: map[string]storage.Value{"DeveloperName": storage.StringValue(granted)}},
		permTestOtherEnt: {Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("Unrelated")}},
	}}
	org.Objects["SetupEntityAccess"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		permTestEntityRow: {Fields: map[string]storage.Value{
			"ParentId":      storage.IDValue(permTestSet),
			"SetupEntityId": storage.IDValue(permTestEntityID),
		}},
	}}
	assignments := map[storage.ID]storage.Record{}
	if assigned {
		assignments[permTestAssign] = storage.Record{Fields: map[string]storage.Value{
			"AssigneeId":      storage.IDValue(permTestUser),
			"PermissionSetId": storage.IDValue(permTestSet),
		}}
	}
	org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: assignments}
	return org
}

// Rows: validation rules with $Permission and REGEX braces/anchors
// (internal/apextest/testdata/conformance/native_formula_permission_regex/org.tsv).
func TestValidationRulePermissionAndRegexQuantifier(t *testing.T) {
	const formula = `AND(NOT($Permission.Bypass), IF(NOT(ISBLANK(Phone__c)), NOT(REGEX(Phone__c, "^[0-9]{10}$")), False))`
	definition := storage.ObjectDefinition{APIName: "Probe__c", ValidationRules: []storage.ValidationRule{
		{Name: "Phone", Active: true, ErrorConditionFormula: formula, ErrorMessage: "bad phone"},
	}}
	for _, tc := range []struct {
		name     string
		assigned bool
		granted  string
		phone    string
		deny     bool
	}{
		{"valid unassigned", false, "Bypass", "5551234567", false},
		{"invalid unassigned", false, "Bypass", "555123456", true},
		{"invalid assigned bypasses", true, "Bypass", "555123456", false},
		{"valid assigned", true, "Bypass", "5551234567", false},
		{"assigned other permission", true, "Unrelated", "555123456", true},
		{"too long", false, "Bypass", "55512345678", true},
		{"blank", false, "Bypass", "", false},
		{"non digit", false, "Bypass", "55512345ab", true},
	} {
		org := permissionRegexOrg(tc.assigned, tc.granted)
		engine := NewEngine(&org)
		engine.UserID = permTestUser
		fields := map[string]storage.Value{}
		if tc.phone != "" {
			fields["Phone__c"] = storage.StringValue(tc.phone)
		}
		record := storage.Record{Object: definition.APIName, Fields: fields}
		err := engine.validateValidationRules(definition.APIName, definition, record, &record, true)
		if (err != nil) != tc.deny {
			t.Errorf("%s: error=%v; want denied=%v", tc.name, err, tc.deny)
		}
	}
}

// REGEX matches the whole text: an unanchored pattern must cover the value
// (native_formula_permission_regex/org.tsv, CodeShape and CodeDigitsMatch).
func TestFormulaRegexMatchesWholeText(t *testing.T) {
	for _, tc := range []struct {
		value, pattern string
		want           bool
	}{
		{"12345", "[0-9]{3}", false},
		{"123", "[0-9]{3}", true},
		{"AB12345", "[A-Z]{2}[0-9]{3,4}", false},
		{"xAB123", "[A-Z]{2}[0-9]{3,4}", false},
		{"AB1234", "[A-Z]{2}[0-9]{3,4}", true},
		{"abc-12", "^[a-z]+-[0-9]{2,}$", true},
		{"abc-12x", "^[a-z]+-[0-9]{2,}$", false},
		{"a", "a|b", true},
		{"ab", "a|b", false},
	} {
		definition := storage.ObjectDefinition{APIName: "Probe__c", ValidationRules: []storage.ValidationRule{
			{Name: "Match", Active: true, ErrorConditionFormula: `REGEX(Name, "` + tc.pattern + `")`, ErrorMessage: "matched"},
		}}
		org := storage.NewOrgState()
		engine := NewEngine(&org)
		record := storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue(tc.value)}}
		err := engine.validateValidationRules(definition.APIName, definition, record, &record, true)
		if (err != nil) != tc.want {
			t.Errorf("REGEX(%q, %q): error=%v; want match=%v", tc.value, tc.pattern, err, tc.want)
		}
	}
}

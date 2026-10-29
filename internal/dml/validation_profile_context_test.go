package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native administrator and Standard User acceptance/rejection twins:
// internal/apextest/testdata/conformance/native_profile_context/org.tsv:2-5.
func TestValidationProfileNameUsesExecutionUser(t *testing.T) {
	org := storage.NewOrgState()
	const adminUser storage.ID = "005000000000001AAA"
	const standardUser storage.ID = "005000000000002AAA"
	const adminProfile storage.ID = "00e000000000001AAA"
	const standardProfile storage.ID = "00e000000000002AAA"
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		adminUser:    {Fields: map[string]storage.Value{"ProfileId": storage.IDValue(adminProfile)}},
		standardUser: {Fields: map[string]storage.Value{"ProfileId": storage.IDValue(standardProfile)}},
	}}
	org.Objects["Profile"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		adminProfile:    {Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}},
		standardProfile: {Fields: map[string]storage.Value{"Name": storage.StringValue("Standard User")}},
	}}
	definition := storage.ObjectDefinition{APIName: "ContextRecord__c", ValidationRules: []storage.ValidationRule{
		{Name: "AdministratorOnly", Active: true, ErrorConditionFormula: `AND(Name = "restricted", $Profile.Name <> "System Administrator")`, ErrorMessage: "Administrator required"},
		{Name: "StandardOnly", Active: true, ErrorConditionFormula: `AND(Name = "standard-only", $Profile.Name <> "Standard User")`, ErrorMessage: "Standard profile required"},
	}}
	engine := NewEngine(&org)
	for _, tc := range []struct {
		user storage.ID
		name string
		deny bool
	}{
		{adminUser, "restricted", false},
		{adminUser, "standard-only", true},
		{standardUser, "restricted", true},
		{standardUser, "standard-only", false},
		{adminUser, "ordinary", false},
		{standardUser, "ordinary", false},
		{adminUser, "restricted", false}, // Restore the caller after a user switch.
	} {
		engine.UserID = tc.user
		record := storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue(tc.name)}}
		for _, insert := range []bool{true, false} {
			err := engine.validateValidationRules(definition.APIName, definition, record, &record, insert)
			if (err != nil) != tc.deny {
				t.Errorf("user=%s name=%s insert=%v: error=%v; want denied=%v", tc.user, tc.name, insert, err, tc.deny)
			}
		}
	}
}

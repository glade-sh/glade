package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// SECURITY_ENFORCED supports cross-object SELECT fields with read access:
// https://developer.salesforce.com/blogs/2020/01/learn-moar-in-spring-20-with-field-level-security-in-apex
func TestExecSOQLSecurityUsesDeclaredRelationshipField(t *testing.T) {
	for _, tc := range []struct {
		name       string
		projection string
		where      string
		security   string
		wantCount  int
		wantError  string
	}{
		{name: "empty", projection: "Shelf__r.Code__c", where: " WHERE Name = 'absent'", security: " WITH SECURITY_ENFORCED"},
		{name: "populated", projection: "Shelf__r.Code__c", security: " WITH SECURITY_ENFORCED", wantCount: 1},
		{name: "qualified", projection: "AtlasEntry__c.Shelf__r.Code__c", security: " WITH SECURITY_ENFORCED", wantCount: 1},
		{name: "reference_control", projection: "Shelf__c", security: " WITH SECURITY_ENFORCED", wantCount: 1},
		{name: "ordinary_query_control", projection: "Shelf__r.Code__c", wantCount: 1},
		{name: "denied_parent_field_control", projection: "Shelf__r.Secret__c", security: " WITH SECURITY_ENFORCED", wantError: "QueryException"},
		{name: "denied_local_field_control", projection: "Secret__c", security: " WITH SECURITY_ENFORCED", wantError: "QueryException"},
		{name: "unknown_relationship_control", projection: "Missing__r.Code__c", security: " WITH SECURITY_ENFORCED", wantError: "QueryException"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := relationshipSecurityTestOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			user := Object("User")
			user.Fields["Id"] = String("005000000000089")
			profile := Object("Profile")
			profile.Fields["Name"] = String("Standard User")
			user.Fields["Profile"] = profile
			machine.executionUser = user
			if !machine.currentUserFieldPermission("AtlasEntry__c", "Shelf__c", "isAccessible") ||
				!machine.currentUserFieldPermission("AtlasShelf__c", "Code__c", "isAccessible") {
				t.Fatal("fixture requires read access to the reference and parent field")
			}

			source := fmt.Sprintf("List<SObject> rows = Database.query('SELECT Id, %s FROM AtlasEntry__c%s%s'); System.assertEquals(%d, rows.size());",
				tc.projection, strings.ReplaceAll(tc.where, "'", "\\'"), tc.security, tc.wantCount)
			if tc.name == "populated" {
				source += "System.assertEquals('visible', rows[0].getSObject('Shelf__r').get('Code__c'));"
			}
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "50.0"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("query error = %v, want %s", err, tc.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func relationshipSecurityTestOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["AtlasShelf__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "AtlasShelf__c", KeyPrefix: "a10", Fields: map[string]storage.Field{
			"Name":      {APIName: "Name", Type: storage.FieldString},
			"Code__c":   {APIName: "Code__c", Type: storage.FieldString},
			"Secret__c": {APIName: "Secret__c", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{"a10000000000001": {ID: "a10000000000001", Object: "AtlasShelf__c", Fields: map[string]storage.Value{
			"Name": storage.StringValue("shelf"), "Code__c": storage.StringValue("visible"),
		}}},
	}
	org.Objects["AtlasEntry__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "AtlasEntry__c", KeyPrefix: "a11", Fields: map[string]storage.Field{
			"Name":      {APIName: "Name", Type: storage.FieldString},
			"Secret__c": {APIName: "Secret__c", Type: storage.FieldString},
			"Shelf__c":  {APIName: "Shelf__c", Type: storage.FieldReference, Required: true, ReferenceTo: []string{"AtlasShelf__c"}},
		}, Relations: []storage.Relationship{{Field: "Shelf__c", ParentObjects: []string{"AtlasShelf__c"}, ParentRelationship: "Shelf__r", ChildRelationship: "Entries__r"}}},
		Records: map[storage.ID]storage.Record{"a11000000000001": {ID: "a11000000000001", Object: "AtlasEntry__c", Fields: map[string]storage.Value{
			"Name": storage.StringValue("entry"), "Shelf__c": storage.IDValue("a10000000000001"),
		}}},
	}
	org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		"0Pa000000000089": {ID: "0Pa000000000089", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{
			"AssigneeId": storage.IDValue("005000000000089"), "PermissionSetId": storage.IDValue("0PS000000000089"),
		}},
	}}
	org.Objects["ObjectPermissions"] = storage.ObjectState{Records: make(map[storage.ID]storage.Record)}
	org.Objects["FieldPermissions"] = storage.ObjectState{Records: make(map[storage.ID]storage.Record)}
	for i, object := range []string{"AtlasEntry__c", "AtlasShelf__c"} {
		id := storage.ID(fmt.Sprintf("11000000000008%d", i))
		org.Objects["ObjectPermissions"].Records[id] = storage.Record{ID: id, Object: "ObjectPermissions", Fields: map[string]storage.Value{
			"ParentId": storage.IDValue("0PS000000000089"), "SObjectType": storage.StringValue(object), "PermissionsRead": storage.BooleanValue(true),
		}}
	}
	for i, field := range []string{"AtlasEntry__c.Shelf__c", "AtlasShelf__c.Code__c"} {
		id := storage.ID(fmt.Sprintf("0FP00000000008%d", i))
		object, _, _ := strings.Cut(field, ".")
		org.Objects["FieldPermissions"].Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{
			"ParentId": storage.IDValue("0PS000000000089"), "SObjectType": storage.StringValue(object), "Field": storage.StringValue(field), "PermissionsRead": storage.BooleanValue(true),
		}}
	}
	return org
}

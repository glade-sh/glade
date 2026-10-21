package vm

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestNamespacedContactFieldPermissionConformance(t *testing.T) {
	runNamespacedFieldPermissionConformance(t, "namespaced_contact_fls.json", "Contact", false)
}

func TestNamespacedCustomObjectFieldPermissionConformance(t *testing.T) {
	runNamespacedFieldPermissionConformance(t, "namespaced_custom_object_fls.json", "parcel__OwnedFls__c", true)
}

func runNamespacedFieldPermissionConformance(t *testing.T, filename, objectName string, custom bool) {
	t.Helper()
	var native struct {
		Cells []struct {
			SourceAPI           string `json:"sourceApi"`
			Scenario            string `json:"scenario"`
			Object              []bool `json:"object"`
			A                   []bool `json:"a"`
			B                   []bool `json:"b"`
			ExplicitAssignments int    `json:"explicitAssignments"`
			ProfileAssignments  int    `json:"profileAssignments"`
			ProfileOwnedRows    int    `json:"profileOwnedFieldRows"`
		} `json:"cells"`
	}
	raw, err := os.ReadFile("../apextest/testdata/conformance/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	if len(native.Cells) != 12 {
		t.Fatalf("native cells = %d, want 12", len(native.Cells))
	}
	seen := map[string]bool{}
	for _, cell := range native.Cells {
		key := cell.SourceAPI + "/" + cell.Scenario
		if seen[key] || len(cell.Object) != 2 || len(cell.A) != 2 || len(cell.B) != 2 || cell.ProfileAssignments != 1 || cell.ProfileOwnedRows != 0 {
			t.Fatalf("invalid native cell %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			org := testDataOrg()
			org.Namespace = "parcel"
			storage.EnsureDeterministicPlatformData(&org)
			if custom {
				org.Objects[objectName] = storage.ObjectState{Definition: storage.ObjectDefinition{
					APIName: objectName, KeyPrefix: "a08", Fields: map[string]storage.Field{
						"Id": {APIName: "Id", Type: storage.FieldID}, "Name": {APIName: "Name", Type: storage.FieldString, Required: true},
					},
				}, Records: map[storage.ID]storage.Record{}}
			}
			contact := org.Objects[objectName]
			contact.Definition.Fields = maps.Clone(contact.Definition.Fields)
			for _, name := range []string{"parcel__OwnedFlagA__c", "parcel__OwnedFlagB__c"} {
				contact.Definition.Fields[name] = storage.Field{APIName: name, Type: storage.FieldBoolean}
			}
			org.Objects[objectName] = contact
			seed := func(object, id string, fields map[string]storage.Value) {
				storage.EnsureStandardObject(&org, object)
				state := org.Objects[object]
				state.Records[storage.ID(id)] = storage.Record{ID: storage.ID(id), Object: object, Fields: fields}
				org.Objects[object] = state
			}
			for i, name := range []string{"Sparse", "Positive", "Unrelated"} {
				id := fmt.Sprintf("0PS0000000008%02d", i)
				seed("PermissionSet", id, map[string]storage.Value{"Name": storage.StringValue("Owned" + name), "IsOwnedByProfile": storage.BooleanValue(false)})
				if i < 2 || custom {
					seed("ObjectPermissions", fmt.Sprintf("1100000000008%02d", i), map[string]storage.Value{
						"ParentId": storage.IDValue(storage.ID(id)), "SObjectType": storage.StringValue(objectName),
						"PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true),
					})
				}
			}
			for i, field := range []string{"parcel__OwnedFlagA__c", "parcel__OwnedFlagA__c", "parcel__OwnedFlagB__c"} {
				parent := "0PS000000000800"
				if i > 0 {
					parent = "0PS000000000801"
				}
				seed("FieldPermissions", fmt.Sprintf("0FP0000000008%02d", i), map[string]storage.Value{
					"ParentId": storage.IDValue(storage.ID(parent)), "SObjectType": storage.StringValue(objectName), "Field": storage.StringValue(objectName + "." + field),
					"PermissionsRead": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(i > 0),
				})
			}
			assignment := ""
			assigned := 0
			for i, scenario := range []string{"noEdit", "grant", "missing"} {
				if cell.Scenario == scenario || (cell.Scenario == "combined" && i < 2) {
					assignment += fmt.Sprintf("insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId='0PS0000000008%02d');\n", i)
					assigned++
				}
			}
			if assigned != cell.ExplicitAssignments {
				t.Fatal("assignment count differs from native actor")
			}
			source := fmt.Sprintf(`
Profile p=[SELECT Id FROM Profile WHERE Name='Standard User'];
User actor=new User(ProfileId=p.Id,Username='owned.fls@example.invalid',Email='owned@example.invalid',LastName='Owned',Alias='owned',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
insert actor;
%s
System.runAs(actor) {
 System.assertEquals(%t,Contact.SObjectType.getDescribe().isAccessible(),'object read');
 System.assertEquals(%t,Contact.SObjectType.getDescribe().isUpdateable(),'object edit');
 System.assertEquals(%t,Contact.parcel__OwnedFlagA__c.getDescribe().isAccessible(),'A read');
 System.assertEquals(%t,Contact.parcel__OwnedFlagA__c.getDescribe().isUpdateable(),'A edit');
 System.assertEquals(%t,Contact.parcel__OwnedFlagB__c.getDescribe().isAccessible(),'B read');
 System.assertEquals(%t,Contact.parcel__OwnedFlagB__c.getDescribe().isUpdateable(),'B edit');
}
`, assignment, cell.Object[0], cell.Object[1], cell.A[0], cell.A[1], cell.B[0], cell.B[1])
			program, err := CompileAnonymousWithOptions(strings.ReplaceAll(source, "Contact", objectName), CompileOptions{APIVersion: cell.SourceAPI})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

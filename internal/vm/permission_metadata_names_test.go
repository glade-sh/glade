package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecPermissionMetadataNamespaceGrants(t *testing.T) {
	for _, tc := range []struct {
		name, object, fieldObject, field                 string
		assigned, read, fieldRead, wantObject, wantField bool
	}{
		{"local", "Study__c", "Study__c", "Study__c.Payload__c", true, true, true, true, true},
		{"qualified", "parcel__Study__c", "parcel__Study__c", "parcel__Study__c.parcel__Payload__c", true, true, true, true, true},
		{"mixed", "Study__c", "parcel__Study__c", "parcel__Study__c.Payload__c", true, true, true, true, true},
		{"unassigned", "Study__c", "Study__c", "Study__c.Payload__c", false, true, true, false, false},
		{"foreign_object", "foreign__Study__c", "foreign__Study__c", "foreign__Study__c.Payload__c", true, true, true, false, false},
		{"foreign_field_object", "parcel__Study__c", "parcel__Study__c", "foreign__Study__c.Payload__c", true, true, true, true, false},
		{"foreign_field", "parcel__Study__c", "parcel__Study__c", "parcel__Study__c.foreign__Payload__c", true, true, true, true, false},
		{"explicit_denial", "Study__c", "Study__c", "Study__c.Payload__c", true, false, true, false, false},
		{"explicit_field_denial", "Study__c", "Study__c", "Study__c.Payload__c", true, true, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testDataOrg()
			org.Namespace = "parcel"
			storage.EnsureDeterministicPlatformData(&org)
			org.Objects["parcel__Study__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName: "parcel__Study__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
					"Id":                 {APIName: "Id", Type: storage.FieldID},
					"parcel__Payload__c": {APIName: "parcel__Payload__c", Type: storage.FieldString},
				},
			}, Records: map[storage.ID]storage.Record{}}
			seed := func(object string, id storage.ID, fields map[string]storage.Value) {
				storage.EnsureStandardObject(&org, object)
				state := org.Objects[object]
				state.Records[id] = storage.Record{ID: id, Object: object, Fields: fields}
				org.Objects[object] = state
			}
			seed("PermissionSet", "0PS000000000701", map[string]storage.Value{"Name": storage.StringValue("StudyRead"), "Label": storage.StringValue("Study Read")})
			seed("ObjectPermissions", "0OP000000000701", map[string]storage.Value{
				"ParentId": storage.IDValue("0PS000000000701"), "SObjectType": storage.StringValue(tc.object),
				"PermissionsRead": storage.BooleanValue(tc.read), "PermissionsEdit": storage.BooleanValue(false),
			})
			seed("FieldPermissions", "0FP000000000701", map[string]storage.Value{
				"ParentId": storage.IDValue("0PS000000000701"), "SObjectType": storage.StringValue(tc.fieldObject),
				"Field": storage.StringValue(tc.field), "PermissionsRead": storage.BooleanValue(tc.fieldRead), "PermissionsEdit": storage.BooleanValue(false),
			})
			assignment := ""
			if tc.assigned {
				assignment = "insert new PermissionSetAssignment(AssigneeId=actor.Id, PermissionSetId='0PS000000000701');"
			}
			program, err := CompileAnonymous(fmt.Sprintf(`
Profile minimal = [SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce'];
User actor = new User(Username='study@example.invalid',Alias='study',Email='study@example.invalid',LastName='Study',ProfileId=minimal.Id,TimeZoneSidKey='UTC',LocaleSidKey='en_US',LanguageLocaleKey='en_US',EmailEncodingKey='UTF-8');
insert actor;
%s
System.runAs(actor) {
 System.assertEquals(%t, parcel__Study__c.SObjectType.getDescribe().isAccessible(), 'object grant');
 System.assertEquals(%t, parcel__Study__c.parcel__Payload__c.getDescribe().isAccessible(), 'field grant');
 System.assertEquals(false, parcel__Study__c.SObjectType.getDescribe().isUpdateable(), 'object edit denied');
 System.assertEquals(false, parcel__Study__c.parcel__Payload__c.getDescribe().isUpdateable(), 'field edit denied');
}
`, assignment, tc.wantObject, tc.wantField))
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

func TestExecPermissionMetadataNamespaceProfileAllowlist(t *testing.T) {
	org := testDataOrg()
	org.Namespace = "parcel"
	storage.EnsureDeterministicPlatformData(&org)
	org.Objects["parcel__Study__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "parcel__Study__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
			"Id":                 {APIName: "Id", Type: storage.FieldID},
			"parcel__Payload__c": {APIName: "parcel__Payload__c", Type: storage.FieldString},
			"parcel__Other__c":   {APIName: "parcel__Other__c", Type: storage.FieldString},
		},
	}, Records: map[storage.ID]storage.Record{}}
	org.Objects["Profile"].Records["00e000000000701"] = storage.Record{
		ID: "00e000000000701", Object: "Profile", Fields: map[string]storage.Value{
			"Name": storage.StringValue("Read Only"), "UserType": storage.StringValue("Standard"),
		},
	}
	org.Objects["PermissionSet"].Records["0PS000000000701"] = storage.Record{
		ID: "0PS000000000701", Object: "PermissionSet", Fields: map[string]storage.Value{
			"Name": storage.StringValue("StudyProfile"), "ProfileId": storage.IDValue("00e000000000701"), "IsOwnedByProfile": storage.BooleanValue(true),
		},
	}

	program, err := CompileAnonymous(`
Profile p = [SELECT Id FROM Profile WHERE Name='Read Only'];
PermissionSet profileGrant = [SELECT Id FROM PermissionSet WHERE ProfileId=:p.Id AND IsOwnedByProfile=true];
insert new ObjectPermissions(ParentId=profileGrant.Id,SObjectType='Study__c',PermissionsRead=true);
insert new FieldPermissions(ParentId=profileGrant.Id,SObjectType='Study__c',Field='Study__c.Payload__c',PermissionsRead=true);
User actor = new User(Username='allowlist@example.invalid',Alias='allowlist',Email='allowlist@example.invalid',LastName='Allowlist',ProfileId=p.Id,TimeZoneSidKey='UTC',LocaleSidKey='en_US',LanguageLocaleKey='en_US',EmailEncodingKey='UTF-8');
insert actor;
System.runAs(actor) {
 System.assertEquals(true, parcel__Study__c.parcel__Payload__c.getDescribe().isAccessible());
 System.assertEquals(false, parcel__Study__c.parcel__Other__c.getDescribe().isAccessible());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if machine.permissionMetadataNameMatches("Account", "parcel__Account") {
		t.Fatal("custom namespace aliases must not match standard object names")
	}
}

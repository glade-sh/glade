package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The first six contracts were accepted by the owned API 53 Salesforce proof.
// Mixed-row and all-or-none controls below are additional local regressions.
func TestObjectRecordGrantsAPI53(t *testing.T) {
	for _, tc := range []struct {
		name               string
		viewAll, modifyAll bool
		body               string
	}{
		{"viewAllReadsOtherOwner", true, false, `System.runAs(u){System.assertEquals(1,[SELECT COUNT() FROM Contact WHERE Id=:contactId]);}`},
		{"objectCrudAloneDoesNotReadPrivateContact", false, false, `System.runAs(u){System.assertEquals(0,[SELECT COUNT() FROM Contact WHERE Id=:contactId]);}`},
		{"viewAllDoesNotGrantDelete", true, false, `System.runAs(u){UserRecordAccess access=[SELECT RecordId,HasReadAccess,HasDeleteAccess FROM UserRecordAccess WHERE UserId=:u.Id AND RecordId=:contactId];System.assertEquals(true,access.HasReadAccess);System.assertEquals(false,access.HasDeleteAccess);}`},
		{"viewAllCannotUpdate", true, false, `System.runAs(u){Database.SaveResult result=Database.update(new Contact(Id=contactId,LastName='forbidden'),false);System.assertEquals(false,result.isSuccess());System.assertEquals(StatusCode.INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY,result.getErrors()[0].getStatusCode());}System.assertEquals('Other owner',[SELECT LastName FROM Contact WHERE Id=:contactId].LastName);`},
		{"viewAllCannotDelete", true, false, `System.runAs(u){Database.DeleteResult result=Database.delete(new Contact(Id=contactId),false);System.assertEquals(false,result.isSuccess());System.assertEquals(StatusCode.INSUFFICIENT_ACCESS_OR_READONLY,result.getErrors()[0].getStatusCode());}System.assertEquals(1,[SELECT COUNT() FROM Contact WHERE Id=:contactId]);`},
		{"modifyAllCanUpdateAndDelete", true, true, `System.runAs(u){Database.SaveResult updated=Database.update(new Contact(Id=contactId,LastName='allowed'),false);System.assertEquals(true,updated.isSuccess());System.assertEquals('allowed',[SELECT LastName FROM Contact WHERE Id=:contactId].LastName);Database.DeleteResult removed=Database.delete(new Contact(Id=contactId),false);System.assertEquals(true,removed.isSuccess());System.assertEquals(0,[SELECT COUNT() FROM Contact WHERE Id=:contactId]);}`},

		// Local-only controls; these are not part of the six Salesforce observations.
		{"partialUpdateKeepsOrderAndOwnedSuccess", true, false, `System.runAs(u){Contact own=new Contact(LastName='Owned');insert own;List<Database.SaveResult> results=Database.update(new List<Contact>{new Contact(Id=contactId,LastName='forbidden'),new Contact(Id=own.Id,LastName='changed')},false);System.assertEquals(2,results.size());System.assertEquals(false,results[0].isSuccess());System.assertEquals(true,results[1].isSuccess());System.assertEquals('changed',[SELECT LastName FROM Contact WHERE Id=:own.Id].LastName);}System.assertEquals('Other owner',[SELECT LastName FROM Contact WHERE Id=:contactId].LastName);`},
		{"allOrNoneDoesNotChangeOwnedRecord", true, false, `System.runAs(u){Contact own=new Contact(LastName='Owned');insert own;Boolean caught=false;try{Database.update(new List<Contact>{new Contact(Id=own.Id,LastName='changed'),new Contact(Id=contactId,LastName='forbidden')},true);}catch(DmlException e){caught=true;}System.assertEquals(true,caught);System.assertEquals('Owned',[SELECT LastName FROM Contact WHERE Id=:own.Id].LastName);}`},
		{"partialDeleteKeepsOwnedSuccess", true, false, `System.runAs(u){Contact own=new Contact(LastName='Owned');insert own;List<Database.DeleteResult> results=Database.delete(new List<Contact>{own,new Contact(Id=contactId)},false);System.assertEquals(true,results[0].isSuccess());System.assertEquals(false,results[1].isSuccess());System.assertEquals(0,[SELECT COUNT() FROM Contact WHERE Id=:own.Id]);}System.assertEquals(1,[SELECT COUNT() FROM Contact WHERE Id=:contactId]);`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf(`Boolean viewAll=%t; Boolean modifyAll=%t;`, tc.viewAll, tc.modifyAll) + `Profile p=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce'];User u=new User(Username='glade109.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='reader',Email='reader@example.invalid',LastName='Reader',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');insert u;PermissionSet ps=new PermissionSet(Name='Glade109',Label='Glade109');insert ps;insert new ObjectPermissions(ParentId=ps.Id,SObjectType='Contact',PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true,PermissionsDelete=true,PermissionsViewAllRecords=viewAll,PermissionsModifyAllRecords=modifyAll);insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps.Id);Id contactId;System.runAs(new User(Id=UserInfo.getUserId())){Contact c=new Contact(LastName='Other owner');insert c;contactId=c.Id;}` + tc.body
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "53.0"})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "Contact")
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if err := machine.RegisterClass(Class{Name: "SharingProbe", APIVersion: "53.0", Modifiers: []string{"with sharing"}}); err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(program, "SharingProbe"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Local-only safety regression: read visibility must not become write authority.
func TestReadSharingDoesNotGrantRecordWrites(t *testing.T) {
	for _, tc := range []struct {
		name, model string
		share       bool
	}{
		{"Read", "Read", false}, {"ReadOnly", "ReadOnly", false},
		{"PublicRead", "PublicRead", false}, {"PublicReadOnly", "PublicReadOnly", false},
		{"manualReadShare", "Private", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Account")
			account := org.Objects["Account"]
			account.Definition.SharingModel = tc.model
			id := storage.ID("001000000000099")
			account.Records = map[storage.ID]storage.Record{id: {ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Unchanged")}, System: storage.SystemFields{OwnerID: "005000000000002"}}}
			org.Objects["Account"] = account
			if tc.share {
				storage.EnsureStandardObject(&org, "AccountShare")
				shares := org.Objects["AccountShare"]
				shareID := storage.ID("00r000000000001")
				shares.Records = map[storage.ID]storage.Record{shareID: {ID: shareID, Object: "AccountShare", Fields: map[string]storage.Value{"AccountId": storage.IDValue(id), "UserOrGroupId": storage.IDValue("005000000000001"), "AccountAccessLevel": storage.StringValue("Read")}}}
				org.Objects["AccountShare"] = shares
			}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
			if err := machine.RegisterClass(Class{Name: "SharingProbe", APIVersion: "53.0", Modifiers: []string{"with sharing"}}); err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymousWithOptions(`
    Id recordId='001000000000099';
    System.assertEquals(1,[SELECT COUNT() FROM Account WHERE Id=:recordId]);
    UserRecordAccess access=[SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess FROM UserRecordAccess WHERE UserId=:UserInfo.getUserId() AND RecordId=:recordId];
    System.assertEquals(true,access.HasReadAccess);
    System.assertEquals(false,access.HasEditAccess);
    System.assertEquals(false,access.HasDeleteAccess);
    Database.SaveResult updated=Database.update(new Account(Id=recordId,Name='Forbidden'),false);
    System.assertEquals(false,updated.isSuccess());
    Database.DeleteResult removed=Database.delete(new Account(Id=recordId),false);
    System.assertEquals(false,removed.isSuccess());
    System.assertEquals('Unchanged',[SELECT Name FROM Account WHERE Id=:recordId].Name);
   `, CompileOptions{APIVersion: "53.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(program, "SharingProbe"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Owned API 62/67 observations retain a denied access row while an Account is
// deleted, then restore its read and delete grants after undelete.
func TestDeletedRecordAccessNativeLifecycle(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Account target = new Account(Name='Owned access lifecycle');
insert target;
Id recordId = target.Id;
Id actorId = UserInfo.getUserId();
UserRecordAccess before = [SELECT RecordId,HasReadAccess,HasDeleteAccess FROM UserRecordAccess WHERE RecordId=:recordId AND UserId=:actorId];
System.assertEquals(recordId,before.RecordId);
System.assertEquals(true,before.HasReadAccess);
System.assertEquals(true,before.HasDeleteAccess);
delete target;
UserRecordAccess removed = [SELECT RecordId,HasReadAccess,HasEditAccess,HasDeleteAccess,HasTransferAccess,HasAllAccess,MaxAccessLevel FROM UserRecordAccess WHERE RecordId=:recordId AND UserId=:actorId];
System.assertEquals(recordId,removed.RecordId);
System.assertEquals(false,removed.HasReadAccess);
System.assertEquals(false,removed.HasEditAccess);
System.assertEquals(false,removed.HasDeleteAccess);
System.assertEquals(false,removed.HasTransferAccess);
System.assertEquals(false,removed.HasAllAccess);
System.assertEquals('None',removed.MaxAccessLevel);
System.assertEquals(0,[SELECT COUNT() FROM Account WHERE Id=:recordId]);
undelete target;
UserRecordAccess restored = [SELECT RecordId,HasReadAccess,HasDeleteAccess FROM UserRecordAccess WHERE RecordId=:recordId AND UserId=:actorId];
System.assertEquals(recordId,restored.RecordId);
System.assertEquals(true,restored.HasReadAccess);
System.assertEquals(true,restored.HasDeleteAccess);
System.assertEquals(1,[SELECT COUNT() FROM Account WHERE Id=:recordId]);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			org.APIVersion = api
			storage.EnsureStandardObject(&org, "Account")
			storage.EnsureStandardObject(&org, "UserRecordAccess")
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(storage.Record{ID: storage.DefaultUserID, Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(storage.DefaultUserID)}})
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

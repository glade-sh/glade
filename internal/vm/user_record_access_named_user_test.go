package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A UserRecordAccess query naming another user by Id synthesizes that user's
// rows only for the portal user types portal_sharing_timing_c4 captures
// (CspLitePortal, PowerCustomerSuccess). An internal user named the same way
// keeps the earlier result: no rows.
func TestUserRecordAccessNamedUserRowsOnlyForCapturedPortalUsers(t *testing.T) {
	const internalUser = "005000000000001"
	const otherInternalUser = "005000000000003"
	const portalUser = "005000000000002"
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'One');
insert a;
String actor = String.valueOf(UserInfo.getUserId());
String internalId = actor.startsWith('` + internalUser + `') ? '` + otherInternalUser + `' : '` + internalUser + `';
Integer internalRows = Database.query('SELECT RecordId FROM UserRecordAccess WHERE UserId = \'' + internalId + '\' AND RecordId = \'' + a.Id + '\'').size();
Integer portalRows = Database.query('SELECT RecordId FROM UserRecordAccess WHERE UserId = \'` + portalUser + `\' AND RecordId = \'' + a.Id + '\'').size();
System.assertEquals(0, internalRows, 'internal user ' + internalId + ' actor ' + actor);
System.assertEquals(1, portalRows, 'portal user');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "UserRecordAccess")
	storage.EnsureStandardObject(&org, "User")
	users := org.Objects["User"]
	if users.Records == nil {
		users.Records = make(map[storage.ID]storage.Record)
	}
	users.Records[internalUser] = storage.Record{ID: internalUser, Object: "User", Fields: map[string]storage.Value{"UserType": storage.StringValue("Standard"), "IsActive": storage.BooleanValue(true)}}
	users.Records[otherInternalUser] = storage.Record{ID: otherInternalUser, Object: "User", Fields: map[string]storage.Value{"UserType": storage.StringValue("Standard"), "IsActive": storage.BooleanValue(true)}}
	users.Records[portalUser] = storage.Record{ID: portalUser, Object: "User", Fields: map[string]storage.Value{"UserType": storage.StringValue("CspLitePortal"), "IsActive": storage.BooleanValue(true)}}
	org.Objects["User"] = users
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

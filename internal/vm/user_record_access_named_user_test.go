package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A UserRecordAccess query naming another user by Id synthesizes that user's
// rows only for the portal user types portal_sharing_timing_c4 captures
// (CspLitePortal, PowerCustomerSuccess) and for Standard users
// (external_user_case_lead_sharing default/org.tsv:8-9, APIs 62.0 and 67.0).
// Any other user type named the same way keeps the earlier result: no rows.
func TestUserRecordAccessNamedUserRowsOnlyForCapturedUserTypes(t *testing.T) {
	const internalUser = "005000000000001"
	const otherInternalUser = "005000000000003"
	const portalUser = "005000000000002"
	const automatedUser = "005000000000004"
	program, err := CompileAnonymous(`
Account a = new Account(Name = 'One');
insert a;
String actor = String.valueOf(UserInfo.getUserId());
String internalId = actor.startsWith('` + internalUser + `') ? '` + otherInternalUser + `' : '` + internalUser + `';
Integer internalRows = Database.query('SELECT RecordId FROM UserRecordAccess WHERE UserId = \'' + internalId + '\' AND RecordId = \'' + a.Id + '\'').size();
Integer portalRows = Database.query('SELECT RecordId FROM UserRecordAccess WHERE UserId = \'` + portalUser + `\' AND RecordId = \'' + a.Id + '\'').size();
Integer automatedRows = Database.query('SELECT RecordId FROM UserRecordAccess WHERE UserId = \'` + automatedUser + `\' AND RecordId = \'' + a.Id + '\'').size();
System.assertEquals(1, internalRows, 'internal user ' + internalId + ' actor ' + actor);
System.assertEquals(0, automatedRows, 'automated user');
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
	users.Records[automatedUser] = storage.Record{ID: automatedUser, Object: "User", Fields: map[string]storage.Value{"UserType": storage.StringValue("AutomatedProcess"), "IsActive": storage.BooleanValue(true)}}
	org.Objects["User"] = users
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The success cases preserve the behavior of main; only the missing sender in
// a populated catalog is supported by the native absent_org_wide_email rows.
func TestSendEmailOrgWideSenderCatalogBoundary(t *testing.T) {
	const sender = storage.ID("0D2000000000001")
	const other = storage.ID("0D2000000000002")
	for _, tc := range []struct {
		name      string
		id        string
		records   map[storage.ID]storage.Record
		wantError bool
	}{
		{name: "unpopulated", id: string(sender)},
		{name: "existing", id: string(sender), records: map[storage.ID]storage.Record{sender: {ID: sender}}},
		{name: "deleted", id: string(sender), records: map[storage.ID]storage.Record{sender: {ID: sender, System: storage.SystemFields{IsDeleted: true}}}},
		{name: "missing in populated catalog", id: string(sender), records: map[storage.ID]storage.Record{other: {ID: other}}, wantError: true},
		{name: "other prefix", id: "001000000000001", records: map[storage.ID]storage.Record{other: {ID: other}}},
		{name: "malformed sender", id: "0D2bad", records: map[storage.ID]storage.Record{other: {ID: other}}},
		{name: "bad checksum", id: "0D2000000000001AAA", records: map[storage.ID]storage.Record{other: {ID: other}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "OrgWideEmailAddress")
			object := org.Objects["OrgWideEmailAddress"]
			object.Records = tc.records
			org.Objects["OrgWideEmailAddress"] = object
			assertion := "System.assertEquals(true, result.isSuccess());"
			if tc.wantError {
				assertion = `System.assertEquals(false, result.isSuccess());
System.assertEquals(1, result.getErrors().size());
System.assertEquals('INVALID_CROSS_REFERENCE_KEY', String.valueOf(result.getErrors()[0].getStatusCode()));
System.assertEquals('invalid cross reference id', result.getErrors()[0].getMessage());`
			}
			program, err := CompileAnonymous(fmt.Sprintf(`
Messaging.SingleEmailMessage message = new Messaging.SingleEmailMessage();
message.setToAddresses(new String[]{'recipient@example.invalid'});
message.setSubject('probe'); message.setPlainTextBody('probe');
message.setOrgWideEmailAddressId('%s');
Messaging.SendEmailResult result = Messaging.sendEmail(new Messaging.SingleEmailMessage[]{message}, false)[0];
%s`, tc.id, assertion))
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

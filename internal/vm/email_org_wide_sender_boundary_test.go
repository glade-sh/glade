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
		{name: "missing with only unrelated deleted sender", id: string(sender), records: map[storage.ID]storage.Record{other: {ID: other, System: storage.SystemFields{IsDeleted: true}}}},
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

// Main accepts a missing sender for these overloads. Compare with the same
// message without a sender so each overload keeps that behavior independently.
func TestSendEmailOrgWideSenderUncapturedOverloadsMatchMain(t *testing.T) {
	for _, tc := range []struct {
		name  string
		extra []Value
	}{
		{name: "default"},
		{name: "all or nothing", extra: []Value{Bool(true)}},
		{name: "send options", extra: []Value{Object("Messaging.SendEmailOptions")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			send := func(sender string) (Value, error) {
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "OrgWideEmailAddress")
				object := org.Objects["OrgWideEmailAddress"]
				const other = storage.ID("0D2000000000002")
				object.Records = map[storage.ID]storage.Record{other: {ID: other}}
				org.Objects["OrgWideEmailAddress"] = object
				machine := New(nil)
				machine.SetOrg(&org)
				message := newSingleEmailMessage()
				message.Fields["toAddresses"] = List(String("recipient@example.invalid"))
				message.Fields["subject"] = String("probe")
				message.Fields["plainTextBody"] = String("probe")
				if sender != "" {
					message.Fields["orgWideEmailAddressId"] = String(sender)
				}
				return machine.sendEmail(append([]Value{List(message)}, tc.extra...), &Result{})
			}
			want, err := send("")
			if err != nil {
				t.Fatalf("main control: %v", err)
			}
			got, err := send("0D2000000000001")
			if err != nil {
				t.Fatalf("missing sender differs from main: %v", err)
			}
			if len(want.List) != 1 || !want.List[0].Fields["success"].Bool || len(want.List[0].Fields["errors"].List) != 0 {
				t.Fatal("main control did not succeed without errors")
			}
			if len(got.List) != 1 || !got.List[0].Fields["success"].Bool || len(got.List[0].Fields["errors"].List) != 0 {
				t.Fatal("missing sender differs from main control")
			}
		})
	}
}

// These neighbors had no current-user activity error on the base tree.
func TestCurrentUserActivityUncapturedNeighborsMatchMain(t *testing.T) {
	for _, tc := range []struct {
		name      string
		api       string
		namespace string
		cc        bool
		sender    string
	}{
		{name: "anonymous API 68", api: "68.0", sender: "0D2000000000001"},
		{name: "namespaced", api: "67.0", namespace: "example", sender: "0D2000000000001"},
		{name: "copied recipient", api: "67.0", cc: true, sender: "0D2000000000001"},
		{name: "no sender", api: "67.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Namespace = tc.namespace
			machine := New(nil)
			machine.SetOrg(&org)
			machine.currentMethod = Method{APIVersion: tc.api}
			message := newSingleEmailMessage()
			message.Fields["targetObjectId"] = String(string(storage.DefaultUserID))
			message.Fields["orgWideEmailAddressId"] = String(tc.sender)
			message.Fields["toAddresses"] = List(String("recipient@example.invalid"))
			if tc.cc {
				message.Fields["ccAddresses"] = List(String("copy@example.invalid"))
			}
			if got := machine.currentUserTargetActivityValidation(message, true); got.Kind != ValueNull {
				t.Fatalf("uncaptured neighbor differs from main: %v", got)
			}
		})
	}
}

// The missing-sender rule is not captured at API 68 or in namespaces.
func TestSendEmailSenderUncapturedNeighborsMatchMain(t *testing.T) {
	for _, tc := range []struct{ name, api, namespace string }{
		{"API 68", "68.0", ""},
		{"namespace", "67.0", "example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Namespace = tc.namespace
			storage.EnsureStandardObject(&org, "OrgWideEmailAddress")
			object := org.Objects["OrgWideEmailAddress"]
			const other = storage.ID("0D2000000000002")
			object.Records = map[storage.ID]storage.Record{other: {ID: other}}
			org.Objects["OrgWideEmailAddress"] = object
			machine := New(nil)
			machine.SetOrg(&org)
			machine.currentMethod = Method{APIVersion: tc.api}
			message := newSingleEmailMessage()
			message.Fields["toAddresses"] = List(String("recipient@example.invalid"))
			message.Fields["subject"] = String("probe")
			message.Fields["plainTextBody"] = String("probe")
			message.Fields["orgWideEmailAddressId"] = String("0D2000000000001")
			got, err := machine.sendEmail([]Value{List(message), Bool(false)}, &Result{})
			if err != nil || len(got.List) != 1 || !got.List[0].Fields["success"].Bool {
				t.Fatalf("uncaptured neighbor differs from main: result=%v err=%v", got, err)
			}
		})
	}
}

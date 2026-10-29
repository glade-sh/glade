package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func customNotificationContextVM(testContext bool) *VM {
	machine := New(nil)
	org := storage.NewOrgState()
	for _, objectName := range []string{"User", "CustomNotificationType"} {
		storage.EnsureStandardObject(&org, objectName)
	}
	types := org.Objects["CustomNotificationType"]
	types.Records = map[storage.ID]storage.Record{
		"0ML000000000001": {ID: "0ML000000000001", Object: "CustomNotificationType", Fields: map[string]storage.Value{
			"DeveloperName": storage.StringValue("ProbeNotice"),
		}},
	}
	org.Objects["CustomNotificationType"] = types
	users := org.Objects["User"]
	users.Records = map[storage.ID]storage.Record{
		"005000000000001": {ID: "005000000000001", Object: "User"},
	}
	org.Objects["User"] = users
	machine.SetOrg(&org)
	if testContext {
		machine.EnableTestContext()
	}
	return machine
}

// Named-test sends return without delivering or validating the payload.
// Native observations are retained in collection_and_boundary/org.tsv.
func TestExecCustomNotificationConfiguredSendInTestContext(t *testing.T) {
	program, err := CompileAnonymous(`
Messaging.CustomNotification notice = new Messaging.CustomNotification();
notice.setTitle('Probe title');
notice.setBody('Probe body');
notice.setTargetId(UserInfo.getUserId());
notice.setNotificationTypeId([SELECT Id FROM CustomNotificationType WHERE DeveloperName='ProbeNotice' LIMIT 1].Id);
notice.send(new Set<String>{UserInfo.getUserId()});
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := customNotificationContextVM(true)
	machine.SetTraceEnabled(true)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range result.Trace {
		if event.Name == "apex.notification.custom.send" {
			t.Fatal("suppressed test send emitted a hosted delivery trace")
		}
	}
}

func configuredTestNotification() Value {
	n := newCustomNotification(nil)
	for field, text := range map[string]string{"notificationTypeId": "0ML000000000001", "targetId": "001000000000001", "title": "Probe title", "body": "Probe body"} {
		n.Fields[field] = String(text)
	}
	return n
}

func TestCustomNotificationTestSendSuppressesPayloadValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Value, *Value)
	}{
		{"missingType", func(n, _ *Value) { n.Fields["notificationTypeId"] = Null }},
		{"emptyType", func(n, _ *Value) { n.Fields["notificationTypeId"] = String("") }},
		{"invalidType", func(n, _ *Value) { n.Fields["notificationTypeId"] = String("bad-id") }},
		{"missingTitle", func(n, _ *Value) { n.Fields["title"] = Null }},
		{"emptyTitle", func(n, _ *Value) { n.Fields["title"] = String("") }},
		{"longTitle", func(n, _ *Value) { n.Fields["title"] = String(strings.Repeat("t", 251)) }},
		{"missingBody", func(n, _ *Value) { n.Fields["body"] = Null }},
		{"emptyBody", func(n, _ *Value) { n.Fields["body"] = String("") }},
		{"longBody", func(n, _ *Value) { n.Fields["body"] = String(strings.Repeat("b", 751)) }},
		{"missingTarget", func(n, _ *Value) { n.Fields["targetId"] = Null }},
		{"emptyTarget", func(n, _ *Value) { n.Fields["targetId"] = String("") }},
		{"invalidTarget", func(n, _ *Value) { n.Fields["targetId"] = String("bad-id") }},
		{"invalidPageTarget", func(n, _ *Value) {
			n.Fields["targetId"] = Null
			n.Fields["targetPageRef"] = String("not-json")
		}},
		{"nullRecipients", func(_ *Value, r *Value) { *r = Null }},
		{"emptyRecipients", func(_ *Value, r *Value) { *r = Set() }},
		{"tooManyRecipients", func(_ *Value, r *Value) {
			for i := 2; i <= 501; i++ {
				r.Set = append(r.Set, String(fmt.Sprintf("005%012d", i)))
			}
		}},
		{"nullRecipient", func(_ *Value, r *Value) { *r = Set(Null) }},
		{"invalidRecipientChecksum", func(_ *Value, r *Value) { *r = Set(String("005000000000001AAB")) }},
		{"invalidSender", func(n, _ *Value) { n.Fields["senderId"] = String("bad-id") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := customNotificationContextVM(true)
			n := configuredTestNotification()
			recipients := Set(String("005000000000001"))
			tc.change(&n, &recipients)
			_, _, _, handled, err := machine.callCustomNotificationMember(n, "send", []Value{recipients}, &Result{})
			if !handled || err != nil {
				t.Fatalf("test send was not suppressed: handled=%t, err=%v", handled, err)
			}
		})
	}
}

func TestCustomNotificationTestSendAcceptsTargetsAndRecipientLimit(t *testing.T) {
	for _, pageTarget := range []bool{false, true} {
		t.Run(fmt.Sprintf("page=%t", pageTarget), func(t *testing.T) {
			machine := New(nil)
			machine.EnableTestContext()
			n := configuredTestNotification()
			n.Fields["title"] = String(strings.Repeat("t", 250))
			n.Fields["body"] = String(strings.Repeat("b", 750))
			if pageTarget {
				n.Fields["targetId"] = Null
				n.Fields["targetPageRef"] = String(`{"type":"standard__namedPage","attributes":{"pageName":"home"}}`)
			}
			recipients := Set()
			for i := 1; i <= 500; i++ {
				recipients.Set = append(recipients.Set, String(fmt.Sprintf("005%012d", i)))
			}
			_, _, _, handled, err := machine.callCustomNotificationMember(n, "send", []Value{recipients}, &Result{})
			if !handled || err != nil {
				t.Fatalf("valid test send rejected: handled=%t, err=%v", handled, err)
			}
		})
	}
}

func TestCustomNotificationNonTestSendKeepsHostedBoundary(t *testing.T) {
	machine := customNotificationContextVM(false)
	_, _, _, handled, err := machine.callCustomNotificationMember(configuredTestNotification(), "send", []Value{Set(String("005000000000001"))}, &Result{})
	if !handled || err == nil || !strings.Contains(err.Error(), "requires hosted notification delivery") {
		t.Fatalf("non-test boundary: handled=%t, err=%v", handled, err)
	}
}

// An offline schema can describe this standard object without supplying a
// notification-type catalog. Keep that existing hosted-service boundary;
// native configured delivery is recorded by hosted_boundaries/org.tsv.
func TestCustomNotificationUncataloguedHostedBoundary(t *testing.T) {
	for _, schemaOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("schemaOnly=%t", schemaOnly), func(t *testing.T) {
			machine := New(nil)
			if schemaOnly {
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "CustomNotificationType")
				machine.SetOrg(&org)
			}
			_, _, _, handled, err := machine.callCustomNotificationMember(configuredTestNotification(), "send", []Value{Set(String("005000000000001"))}, &Result{})
			if !handled || err == nil || !strings.Contains(err.Error(), "requires hosted notification delivery") {
				t.Fatalf("uncatalogued boundary: handled=%t, err=%v", handled, err)
			}
		})
	}
}

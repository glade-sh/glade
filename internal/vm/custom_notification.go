package vm

import (
	"encoding/json"
	"fmt"

	"github.com/glade-sh/glade/internal/storage"
)

// Non-test sends validate the captured input boundaries before reaching the
// local hosted-delivery boundary. Named tests bypass this validation entirely.
//
// Native precedence (case289_followups pairs, API 62.0 and 67.0), first error
// wins: missing customNotifTypeId, null recipients, missing title, missing
// body; then invalid senderId, the recipient set (empty, over the limit, invalid
// Id), targetId, targetPageRef or a missing navigation target, and last an
// invalid customNotifTypeId.
func (vm *VM) validateCustomNotificationSend(notification Value, args []Value) error {
	missing := func(name string) error {
		return newExceptionError("HandledException", "Missing required input parameter: "+name)
	}
	invalid := func(name string) error {
		return newExceptionError("HandledException", "Invalid parameter value for: "+name)
	}
	empty := func(value Value) bool {
		return value.Kind == ValueNull || value.Kind == ValueString && value.Text == ""
	}
	typeID := notification.Fields["notificationTypeId"]
	if empty(typeID) {
		return missing("customNotifTypeId")
	}
	if len(args) != 1 {
		return fmt.Errorf("Messaging.CustomNotification.send expects Set<String>")
	}
	recipients := args[0]
	if recipients.Kind == ValueNull {
		return missing("recipientIds")
	}
	if recipients.Kind != ValueSet {
		return fmt.Errorf("Messaging.CustomNotification.send expects Set<String>")
	}
	for _, field := range []string{"title", "body"} {
		if empty(notification.Fields[field]) {
			return missing(field)
		}
	}
	if sender := notification.Fields["senderId"]; !empty(sender) {
		if sender.Kind != ValueString || validateApexID(sender.Text) != nil {
			return invalid("senderId")
		}
	}
	if len(recipients.Set) == 0 {
		return newExceptionError("HandledException", "Missing parameter: recipientIds")
	}
	if len(recipients.Set) > 500 {
		return newExceptionError("HandledException", "Limit exceeded. Number of recipientIds must be 500 or less.")
	}
	for _, recipient := range recipients.Set {
		if recipient.Kind != ValueString || validateApexID(recipient.Text) != nil {
			return invalid("recipientIds")
		}
	}
	target := notification.Fields["targetId"]
	if !empty(target) && (target.Kind != ValueString || validateApexID(target.Text) != nil) {
		return invalid("targetId")
	}
	// A supplied page reference is checked even when a target Id is present.
	// It must be a JSON object that declares both "type" and "attributes".
	if page := notification.Fields["targetPageRef"]; !empty(page) {
		var reference map[string]json.RawMessage
		if page.Kind != ValueString || json.Unmarshal([]byte(page.Text), &reference) != nil {
			return invalid("targetPageRef")
		}
		_, hasType := reference["type"]
		_, hasAttributes := reference["attributes"]
		if !hasType || !hasAttributes {
			return invalid("targetPageRef")
		}
	} else if empty(target) {
		return newExceptionError("HandledException", "You can't send a custom notification without a navigation target. Specify either a Target ID or Target Page Reference on the Send Custom Notification action.")
	}
	if typeID.Kind != ValueString || validateApexIDShape(typeID.Text) != nil {
		return invalid("customNotifTypeId")
	}
	// A supplied metadata catalog can distinguish an unknown, well-formed Id.
	// Without that catalog, keep the explicit local hosted-service boundary.
	// A type declared only in the org is indistinguishable from an unknown Id.
	if vm.Org != nil {
		if name, found := storage.ResolveObjectName(*vm.Org, "CustomNotificationType"); found {
			records := vm.Org.Objects[name].Records
			if len(records) != 0 {
				if _, _, found := storage.LookupRecordByID(records, storage.ID(typeID.Text)); !found {
					return invalid("customNotifTypeId")
				}
			}
		}
	}
	return nil
}

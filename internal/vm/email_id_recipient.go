package vm

import (
	"net/mail"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

// Native single-recipient observations cover the current User and owned stored
// Account, Contact and Lead records. Keep other address and record shapes on
// their existing validation paths.
func (vm *VM) singleEmailIDRecipientValidation(message, address Value) (Value, bool) {
	if vm == nil || vm.Org == nil || address.Kind != ValueString ||
		len(message.Fields["toAddresses"].List) != 1 ||
		len(message.Fields["ccAddresses"].List) != 0 ||
		len(message.Fields["bccAddresses"].List) != 0 ||
		emailFieldString(message, "targetObjectId") != "" {
		return Null, false
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	if !apexversion.AtLeast(version, 62) || apexversion.AtLeast(version, 69) {
		return Null, false
	}
	id := storage.ID(address.Text)
	if storage.ValidateID(id) != nil {
		return Null, false
	}
	objectName, known := vm.sObjectNameForID(address.Text)
	if !known {
		return Null, false
	}
	currentUserID := storage.ID(vm.currentUserInfoField("Id", storage.DefaultUserID))
	if strings.EqualFold(objectName, "User") && storage.IDsEqual(id, currentUserID) {
		return Null, true
	}
	// A User ID identifies the same running user across execution namespaces.
	// Other namespaced record-recipient shapes keep their existing validation.
	if vm.Org.Namespace != "" || vm.currentMethod.Namespace != "" {
		return Null, false
	}
	if objectName != "Account" && objectName != "Contact" && objectName != "Lead" {
		return Null, false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false
	}
	_, record, ok := storage.LookupRecordByID(object.Records, id)
	if !ok || record.System.IsDeleted || !storage.IDsEqual(record.System.OwnerID, currentUserID) {
		return Null, false
	}
	if objectName != "Account" {
		email, present := record.GetField("Email")
		if present && email.Kind == storage.ValueString {
			if _, err := mail.ParseAddress(email.String); err == nil {
				return Null, true
			}
			return Null, false
		}
		if objectName != "Contact" || present && email.Kind != storage.ValueNull {
			return Null, false
		}
	}
	validation := newSendEmailValidationError("ID is invalid or you do not have access to the record.", "INVALID_ID_FIELD", "")
	validation.Fields["fields"] = List(String("toAddresses"), String(address.Text))
	return validation, true
}

func (vm *VM) plainSingleEmailTestObservation(message Value) bool {
	if vm == nil || vm.Org == nil || vm.testContext == nil || message.Type != "Messaging.SingleEmailMessage" ||
		vm.Org.Namespace != "" || vm.currentMethod.Namespace != "" ||
		len(message.Fields["ccAddresses"].List) != 0 || len(message.Fields["bccAddresses"].List) != 0 ||
		emailFieldString(message, "templateId") != "" || emailFieldString(message, "whatId") != "" {
		return false
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	major, known := apexversion.Major(version)
	return known && (major == 62 || major == 67 || major == 68)
}

// A current-User target with the default activity flag has a distinct native
// error from the accepted current-User ID in toAddresses.
func (vm *VM) currentUserTargetActivityValidation(message Value, partialSend bool) Value {
	previouslyCovered := vm.plainSingleEmailTestObservation(message) && len(message.Fields["toAddresses"].List) == 0
	sender := emailFieldString(message, "orgWideEmailAddressId")
	capturedSenderTarget := partialSend && vm.capturedOrgWideEmailContext() &&
		message.Type == "Messaging.SingleEmailMessage" && strings.HasPrefix(sender, "0D2") && validateApexID(sender) == nil &&
		len(message.Fields["toAddresses"].List) == 1 && len(message.Fields["ccAddresses"].List) == 0 && len(message.Fields["bccAddresses"].List) == 0 &&
		emailFieldString(message, "templateId") == "" && emailFieldString(message, "whatId") == ""
	if !previouslyCovered && !capturedSenderTarget {
		return Null
	}
	if !emailFieldBool(message, "saveAsActivity") || !emailFieldBool(message, "treatTargetObjectAsRecipient") {
		return Null
	}
	target := storage.ID(emailFieldString(message, "targetObjectId"))
	current := storage.ID(vm.currentUserInfoField("Id", storage.DefaultUserID))
	if target == "" || !storage.IDsEqual(target, current) {
		return Null
	}
	validation := newSendEmailValidationError("saveAsActivity must be false when sending mail to users.", "INVALID_SAVE_AS_ACTIVITY_FLAG", string(target))
	validation.Fields["fields"] = List(String("saveAsActivity"), String("true"))
	return validation
}

func (vm *VM) capturedOrgWideEmailContext() bool {
	if vm == nil || vm.Org == nil || vm.Org.Namespace != "" || vm.currentMethod.Namespace != "" {
		return false
	}
	version := vm.currentMethod.APIVersion
	if version == "" {
		version = vm.Org.APIVersion
	}
	major, known := apexversion.Major(version)
	return known && major >= 62 && major <= 67
}

// The sender rows cover a single explicit recipient, with an optional current
// user target, and an active-template send to the current user. Other message
// shapes keep their previous validation behavior.
func (vm *VM) capturedOrgWideEmailShape(message Value) bool {
	if message.Type != "Messaging.SingleEmailMessage" ||
		len(message.Fields["ccAddresses"].List) != 0 || len(message.Fields["bccAddresses"].List) != 0 ||
		emailFieldString(message, "whatId") != "" {
		return false
	}
	target := storage.ID(emailFieldString(message, "targetObjectId"))
	template := emailFieldString(message, "templateId")
	toCount := len(message.Fields["toAddresses"].List)
	if toCount == 1 && target == "" {
		return true
	}
	current := storage.ID(vm.currentUserInfoField("Id", storage.DefaultUserID))
	if target == "" || !storage.IDsEqual(target, current) {
		return false
	}
	return (toCount == 1 && template == "") ||
		(toCount == 0 && template != "" && !emailFieldBool(message, "saveAsActivity"))
}

func (vm *VM) singleNullEmailAddressObservation(message, address Value) bool {
	return vm.plainSingleEmailTestObservation(message) && address.Kind == ValueNull &&
		len(message.Fields["toAddresses"].List) == 1 && emailFieldString(message, "targetObjectId") == ""
}

package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// email_message_identifier.json C126EMAIL_001 observes the running User ID
// recipient at 62/67/68. setToAddresses documents User IDs as valid recipients.
func TestSingleEmailCurrentUserIDNamespace(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, namespace := range []string{"", "sprig"} {
			t.Run(api+"/"+namespace, func(t *testing.T) {
				org := storage.NewOrgState()
				storage.EnsureDeterministicPlatformData(&org)
				org.Namespace = namespace
				machine := New(nil)
				machine.SetOrg(&org)
				machine.currentMethod = Method{APIVersion: api, Namespace: namespace}
				for _, id := range []string{storage.DefaultUserID[:15], storage.DefaultUserID} {
					message := Object("Messaging.SingleEmailMessage")
					message.Fields["toAddresses"] = List(String(id))
					validation, handled := machine.singleEmailIDRecipientValidation(message, String(id))
					if !handled || validation.Kind != ValueNull {
						t.Fatalf("current User recipient: handled=%t validation=%v", handled, validation)
					}
				}
				message := Object("Messaging.SingleEmailMessage")
				message.Fields["toAddresses"] = List(String("synthetic@example.invalid"))
				if _, handled := machine.singleEmailIDRecipientValidation(message, String("synthetic@example.invalid")); handled {
					t.Fatal("an email address must keep the ordinary validation path")
				}
				message.Fields["toAddresses"] = List(String(storage.DefaultUserID), String(storage.DefaultUserID))
				if _, handled := machine.singleEmailIDRecipientValidation(message, String(storage.DefaultUserID)); handled {
					t.Fatal("multiple recipients must keep the existing validation path")
				}
			})
		}
	}
}

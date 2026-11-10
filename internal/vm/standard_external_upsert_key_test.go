package vm

import (
	"io"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The standard-object external-key grant rule covers the unique external IDs
// the native rows observed; other keys keep the pre-existing path.
func TestStandardExternalUpsertKeyRequiresUniqueExternalID(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	contact := org.Objects["Contact"]
	contact.Definition.Fields["Unique__c"] = storage.Field{APIName: "Unique__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	contact.Definition.Fields["Plain__c"] = storage.Field{APIName: "Plain__c", Type: storage.FieldString, ExternalID: true}
	contact.Definition.Fields["NotKey__c"] = storage.Field{APIName: "NotKey__c", Type: storage.FieldString, Unique: true}
	org.Objects["Contact"] = contact
	machine := New(io.Discard)
	machine.SetOrg(&org)
	for field, want := range map[string]bool{"Unique__c": true, "Plain__c": false, "NotKey__c": false, "Email": false} {
		if got := machine.standardExternalUpsertKey("Contact", field); got != want {
			t.Errorf("%s: got %v want %v", field, got, want)
		}
	}
}

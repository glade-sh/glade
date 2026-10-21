package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestContactPublicParentReadSharing(t *testing.T) {
	for _, model := range []string{"", "ControlledByParent"} {
		t.Run(model, func(t *testing.T) {
			machine, contact := contactParentReadFixture(t)
			state := machine.Org.Objects[contact.Object]
			state.Definition.SharingModel = model
			machine.Org.Objects[contact.Object] = state
			if !machine.userModeRecordVisible(contact.Object, contact, machine.currentUserID()) {
				t.Fatal("contact is hidden despite its public account")
			}
			projected := contact
			projected.Fields = map[string]storage.Value{}
			if !machine.userModeRecordVisible(projected.Object, projected, machine.currentUserID()) {
				t.Fatal("omitting AccountId from SELECT changed contact visibility")
			}
			if machine.soqlObjectHasPublicReadSharing(contact.Object) {
				t.Fatal("contact visibility became an unconditional object grant")
			}
		})
	}
}

func TestContactPublicParentReadSharingBoundaries(t *testing.T) {
	for _, name := range []string{"private contact", "private account", "missing account", "deleted account", "no account"} {
		t.Run(name, func(t *testing.T) {
			machine, contact := contactParentReadFixture(t)
			state := machine.Org.Objects["Contact"]
			switch name {
			case "private contact":
				state.Definition.SharingModel = "Private"
				machine.Org.Objects["Contact"] = state
			case "private account":
				account := machine.Org.Objects["Account"]
				account.Definition.SharingModel = "Private"
				machine.Org.Objects["Account"] = account
			case "missing account":
				delete(machine.Org.Objects["Account"].Records, "001000000000001")
			case "deleted account":
				account := machine.Org.Objects["Account"].Records["001000000000001"]
				account.System.IsDeleted = true
				machine.Org.Objects["Account"].Records[account.ID] = account
			case "no account":
				delete(contact.Fields, "AccountId")
			}
			machine.Org.Objects["Contact"].Records[contact.ID] = contact
			if machine.userModeRecordVisible(contact.Object, contact, machine.currentUserID()) {
				t.Fatal("contact without a public controlling account became visible")
			}
			if !machine.userModeRecordVisible(contact.Object, contact, string(contact.System.OwnerID)) {
				t.Fatal("existing contact-owner visibility changed")
			}
		})
	}
}

func contactParentReadFixture(t *testing.T) (*VM, storage.Record) {
	t.Helper()
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	account := storage.Record{ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: "005000000000001"}}
	contact := storage.Record{ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(account.ID)}, System: storage.SystemFields{OwnerID: account.System.OwnerID}}
	org.Objects["Account"].Records[account.ID] = account
	org.Objects["Contact"].Records[contact.ID] = contact
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000002", Object: "User"})
	if machine.currentUserBypassesRecordSharing() || machine.currentUserObjectRecordGrant("Contact", "PermissionsViewAllRecords", "PermissionsModifyAllRecords") {
		t.Fatal("fixture actor bypasses sharing")
	}
	return machine, contact
}

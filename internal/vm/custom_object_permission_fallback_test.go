package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestStandardUserCustomObjectPermissionFallback(t *testing.T) {
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	org.Objects["TriadParcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "TriadParcel__c", Fields: map[string]storage.Field{
			"ParcelKey__c": {APIName: "ParcelKey__c", Type: storage.FieldString},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	for id, profile := range org.Objects["Profile"].Records {
		name, _ := profile.GetField("Name")
		if name.String != "System Administrator" {
			continue
		}
		user := Object("User")
		user.Fields["Id"] = String("005000000000099")
		user.Fields["ProfileId"] = String(string(id))
		machine.executionUser = user
		want := name.String == "System Administrator"
		for _, method := range []string{"isAccessible", "isCreateable", "isUpdateable", "isDeletable"} {
			if got := machine.currentUserObjectPermission("TriadParcel__c", method); got != want {
				t.Fatalf("%s %s = %v, want %v", name.String, method, got, want)
			}
		}
	}
}

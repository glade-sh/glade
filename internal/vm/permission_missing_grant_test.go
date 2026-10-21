package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Owned user_field_permission_fallback profile-only observations deny ordinary
// custom-object CRUD. AccountBrand retains its separate existing local control.
func TestStandardUserMissingCustomAndFeatureObjectGrants(t *testing.T) {
	o := testDataOrg()
	storage.EnsureStandardObject(&o, "AccountBrand")
	o.Objects["TriadParcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "TriadParcel__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}}
	m := New(nil)
	m.SetOrg(&o)
	u := Object("User")
	u.Fields["Id"] = String("005000000000002")
	pr := Object("Profile")
	pr.Fields["Name"] = String("Standard User")
	u.Fields["Profile"] = pr
	m.executionUser = u
	for _, name := range []string{"TriadParcel__c", "AccountBrand"} {
		for _, method := range []string{"isAccessible", "isUpdateable", "isDeletable"} {
			want := false
			if got := m.currentUserObjectPermission(name, method); got != want {
				t.Errorf("ungranted %s %s = %v, want %v", name, method, got, want)
			}
		}
	}
	pr.Fields["Name"] = String("System Administrator")
	if !m.currentUserObjectPermission("TriadParcel__c", "isDeletable") {
		t.Fatal("administrator changed")
	}
}

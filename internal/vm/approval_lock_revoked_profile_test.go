package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestApprovalLockedMutationRejectsRevokedProfileGrant(t *testing.T) {
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "User")
	storage.EnsureStandardObject(&org, "Profile")
	uid := storage.ID("005000000000991AAA")
	pid := storage.ID("00e000000000991AAA")
	user := storage.Record{ID: uid, Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(uid), "ProfileId": storage.IDValue(pid)}}
	users := org.Objects["User"]
	users.Records[uid] = user
	org.Objects["User"] = users
	profiles := org.Objects["Profile"]
	profiles.Records[pid] = storage.Record{ID: pid, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator"), "PermissionsModifyAllData": storage.BooleanValue(false)}}
	org.Objects["Profile"] = profiles
	engine := dml.NewEngine(&org)
	created := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}}})
	if !created[0].Success {
		t.Fatal(created)
	}
	id := created[0].ID
	if result := engine.Lock([]storage.Record{{Object: "Account", ID: id}}); !result[0].Success {
		t.Fatal(result)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(user)
	local := machine.newDMLEngine(nil)
	result := local.Update([]storage.Record{{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Changed")}}})
	if result[0].Success || result[0].StatusCode != "ENTITY_IS_LOCKED" {
		t.Fatalf("revoked profile result=%+v", result[0])
	}
}

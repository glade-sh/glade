package dml

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// Each row is a native update with global and object grants. The synthetic setup has
// the same effective assignment and group states as the capture.
func TestApprovalLockedGlobalGrantNativeRows(t *testing.T) {
	for _, api := range []string{"api62", "api67"} {
		for _, route := range []string{"anonymous", "istest"} {
			path := filepath.Join("../apextest/testdata/conformance/approval_global_grant", api, route, "org.tsv")
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			scanner := bufio.NewScanner(f)
			scanner.Scan()
			for scanner.Scan() {
				parts := strings.SplitN(scanner.Text(), "\t", 2)
				if len(parts) != 2 {
					t.Fatalf("bad row: %s", scanner.Text())
				}
				state, want := parts[0], strings.Trim(parts[1], "\"")
				t.Run(api+"/"+route+"/"+state, func(t *testing.T) {
					org := testOrg()
					for _, name := range []string{"User", "Profile", "PermissionSet", "PermissionSetAssignment", "PermissionSetGroupComponent", "MutingPermissionSet", "ObjectPermissions"} {
						storage.EnsureStandardObject(&org, name)
					}
					uid := storage.ID("005000000000991AAA")
					pid := storage.ID("00e000000000991AAA")
					group := storage.ID("0PG000000000991AAA")
					set := storage.ID("0PS000000000991AAA")
					mute := storage.ID("0PS000000000992AAA")
					direct := storage.ID("0PS000000000993AAA")
					users := org.Objects["User"]
					users.Records[uid] = storage.Record{ID: uid, Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue(pid)}}
					org.Objects["User"] = users
					profiles := org.Objects["Profile"]
					profiles.Records[pid] = storage.Record{ID: pid, Object: "Profile"}
					org.Objects["Profile"] = profiles
					sets := org.Objects["PermissionSet"]
					sets.Records[set] = storage.Record{ID: set, Object: "PermissionSet", Fields: map[string]storage.Value{"PermissionsModifyAllData": storage.BooleanValue(true)}}
					sets.Records[direct] = storage.Record{ID: direct, Object: "PermissionSet", Fields: map[string]storage.Value{"PermissionsModifyAllData": storage.BooleanValue(true)}}
					org.Objects["PermissionSet"] = sets
					mutings := org.Objects["MutingPermissionSet"]
					mutings.Records[mute] = storage.Record{ID: mute, Object: "MutingPermissionSet", Fields: map[string]storage.Value{"PermissionsModifyAllData": storage.BooleanValue(state != "object_muted")}}
					org.Objects["MutingPermissionSet"] = mutings
					perms := org.Objects["ObjectPermissions"]
					for i, parent := range []storage.ID{set, mute, direct} {
						if parent == mute && state == "global_muted" {
							continue
						}
						id := storage.ID([]string{"110000000000991AAA", "110000000000992AAA", "110000000000993AAA"}[i])
						perms.Records[id] = storage.Record{ID: id, Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent), "SObjectType": storage.StringValue("Account"), "PermissionsModifyAllRecords": storage.BooleanValue(true)}}
					}
					org.Objects["ObjectPermissions"] = perms
					components := org.Objects["PermissionSetGroupComponent"]
					c1 := storage.ID("0PC000000000991AAA")
					components.Records[c1] = storage.Record{ID: c1, Object: "PermissionSetGroupComponent", Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue(group), "PermissionSetId": storage.IDValue(set)}}
					{
						c2 := storage.ID("0PC000000000992AAA")
						components.Records[c2] = storage.Record{ID: c2, Object: "PermissionSetGroupComponent", Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue(group), "PermissionSetId": storage.IDValue(mute)}}
					}
					org.Objects["PermissionSetGroupComponent"] = components
					assigns := org.Objects["PermissionSetAssignment"]
					a1 := storage.ID("0Pa000000000991AAA")
					assigns.Records[a1] = storage.Record{ID: a1, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(uid), "PermissionSetGroupId": storage.IDValue(group), "IsActive": storage.BooleanValue(false)}}
					if state == "direct_global" || state == "revoked_global" {
						a2 := storage.ID("0Pa000000000992AAA")
						assigns.Records[a2] = storage.Record{ID: a2, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(uid), "PermissionSetId": storage.IDValue(direct), "IsRevoked": storage.BooleanValue(state == "revoked_global")}}
					}
					org.Objects["PermissionSetAssignment"] = assigns
					engine := NewEngine(&org)
					engine.Now = func() time.Time { return time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC) }
					created := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}}})
					if !created[0].Success {
						t.Fatal(created)
					}
					id := created[0].ID
					if result := engine.Lock([]storage.Record{{Object: "Account", ID: id}}); !result[0].Success {
						t.Fatal(result)
					}
					engine.UserID = uid
					result := engine.Update([]storage.Record{{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Changed")}}})[0]
					got := "false:" + result.StatusCode
					if result.Success {
						got = "true:NONE"
					}
					if got != want {
						t.Fatalf("got %s want %s", got, want)
					}
				})
			}
			if err := scanner.Err(); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}

// User has no Modify All Data field in the captured schema, so a stray field on
// a user row must not unlock records. Only profile and permission-set grants,
// captured in the lock mutation and global grant rows, may do that.
func TestApprovalLockedUserFieldIsNotGrant(t *testing.T) {
	org := testOrg()
	for _, name := range []string{"User", "Profile"} {
		storage.EnsureStandardObject(&org, name)
	}
	uid := storage.ID("005000000000991AAA")
	pid := storage.ID("00e000000000991AAA")
	users := org.Objects["User"]
	users.Records[uid] = storage.Record{ID: uid, Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue(pid), "PermissionsModifyAllData": storage.BooleanValue(true)}}
	org.Objects["User"] = users
	profiles := org.Objects["Profile"]
	profiles.Records[pid] = storage.Record{ID: pid, Object: "Profile"}
	org.Objects["Profile"] = profiles
	engine := NewEngine(&org)
	created := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}}})
	if !created[0].Success {
		t.Fatal(created)
	}
	id := created[0].ID
	if result := engine.Lock([]storage.Record{{Object: "Account", ID: id}}); !result[0].Success {
		t.Fatal(result)
	}
	engine.UserID = uid
	result := engine.Update([]storage.Record{{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Changed")}}})[0]
	if result.Success || result.StatusCode != "ENTITY_IS_LOCKED" {
		t.Fatalf("got success=%v status=%s", result.Success, result.StatusCode)
	}
}

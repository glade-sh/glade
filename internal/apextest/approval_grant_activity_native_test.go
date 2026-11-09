package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Replay the captured anonymous program or captured test method for every
// effective-grant state, rather than replacing the Apex route with Go DML.
func TestApprovalGrantActivityNativeConformance(t *testing.T) {
	fixture := "testdata/conformance/approval_grant_activity"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				sources := map[string]string{}
				if err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, e fs.DirEntry, err error) error {
					if err != nil || e.IsDir() {
						return err
					}
					rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
					if err != nil {
						return err
					}
					b, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					if strings.HasSuffix(rel, "-meta.xml") {
						b = []byte(strings.ReplaceAll(string(b), "62.0", api))
					}
					sources[rel] = string(b)
					writeFile(t, filepath.Join(root, rel), string(b))
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				capture := filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), route)
				run, err := os.ReadFile(filepath.Join(capture, "RUN-FACTS.txt"))
				if err != nil {
					t.Fatal(err)
				}
				for _, fact := range []string{"api=" + api, "context=" + route, "principal=standard", "rows=4", "native_complete=true", "deployment_success=true"} {
					if !strings.Contains("\n"+string(run), "\n"+fact+"\n") {
						t.Fatalf("missing %s", fact)
					}
				}
				for name, src := range sources {
					if !strings.Contains(string(run), fmt.Sprintf("source_sha256:%s=%x\n", name, sha256.Sum256([]byte(src)))) {
						t.Fatalf("source changed %s", name)
					}
				}
				data, err := os.ReadFile(filepath.Join(capture, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				lines := strings.Split(strings.TrimSpace(string(data)), "\n")
				if len(lines) != 5 {
					t.Fatalf("invalid row count %d", len(lines)-1)
				}
				for _, line := range lines[1:] {
					name, raw, ok := strings.Cut(line, "\t")
					if !ok {
						t.Fatal(line)
					}
					var want string
					if err := json.Unmarshal([]byte(raw), &want); err != nil {
						t.Fatal(err)
					}
					t.Run(name, func(t *testing.T) {
						org := orgFromIndex(index)
						org.APIVersion = api
						user := approvalGrantActivitySetup(t, &org, name)
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						machine := runner.newMachine()
						source := sources["anonymous.apex"]
						if route == "anonymous" {
							machine.SetCurrentUser(user)
						} else {
							source = "ApprovalGrantCaptureTest.capture();"
							machine.EnableTestContext()
							if err := RegisterTestMethodForRequest(machine, index, "ApprovalGrantCaptureTest", "capture"); err != nil {
								t.Fatal(err)
							}
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						var payload string
						if route == "anonymous" {
							result, err := machine.Execute(program)
							if err != nil {
								t.Fatal(err)
							}
							for _, line := range result.Debug {
								if _, s, ok := strings.Cut(line, "PERMISSION_ROW|"); ok {
									payload = s
									break
								}
							}
						} else {
							_, err := machine.ExecuteInClass(program, "ApprovalGrantCaptureTest")
							var fault *vm.RuntimeError
							if !errors.As(err, &fault) {
								t.Fatalf("missing observation: %v", err)
							}
							_, payload, _ = strings.Cut(fault.ExceptionMessage(), "PERMISSION_ROW|")
						}
						if payload != want {
							t.Fatalf("got %q want %q", payload, want)
						}
					})
				}
			})
		}
	}
}

func approvalGrantActivitySetup(t *testing.T, org *storage.OrgState, state string) storage.Record {
	t.Helper()
	for _, name := range []string{"PermissionSetAssignment", "PermissionSetGroupComponent", "MutingPermissionSet", "ObjectPermissions"} {
		storage.EnsureStandardObject(org, name)
	}
	var profile storage.ID
	for id, p := range org.Objects["Profile"].Records {
		if p.Fields["Name"].String == "Standard User" {
			profile = id
			break
		}
	}
	if profile == "" {
		t.Fatal("standard profile missing")
	}
	uid := storage.ID("005000000000991AAA")
	group := storage.ID("0PG000000000991AAA")
	set := storage.ID("0PS000000000991AAA")
	mute := storage.ID("0PS000000000992AAA")
	direct := storage.ID("0PS000000000993AAA")
	user := storage.Record{ID: uid, Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(uid), "ProfileId": storage.IDValue(profile), "IsActive": storage.BooleanValue(true)}}
	users := org.Objects["User"]
	users.Records[uid] = user
	org.Objects["User"] = users
	parent := profile
	for id, p := range org.Objects["PermissionSet"].Records {
		if p.Fields["IsOwnedByProfile"].Boolean && storage.IDsEqual(p.Fields["ProfileId"].ID, profile) {
			parent = id
			break
		}
	}
	sets := org.Objects["PermissionSet"]
	sets.Records[set] = storage.Record{ID: set, Object: "PermissionSet"}
	sets.Records[direct] = storage.Record{ID: direct, Object: "PermissionSet"}
	org.Objects["PermissionSet"] = sets
	mutings := org.Objects["MutingPermissionSet"]
	mutings.Records[mute] = storage.Record{ID: mute, Object: "MutingPermissionSet"}
	org.Objects["MutingPermissionSet"] = mutings
	perms := org.Objects["ObjectPermissions"]
	for id, row := range perms.Records {
		object, _ := row.GetField("SObjectType")
		if storage.IDsEqual(row.Fields["ParentId"].ID, parent) && strings.EqualFold(object.String, "Account") {
			delete(perms.Records, id)
		}
	}
	for i, id := range []storage.ID{parent, set, mute, direct} {
		rid := storage.ID([]string{"110000000000990AAA", "110000000000991AAA", "110000000000992AAA", "110000000000993AAA"}[i])
		fields := map[string]storage.Value{"ParentId": storage.IDValue(id), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true)}
		if id != parent {
			fields["PermissionsModifyAllRecords"] = storage.BooleanValue(true)
		}
		perms.Records[rid] = storage.Record{ID: rid, Object: "ObjectPermissions", Fields: fields}
	}
	org.Objects["ObjectPermissions"] = perms
	components := org.Objects["PermissionSetGroupComponent"]
	c1 := storage.ID("0PC000000000991AAA")
	components.Records[c1] = storage.Record{ID: c1, Object: "PermissionSetGroupComponent", Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue(group), "PermissionSetId": storage.IDValue(set)}}
	if state != "unmuted_group" {
		c2 := storage.ID("0PC000000000992AAA")
		components.Records[c2] = storage.Record{ID: c2, Object: "PermissionSetGroupComponent", Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue(group), "PermissionSetId": storage.IDValue(mute)}}
	}
	org.Objects["PermissionSetGroupComponent"] = components
	assigns := org.Objects["PermissionSetAssignment"]
	a1 := storage.ID("0Pa000000000991AAA")
	assigns.Records[a1] = storage.Record{ID: a1, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(uid), "PermissionSetGroupId": storage.IDValue(group)}}
	a2 := storage.ID("0Pa000000000992AAA")
	expiration := "2025-01-01T00:00:00Z"
	if state == "active_direct" || state == "revoked_muted" {
		expiration = "2035-01-01T00:00:00Z"
	}
	assigns.Records[a2] = storage.Record{ID: a2, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(uid), "PermissionSetId": storage.IDValue(direct), "ExpirationDate": storage.DateTimeValue(expiration), "IsRevoked": storage.BooleanValue(state == "revoked_muted")}}
	org.Objects["PermissionSetAssignment"] = assigns
	return user
}

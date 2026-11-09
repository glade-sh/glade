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

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestApprovalLockMutationNativeConformance(t *testing.T) {
	fixture := "testdata/conformance/approval_lock_mutation"
	for _, api := range []string{"62.0", "67.0"} {
		for _, role := range []string{"admin", "standard"} {
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(api+"/"+role+"/"+route, func(t *testing.T) {
					root := t.TempDir()
					sources := map[string]string{}
					err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, e fs.DirEntry, err error) error {
						if err != nil || e.IsDir() {
							return err
						}
						rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
						if err != nil {
							return err
						}
						if strings.HasPrefix(rel, ".sf/") || strings.HasPrefix(rel, ".sfdx/") {
							return nil
						}
						b, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						if strings.HasSuffix(rel, "-meta.xml") || rel == "sfdx-project.json" {
							b = []byte(strings.ReplaceAll(string(b), "62.0", api))
						}
						sources[rel] = string(b)
						writeFile(t, filepath.Join(root, rel), string(b))
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					index := loadTestIndex(t, root)
					if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					var standard storage.Record
					if role == "standard" {
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
						// The captured Standard User can create and edit Accounts; give
						// the local profile those same prerequisite object permissions.
						parent := profile
						for id, set := range org.Objects["PermissionSet"].Records {
							if set.Fields["IsOwnedByProfile"].Boolean && storage.IDsEqual(set.Fields["ProfileId"].ID, profile) {
								parent = id
								break
							}
						}
						storage.EnsureStandardObject(&org, "ObjectPermissions")
						perms := org.Objects["ObjectPermissions"]
						for id, permission := range perms.Records {
							objectType, _ := permission.GetField("SObjectType")
							if storage.IDsEqual(permission.Fields["ParentId"].ID, parent) && strings.EqualFold(objectType.String, "Account") {
								delete(perms.Records, id)
							}
						}
						permissionID := storage.ID("110000000000991AAA")
						perms.Records[permissionID] = storage.Record{ID: permissionID, Object: "ObjectPermissions", Fields: map[string]storage.Value{"Id": storage.IDValue(permissionID), "ParentId": storage.IDValue(parent), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true)}}
						org.Objects["ObjectPermissions"] = perms
						if route == "anonymous" {
							standard = storage.Record{ID: "005000000000991AAA", Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue("005000000000991AAA"), "ProfileId": storage.IDValue(profile)}}
							users := org.Objects["User"]
							users.Records[standard.ID] = standard
							org.Objects["User"] = users
						}
					}
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					machine := runner.newMachine()
					if standard.ID != "" {
						machine.SetCurrentUser(standard)
					}
					source := sources["anonymous.apex"]
					method := "captureAdmin"
					if role == "standard" {
						method = "captureStandard"
					}
					if route == "istest" {
						source = "ApprovalMutationCaptureTest." + method + "();"
						machine.EnableTestContext()
						if err := RegisterTestMethodForRequest(machine, index, "ApprovalMutationCaptureTest", method); err != nil {
							t.Fatal(err)
						}
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					var payload string
					if route == "istest" {
						_, err = machine.ExecuteInClass(program, "ApprovalMutationCaptureTest")
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) {
							t.Fatalf("missing observation: %v", err)
						}
						var ok bool
						_, payload, ok = strings.Cut(fault.ExceptionMessage(), "MUTATE_ROWS|")
						if !ok {
							t.Fatalf("missing observation: %v", err)
						}
					} else {
						result, executionError := machine.Execute(program)
						if executionError != nil {
							t.Fatal(executionError)
						}
						for _, line := range result.Debug {
							if _, value, ok := strings.Cut(line, "MUTATE_ROWS|"); ok {
								payload = value
								break
							}
						}
						if payload == "" {
							t.Fatalf("missing observation: %v", result.Debug)
						}
					}
					got := map[string]string{}
					for _, pair := range strings.Split(payload, ";") {
						if pair == "" {
							continue
						}
						key, value, ok := strings.Cut(pair, "=")
						if !ok {
							t.Fatal(pair)
						}
						got[key] = value
					}
					capture := filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), route+"-"+role)
					run, err := os.ReadFile(filepath.Join(capture, "RUN-FACTS.txt"))
					if err != nil {
						t.Fatal(err)
					}
					for _, fact := range []string{"api=" + api, "context=" + route, "principal=" + role, "rows=3", "native_complete=true", "deployment_success=true"} {
						if !strings.Contains("\n"+string(run), "\n"+fact+"\n") {
							t.Fatalf("missing %s", fact)
						}
					}
					for name, src := range sources {
						if !strings.Contains(string(run), fmt.Sprintf("source_sha256:%s=%x\n", name, sha256.Sum256([]byte(src)))) {
							t.Fatalf("source changed %s", name)
						}
					}
					rows, err := os.ReadFile(filepath.Join(capture, "org.tsv"))
					if err != nil {
						t.Fatal(err)
					}
					lines := strings.Split(strings.TrimSpace(string(rows)), "\n")
					if len(lines) != 4 {
						t.Fatalf("row count %d", len(lines)-1)
					}
					for _, line := range lines[1:] {
						key, raw, ok := strings.Cut(line, "\t")
						if !ok {
							t.Fatal(line)
						}
						var want string
						if err := json.Unmarshal([]byte(raw), &want); err != nil {
							t.Fatal(err)
						}
						if got[key] != want {
							t.Errorf("%s got %q want %q", key, got[key], want)
						}
					}
				})
			}
		}
	}
}

package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPermissionMetadataRecordsUseCanonicalFieldNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "OwnedReader.permissionset-meta.xml")
	writeFile(t, path, `<PermissionSet><label>Owned reader</label><objectPermissions><object>Account</object><allowRead>true</allowRead></objectPermissions><fieldPermissions><field>Account.Name</field><readable>true</readable><editable>false</editable></fieldPermissions></PermissionSet>`)
	org := storage.NewOrgState()
	ApplyProjectPermissionSets(&org, project.Project{PermissionSetFiles: []string{path}})
	for _, name := range []string{"ObjectPermissions", "FieldPermissions"} {
		t.Run(name, func(t *testing.T) {
			object := org.Objects[name]
			if len(object.Records) != 1 {
				t.Fatalf("permission record count = %d", len(object.Records))
			}
			for _, record := range object.Records {
				if got := record.Fields["SobjectType"]; got.Kind != storage.ValueString || got.String != "Account" {
					t.Fatalf("canonical object type = %#v", got)
				}
				if _, found := record.Fields["SObjectType"]; found {
					t.Fatal("permission record retains noncanonical field key")
				}
			}
		})
	}
}

func TestBarePermissionFixtureNativeConformance(t *testing.T) {
	var data struct {
		Rows []struct {
			ID, APIVersion, Route, Source, ExpectedCompilation, ExpectedRuntime string
			SourceSHA256                                                        map[string]string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/iterable_assignability.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if row.ID != "H1XT13-A" {
			continue
		}
		t.Run(row.APIVersion, func(t *testing.T) {
			if seen[row.APIVersion] || row.Route != "anonymous" || row.ExpectedCompilation != "compiled" || row.ExpectedRuntime == "" {
				t.Fatal("invalid native permission field row")
			}
			seen[row.APIVersion] = true
			sum := sha256.Sum256([]byte(row.Source))
			if hex.EncodeToString(sum[:]) != row.SourceSHA256["anonymous"] {
				t.Fatal("captured source changed")
			}
			terminal := "System.assert(false, 'P|" + row.ID + "|' + observed);"
			if strings.Count(row.Source, terminal) != 1 {
				t.Fatal("native terminal observation missing")
			}
			source := strings.Replace(row.Source, terminal, "System.assertEquals("+conformanceApexString(row.ExpectedRuntime)+", observed);", 1)
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row.APIVersion})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = row.APIVersion
			storage.EnsureDeterministicPlatformData(&org)
			machine := vm.New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if len(seen) != 2 || !seen["62.0"] || !seen["67.0"] {
		t.Fatal("missing native floor or ceiling")
	}
}

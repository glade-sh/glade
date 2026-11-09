package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Native rows: testdata/conformance/deployed_permissions_defaults/{,enterprise/}org.tsv.
func TestDeployedPermissionSetNativeDefaults(t *testing.T) {
	root := filepath.Join("testdata", "conformance", "deployed_permissions_defaults")
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			captured := map[string]bool{}
			editions := []string{"versions"}
			if api == "62.0" || api == "67.0" {
				editions = append(editions, "", "enterprise", "features", "order_sales")
			}
			for _, edition := range editions {
				raw, err := os.ReadFile(filepath.Join(root, edition, "fields-"+api+".txt"))
				if err != nil {
					t.Fatal(err)
				}
				for _, field := range strings.Fields(string(raw)) {
					captured[strings.ToLower(field)] = true
				}
			}
			p := project.Project{}
			for _, name := range []string{"C27_Empty", "C27_Disabled", "C27_Enabled"} {
				p.PermissionSetFiles = append(p.PermissionSetFiles, filepath.Join(root, "project", name+".permissionset-meta.xml"))
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			ApplyProjectPermissionSets(&org, p)
			ApplyProjectPermissionSets(&org, p) // existing seeded rows retain the same values
			state := org.Objects["PermissionSet"]
			checkedSets := 0
			for _, record := range state.Records {
				name := record.Fields["Name"].String
				if !strings.HasPrefix(name, "C27_") {
					continue
				}
				checkedSets++
				count := 0
				for fieldName, field := range state.Definition.Fields {
					if field.Type != storage.FieldBoolean || !strings.HasPrefix(fieldName, "Permissions") || !captured[strings.ToLower(fieldName)] {
						continue
					}
					count++
					got := record.Fields[fieldName]
					want := name == "C27_Enabled" && (strings.EqualFold(fieldName, "PermissionsEmailSingle") || strings.EqualFold(fieldName, "PermissionsEditTask"))
					if got.Kind != storage.ValueBoolean || got.Boolean != want {
						t.Errorf("%s %s: got %#v, want %t", name, fieldName, got, want)
					}
				}
				if count < 3 {
					t.Fatalf("only %d modeled native fields were checked", count)
				}
			}
			if checkedSets != 3 {
				t.Fatalf("got %d deployed sets, want 3", checkedSets)
			}
		})
	}
}

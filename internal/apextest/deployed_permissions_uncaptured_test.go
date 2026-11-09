package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Preserve the prior absent value for modeled fields not observed in the
// corresponding native describe inventory.
func TestDeployedPermissionSetLeavesUncapturedFieldUnchanged(t *testing.T) {
	root := filepath.Join("testdata", "conformance", "deployed_permissions_defaults")
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			observed := map[string]bool{}
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
					observed[strings.ToLower(field)] = true
				}
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			ApplyProjectPermissionSets(&org, project.Project{PermissionSetFiles: []string{filepath.Join(root, "project", "C27_Empty.permissionset-meta.xml")}})
			state := org.Objects["PermissionSet"]
			for _, record := range state.Records {
				if record.Fields["Name"].String != "C27_Empty" {
					continue
				}
				count := 0
				for name, field := range state.Definition.Fields {
					if field.Type != storage.FieldBoolean || !strings.HasPrefix(name, "Permissions") || observed[strings.ToLower(name)] {
						continue
					}
					count++
					if _, exists := record.GetField(name); exists {
						t.Errorf("uncaptured field %s was given an inferred value", name)
					}
				}
				t.Logf("%d modeled fields not observed at API %s", count, api)
				return
			}
			t.Fatal("deployed permission set missing")
		})
	}
}

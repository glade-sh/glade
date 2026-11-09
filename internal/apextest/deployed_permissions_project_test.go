package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Supplemental host seed coverage for the native deployed-permission rows.
func TestDeployedPermissionDefaultsProjectOrg(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			for _, name := range []string{"C27_Empty", "C27_Enabled"} {
				file := name + ".permissionset-meta.xml"
				body, err := os.ReadFile(filepath.Join("testdata", "conformance", "deployed_permissions_defaults", "project", file))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app", "main", "default", "permissionsets", file), string(body))
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, schema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			org := OrgFromIndex(index)
			found := map[string]bool{}
			for _, record := range org.Objects["PermissionSet"].Records {
				name := record.Fields["Name"].String
				if name != "C27_Empty" && name != "C27_Enabled" {
					continue
				}
				found[name] = true
				for _, field := range []string{"PermissionsEditTask", "PermissionsEmailSingle", "PermissionsModifyAllData"} {
					value := record.Fields[field]
					want := name == "C27_Enabled" && field != "PermissionsModifyAllData"
					if value.Kind != storage.ValueBoolean || value.Boolean != want {
						t.Errorf("%s %s = %#v, want %t", name, field, value, want)
					}
				}
			}
			if len(found) != 2 {
				t.Fatalf("deployed sets found: %v", found)
			}
		})
	}
}

package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// case289_followups org.tsv ST1-ST4 (anonymous, static): ObjectPermissions.
// PermissionsViewAllFields is a compile error at source API 62.0 and compiles
// at 67.0. ST3 carries the native SOQL column message (org.tsv:172).
func TestObjectPermissionsViewAllFieldsStaticAPIGate(t *testing.T) {
	cases := []struct{ id, body, want string }{
		{"ST1", "Schema.SObjectField f=ObjectPermissions.PermissionsViewAllFields;", "Variable does not exist: PermissionsViewAllFields"},
		{"ST2", "ObjectPermissions g=new ObjectPermissions();g.PermissionsViewAllFields=true;", "Variable does not exist: PermissionsViewAllFields"},
		{"ST3", "List<ObjectPermissions> r=[SELECT Id, PermissionsViewAllFields FROM ObjectPermissions LIMIT 1];", "No such column 'PermissionsViewAllFields' on entity 'ObjectPermissions'"},
		{"ST4", "ObjectPermissions g=new ObjectPermissions(PermissionsViewAllFields=true);", "Field does not exist: PermissionsViewAllFields on ObjectPermissions"},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, c := range cases {
			t.Run(api+"/"+c.id, func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, "Empty.cls")
				writeSemaFile(t, file, "public class Empty {}")
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{file}}, schema.Schema{})
				result := AnalyzeAnonymous(index, c.body, api)
				var got string
				for _, d := range result.Diagnostics {
					msg := d.NativeMessage
					if msg == "" {
						msg = d.Message
					}
					got += msg + "\n"
				}
				if api == "67.0" {
					if result.HasErrors() {
						t.Fatalf("API 67 must compile: %s", got)
					}
					return
				}
				if !result.HasErrors() || !strings.Contains(got, c.want) {
					t.Fatalf("API 62 want %q; got %q", c.want, got)
				}
			})
		}
	}
}

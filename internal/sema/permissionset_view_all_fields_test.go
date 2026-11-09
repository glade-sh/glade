package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPermissionSetViewAllFieldsAbsent(t *testing.T) {
	for _, api := range []string{"62.0", "63.0", "67.0"} {
		for _, tc := range []struct{ name, body, want string }{
			{"token", "System.debug(PermissionSet.PermissionsViewAllFields);", "Variable does not exist: PermissionsViewAllFields"},
			{"soql", "if (false) { List<PermissionSet> p = [SELECT PermissionsViewAllFields FROM PermissionSet]; }", "PermissionsViewAllFields"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, "Empty.cls")
				writeSemaFile(t, file, "public class Empty {}")
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{file}}, schema.Schema{})
				result := AnalyzeAnonymous(index, tc.body, api)
				found := false
				for _, d := range result.Diagnostics {
					if strings.Contains(d.Message, tc.want) || strings.Contains(d.NativeMessage, tc.want) {
						found = true
					}
				}
				if !result.HasErrors() || !found {
					t.Fatalf("expected %q error, got diagnostics: %+v", tc.want, result.Diagnostics)
				}
			})
		}
	}
}

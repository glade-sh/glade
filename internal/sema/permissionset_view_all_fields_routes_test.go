package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Supplemental route coverage: the native capture exercises anonymous Apex.
func TestPermissionSetViewAllFieldsNamedRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, source string }{
			{"project", "public class TokenProbe { public static void run() { System.debug(PermissionSet.PermissionsViewAllFields); } }"},
			{"istest", "@IsTest private class TokenProbe { @IsTest static void run() { System.debug(PermissionSet.PermissionsViewAllFields); } }"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "TokenProbe.cls")
				writeSemaFile(t, path, tc.source)
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				for i := 0; i < 2; i++ {
					result := Analyze(index)
					found := false
					for _, d := range result.Diagnostics {
						found = found || strings.Contains(d.Message, "Variable does not exist: PermissionsViewAllFields")
					}
					if !result.HasErrors() || !found {
						t.Fatalf("analysis %d: expected missing-field error, got %v", i, result.Diagnostics)
					}
				}
			})
		}
	}
}

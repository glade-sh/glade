package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Preserve the existing schema-token paths while checking instance fields.
func TestAuthSessionSchemaTokenPaths(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, expr := range []string{"AuthSession.Fields.UsersId", "AuthSession.SObjectType.fields.UsersId"} {
			t.Run(api+"/"+expr, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "TokenProbe.cls")
				writeSemaFile(t, path, "public class TokenProbe { public static void observe() { Object token = "+expr+"; } }")
				result := Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{}))
				if result.HasErrors() {
					t.Fatalf("existing schema-token path rejected: %v", result.Diagnostics)
				}
			})
		}
	}
}

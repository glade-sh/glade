package sema

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// A dependency's snapshot describes its own org; it cannot assert the host
// org's standard-object field surface is complete. Native rejection controls:
// standard_field_access/org.tsv:2,6,18,22. This is a local provenance boundary.
func TestStandardSnapshotDependencyCannotCloseHost(t *testing.T) {
	for _, object := range []string{"Account", "Contact"} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(object+"/"+api, func(t *testing.T) {
				depRoot := t.TempDir()
				raw, err := json.Marshal(map[string]any{"objects": []any{map[string]any{"name": object, "fieldsComplete": true, "fields": []any{map[string]any{"name": "DependencyField__c", "type": "Text"}}}}})
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(depRoot, "snapshot.json")
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(raw)
				dep := project.Project{Root: depRoot, SchemaSnapshot: path, SchemaSnapshotSHA256: fmt.Sprintf("%x", sum)}
				root := t.TempDir()
				host := project.Project{Root: root, SourceAPIVersion: api, ManagedPackageDependencies: []project.ManagedPackageDependency{{Namespace: "fixture", SourceRoot: depRoot, Status: "loaded", Project: &dep}}}
				index := typesys.Build(host, schema.Schema{Objects: []schema.Object{{Name: object}}})
				for _, phase := range []string{"cold", "warm"} {
					result := AnalyzeAnonymous(index, "Object x = new "+object+"().HostOnly__c;", api)
					if result.HasErrors() {
						t.Errorf("%s dependency snapshot closed host schema: %v", phase, result.Diagnostics)
					}
				}
			})
		}
	}
}

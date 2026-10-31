package frontendcache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native missing-field rows are standard_field_access/org.tsv:18,22. The
// snapshot assertion and persisted index are local completeness boundaries.
func TestSnapshotAssertionRestoredFrontend(t *testing.T) {
	for _, tc := range []struct {
		name   string
		marker any
		closed bool
	}{{name: "legacy"}, {name: "false", marker: false}, {name: "true", marker: true, closed: true}} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			file := filepath.Join(root, "Probe.cls")
			if err := os.WriteFile(file, []byte("public class Probe {}"), 0600); err != nil {
				t.Fatal(err)
			}
			objects := []any{}
			for _, name := range []string{"Account", "Contact"} {
				obj := map[string]any{"name": name}
				if tc.marker != nil {
					obj["fieldsComplete"] = tc.marker
				}
				objects = append(objects, obj)
			}
			raw, err := json.Marshal(map[string]any{"objects": objects})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, "snapshot.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(raw)
			p := project.Project{Root: root, ApexFiles: []string{file}, SourceAPIVersion: "67.0", SchemaSnapshot: path, SchemaSnapshotSHA256: fmt.Sprintf("%x", sum)}
			loaded, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			build := func() (typesys.Index, typesys.BuildArtifacts) { return typesys.BuildWithArtifacts(p, loaded) }
			first := LoadOrBuild(p, loaded, build)
			restored := LoadOrBuild(p, loaded, build)
			if first.Hit || !restored.Hit {
				t.Fatalf("cold=%t restored=%t", first.Hit, restored.Hit)
			}
			for _, obj := range []string{"Account", "Contact"} {
				for _, idx := range []typesys.Index{first.Index, restored.Index} {
					result := sema.AnalyzeAnonymous(idx, "Object x = new "+obj+"().Missing__c;", "67.0")
					if result.HasErrors() != tc.closed {
						t.Errorf("%s closed=%t diagnostics=%v", obj, tc.closed, result.Diagnostics)
					}
				}
			}
		})
	}
}

// A restored host index must not gain authority from a dependency snapshot.
func TestSnapshotDependencyRestoredFrontend(t *testing.T) {
	depRoot := t.TempDir()
	raw, err := json.Marshal(map[string]any{"objects": []any{map[string]any{"name": "Account", "fieldsComplete": true}, map[string]any{"name": "Contact", "fieldsComplete": true}}})
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
	source := filepath.Join(root, "Probe.cls")
	if err := os.WriteFile(source, []byte("public class Probe {}"), 0600); err != nil {
		t.Fatal(err)
	}
	host := project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{source}, ManagedPackageDependencies: []project.ManagedPackageDependency{{Namespace: "fixture", SourceRoot: depRoot, Status: "loaded", Project: &dep}}}
	shape := schema.Schema{Objects: []schema.Object{{Name: "Account"}, {Name: "Contact"}}}
	build := func() (typesys.Index, typesys.BuildArtifacts) { return typesys.BuildWithArtifacts(host, shape) }
	first := LoadOrBuild(host, shape, build)
	restored := LoadOrBuild(host, shape, build)
	if first.Hit || !restored.Hit {
		t.Fatalf("cold=%t restored=%t", first.Hit, restored.Hit)
	}
	for _, name := range []string{"Account", "Contact"} {
		for label, idx := range map[string]typesys.Index{"cold": first.Index, "restored": restored.Index} {
			result := sema.AnalyzeAnonymous(idx, "Object x = new "+name+"().HostField__c;", "67.0")
			if result.HasErrors() {
				t.Errorf("%s %s dependency snapshot closed host field access: %v", label, name, result.Diagnostics)
			}
		}
	}
}

package frontendcache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardSnapshotGenerationRestoredFrontend(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "Probe.cls")
	if err := os.WriteFile(source, []byte("public class Probe {}"), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(schema.Schema{Objects: []schema.Object{{Name: "Account", CompleteSnapshot: true}, {Name: "Contact", CompleteSnapshot: true}}})
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.ReplaceAll(string(raw), `"completeSnapshot"`, `"fieldsComplete"`))
	path := filepath.Join(root, "snapshot.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	hash := fmt.Sprintf("%x", sum)
	cfg := filepath.Join(root, "glade.yml")
	if err := os.WriteFile(cfg, []byte(fmt.Sprintf("project:\n  schemaSnapshot: snapshot.json\n  schemaSnapshotSHA256: %s\n", hash)), 0600); err != nil {
		t.Fatal(err)
	}
	p := project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{source}, SchemaSnapshot: path, SchemaSnapshotSHA256: hash}
	loaded, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	build := func() (typesys.Index, typesys.BuildArtifacts) { return typesys.BuildWithArtifacts(p, loaded) }
	first := LoadOrBuild(p, loaded, build)
	restored := LoadOrBuild(p, loaded, build)
	if first.Hit || !restored.Hit {
		t.Fatalf("frontend cache cold=%t restored=%t", first.Hit, restored.Hit)
	}
	if err := os.Remove(cfg); err != nil {
		t.Fatal(err)
	}
	for _, object := range []string{"Account", "Contact"} {
		for label, index := range map[string]typesys.Index{"cold": first.Index, "restored": restored.Index} {
			result := sema.AnalyzeAnonymous(index, "Object x = new "+object+"().Missing__c;", "67.0")
			if !result.HasErrors() {
				t.Errorf("%s %s: complete indexed snapshot reopened after config removal", label, object)
			}
		}
	}
}

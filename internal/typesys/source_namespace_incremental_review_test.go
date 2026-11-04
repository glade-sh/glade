package typesys

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

// Project ownership and index invalidation are local contracts. Native
// dispatch is recorded in subproject_namespace_runtime/org.tsv:2-7.
func TestNestedSourceNamespaceIncrementalReview(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "packs", "child")
	file := filepath.Join(child, "force-app", "main", "classes", "Nested.cls")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"packs","default":true}],"namespace":""}`)
	config := filepath.Join(child, "sfdx-project.json")
	writeFile(t, config, `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"First"}`)
	writeFile(t, file, `global class Nested { global Integer size() { return 1; } }`)
	load := func() project.Project {
		t.Helper()
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := load()
	previous := Build(p, schema.Schema{})
	writeFile(t, file, `global class Nested { global Integer size() { return 2; } }`)
	updated, ok := updateApexFilesIncremental(previous, []string{file}, nil)
	if !ok {
		t.Fatal("source edit did not use the incremental path")
	}
	fresh := Build(load(), schema.Schema{})
	compareIncrementalTypeSymbolAtPath(t, file, updated, fresh)
	found := false
	for _, typ := range updated.Types {
		if typ.File == file && typ.Name == "Nested" {
			found = true
			if typ.Namespace != "First" {
				t.Fatalf("incremental namespace = %q", typ.Namespace)
			}
		}
	}
	if !found {
		t.Fatal("updated source type absent")
	}
	writeFile(t, config, `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"Second"}`)
	if incrementalProjectIdentity(p) == incrementalProjectIdentity(load()) {
		t.Fatal("namespace owner change did not invalidate project identity")
	}
}

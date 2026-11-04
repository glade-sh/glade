package project

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestWalkedNamespaceInventoryUsesOwnedSourceReview(t *testing.T) {
	for _, includeOuterSource := range []bool{false, true} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"packs","default":true}]}`)
		writeFile(t, filepath.Join(root, "packs", "outer", "sfdx-project.json"), `{"namespace":"First"}`)
		writeFile(t, filepath.Join(root, "packs", "outer", "inner", "sfdx-project.json"), `{"namespace":"Second"}`)
		writeFile(t, filepath.Join(root, "packs", "outer", "inner", "Child.cls"), `public class Child {}`)
		if includeOuterSource {
			writeFile(t, filepath.Join(root, "packs", "outer", "Parent.cls"), `public class Parent {}`)
		}
		p, err := Load(root)
		if err != nil {
			t.Fatal(err)
		}
		want := []string{"Second"}
		if includeOuterSource {
			want = []string{"First", "Second"}
		}
		if !reflect.DeepEqual(p.SourceNamespaces, want) {
			t.Fatalf("include outer source=%v: namespaces=%q want %q", includeOuterSource, p.SourceNamespaces, want)
		}
	}
}

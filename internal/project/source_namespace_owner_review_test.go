package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceNamespaceForFileReview(t *testing.T) {
	root := t.TempDir()
	p := Project{SourceNamespaceRoots: map[string]string{filepath.Join(root, "pkg"): "First", filepath.Join(root, "pkg", "child"): "Second", filepath.Join(root, "pkg", "plain"): ""}}
	for _, tc := range []struct{ path, fallback, want string }{
		{filepath.Join(root, "pkg", "force-app", "A.cls"), "Root", "First"},
		{filepath.Join(root, "pkg", "child", "B.cls"), "Root", "Second"},
		{filepath.Join(root, "pkg", "plain", "E.cls"), "Root", ""},
		{filepath.Join(root, "pkg-other", "C.cls"), "Root", "Root"},
		{filepath.Join(root, "elsewhere", "D.cls"), "", ""},
	} {
		if got := p.SourceNamespaceForFile(tc.path, tc.fallback); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.path, got, tc.want)
		}
	}
}

func TestLoadNestedEmptyNamespaceBoundaryReview(t *testing.T) {
	root := t.TempDir()
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "sfdx-project.json"), `{"namespace":"Root","packageDirectories":[{"path":"packs","default":true}]}`)
	write(filepath.Join(root, "packs", "parent", "sfdx-project.json"), `{"namespace":"First","packageDirectories":[{"path":"force-app","default":true}]}`)
	write(filepath.Join(root, "packs", "parent", "plain", "sfdx-project.json"), `{"namespace":"","packageDirectories":[{"path":"force-app","default":true}]}`)
	child := filepath.Join(root, "packs", "parent", "plain", "force-app", "A.cls")
	write(child, "public class A {}")
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.SourceNamespaceForFile(child, p.Namespace); got != "" {
		t.Fatalf("unnamespaced subproject resolved to %q", got)
	}
}

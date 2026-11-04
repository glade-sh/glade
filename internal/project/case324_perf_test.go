package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCase324SiblingNamespaceWorkload(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "consumer")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30; i++ {
		d := filepath.Join(parent, fmt.Sprintf("dep%02d", i))
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
		data := fmt.Sprintf(`{"namespace":"ns%02d","packageDirectories":[{"path":"src","default":true}]}`, i)
		if err := os.WriteFile(filepath.Join(d, "sfdx-project.json"), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 30; i++ {
		data := strings.Repeat("public class Example {}\n", 500)
		if i == 29 {
			data += " ns29__Value"
		}
		if err := os.WriteFile(filepath.Join(root, "src", fmt.Sprintf("File%02d.cls", i)), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p := Project{PackageDirectories: []PackageDirectory{{Path: "src"}}}
	for i := 0; i < 6; i++ {
		got := findReferencedNamespaceSiblingSFDXPackageDependencyRoots(root, []string{"unresolved"}, p, true)
		if len(got) != 1 || filepath.Base(got[0]) != "dep29" {
			t.Fatalf("unexpected roots: %v", got)
		}
	}
}

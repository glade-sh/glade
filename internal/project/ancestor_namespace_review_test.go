package project

import (
	"path/filepath"
	"testing"
)

func TestSelectedPackageAncestorNamespaceReview(t *testing.T) {
	for _, tc := range []struct{ root, outer, inner string }{
		{"Root", "First", "Second"},
		{"Root", "First", ""},
		{"", "", "Second"},
	} {
		t.Run(tc.root+"/"+tc.outer+"/"+tc.inner, func(t *testing.T) {
			root := t.TempDir()
			outer := filepath.Join(root, "packages")
			inner := filepath.Join(outer, "sub")
			file := filepath.Join(inner, "gap", "force-app", "classes", "Example.cls")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"`+tc.root+`","packageDirectories":[{"path":"packages/sub/gap/force-app","default":true}]}`)
			writeFile(t, filepath.Join(outer, "sfdx-project.json"), `{"namespace":"`+tc.outer+`","packageDirectories":[{"path":"sub","default":true}]}`)
			writeFile(t, filepath.Join(inner, "sfdx-project.json"), `{"namespace":"`+tc.inner+`","packageDirectories":[{"path":"gap/force-app","default":true}]}`)
			writeFile(t, file, `public class Example {}`)
			p, err := Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if got := p.SourceNamespaceForFile(file, p.Namespace); got != tc.inner {
				t.Fatalf("selected source namespace = %q, want %q", got, tc.inner)
			}
			if got, ok := p.SourceNamespaceRoots[outer]; !ok || got != tc.outer {
				t.Fatalf("outer config namespace = %q, present=%v", got, ok)
			}
		})
	}
}

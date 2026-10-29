package startupcache

import (
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStandardFieldShapeDeclarationInvalidatesCache(t *testing.T) {
	for _, declaration := range []struct{ name, path, source string }{
		{"scratch", "config/project-scratch-def.json", `{"edition":"Developer"}`},
		{"glade-empty-features", "glade.yml", "org:\n  features: []\n"},
	} {
		t.Run(declaration.name, func(t *testing.T) {
			root := t.TempDir()
			writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeStartupCacheTestFile(t, filepath.Join(root, "force-app/main/default/classes/Shape.cls"), "public class Shape {}")
			build := func() Manifest {
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				return BuildManifest(root, p, typesys.Index{})
			}
			unknown := build()
			writeStartupCacheTestFile(t, filepath.Join(root, declaration.path), declaration.source)
			declared := build()
			if reflect.DeepEqual(unknown.Features, declared.Features) {
				t.Fatalf("uncertain and declared bare shapes share features: %v", declared.Features)
			}
			if unknown.ProjectDigest == declared.ProjectDigest {
				t.Fatal("shape change did not invalidate semantic project digest")
			}
		})
	}
}

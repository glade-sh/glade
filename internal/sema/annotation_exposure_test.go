package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Dependency collisions are synthetic regression coverage. Native controls
// cover enclosing exposure in unpackaged sources, without a managed namespace.
func TestNamedAnnotationExposureUsesEnclosingDeclaration(t *testing.T) {
	for _, apiVersion := range []string{"62.0", "67.0"} {
		for _, dependencyNamespace := range []string{"aaapkg", "zzzpkg"} {
			for _, test := range []struct {
				name, enclosing, dependency string
				wantError                   bool
			}{
				{"annotated enclosing", "@NamespaceAccessible public", "public", false},
				{"global enclosing", "global", "public", false},
				{"unannotated enclosing with annotated dependency", "public", "@NamespaceAccessible public", true},
				{"unannotated enclosing with global dependency", "public", "global", true},
			} {
				t.Run(apiVersion+"/"+dependencyNamespace+"/"+test.name, func(t *testing.T) {
					root := t.TempDir()
					file := filepath.Join(root, "Widget.cls")
					dependencyRoot := t.TempDir()
					dependencyFile := filepath.Join(dependencyRoot, "Widget.cls")
					writeSemaFile(t, file, test.enclosing+" class Widget { @NamespaceAccessible public class Item {} }")
					writeSemaFile(t, dependencyFile, test.dependency+" class Widget {}")
					dependency := project.Project{
						Root: dependencyRoot, Namespace: dependencyNamespace,
						SourceAPIVersion: apiVersion, ApexFiles: []string{dependencyFile},
					}
					index := typesys.Build(project.Project{
						Root: root, Namespace: "localpkg", SourceAPIVersion: apiVersion,
						ApexFiles: []string{file},
						ManagedPackageDependencies: []project.ManagedPackageDependency{{
							Namespace: dependencyNamespace, SourceRoot: dependencyRoot,
							Project: &dependency, Status: "loaded",
						}},
					}, schema.Schema{})
					if index.HasErrors() {
						t.Fatalf("index: %#v", index.Diagnostics)
					}
					result := Analyze(index)
					if !test.wantError {
						if result.HasErrors() {
							t.Fatalf("unrelated enclosing declaration affected exposure: %#v", result.Diagnostics)
						}
						return
					}
					const message = "Enclosing type for NamespaceAccessible classes in apex classes require at least one of the following NamespaceAccessible, global"
					if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA032" || result.Diagnostics[0].NativeMessage != message || result.Diagnostics[0].File != file {
						t.Fatalf("missing enclosing declaration rejection: %#v", result.Diagnostics)
					}
				})
			}
		}
	}
}

package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestOpaqueAliasDoesNotOpenUnknownStaticReceivers(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "conformance", "namespace_unqualified_root", "anon")
	for _, api := range []string{"62.0", "67.0"} {
		for _, name := range []string{"R03_static_field.apex", "R04_static_method.apex"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				source, err := os.ReadFile(filepath.Join(root, name))
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(project.Project{Root: t.TempDir(), SourceAPIVersion: api}, schema.Schema{})
				index.Dependencies = []typesys.DependencyInfo{{Namespace: "synthetic-unavailable", Status: "missing"}}
				for _, d := range AnalyzeAnonymous(index, string(source), api).Diagnostics {
					if d.Severity == diagnostic.Error && (d.NativeMessage == "Variable does not exist: RootV035.Inner" || d.Message == "Variable does not exist: RootV035.Inner") {
						return
					}
				}
				t.Fatal("missing captured static-receiver rejection")
			})
		}
	}
}

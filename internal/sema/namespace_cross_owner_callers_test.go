package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestNamespaceCrossOwnerCallerPositions(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for name, tc := range map[string]struct{ body, want string }{
			"class-literal": {"Type value = host.ForeignShape.class;", "Invalid type: host.ForeignShape"},
			"static-call":   {"host.ForeignShape.missing();", "Variable does not exist: host.ForeignShape"},
			"catch":         {"try { Integer value = 1; } catch (host.ForeignShape e) { }", "Invalid type: host.ForeignShape"},
			"foreach":       {"for (host.ForeignShape item : new List<host.ForeignShape>()) { }", "Invalid type: host.ForeignShape"},
		} {
			t.Run(api+"/"+name, func(t *testing.T) {
				root := t.TempDir()
				child := filepath.Join(root, "child")
				if err := os.MkdirAll(child, 0755); err != nil {
					t.Fatal(err)
				}
				foreign := filepath.Join(child, "ForeignShape.cls")
				test := filepath.Join(root, "CallerTest.cls")
				writeSemaFile(t, foreign, "public class ForeignShape {}")
				writeSemaFile(t, test, "@IsTest private class CallerTest { @IsTest static void check() { "+tc.body+" } }")
				idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: api, SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{foreign, test}}, schema.Schema{})
				for route, result := range map[string]Result{"anonymous": AnalyzeAnonymous(idx, tc.body, api), "istest": Analyze(idx)} {
					found := false
					for _, d := range result.Diagnostics {
						if d.NativeMessage == tc.want {
							found = true
						}
					}
					if !result.HasErrors() || !found {
						t.Errorf("%s wanted %q: %+v", route, tc.want, result.Diagnostics)
					}
				}
			})
		}
	}
}

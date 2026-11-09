package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The namespace owner rejects the foreign suffix in a constructor, cast and
// generic local on both routes; see testdata/namespace_cross_owner/org.tsv.
func TestNamespaceCrossOwnerTypePositions(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for name, body := range map[string]string{
			"construct": "Object value = new host.ForeignShape();",
			"cast":      "Object value = null; Object casted = (host.ForeignShape)value;",
			"generic":   "List<host.ForeignShape> value;",
		} {
			t.Run(api+"/"+name, func(t *testing.T) {
				root := t.TempDir()
				child := filepath.Join(root, "child")
				if err := os.MkdirAll(child, 0755); err != nil {
					t.Fatal(err)
				}
				foreign := filepath.Join(child, "ForeignShape.cls")
				test := filepath.Join(root, "PositionTest.cls")
				writeSemaFile(t, foreign, "public class ForeignShape {}")
				writeSemaFile(t, test, "@IsTest private class PositionTest { @IsTest static void check() { "+body+" } }")
				idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: api, SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{foreign, test}}, schema.Schema{})
				for route, result := range map[string]Result{"anonymous": AnalyzeAnonymous(idx, body, api), "istest": Analyze(idx)} {
					if !result.HasErrors() {
						t.Errorf("%s accepted %s: %+v", route, body, result.Diagnostics)
						continue
					}
					found := false
					for _, d := range result.Diagnostics {
						if strings.Contains(d.NativeMessage, "Invalid type: host.ForeignShape") {
							found = true
						}
					}
					if !found {
						t.Errorf("%s missing native rejection: %+v", route, result.Diagnostics)
					}
				}
			})
		}
	}
}

package sema

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestInheritedOverloadOrderedDiagnosticSnapshot(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "ScopeBase.cls")
			child := filepath.Join(root, "ScopeChild.cls")
			writeSemaFile(t, base, `public virtual class ScopeBase { public String value; public virtual String pick(String input) { return input; } public virtual Integer pick(Integer input) { return input; } }`)
			writeSemaFile(t, child, `public class ScopeChild extends ScopeBase { public void probe() { Integer wrong = value; String wrong2 = pick(3); pick(true); } }`)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{base, child}}, schema.Schema{})
			for _, route := range []string{"project", "anonymous"} {
				analyze := func() []diagnostic.Diagnostic {
					var rows []diagnostic.Diagnostic
					if route == "project" {
						rows = Analyze(index).Diagnostics
					} else {
						rows = AnalyzeAnonymous(index, `ScopeChild obj = new ScopeChild(); Integer wrong = obj.value; obj.pick(true);`, api).Diagnostics
					}
					for i := range rows {
						if rows[i].File != "" {
							rows[i].File = filepath.Base(rows[i].File)
						}
					}
					return rows
				}
				first, warm := analyze(), analyze()
				if len(first) == 0 {
					t.Fatalf("%s: expected diagnostic controls", route)
				}
				if !reflect.DeepEqual(first, warm) {
					t.Fatalf("%s: warm diagnostics differ: %#v / %#v", route, first, warm)
				}
				data, err := json.Marshal(first)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("SNAPSHOT %s %s %s", api, route, data)
			}
		})
	}
}

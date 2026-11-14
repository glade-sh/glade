package sema

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// These free-dot type neighbors have no matching native message row. Their
// complete JSON diagnostics are the output observed on origin/main at 67.0.
func TestReceiverUncapturedTypeNeighbors(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"earlier parse error", "Integer x = ;\nReceiverProbe.\nvalue();", `[{"severity":"error","code":"GLADESEMA_ANONYMOUS_PARSE","message":"unexpected token \";\" at byte 12","range":{"start":{"line":1,"column":1},"end":{"line":1,"column":38,"offset":37}}}]`},
		{"generic", "{ List<System\n.String> s; }", `[{"severity":"error","code":"GLADESEMA006","message":"Invalid type: System.String","range":{"start":{"line":1,"column":3,"offset":2},"end":{"line":2,"column":8,"offset":21}},"nativeMessage":"Invalid type: System.String"}]`},
		{"cast", "{ Object s = (System\n.String) 'x'; }", "null"},
		{"instanceof", "{ Boolean b = 'x' instanceof System\n.String; }", "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(AnalyzeAnonymous(typesys.Index{}, tc.source, "67.0").Diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("diagnostics %s, want origin/main %s", got, tc.want)
			}
		})
	}
}

// A static field that shares a name with a nested type is still a value on
// the left of a free dot. S1 covers static-field values (org.tsv:174-175).
func TestReceiverStaticFieldShadowsType(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ShadowProbe.cls")
	writeSemaFile(t, path, "public class ShadowProbe { public enum Level { LOW } public static Level level = null; }")
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
		got, err := json.Marshal(AnalyzeAnonymous(index, "String s = ShadowProbe.level\n.name();", api).Diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "null" {
			t.Errorf("api %s: diagnostics %s, want origin/main null", api, got)
		}
	}
}

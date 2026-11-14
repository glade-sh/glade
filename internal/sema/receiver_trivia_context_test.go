package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// L1/P1 accept local-value free dots; M6 rejects a detached type receiver,
// and M2/D2 cover block initializer bound-dot errors (org.tsv:148-149,
// 168-169, 162-163, 154-155, 196-197).
func TestReceiverTriviaExpressionContexts(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"instanceof local newline", "Object other = new CrProbe(); CrProbe obj = new CrProbe(); Boolean b = other instanceof CrProbe && obj\n.flag;", ""},
		{"instanceof local space", "Object other = new CrProbe(); CrProbe obj = new CrProbe(); Boolean b = other instanceof CrProbe && obj .flag;", ""},
		{"instanceof type receiver", "Object other = 'x'; Boolean b = other instanceof String && CrProbe\n.value() == 'x';", "Variable does not exist: CrProbe"},
		{"comparison bound dot", "Integer x = 1; Integer y = 2; Boolean b = x < y && String.\nvalueOf(1) == '1';", "Missing ';' at 'valueOf'"},
	}
	for _, api := range []string{"62.0", "64.0", "67.0"} {
		for _, route := range []string{"anonymous", "class", "trigger"} {
			for _, tc := range cases {
				t.Run(api+"/"+route+"/"+tc.name, func(t *testing.T) {
					root := t.TempDir()
					probe := filepath.Join(root, "CrProbe.cls")
					writeSemaFile(t, probe, "public class CrProbe { public Boolean flag = true; public static String value() { return 'x'; } }")
					files := []string{probe}
					body := tc.body
					if route != "anonymous" {
						name, source := "RouteProbe.cls", "public class RouteProbe { public void run() { "+body+" } }"
						if route == "trigger" {
							name, source = "RouteProbe.trigger", "trigger RouteProbe on Account (before insert) { "+body+" }"
						}
						path := filepath.Join(root, name)
						writeSemaFile(t, path, source)
						files = append(files, path)
					} else if tc.name == "comparison bound dot" {
						body = "{ " + body + " }"
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
					result := Analyze(index)
					if route == "anonymous" {
						result = AnalyzeAnonymous(index, body, api)
					}
					if tc.want == "" {
						if result.HasErrors() {
							t.Errorf("unexpected diagnostics: %v", result.Diagnostics)
						}
						return
					}
					for _, d := range result.Diagnostics {
						if d.NativeMessage == tc.want {
							return
						}
					}
					t.Errorf("diagnostics %v, want %q", result.Diagnostics, tc.want)
				})
			}
		}
	}
}

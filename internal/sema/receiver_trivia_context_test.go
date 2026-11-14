package sema

import (
	"encoding/json"
	"path/filepath"
	"strings"
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
		{"comparison local free dot", "Integer x = 1; CrProbe obj = new CrProbe(); Boolean b = x < obj\n.countValue;", ""},
		{"comparison type free dot", "Integer x = 1; Boolean b = x < Integer\n.valueOf('2');", "Variable does not exist: Integer"},
		{"comparison conditional type", "Integer x = 1; Boolean b = x < Integer\n.valueOf('2') ? x > 0 : false;", "Variable does not exist: Integer"},
		{"comparison bound dot", "Integer x = 1; Integer y = 2; Boolean b = x < y && String.\nvalueOf(1) == '1';", "Missing ';' at 'valueOf'"},
		// The ternary row (org.tsv:254-255) breaks in the true arm. A closed
		// conditional leaves the innermost construct in charge (M1, M2); the
		// false arm is rejected natively with an uncaptured message.
		{"closed ternary bound dot", "String s = (true ? 'a' : 'b') + String.\nvalueOf(1);", "Missing ';' at 'valueOf'"},
		{"ternary argument bound dot", "String s = true ? String.valueOf(String.\nvalueOf(1)) : 'x';", "Expecting ')' but was: 'valueOf'"},
		{"earlier ternary argument bound dot", "String s = String.join(new List<String>{true ? 'a' : 'b'}, String.\nvalueOf(1));", "Expecting ')' but was: 'valueOf'"},
		{"true arm bound dot", "System.debug(true ? String.\nvalueOf(1) : 'x');", "Missing ':' at 'valueOf'"},
		{"false arm bound dot", "String s = true ? 'a' : String.\nvalueOf(1);", ""},
	}
	for _, api := range []string{"62.0", "64.0", "67.0"} {
		for _, route := range []string{"anonymous", "class", "trigger"} {
			for _, tc := range cases {
				t.Run(api+"/"+route+"/"+tc.name, func(t *testing.T) {
					root := t.TempDir()
					probe := filepath.Join(root, "CrProbe.cls")
					writeSemaFile(t, probe, "public class CrProbe { public Boolean flag = true; public Integer countValue = 2; public static String value() { return 'x'; } }")
					files := []string{probe}
					body := tc.body
					if strings.HasSuffix(tc.name, "bound dot") {
						body = "{ " + body + " }"
					}
					if route != "anonymous" {
						name, source := "RouteProbe.cls", "public class RouteProbe { public void run() { "+body+" } }"
						if route == "trigger" {
							name, source = "RouteProbe.trigger", "trigger RouteProbe on Account (before insert) { "+body+" }"
						}
						path := filepath.Join(root, name)
						writeSemaFile(t, path, source)
						files = append(files, path)
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
					result := Analyze(index)
					if route == "anonymous" {
						result = AnalyzeAnonymous(index, body, api)
					}
					if tc.want == "" {
						got, err := json.Marshal(result.Diagnostics)
						if err != nil {
							t.Fatal(err)
						}
						if string(got) != "null" {
							t.Errorf("diagnostics %s, want origin/main null", got)
						}
						return
					}
					if len(result.Diagnostics) != 1 || result.Diagnostics[0].NativeMessage != tc.want {
						t.Errorf("diagnostics %v, want exactly %q", result.Diagnostics, tc.want)
					}
				})
			}
		}
	}
}

// A parameter of an anonymous local method is a value on the left of a free
// dot, including one named like a type (S5, L1, P1: org.tsv:178-179, 148-149,
// 168-169). A type there is still rejected (M6: 162-163).
func TestReceiverLocalMethodParameterFreeDot(t *testing.T) {
	root := t.TempDir()
	probe := filepath.Join(root, "CrProbe.cls")
	writeSemaFile(t, probe, "public class CrProbe { public String instanceValue() { return 'x'; } public static String value() { return 'x'; } }")
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{probe}}, schema.Schema{})
		for _, tc := range []struct{ source, want string }{
			{"void f(CrProbe CrProbe) { System.debug(CrProbe\n.instanceValue()); } f(new CrProbe());", ""},
			{"void f(CrProbe p) { System.debug(p .instanceValue()); } f(new CrProbe());", ""},
			{"void f(CrProbe p) { System.debug(CrProbe\n.value()); } f(new CrProbe());", "Variable does not exist: CrProbe"},
			{"void f(CrProbe CrProbe) { } System.debug(CrProbe\n.value());", "Variable does not exist: CrProbe"},
		} {
			got := AnalyzeAnonymous(index, tc.source, api).Diagnostics
			if tc.want == "" && len(got) != 0 || tc.want != "" && (len(got) != 1 || got[0].NativeMessage != tc.want) {
				t.Errorf("api %s %q: diagnostics %v, want %q", api, tc.source, got, tc.want)
			}
		}
	}
}

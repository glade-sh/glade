package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// P1, Q1-Q2, S1-S2, S5, CA and class_literal_before_dot are the native
// acceptance controls (org.tsv:168-179, 190-191, 80-81).
func TestReceiverTriviaAcceptanceRoutes(t *testing.T) {
	controls := []struct {
		name, body string
		classOnly  bool
	}{
		{"local free before", "CrProbe x = new CrProbe(); x\n.instanceValue();", false},
		{"local free both", "CrProbe x = new CrProbe(); x . instanceValue();", false},
		{"call result before", "CrProbe x = new CrProbe(); x.instanceValue()\n.toUpperCase();", false},
		{"call result after", "CrProbe x = new CrProbe(); x.instanceValue().\ntoUpperCase();", false},
		{"index result", "List<CrProbe> xs = new List<CrProbe>{new CrProbe()}; xs[0].\ninstanceValue();", false},
		{"literal result", "'ab'.\nlength();", false},
		{"safe after", "CrProbe x = new CrProbe(); x?.\ninstanceValue();", false},
		{"safe before", "CrProbe x = new CrProbe(); x\n?.instanceValue();", false},
		{"this free", "this\n.helper();", true},
		{"static value", "CrProbe.SF\n.length();", false},
		{"enum value", "CrProbe.Color.RED\n.name();", false},
		{"class literal", "System.debug(CrProbe .class);", false},
		{"call space", "CrProbe.value ();", false},
		{"shadowed type", "CrProbe CrProbe = new CrProbe(); CrProbe\n.instanceValue();", false},
		{"string text", "String s = 'x. y';", false},
		{"comment text", "/* x. y */ Integer n = 1;", false},
		{"query text", "List<Account> a = [SELECT Id FROM Account WHERE Name = 'x. y'];", false},
	}
	support, err := os.ReadFile(filepath.Join("testdata/receiver_split", "CrProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		for _, route := range []string{"anonymous", "class", "trigger"} {
			for _, tc := range controls {
				if tc.classOnly && route != "class" {
					continue
				}
				t.Run(api+"/"+route+"/"+tc.name, func(t *testing.T) {
					root := t.TempDir()
					probe := filepath.Join(root, "CrProbe.cls")
					writeSemaFile(t, probe, string(support))
					files := []string{probe}
					if route != "anonymous" {
						name, src := "RouteProbe.cls", "public class RouteProbe { public void helper() {} public void run() { "+tc.body+" } }"
						if route == "trigger" {
							name, src = "RouteProbe.trigger", "trigger RouteProbe on Account (before insert) { "+tc.body+" }"
						}
						path := filepath.Join(root, name)
						writeSemaFile(t, path, src)
						files = append(files, path)
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					result := Analyze(index)
					if route == "anonymous" {
						result = AnalyzeAnonymous(index, tc.body, api)
					}
					if result.HasErrors() {
						t.Errorf("accepted native control: %v", result.Diagnostics)
					}
				})
			}
		}
	}
}

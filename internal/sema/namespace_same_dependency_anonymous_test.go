package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSameNamespaceDependencyAnonymousVisibility(t *testing.T) {
	provider, err := os.ReadFile(filepath.Join("..", "apextest", "testdata", "conformance", "nested_type_visibility", "classes", "V035Vis.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name   string
			reject bool
		}{{"Pv", true}, {"Dn", true}, {"Tv", true}, {"Pb", false}} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				tmp := t.TempDir()
				consumer := filepath.Join(tmp, "consumer")
				dep := filepath.Join(tmp, "provider")
				for _, dir := range []string{filepath.Join(consumer, "force-app"), filepath.Join(dep, "force-app")} {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
				}
				writeSemaFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"namespace":"v036ns","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"synthetic-provider"}]}]}`)
				writeSemaFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  dependencyPaths: [../provider]\n")
				writeSemaFile(t, filepath.Join(dep, "sfdx-project.json"), `{"namespace":"v036ns","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"synthetic-provider"}]}`)
				writeSemaFile(t, filepath.Join(dep, "force-app", "V035Vis.cls"), string(provider))
				p, err := project.Load(consumer)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, schema.Schema{})
				loaded := false
				for _, d := range index.Dependencies {
					loaded = loaded || d.Status == "loaded"
				}
				if !loaded {
					t.Fatal("provider dependency not loaded")
				}
				result := AnalyzeAnonymous(index, "V035Vis."+tc.name+" value;", api)
				want := "Type is not visible: v036ns.V035Vis." + tc.name
				matched := false
				var errors []string
				for _, d := range result.Diagnostics {
					if d.Severity == diagnostic.Error {
						errors = append(errors, d.Message+" | "+d.NativeMessage)
						matched = matched || d.Message == want || d.NativeMessage == want
					}
				}
				if tc.reject && !matched {
					t.Fatalf("expected %q, got %s", want, strings.Join(errors, "; "))
				}
				if !tc.reject && len(errors) != 0 {
					t.Fatalf("public nested type rejected: %s", strings.Join(errors, "; "))
				}
			})
		}
	}
}

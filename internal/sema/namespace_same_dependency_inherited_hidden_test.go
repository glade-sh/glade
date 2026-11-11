package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native: standard_object_shape_compile/org.tsv:261-264,270,306-309,315
// establishes inherited private/default rejection and public acceptance.
// namespace_same_dependency_visibility/org.tsv:3-4,8-9 establishes the
// namespace prefix in private/default visibility diagnostics.
func TestSameNamespaceDependencyInheritedNestedVisibility(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "standard_object_shape_compile")
	provider, err := os.ReadFile(filepath.Join(fixture, "classes", "NsOuter.cls"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		id, hidden string
	}{
		{"I01", "Pv"}, // private, qualified
		{"I02", "Pv"}, // private, inherited bare
		{"I03", "Dn"}, // default, qualified
		{"I04", "Dn"}, // default, inherited bare
		{"I10", ""},   // public inherited bare positive
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.id, func(t *testing.T) {
				consumer, err := os.ReadFile(filepath.Join(fixture, "named", tc.id+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				tmp := t.TempDir()
				consumerDir, dependencyDir := filepath.Join(tmp, "consumer"), filepath.Join(tmp, "provider")
				for _, dir := range []string{filepath.Join(consumerDir, "force-app"), filepath.Join(dependencyDir, "force-app")} {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
				}
				writeSemaFile(t, filepath.Join(consumerDir, "sfdx-project.json"), `{"namespace":"v036ns","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"synthetic-provider"}]}]}`)
				writeSemaFile(t, filepath.Join(consumerDir, "glade.yml"), "project:\n  dependencyPaths: [../provider]\n")
				writeSemaFile(t, filepath.Join(dependencyDir, "sfdx-project.json"), `{"namespace":"v036ns","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"synthetic-provider"}]}`)
				writeSemaFile(t, filepath.Join(dependencyDir, "force-app", "NsOuter.cls"), string(provider))
				consumerPath := filepath.Join(consumerDir, "force-app", tc.id+".cls")
				writeSemaFile(t, consumerPath, string(consumer))
				p, err := project.Load(consumerDir)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, schema.Schema{})
				loaded := false
				for _, dep := range index.Dependencies {
					loaded = loaded || dep.Status == "loaded" && strings.EqualFold(dep.Namespace, "v036ns")
				}
				if !loaded {
					t.Fatal("same-namespace provider source was not loaded")
				}
				want := ""
				if tc.hidden != "" {
					want = fmt.Sprintf("Type is not visible: v036ns.NsOuter.%s", tc.hidden)
				}
				matched := false
				for _, d := range Analyze(index).Diagnostics {
					if d.Severity == diagnostic.Error {
						if want != "" && (d.Message == want || d.NativeMessage == want) && filepath.Clean(d.File) == filepath.Clean(consumerPath) {
							matched = true
						} else {
							t.Errorf("unexpected error: %+v", d)
						}
					} else if want == "" && (strings.Contains(d.Message, "Pb") || strings.Contains(d.NativeMessage, "Pb")) {
						t.Errorf("native public inherited type compiled, offline warning: %+v", d)
					}
				}
				if want != "" && !matched {
					t.Errorf("missing native inherited-type visibility diagnostic %q", want)
				}
			})
		}
	}
}

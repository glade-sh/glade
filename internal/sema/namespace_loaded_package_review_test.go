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

func TestNamespaceLoadedUnnamespacedPackageMembers(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "namespace_unlocked_subscriber", "provider", "package", "main", "default", "classes", "V036PkgOuter.cls")
	providerSource, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name   string
			member string
			reject bool
			subset bool
		}{{"Box", "Box", false, false}, {"Shape", "Shape", false, false}, {"Absent", "Absent", true, false}, {"PartialAbsent", "Absent", false, true}} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				consumer := filepath.Join(root, "consumer")
				provider := filepath.Join(root, "provider")
				for _, dir := range []string{filepath.Join(consumer, "force-app"), filepath.Join(provider, "force-app")} {
					if err := os.MkdirAll(dir, 0755); err != nil {
						t.Fatal(err)
					}
				}
				writeSemaFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"synthetic-provider"}]}]}`)
				writeSemaFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  dependencyPaths: [../provider]\n")
				writeSemaFile(t, filepath.Join(provider, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"package":"synthetic-provider"}]}`)
				writeSemaFile(t, filepath.Join(provider, "force-app", "V036PkgOuter.cls"), string(providerSource))
				consumerFile := filepath.Join(consumer, "force-app", "V036Consumer.cls")
				writeSemaFile(t, consumerFile, fmt.Sprintf("public class V036Consumer { public V036PkgOuter.%s value; }", tc.member))
				p, err := project.Load(consumer)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, schema.Schema{})
				loaded := false
				for _, d := range index.Dependencies {
					if d.Status == "loaded" {
						loaded = true
					}
				}
				if !loaded {
					t.Fatalf("provider not loaded: %+v", index.Dependencies)
				}
				index.Subset = tc.subset
				var errors []string
				for _, d := range Analyze(index).Diagnostics {
					if d.Severity == diagnostic.Error {
						errors = append(errors, d.Message+" | "+d.NativeMessage)
					}
				}
				if !tc.reject && len(errors) != 0 {
					t.Fatalf("captured member rejected: %v", errors)
				}
				if tc.reject && !strings.Contains(strings.Join(errors, "\n"), "Invalid type: V036PkgOuter.Absent") {
					t.Fatalf("captured missing member accepted: %v; symbols=%+v; closed=%v", errors, index.Types, buildTypeMembers(index).completeDependencyNestedTypes)
				}
				if tc.subset {
					return
				}
				for _, position := range []struct{ name, body string }{
					{"parameter", "public void run(V036PkgOuter.%s value) {}"},
					{"local", "public void run() { V036PkgOuter.%s value; }"},
				} {
					t.Run(position.name, func(t *testing.T) {
						writeSemaFile(t, consumerFile, "public class V036Consumer { "+fmt.Sprintf(position.body, tc.member)+" }")
						var errors []string
						for _, d := range Analyze(typesys.Build(p, schema.Schema{})).Diagnostics {
							if d.Severity == diagnostic.Error {
								errors = append(errors, d.Message+" | "+d.NativeMessage)
							}
						}
						if !tc.reject && len(errors) != 0 {
							t.Fatalf("captured member rejected: %v", errors)
						}
						if tc.reject && !strings.Contains(strings.Join(errors, "\n"), "Invalid type: V036PkgOuter.Absent") {
							t.Fatalf("captured missing member accepted: %v", errors)
						}
					})
				}
			})
		}
	}
}

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

// Installed-package native rows include both a real member and two misses.
// Only indexed provider source establishes the member's identity offline; a
// separate missing alias must not open every other root in the project.
func TestNamespacePackageMembershipNativeBoundaries(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "namespace_unlocked_subscriber")
	provider := filepath.Join(fixture, "provider", "package", "main", "default", "classes", "V036PkgOuter.cls")
	providerSource, err := os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named"} {
			for _, tc := range []struct{ id, anon, named, want string }{
				{"member", "box.apex", "V036NBox.cls", ""},
				{"missing-inner", "missing_inner.apex", "V036NMissingInner.cls", "Invalid type: V036PkgOuter.Absent"},
				{"missing-outer", "missing_outer.apex", "V036NMissingOuter.cls", "Invalid type: V036AbsentOuter.Box"},
			} {
				t.Run(api+"/"+route+"/"+tc.id, func(t *testing.T) {
					path := filepath.Join(fixture, "installed", "anon", tc.anon)
					root := t.TempDir()
					localProvider := filepath.Join(root, "V036PkgOuter.cls")
					writeSemaFile(t, localProvider, string(providerSource))
					p := project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{localProvider}}
					if route == "named" {
						path = filepath.Join(fixture, "installed", "named"+api[:2], tc.named)
						source, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						path = filepath.Join(root, tc.named)
						writeSemaFile(t, path, string(source))
						p.ApexFiles = append(p.ApexFiles, path)
					}
					source, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					index := typesys.Build(p, schema.Schema{})
					if index.HasErrors() {
						t.Fatalf("index: %v", index.Diagnostics)
					}
					index.Dependencies = []typesys.DependencyInfo{{Namespace: "synthetic-unavailable", Status: "missing"}}
					var ds []diagnostic.Diagnostic
					if route == "anonymous" {
						ds = AnalyzeAnonymous(index, string(source), api).Diagnostics
					} else {
						ds = Analyze(index).Diagnostics
					}
					var errors []string
					for _, d := range ds {
						if d.Severity == diagnostic.Error {
							errors = append(errors, d.Message+" | "+d.NativeMessage)
						}
					}
					if tc.want == "" && len(errors) != 0 {
						t.Fatalf("captured member rejected: %v", errors)
					}
					if tc.want != "" && !strings.Contains(strings.Join(errors, "\n"), tc.want) {
						t.Fatalf("want %q, got %v", tc.want, errors)
					}
				})
			}
		}
		t.Run(api+"/unrelated-namespace", func(t *testing.T) {
			root := t.TempDir()
			localProvider := filepath.Join(root, "V036PkgOuter.cls")
			writeSemaFile(t, localProvider, string(providerSource))
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{localProvider}}, schema.Schema{})
			index.Dependencies = []typesys.DependencyInfo{{Namespace: "synthetic-unavailable", Status: "missing"}}
			for _, d := range AnalyzeAnonymous(index, "MissingNsV035.Thing value;", api).Diagnostics {
				if d.Severity == diagnostic.Error && d.Message == "Invalid type: MissingNsV035.Thing" {
					return
				}
			}
			t.Fatal("unrelated missing namespace accepted with an opaque package alias")
		})
	}
}

func TestNamespaceUnresolvedAliasDoesNotDeclareNamespace(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "force-app"), 0755); err != nil {
				t.Fatal(err)
			}
			writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"GhostPkg"}]}]}`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, schema.Schema{})
			for _, d := range AnalyzeAnonymous(index, "GhostPkg.Missing value;", api).Diagnostics {
				if d.Severity == diagnostic.Error && d.Message == "Invalid type: GhostPkg.Missing" {
					return
				}
			}
			t.Fatal("unresolved package alias was treated as a namespace")
		})
	}
}

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

// Foreign-subscriber controls: namespace_subscriber_members/org.tsv:24,25,
// 80,81. A package-owned global inner type is not available unqualified in a
// different namespace. A caller in the provider's own namespace is the
// positive control (namespace_sweep_same_namespace/org.tsv:2,4).
func TestSameNamespaceDependencyDoesNotLeakIntoForeignSource(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			foreignRoot := filepath.Join(root, "foreign")
			providerRoot := filepath.Join(root, "provider")
			for _, dir := range []string{foreignRoot, providerRoot} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			provider := filepath.Join(providerRoot, "V035Provider.cls")
			same := filepath.Join(root, "ScopeSame.cls")
			foreign := filepath.Join(foreignRoot, "ScopeForeign.cls")
			writeSemaFile(t, provider, "global class V035Provider { global class GlobalInner {} }")
			writeSemaFile(t, same, "public class ScopeSame { public static void check() { V035Provider.GlobalInner value; } }")
			writeSemaFile(t, foreign, "public class ScopeForeign { public static void check() { V035Provider.GlobalInner value; } }")
			providerProject := project.Project{Root: providerRoot, Namespace: "host", SourceAPIVersion: api, ApexFiles: []string{provider}}
			p := project.Project{
				Root: root, Namespace: "host", SourceAPIVersion: api,
				SourceNamespaces: []string{"foreign"}, SourceNamespaceRoots: map[string]string{foreignRoot: "foreign"},
				ApexFiles:                  []string{same, foreign},
				ManagedPackageDependencies: []project.ManagedPackageDependency{{Namespace: "host", SourceRoot: providerRoot, Status: "loaded", Project: &providerProject}},
			}
			index := typesys.Build(p, schema.Schema{})
			if index.HasErrors() {
				t.Fatalf("index errors: %+v", index.Diagnostics)
			}
			loaded := false
			for _, dep := range index.Dependencies {
				loaded = loaded || dep.Status == "loaded" && strings.EqualFold(dep.Namespace, "host")
			}
			if !loaded {
				t.Fatal("same-namespace provider source was not loaded")
			}
			want := "Invalid type: V035Provider.GlobalInner"
			foreignRejected := false
			for _, d := range Analyze(index).Diagnostics {
				if filepath.Clean(d.File) == filepath.Clean(same) && (d.Severity == diagnostic.Error || strings.Contains(d.Message, "V035Provider") || strings.Contains(d.NativeMessage, "V035Provider")) {
					t.Errorf("same-namespace source rejected captured positive: %+v", d)
				}
				if d.Severity != diagnostic.Error {
					continue
				}
				if filepath.Clean(d.File) == filepath.Clean(foreign) && (d.Message == want || d.NativeMessage == want) {
					foreignRejected = true
				}
			}
			if !foreignRejected {
				t.Fatal("foreign source accepted the provider's unqualified nested type")
			}
		})
	}
}

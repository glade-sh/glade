package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The source resolver and receiver scan are analysis-local. Reusing an
// Analyzer must observe both source edits and the effective API version.
func TestReceiverAnalysisRefresh(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "RefreshProbe.cls")
	analyzer := NewAnalyzer()
	for _, tc := range []struct {
		api, separator string
		wantError      bool
	}{
		{"67.0", "", false},
		{"67.0", "\n", true},
		{"68.0", "\n", false},
		{"67.0", "", false},
	} {
		writeSemaFile(t, path, "public class RefreshProbe { public static void f() {} public static void run() { RefreshProbe."+tc.separator+"f(); } }")
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: tc.api, ApexFiles: []string{path}}, schema.Schema{})
		result := analyzer.Analyze(index)
		found := false
		for _, d := range result.Diagnostics {
			if d.NativeMessage == "Missing ';' at 'f'" {
				found = true
			}
		}
		if found != tc.wantError {
			t.Errorf("api %s separator %q: diagnostics %v", tc.api, tc.separator, result.Diagnostics)
		}
	}
}

// A source occurrence is identified by its path, namespace and remap. A new
// analysis obtains a fresh captured-source snapshot.
func TestReceiverSourceResolverIdentity(t *testing.T) {
	files := map[string]string{
		"first.cls":  "%%%NAMESPACE_DOT%%%Thing.\ncall(); Alpha.Probe.f();",
		"second.cls": "%%%NAMESPACE_DOT%%%Other.\ncall(); Alpha.Probe.f();",
	}
	resolver := func() *semaSources {
		return newSemaSourcesWithCaptured(nil, func(path string) (string, bool) { value, ok := files[path]; return value, ok }, nil)
	}
	sources := resolver()
	for _, tc := range []struct {
		file, namespace, remap, want string
	}{
		{"first.cls", "one", "Beta", "one.Thing.\ncall(); Beta.Probe.f();"},
		{"first.cls", "two", "Beta", "two.Thing.\ncall(); Beta.Probe.f();"},
		{"first.cls", "one", "Gamma", "one.Thing.\ncall(); Gamma.Probe.f();"},
		{"second.cls", "one", "Beta", "one.Other.\ncall(); Beta.Probe.f();"},
	} {
		typ := typesys.TypeSymbol{File: tc.file, Namespace: tc.namespace, SourceNamespaceRemaps: []namespaceremap.Rule{{From: "Alpha", To: tc.remap}}}
		got, ok := sources.normalizedForType(typ)
		if !ok || got != tc.want {
			t.Errorf("%s/%s/%s: got %q, want %q", tc.file, tc.namespace, tc.remap, got, tc.want)
		}
		if len(scanReceiverTrivia(got)) != 1 {
			t.Errorf("%s: receiver break not found", tc.file)
		}
	}
	files["first.cls"] = strings.ReplaceAll(files["first.cls"], "Thing", "Changed")
	fresh := resolver()
	got, ok := fresh.normalizedForType(typesys.TypeSymbol{File: "first.cls", Namespace: "one"})
	if !ok || !strings.Contains(got, "one.Changed.\ncall()") {
		t.Errorf("fresh snapshot = %q", got)
	}
}

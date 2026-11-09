package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Test the same synthetic local-type rejection through the project test runner
// and anonymous semantic entry point. Native observations are in
// internal/sema/testdata/namespace_cross_owner/org.tsv.
func TestCrossNamespaceOwnerRunnerRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, wrong := range []string{"host.ForeignShape", "other.LocalShape"} {
			t.Run(api+"/"+wrong, func(t *testing.T) {
				root := t.TempDir()
				child := filepath.Join(root, "child")
				local := filepath.Join(root, "LocalShape.cls")
				foreign := filepath.Join(child, "ForeignShape.cls")
				test := filepath.Join(root, "CrossOwnerTest.cls")
				writeFile(t, local, "public class LocalShape {}")
				writeFile(t, foreign, "public class ForeignShape {}")
				writeFile(t, test, "@IsTest private class CrossOwnerTest { @IsTest static void check() { "+wrong+" value; } }")
				for _, file := range []string{local, foreign, test} {
					writeFile(t, file+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				}
				idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: api, SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{local, foreign, test}}, schema.Schema{})
				if idx.HasErrors() {
					t.Fatal(idx.Diagnostics)
				}
				if result := sema.AnalyzeAnonymous(idx, wrong+" value;", api); !result.HasErrors() {
					t.Fatal("anonymous accepted cross-namespace suffix")
				}
				for _, warm := range []bool{false, true} {
					result := Run(idx, Options{NoDiskCache: !warm, Parallelism: 1, SelectedClasses: []string{"CrossOwnerTest"}})
					if result.Summary().CompileErrors != 1 || !strings.Contains(firstRunProblem(result), "Invalid type: "+wrong) {
						t.Fatalf("warm=%v summary=%+v problem=%s", warm, result.Summary(), firstRunProblem(result))
					}
				}
			})
		}
	}
}

func TestCrossNamespaceLoadedDependencyRunner(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			child := filepath.Join(root, "child")
			local := filepath.Join(root, "LocalShape.cls")
			foreign := filepath.Join(child, "ForeignShape.cls")
			test := filepath.Join(root, "CrossOwnerTest.cls")
			writeFile(t, local, "public class LocalShape {}")
			writeFile(t, foreign, "public class ForeignShape {}")
			writeFile(t, test, "@IsTest private class CrossOwnerTest { @IsTest static void check() { other.LocalShape value; } }")
			for _, file := range []string{local, foreign, test} {
				writeFile(t, file+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: api, SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{local, foreign, test}}, schema.Schema{})
			idx.Dependencies = []typesys.DependencyInfo{{Namespace: "other", Status: "loaded", SourceRoot: child}}
			for _, warm := range []bool{false, true} {
				result := Run(idx, Options{NoDiskCache: !warm, Parallelism: 1, SelectedClasses: []string{"CrossOwnerTest"}})
				if result.Summary().CompileErrors != 1 || !strings.Contains(firstRunProblem(result), "Invalid type: other.LocalShape") {
					t.Fatalf("warm=%v summary=%+v problem=%s", warm, result.Summary(), firstRunProblem(result))
				}
			}
		})
	}
}

func TestCrossNamespaceSameNameRunner(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			child := filepath.Join(root, "child")
			local := filepath.Join(root, "SharedShape.cls")
			foreign := filepath.Join(child, "SharedShape.cls")
			test := filepath.Join(root, "TwinTest.cls")
			writeFile(t, local, "public class SharedShape {}")
			writeFile(t, foreign, "public class SharedShape {}")
			writeFile(t, test, "@IsTest private class TwinTest { @IsTest static void check() { host.SharedShape first; other.SharedShape second; } }")
			for _, file := range []string{local, foreign, test} {
				writeFile(t, file+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: api, SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{local, foreign, test}}, schema.Schema{})
			if idx.HasErrors() {
				t.Fatal(idx.Diagnostics)
			}
			for _, warm := range []bool{false, true} {
				result := Run(idx, Options{NoDiskCache: !warm, Parallelism: 1, SelectedClasses: []string{"TwinTest"}})
				if result.Summary().Passed != 1 || result.Summary().CompileErrors != 0 {
					t.Fatalf("warm=%v summary=%+v problem=%s", warm, result.Summary(), firstRunProblem(result))
				}
			}
		})
	}
}

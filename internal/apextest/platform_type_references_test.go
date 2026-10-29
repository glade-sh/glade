package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Native: ../../testdata/conformance/platform_type_references/org.tsv,
// platform_types at API 62/67, named, anonymous and @IsTest. Combining these
// exact declarations with the existing synthetic project pins cross-file
// field/parameter/local/generic/catch resolution, plugin inheritance, interface
// implementation and Schema.AggregateResult query iteration end to end.
func TestCorpusShapePlatformTypeReferences(t *testing.T) {
	const fixture = "../../testdata/conformance/platform_type_references"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyCorpusShapeProject(t, api)
			files, err := filepath.Glob(filepath.Join(fixture, "*.cls"))
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 4 {
				t.Fatalf("fixture classes=%d", len(files))
			}
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "force-app/main/default/classes", filepath.Base(file))
				writeFile(t, path, string(data))
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			t.Run("project-check", func(t *testing.T) {
				if r := sema.Analyze(index); r.HasErrors() {
					t.Fatal(r.Diagnostics)
				}
			})
			t.Run("anonymous", func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				if r := sema.AnalyzeAnonymous(index, string(data), api); r.HasErrors() {
					t.Fatal(r.Diagnostics)
				}
			})
			t.Run("runner", func(t *testing.T) {
				run := Run(index, Options{NoDiskCache: true, Parallelism: 1, SelectedClasses: []string{"PlatformTypeReferencesTest"}})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 || s.CompileErrors != 0 {
					t.Fatalf("summary=%+v excluded=%+v problem=%s", s, run.ExcludedClasses, firstRunProblem(run))
				}
			})
		})
	}
}

// Native invalid_system @IsTest rows reject this body at both source APIs.
func TestPlatformTypeReferencesRunnerInvalidSystem(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{
				"PlatformInvalidTest": "@IsTest private class PlatformInvalidTest { @IsTest static void observe() { System.NotAType value = null; } }",
				"PlatformValidTest":   "@IsTest private class PlatformValidTest { @IsTest static void observe() { System.assert(true); } }",
			}, nil)
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			for _, entry := range run.ExcludedClasses {
				if entry.ClassName == "PlatformInvalidTest" && strings.Contains(entry.Message, "Invalid type: System.NotAType") {
					if run.Summary().Passed != 1 {
						t.Fatal(run.Summary())
					}
					return
				}
			}
			t.Fatalf("native rejection missing: excluded=%+v summary=%+v", run.ExcludedClasses, run.Summary())
		})
	}
}

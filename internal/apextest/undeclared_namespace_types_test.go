package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

// Synthetic: ../../testdata/conformance/undeclared_namespace_types/cases.md.
// A namespace-less project references a managed namespace (bar) and an
// undeclared qualifier (fooNs) that it never declares. Ruling 49 reverses the
// v0.3.1 deferral that compiled BarConsumerTest: an undeclared namespace is
// Invalid type in every position, as an org without the package rejects it
// (namespace_external_positions/org.tsv). Declaring both namespaces as
// dependencies with no source keeps them open (no Invalid type); the missing
// source is then the dependency_missing environment error.
func TestUndeclaredNamespaceTypesProjectRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/undeclared_namespace_types"
	anonymous, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, variant := range []struct{ namespace, glade string }{
			{"", ""},
			{"fooNs", ""},
			{"", "project:\n  managedPackageDependencies: [\"bar:../absent-bar:1.0\", \"fooNs:../absent-fooNs:1.0\"]\n"},
		} {
			declared := variant.glade != ""
			t.Run(fmt.Sprintf("%s/namespace=%s/declared=%v", api, variant.namespace, declared), func(t *testing.T) {
				root := t.TempDir()
				if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
					t.Fatal(err)
				}
				metas, err := filepath.Glob(filepath.Join(root, "force-app/main/default/classes/*.cls-meta.xml"))
				if err != nil || len(metas) != 5 {
					t.Fatalf("class metadata=%d err=%v", len(metas), err)
				}
				for _, meta := range metas {
					data, err := os.ReadFile(meta)
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, meta, strings.Replace(string(data), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>", 1))
				}
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "`+variant.namespace+`", "packageDirectories": [{"path": "force-app", "default": true}], "sourceApiVersion": "`+api+`"}`)
				if declared {
					writeFile(t, filepath.Join(root, "glade.yml"), variant.glade)
				}
				index := loadTestIndex(t, root)
				if !declared && index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				run := Run(index, Options{NoDiskCache: true, Parallelism: 1, SelectedClasses: []string{"BarConsumerTest"}})
				if declared {
					// A declared dependency without source is the
					// dependency_missing environment error (missing source =
					// ENV), and its namespace stays open: no Invalid type.
					for route, ds := range map[string][]diagnostic.Diagnostic{
						"project":   sema.Analyze(index).Diagnostics,
						"anonymous": sema.AnalyzeAnonymous(index, string(anonymous), api).Diagnostics,
					} {
						for _, d := range ds {
							if d.Severity == diagnostic.Error && d.Code != "dependency_missing" {
								t.Errorf("declared %s: unexpected error %s %q", route, d.Code, nativeDiagnosticText(d))
							}
						}
					}
					for _, entry := range run.ExcludedClasses {
						if strings.Contains(entry.Message, "Invalid type") {
							t.Errorf("declared run excluded %s for %q", entry.ClassName, entry.Message)
						}
					}
					if s := run.Summary(); s.Total != 1 || s.Passed != 0 || s.CompileErrors != 1 || len(run.ExcludedClasses) == 0 {
						t.Fatalf("declared summary=%+v excluded=%+v problem=%s", s, run.ExcludedClasses, firstRunProblem(run))
					}
					return
				}
				undeclaredErrors := func(route string, ds []diagnostic.Diagnostic) {
					t.Helper()
					count := 0
					for _, d := range ds {
						if d.Severity != diagnostic.Error {
							continue
						}
						count++
						ref, ok := strings.CutPrefix(nativeDiagnosticText(d), "Invalid type: ")
						if !ok || !strings.HasPrefix(ref, "bar.") && ref != "fooNs.Unknown" && !(variant.namespace == "" && ref == "fooNs.LocalThing") {
							t.Errorf("%s: unexpected error %s %q", route, d.Code, nativeDiagnosticText(d))
						}
					}
					if count == 0 {
						t.Errorf("%s: undeclared namespaces accepted", route)
					}
				}
				undeclaredErrors("project", sema.Analyze(index).Diagnostics)
				undeclaredErrors("anonymous", sema.AnalyzeAnonymous(index, string(anonymous), api).Diagnostics)
				if s := run.Summary(); s.Total != 1 || s.Passed != 0 || s.CompileErrors != 1 {
					t.Fatalf("undeclared summary=%+v excluded=%+v problem=%s", s, run.ExcludedClasses, firstRunProblem(run))
				}
			})
		}
	}
}

// Proven absence stays red in the runner: System's inventory is closed
// (native invalid_system, testdata/conformance/platform_type_references/org.tsv),
// and so is an undeclared namespace (ruling 49; native
// namespace_external_positions A02_new). Each class is excluded on its own.
func TestUndeclaredNamespaceRunnerKeepsSystemAbsence(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{
				"ManagedDeferredTest": "@IsTest private class ManagedDeferredTest { @IsTest static void observe() { Object value = null; if (value != null) { value = new bar.Wrapper(); } System.assertEquals(null, value); } }",
				"SystemAbsentTest":    "@IsTest private class SystemAbsentTest { @IsTest static void observe() { Object value = new System.NotAType(); } }",
			}, nil)
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			systemRejected, managedRejected := false, false
			for _, entry := range run.ExcludedClasses {
				managedRejected = managedRejected || entry.ClassName == "ManagedDeferredTest" && strings.Contains(entry.Message, "Invalid type: bar.Wrapper")
				systemRejected = systemRejected || entry.ClassName == "SystemAbsentTest" && strings.Contains(entry.Message, "System.NotAType")
			}
			if !systemRejected || !managedRejected || len(run.ExcludedClasses) != 2 || run.Summary().Passed != 0 {
				t.Fatalf("excluded=%+v summary=%+v problem=%s", run.ExcludedClasses, run.Summary(), firstRunProblem(run))
			}
		})
	}
}

package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Synthetic: ../../testdata/conformance/undeclared_namespace_types/cases.md.
// A namespace-less project references an installed managed namespace (bar)
// and an undeclared qualifier (fooNs) that it never declares. Glade a recorded revision
// compiled and ran BarConsumerTest; the project route must still do so.
func TestUndeclaredNamespaceTypesProjectRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/undeclared_namespace_types"
	anonymous, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "fooNs"} {
			t.Run(api+"/namespace="+namespace, func(t *testing.T) {
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
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "`+namespace+`", "packageDirectories": [{"path": "force-app", "default": true}], "sourceApiVersion": "`+api+`"}`)
				index := loadTestIndex(t, root)
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				if r := sema.Analyze(index); r.HasErrors() {
					t.Fatalf("project check: %+v", r.Diagnostics)
				}
				if r := sema.AnalyzeAnonymous(index, string(anonymous), api); r.HasErrors() {
					t.Fatalf("anonymous: %+v", r.Diagnostics)
				}
				run := Run(index, Options{NoDiskCache: true, Parallelism: 1, SelectedClasses: []string{"BarConsumerTest"}})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 || s.CompileErrors != 0 || len(run.ExcludedClasses) != 0 {
					t.Fatalf("summary=%+v excluded=%+v problem=%s", s, run.ExcludedClasses, firstRunProblem(run))
				}
			})
		}
	}
}

// Proven absence stays red in the runner: System's inventory is closed
// (native invalid_system, testdata/conformance/platform_type_references/org.tsv).
func TestUndeclaredNamespaceRunnerKeepsSystemAbsence(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{
				"ManagedDeferredTest": "@IsTest private class ManagedDeferredTest { @IsTest static void observe() { Object value = null; if (value != null) { value = new bar.Wrapper(); } System.assertEquals(null, value); } }",
				"SystemAbsentTest":    "@IsTest private class SystemAbsentTest { @IsTest static void observe() { Object value = new System.NotAType(); } }",
			}, nil)
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			rejected := false
			for _, entry := range run.ExcludedClasses {
				if entry.ClassName == "ManagedDeferredTest" {
					t.Fatalf("managed construction excluded: %+v", entry)
				}
				rejected = rejected || entry.ClassName == "SystemAbsentTest" && strings.Contains(entry.Message, "System.NotAType")
			}
			if !rejected || run.Summary().Passed != 1 {
				t.Fatalf("excluded=%+v summary=%+v problem=%s", run.ExcludedClasses, run.Summary(), firstRunProblem(run))
			}
		})
	}
}

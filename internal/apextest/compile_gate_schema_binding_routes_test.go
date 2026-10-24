package apextest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSchemaReferenceGateBindingPolicyRoutes(t *testing.T) {
	for _, tc := range []struct {
		route, namespace, object, source, shadow string
		blocked                                  bool
	}{
		{"local_query", "pkg", "pkg__Parcel__c", `public class Probe { Object x=[SELECT Id FROM Parcel__c]; }`, "", true},
		{"local_trigger", "pkg", "pkg__Parcel__c", `trigger Probe on Parcel__c (before insert) {}`, "", true},
		{"foreign_namespace", "other", "pkg__Parcel__c", `public class Probe { Parcel__c x; }`, "", false},
		{"apex_class_shadow", "", "Account", `public class Probe { Account x; }`, "Account", false},
		{"namespace_qualified_apex_class_shadow", "pkg", "Account", `public class Probe { pkg.Account x; }`, "Account", false},
		{"namespace_qualified_static_class_shadow", "pkg", "Account", `public class Probe { Object x=pkg.Account.someStaticMember; }`, "Account", false},
		{"explicit_schema_bypasses_class", "", "Account", `public class Probe { Schema.Account x; }`, "Account", true},
		{"schema_class_shadow", "", "Parcel__c", `public class Probe { Object x=Schema.Parcel__c; }`, "Schema", false},
	} {
		t.Run(tc.route, func(t *testing.T) {
			root := t.TempDir()
			ext := ".cls"
			if tc.route == "local_trigger" {
				ext = ".trigger"
			}
			path := filepath.Join(root, "Probe"+ext)
			writeFile(t, path, tc.source)
			paths := []string{path}
			if tc.shadow != "" {
				shadow := filepath.Join(root, tc.shadow+".cls")
				writeFile(t, shadow, "public class "+tc.shadow+" {}")
				paths = append(paths, shadow)
			}
			metadata := schemaReferenceGateMetadata()
			metadata.Objects[0].Name = tc.object
			index := typesys.Build(project.Project{Root: root, Namespace: tc.namespace, ApexFiles: paths}, metadata)
			var diagnostics []diagnostic.Diagnostic
			for _, d := range sema.Analyze(index).Diagnostics {
				if d.Code == "GLADESEMA003" {
					diagnostics = append(diagnostics, d)
				}
			}
			if len(diagnostics) == 0 {
				t.Fatal("missing lookup error")
			}
			byFile, ok := (&compileGate{}).attributeDependencyDiagnostics(newCompileGateUnits(index), diagnostics, newSourceCache())
			if !ok || (len(byFile[path]) > 0) != tc.blocked {
				t.Fatalf("schema binding ok=%t files=%v want blocked=%t", ok, byFile, tc.blocked)
			}
		})
	}
}

func TestSchemaReferenceGateUnknownErrorRunnerRoute(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "IndependentTest.cls")
	writeFile(t, path, `@IsTest private class IndependentTest { @IsTest static void observe() { System.assertEquals(7,7); } }`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schemaReferenceGateMetadata())
	index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA003", Message: "unattributed schema error"})
	run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true}, nil)
	if run.Summary().CompileErrors != 1 || run.Summary().Passed != 0 || run.Suites[0].Cases[0].Status != testreport.StatusCompileError {
		t.Fatalf("unknown error bypassed global fallback: %+v", run)
	}
}

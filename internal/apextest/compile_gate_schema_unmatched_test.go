package apextest

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// The unused_metadata native rows reject metadata while unrelated tests run.
// Project compile errors must survive even when no Apex unit owns the object.
func TestSchemaReferenceGateUnmatchedDiagnosticRoutes(t *testing.T) {
	for _, file := range []string{"", "objects/Account/fields/Destination__c.field-meta.xml"} {
		t.Run("metadata_file_"+file, func(t *testing.T) {
			gate := &compileGate{started: time.Now()}
			d := diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA003", SchemaObject: "Account", File: file, Message: "lookup reference target UnavailableDestination does not exist"}
			byFile, ok := gate.attributeDependencyDiagnostics(newCompileGateUnits(typesys.Index{}), []diagnostic.Diagnostic{d}, newSourceCache())
			if !ok || len(byFile) != 0 {
				t.Fatalf("unmatched error should not exclude a class: ok=%t files=%v", ok, byFile)
			}
			run := gate.merge(nil, nil, testreport.Run{})
			if run.Summary().OK() || run.Summary().CompileErrors != 1 || !strings.Contains(run.Suites[0].Cases[0].Problem.Message, d.Message) {
				t.Fatalf("unmatched schema diagnostic disappeared from report: %+v", run)
			}
			if merged := gate.merge(nil, nil, run); merged.Summary().CompileErrors != 1 {
				t.Fatalf("outer subset merge duplicated or lost the project error: %+v", merged)
			}
		})
	}
}

func TestSchemaReferenceGateUnmatchedRunnerProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, selected := range []string{"all", "single", "none"} {
			t.Run(api+"/"+selected, func(t *testing.T) {
				root := t.TempDir()
				var paths []string
				for _, name := range []string{"IndependentTest", "OtherTest"} {
					path := filepath.Join(root, name+".cls")
					writeFile(t, path, "@IsTest private class "+name+" { @IsTest static void observe() { System.assertEquals(7,7); } }")
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schemaRelationshipGateMetadata("Account"))
				cases := Discover(index, Options{})
				wantPass := 2
				if selected == "single" {
					cases = cases[:1]
					wantPass = 1
				}
				if selected == "none" {
					cases = []TestCase{}
					wantPass = 0
				}
				for _, cached := range []bool{false, true, true} {
					run := RunCasesContext(context.Background(), index, Options{NoDiskCache: !cached}, cases)
					s := run.Summary()
					if s.OK() || s.Passed != wantPass || s.CompileErrors != 1 || len(run.ExcludedClasses) != 0 {
						t.Fatalf("cached=%t unmatched metadata must retain project error and independent passes: %+v run=%+v", cached, s, run)
					}
				}
			})
		}
	}
}

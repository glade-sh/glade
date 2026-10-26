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

func TestSchemaReferenceGateSchemaTokenRoutes(t *testing.T) {
	for _, tc := range []struct{ route, expression, namespace string }{
		{"sobject_type_token", "Schema.SObjectType.Parcel__c", ""},
		{"sobject_field_token", "Schema.SObjectType.Parcel__c.fields.Destination__c", ""},
		{"local_namespace_type", "new Parcel__c()", "pkg"},
		{"qualified_namespace_type", "new pkg__Parcel__c()", "pkg"},
		{"change_event_type", "new Parcel__ChangeEvent()", ""},
	} {
		t.Run(tc.route, func(t *testing.T) {
			root := t.TempDir()
			var paths []string
			for name, source := range map[string]string{
				"TokenUse":        `public class TokenUse { public static Object value() { return ` + tc.expression + `; } }`,
				"TokenTest":       `@IsTest private class TokenTest { @IsTest static void observe() { System.assertNotEquals(null,TokenUse.value()); } }`,
				"IndependentTest": `@IsTest private class IndependentTest { @IsTest static void observe() { System.assertEquals(7,7); } }`,
			} {
				path := filepath.Join(root, name+".cls")
				writeFile(t, path, source)
				paths = append(paths, path)
			}
			metadata := schemaReferenceGateMetadata()
			if tc.namespace != "" {
				metadata.Objects[0].Name = tc.namespace + "__Parcel__c"
			}
			index := typesys.Build(project.Project{Root: root, Namespace: tc.namespace, SourceAPIVersion: "67.0", ApexFiles: paths}, metadata)
			var diagnostics []diagnostic.Diagnostic
			for _, d := range sema.Analyze(index).Diagnostics {
				if d.Code == "GLADESEMA003" {
					diagnostics = append(diagnostics, d)
				}
			}
			gate := &compileGate{}
			byFile, ok := gate.attributeDependencyDiagnostics(newCompileGateUnits(index), diagnostics, newSourceCache())
			if !ok || len(byFile[filepath.Join(root, "TokenUse.cls")]) == 0 {
				t.Errorf("schema token attribution ok=%t files=%v", ok, byFile)
			}
			run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true}, nil)
			statuses := map[string]testreport.Status{}
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					statuses[c.ClassName] = c.Status
				}
			}
			if len(statuses) != 3 || statuses["IndependentTest"] != testreport.StatusPass || statuses["TokenTest"] != testreport.StatusCompileError || statuses["project"] != testreport.StatusCompileError {
				t.Fatalf("token gate statuses=%v", statuses)
			}
		})
	}
}

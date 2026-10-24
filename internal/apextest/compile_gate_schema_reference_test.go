package apextest

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// Lookup validity is pinned by sema/testdata/schema_reference_gate/org.tsv.
// Attributing an invalid schema object to static users is local gate policy.
func TestSchemaReferenceGateAttributionRoutes(t *testing.T) {
	for _, tc := range []struct {
		route, source string
		blocked       bool
	}{
		{"declared_type", `public class Probe { Parcel__c item; }`, true},
		{"schema_qualified_type", `public class Probe { Schema.Parcel__c item; }`, true},
		{"constructor", `public class Probe { Object item = new Parcel__c(); }`, true},
		{"field_token", `public class Probe { Object item = Parcel__c.Destination__c; }`, true},
		{"schema_field_token", `public class Probe { Object item = Schema.Parcel__c.Destination__c; }`, true},
		{"soql_from", `public class Probe { Object items = [SELECT Id FROM Parcel__c]; }`, true},
		{"sosl_returning", `public class Probe { Object items = [FIND 'parcel' RETURNING Parcel__c(Id)]; }`, true},
		{"soql_bind", `public class Probe { Object items = [SELECT Id FROM Account WHERE Id = :Parcel__c.Destination__c]; }`, true},
		{"trigger_object", `trigger Probe on Parcel__c (before insert) { }`, true},
		{"trigger_helper", `trigger Probe on Account (before insert) { Parcel__c item; }`, true},
		{"string_and_comment", `public class Probe { String item = 'Parcel__c'; /* Parcel__c */ }`, false},
		{"dynamic_type", `public class Probe { Object item = Type.forName('Parcel__c'); }`, false},
		{"shadowed_qualifier", `public class Probe { void run(String Parcel__c) { Parcel__c.length(); } }`, false},
		{"unrelated_query_field", `public class Probe { Object items = [SELECT Parcel__c FROM Account]; }`, false},
		{"unrelated_type", `public class Probe { Account item; }`, false},
	} {
		t.Run(tc.route, func(t *testing.T) {
			root := t.TempDir()
			ext := ".cls"
			if tc.route == "trigger_object" || tc.route == "trigger_helper" {
				ext = ".trigger"
			}
			path := filepath.Join(root, "Probe"+ext)
			writeFile(t, path, tc.source)
			index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schemaReferenceGateMetadata())
			var diagnostics []diagnostic.Diagnostic
			for _, d := range sema.Analyze(index).Diagnostics {
				if d.Code == "GLADESEMA003" {
					diagnostics = append(diagnostics, d)
				}
			}
			if len(diagnostics) == 0 {
				t.Fatal("missing schema rejection")
			}
			gate := &compileGate{}
			byFile, ok := gate.attributeDependencyDiagnostics(newCompileGateUnits(index), diagnostics, newSourceCache())
			if !ok || (len(byFile[path]) > 0) != tc.blocked {
				t.Fatalf("schema attribution ok=%t files=%v; blocked=%t", ok, byFile, tc.blocked)
			}
			if remaining := gate.withoutScopedDependencyErrors(diagnostics); len(remaining) != 0 {
				t.Fatalf("already attributed schema errors survived subset rerun: %v", remaining)
			}
		})
	}
}

func schemaReferenceGateMetadata() schema.Schema {
	return schema.Schema{Objects: []schema.Object{{Name: "Parcel__c", Fields: []schema.Field{{
		Name: "Destination__c", Type: "Lookup", ReferenceTo: []string{"UnavailableDestination"},
	}}}}}
}

func TestSchemaReferenceGateFilelessRunnerProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			var paths []string
			for name, source := range map[string]string{
				"ParcelUse.cls":         `public class ParcelUse { public static Boolean value() { Parcel__c item = new Parcel__c(); return item.Destination__c == null; } }`,
				"ParcelTest.cls":        `@IsTest private class ParcelTest { @IsTest static void observe() { System.assert(ParcelUse.value()); } }`,
				"ParcelTrigger.trigger": `trigger ParcelTrigger on Parcel__c (before insert) { }`,
				"IndependentTest.cls":   `@IsTest private class IndependentTest { @IsTest static void observe() { System.assertEquals(7,7); } }`,
			} {
				path := filepath.Join(root, name)
				writeFile(t, path, source)
				paths = append(paths, path)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schemaReferenceGateMetadata())
			run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true}, nil)
			statuses := map[string]testreport.Status{}
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					statuses[c.ClassName] = c.Status
				}
			}
			if len(statuses) != 3 || statuses["IndependentTest"] != testreport.StatusPass || statuses["ParcelTest"] != testreport.StatusCompileError || statuses["project"] != testreport.StatusCompileError {
				t.Fatalf("fileless schema gate statuses=%v", statuses)
			}
			excluded := map[string]bool{}
			for _, entry := range run.ExcludedClasses {
				excluded[entry.ClassName] = true
			}
			if len(excluded) != 3 || !excluded["ParcelUse"] || !excluded["ParcelTest"] || !excluded["ParcelTrigger"] {
				t.Fatalf("exclusions=%v", excluded)
			}
		})
	}
}

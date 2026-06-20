package sema

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeSOQLQueryDiagnosticsUseSchemaResolution(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<Account> a = [SELECT Missing__c FROM Account];
    List<Account> b = [SELECT Owner.Missing__c FROM Account];
    List<Account> c = [SELECT Id, (SELECT LastName FROM BadContacts) FROM Account];
    List<Account> d = [SELECT Id FROM Missing__c];
  }
}
`
	result := analyzeQueryProbe(t, source, queryDiagnosticSchema())

	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_FIELD", "Missing__c", 4, 31)
	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_FIELD", "Owner.Missing__c", 5, 31)
	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "BadContacts", 6, 57)
	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_OBJECT", "Missing__c", 7, 39)
}

func TestAnalyzeSOQLRelationshipAndAggregateAliasDiagnostics(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<Account> badRelationship = [SELECT Bogus.Name FROM Account];
    AggregateResult okAlias = [SELECT COUNT(Id) total FROM Account ORDER BY total];
  }
}
`
	result := analyzeQueryProbe(t, source, queryDiagnosticSchema())

	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "Bogus.Name", 4, 45)
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "total")
}

func TestAnalyzeQuerySemanticsAcceptsKnownStandardObjectsWithoutProjectMetadata(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT Id, Name FROM Account];
    List<Contact> contacts = [SELECT Id, LastName FROM Contact];
    List<Profile> profiles = [SELECT Id, Name FROM Profile];
    List<CustomPermission> permissions = [SELECT Id FROM CustomPermission];
    List<ApexClass> classes = [SELECT Id, Name FROM ApexClass];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{
		{Name: "Expression_Function__mdt"},
	}})

	for _, objectName := range []string{"Account", "Contact", "Profile", "CustomPermission", "ApexClass"} {
		assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_OBJECT", objectName)
	}
}

func TestAnalyzeSOQLTypeofBranchObjectDiagnostics(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<Event> rows = [SELECT TYPEOF What WHEN Missing__c THEN Name END FROM Event];
  }
}
`
	result := analyzeQueryProbe(t, source, queryDiagnosticSchema())

	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_OBJECT", "Missing__c", 4, 49)
}

func TestAnalyzeSOSLReturningFieldDiagnostics(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    List<List<SObject>> rows = [FIND 'acme' RETURNING Account(Id, Missing__c), Missing__c(Id)];
  }
}
`
	result := analyzeQueryProbe(t, source, queryDiagnosticSchema())

	assertDiagnosticAt(t, result, "GLADESEMA_SOSL_FIELD", "Missing__c", 4, 67)
	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_OBJECT", "Missing__c", 4, 80)
}

func analyzeQueryProbe(t *testing.T, source string, sch schema.Schema) Result {
	t.Helper()
	root := t.TempDir()
	classPath := filepath.Join(root, "QueryProbe.cls")
	writeSemaFile(t, classPath, source)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, sch)
	return Analyze(index)
}

func queryDiagnosticSchema() schema.Schema {
	return schema.Schema{Objects: []schema.Object{
		{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "Name", Type: "Text"},
				{Name: "OwnerId", Type: "Lookup", RelationshipName: "Owner", ReferenceTo: []string{"User"}},
			},
		},
		{
			Name: "Contact",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "LastName", Type: "Text"},
				{Name: "AccountId", Type: "Lookup", ChildRelationshipName: "Contacts", ReferenceTo: []string{"Account"}},
			},
		},
		{
			Name: "Event",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "WhatId", Type: "Lookup", RelationshipName: "What", ReferenceTo: []string{"Account", "Contact"}},
			},
		},
		{
			Name: "User",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "Name", Type: "Text"},
			},
		},
	}}
}

func assertDiagnosticAt(t *testing.T, result Result, code, text string, line, column int) diagnostic.Diagnostic {
	t.Helper()
	var candidates []string
	for _, diag := range result.Diagnostics {
		if diag.Code != code || diag.Range == nil || !containsString(diag.Message, text) {
			continue
		}
		candidates = append(candidates, fmt.Sprintf("%s %d:%d %s", diag.Code, diag.Range.Start.Line, diag.Range.Start.Column, diag.Message))
		if diag.Range.Start.Line == line && diag.Range.Start.Column == column && containsString(diag.Message, text) {
			return diag
		}
	}
	t.Fatalf("missing diagnostic code=%s text=%q at %d:%d candidates=%#v all=%#v", code, text, line, column, candidates, result.Diagnostics)
	return diagnostic.Diagnostic{}
}

func assertNoDiagnosticContaining(t *testing.T, result Result, code, text string) {
	t.Helper()
	for _, diag := range result.Diagnostics {
		if diag.Code == code && containsString(diag.Message, text) {
			t.Fatalf("unexpected diagnostic code=%s text=%q in %#v", code, text, result.Diagnostics)
		}
	}
}

func containsString(s, sub string) bool {
	return len(sub) == 0 || (len(sub) <= len(s) && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

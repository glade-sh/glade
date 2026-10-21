package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native QD01 (inline_query_delete_results.json), API 62/67/68 both routes:
// a direct SOQL expression is a list argument to Database.delete, including
// LIMIT 1. An explicit SObject cast still selects the scalar overload.
func TestAnalyzeInlineQueryDeleteReturnCardinality(t *testing.T) {
	for _, tc := range []struct {
		name, result, expression string
		reject                   bool
	}{
		{"query", "List<Database.DeleteResult>", "Database.delete([SELECT Id FROM Account WHERE Name = :name])", false},
		{"limited query", "List<Database.DeleteResult>", "Database.delete([SELECT Id FROM Account LIMIT 1])", false},
		{"qualified receiver", "List<Database.DeleteResult>", "System.Database.delete([SELECT Id FROM Account])", false},
		{"partial query", "List<Database.DeleteResult>", "Database.delete([SELECT Id FROM Account], false)", false},
		{"indexed query control", "Database.DeleteResult", "Database.delete([SELECT Id FROM Account LIMIT 1][0])", false},
		{"indexed query wrong result control", "List<Database.DeleteResult>", "Database.delete([SELECT Id FROM Account LIMIT 1][0])", true},
		{"query wrong result", "Database.DeleteResult", "Database.delete([SELECT Id FROM Account])", true},
		{"named list control", "List<Database.DeleteResult>", "Database.delete(accounts)", false},
		{"new list control", "List<Database.DeleteResult>", "Database.delete(new List<Account>{account})", false},
		{"scalar control", "Database.DeleteResult", "Database.delete(account)", false},
		{"cast control", "Database.DeleteResult", "Database.delete((Account)[SELECT Id FROM Account LIMIT 1])", false},
		{"scalar wrong result control", "List<Database.DeleteResult>", "Database.delete(account)", true},
		{"cast wrong result control", "List<Database.DeleteResult>", "Database.delete((Account)[SELECT Id FROM Account LIMIT 1])", true},
		{"list wrong result control", "Database.DeleteResult", "Database.delete(accounts)", true},
		{"wrong result family control", "List<Database.SaveResult>", "Database.delete(accounts)", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "QueryDeleteProbe.cls")
			writeSemaFile(t, path, "public class QueryDeleteProbe { public static "+tc.result+" erase(String name, List<Account> accounts, Account account) { return "+tc.expression+"; } }")
			index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}}})
			result := Analyze(index)
			if result.HasErrors() != tc.reject {
				t.Fatalf("errors=%v, want %v: %#v", result.HasErrors(), tc.reject, result.Diagnostics)
			}
			if tc.reject && !hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
				t.Fatalf("missing invalid return diagnostic: %#v", result.Diagnostics)
			}
		})
	}
}

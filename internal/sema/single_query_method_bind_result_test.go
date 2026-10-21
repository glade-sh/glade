package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSingleRowQueryWithMethodCallBindRetainsObjectType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "SingleQueryBindProbe.cls")
	writeSemaFile(t, path, `
public class SingleQueryBindProbe {
  public void run() {
    List<AggregateResult> totals = [SELECT COUNT(Id) total FROM Account];
    Account updated = [SELECT Name FROM Account WHERE Id = :getAccount().Id];
  }

  private Account getAccount() {
    return null;
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{
		Objects: []schema.Object{{Name: "Account"}},
	})
	result := Analyze(index)
	if got := semaSOQLLiteralType("[SELECT COUNT(Id) total FROM Account]"); got != "List<AggregateResult>" {
		t.Fatalf("aggregate query type = %q, want List<AggregateResult>", got)
	}
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA018" {
			t.Fatalf("single-row Account query should remain Account-typed: %#v", result.Diagnostics)
		}
	}
}

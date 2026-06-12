package enterpriseassess

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/enterprise"
	"github.com/glade-sh/glade/internal/enterprisegraph"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAssessEnterpriseComposedProducesSectionsAndLimitations(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	ctx, err := enterprise.LoadContext(root)
	if err != nil {
		t.Fatalf("LoadContext: %v", err)
	}

	report := Assess(ctx, enterprisegraph.Graph{}, Options{IncludeMetadata: true, IncludeTests: true})

	for _, id := range []string{"inventory", "top_risks", "trigger_map", "soql_dml_map", "async_callout_surface", "fflib_inventory", "test_health", "limitations"} {
		if !hasSection(report.Sections, id) {
			t.Fatalf("missing section %q in %#v", id, report.Sections)
		}
	}
	for _, want := range []string{"static graph references are conservative", "dynamic Apex/custom metadata routing reduce confidence", "does not claim full Salesforce parity", "support-map generation is plugin-owned"} {
		if !hasLimitation(report.Limitations, want) {
			t.Fatalf("missing limitation containing %q: %#v", want, report.Limitations)
		}
	}
}

func TestCountDMLCountsDatabaseCallsOnce(t *testing.T) {
	source := `
public class UsesDML {
  void run(Account a) {
    insert a;
    Database.insert(a);
    Database.update(new List<Account>{a});
  }
}`
	if got := countDML(source); got != 3 {
		t.Fatalf("countDML = %d, want 3", got)
	}
}

func TestSourceFilesIncludeTestsOnlyWhenRequested(t *testing.T) {
	root := t.TempDir()
	prod := filepath.Join(root, "Prod.cls")
	test := filepath.Join(root, "ProdTest.cls")
	writeAssessFile(t, prod, "public class Prod {}")
	writeAssessFile(t, test, "@IsTest private class ProdTest {}")
	ctx := enterprise.Context{
		Project: project.Project{Root: root},
		Index: typesys.Index{Types: []typesys.TypeSymbol{
			{Kind: apexast.DeclarationClass, Name: "Prod", File: prod},
			{Kind: apexast.DeclarationClass, Name: "ProdTest", File: test, IsTest: true},
		}},
	}

	withoutTests := sourceFiles(ctx, false, false)
	withTests := sourceFiles(ctx, false, true)

	if len(withoutTests) != 1 || withoutTests[0].Path != prod {
		t.Fatalf("without tests = %#v", withoutTests)
	}
	if len(withTests) != 2 {
		t.Fatalf("with tests = %#v", withTests)
	}
}

func TestStrictAssessmentTurnsDiagnosticsIntoFindings(t *testing.T) {
	ctx := enterprise.Context{
		Project: project.Project{Root: "."},
		Sema: sema.Result{Diagnostics: []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Message:  "bad type reference",
			File:     "force-app/main/default/classes/Broken.cls",
			Range:    &diagnostic.Range{Start: diagnostic.Position{Line: 7, Column: 3}},
		}}},
	}

	report := Assess(ctx, enterprisegraph.Graph{}, Options{Strict: true})

	if report.Status != enterprise.StatusFail {
		t.Fatalf("status = %q, want fail", report.Status)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("findings = %#v", report.Findings)
	}
	if report.Findings[0].Severity != enterprise.SeverityCritical || report.Findings[0].Location.LineStart != 7 {
		t.Fatalf("finding = %#v", report.Findings[0])
	}
}

func hasSection(sections []enterprise.Section, id string) bool {
	for _, section := range sections {
		if section.ID == id {
			return true
		}
	}
	return false
}

func writeAssessFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasLimitation(limitations []string, text string) bool {
	for _, limitation := range limitations {
		if strings.Contains(limitation, text) {
			return true
		}
	}
	return false
}

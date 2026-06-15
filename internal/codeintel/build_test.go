package codeintel_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestBuildIncludesSOSLUsesFromIndexedApexFiles(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join("force-app", "main", "default", "classes", "Searcher.cls")
	source := "public class Searcher {\n" +
		"  void run() {\n" +
		"    List<List<SObject>> rows = [FIND 'Acme' IN ALL FIELDS RETURNING Account(Id, Name)];\n" +
		"  }\n" +
		"}\n"
	writeBuildTestFile(t, filepath.Join(root, classPath), source)

	index := typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{{
			Kind:  apexast.DeclarationClass,
			Name:  "Searcher",
			File:  classPath,
			Range: diagnostic.Range{Start: diagnostic.Position{Line: 1, Column: 1}, End: diagnostic.Position{Line: 5, Column: 2}},
		}},
		Objects: []schema.Object{{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "Name", Type: "String"},
			},
		}},
	}

	graph := codeintel.Build(index)

	assertBuildUseAt(t, graph.Uses, codeintel.ApexTypeID("", "Searcher"), codeintel.UseDeclaration, "Searcher", 1, 1)
	assertBuildUseAt(t, graph.Uses, codeintel.SObjectID("Account"), codeintel.UseQuery, "Account", 3, 69)
	assertBuildUseAt(t, graph.Uses, codeintel.SObjectFieldID("Account", "Id"), codeintel.UseQuery, "Id", 3, 77)
	assertBuildUseAt(t, graph.Uses, codeintel.SObjectFieldID("Account", "Name"), codeintel.UseQuery, "Name", 3, 81)
}

func writeBuildTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func assertBuildUseAt(t *testing.T, uses []codeintel.Use, id codeintel.SymbolID, kind codeintel.UseKind, name string, line, column int) {
	t.Helper()
	for _, use := range uses {
		if use.SymbolID != id || use.Kind != kind || use.Name != name {
			continue
		}
		if !use.Resolved {
			t.Fatalf("use %s = %#v", id, use)
		}
		if use.Range.Start.Line != line || use.Range.Start.Column != column {
			t.Fatalf("use %s range = %#v, want %d:%d", id, use.Range, line, column)
		}
		return
	}
	t.Fatalf("missing use %s in %#v", id, uses)
}

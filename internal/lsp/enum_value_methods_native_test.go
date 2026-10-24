package lsp

import (
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testutil/enumvaluefixture"
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

func TestEnumValueMethodsNativeLSPNamedIsTestRoutes(t *testing.T) {
	fixture, rows := enumvaluefixture.Load(t)
	for _, row := range rows {
		if row.Case.Route == "anonymous" {
			continue
		}
		t.Run(row.API+"/"+row.Case.ID, func(t *testing.T) {
			root := t.TempDir()
			paths, target := enumvaluefixture.WriteProject(t, root, fixture, row)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row.API, ApexFiles: paths}, schema.Schema{})
			if index.HasErrors() {
				t.Fatalf("index: %+v", index.Diagnostics)
			}
			handler := NewHandler(index)
			defer handler.parser.Close()
			enumvaluefixture.AssertDiagnostics(t, row, handler.diagnosticsForDocument(uriFromPath(target)))
		})
	}
}

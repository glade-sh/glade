package lsp

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDottedReceiverLSPNamedTestAndTriggerRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/type_name_resolution_pins"
	read := func(name string) [][]string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixture, "receiver_routes", name))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(strings.NewReader(string(data)))
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	sources := map[string]string{}
	for _, row := range read("cases.tsv")[1:] {
		sources[row[1]+"/"+row[0]] = row[2]
	}
	rows := read("org.tsv")
	if len(rows) != 25 {
		t.Fatalf("native rows: %d", len(rows))
	}
	for _, row := range rows[1:] {
		if row[1] == "anonymous" {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			if row[3] != "false" {
				t.Fatalf("expected native rejection: %v", row)
			}
			root := t.TempDir()
			var paths []string
			for _, source := range []string{"project/force-app/main/default/classes/KnownType.cls", "project/force-app/main/default/classes/TypeNamePinInstance.cls", sources[row[1]+"/"+row[2]]} {
				data, err := os.ReadFile(filepath.Join(fixture, source))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, filepath.Base(source))
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				paths = append(paths, path)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: paths}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
			if index.HasErrors() {
				t.Fatalf("index errors: %v", index.Diagnostics)
			}
			handler := NewHandler(index)
			defer handler.parser.Close()
			for _, d := range handler.diagnosticsForDocument(uriFromPath(paths[len(paths)-1])) {
				if d.Severity == diagnostic.Error && d.NativeMessage == row[4] {
					return
				}
			}
			t.Fatalf("native rejected with %q; LSP emitted no matching error", row[4])
		})
	}
}

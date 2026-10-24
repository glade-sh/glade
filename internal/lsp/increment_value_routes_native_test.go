package lsp

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestIncrementValueRoutesNativeLSP(t *testing.T) {
	runIncrementValueRoutesNativeLSP(t, false)
}

// Comments preserve the captured rejection token stream, including semicolons
// embedded in comments before and after the postfix operator.
func TestIncrementValueRoutesPostfixCommentsLSP(t *testing.T) {
	runIncrementValueRoutesNativeLSP(t, true)
}

func runIncrementValueRoutesNativeLSP(t *testing.T, comments bool) {
	const dir = "../apextest/testdata/conformance/increment_value_routes"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Body string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{}
	for _, row := range cases {
		bodies[row.ID] = row.Body
	}
	helper, err := os.ReadFile(filepath.Join(dir, "OperatorCell.cls"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 149 {
		t.Fatalf("native rows: %d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		if comments && row[2] != "string_local" && row[2] != "string_field" && row[2] != "boxed_object" {
			continue
		}
		if row[1] != "isTest" || row[3] == "unresolved" {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			body := bodies[row[2]]
			if comments {
				body = strings.Replace(body, "++;", "/* ; */++/* ; */;", 1)
			}
			if body == "" {
				t.Fatal("missing captured case")
			}
			root := t.TempDir()
			helperPath := filepath.Join(root, "OperatorCell.cls")
			path := filepath.Join(root, "OperatorObservationTest.cls")
			source := "@IsTest private class OperatorObservationTest { @IsTest static void " + row[2] + "() {\n" + body + "\n} }"
			for name, text := range map[string]string{helperPath: string(helper), path: source} {
				if err := os.WriteFile(name, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: []string{helperPath, path}}, schema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			handler := NewHandler(index)
			t.Cleanup(func() { handler.parser.Close() })
			notifications := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uriFromPath(path), Version: 1, Text: source}})
			if len(notifications) != 1 {
				t.Fatalf("notifications: %+v", notifications)
			}
			payload, ok := notifications[0].Params.(PublishDiagnosticsParams)
			if !ok {
				t.Fatal("missing diagnostics payload")
			}
			var errors []Diagnostic
			for _, d := range payload.Diagnostics {
				if d.Severity == 1 {
					errors = append(errors, d)
				}
			}
			if rejected := row[3] == "rejection"; rejected != (len(errors) > 0) {
				t.Fatalf("native rejected=%t; LSP=%+v", rejected, payload.Diagnostics)
			}
		})
	}
}

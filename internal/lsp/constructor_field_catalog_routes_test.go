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

func TestConstructorFieldCatalogLSPNamedRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, c := range []struct {
			object, field   string
			partial, reject bool
		}{
			{"Account", "Missing__other", false, true},
			{"Account", "pkg__Missing__other", false, true},
			{"Account", "Missing__c", false, true},
			{"Account", "FirstName", false, true},
			{"Account", "LastName", false, true},
			{"Account", "Bloom__c", false, false},
			{"Account", "Tradestyle", false, false},
		} {
			t.Run(api+"/"+c.object+"/"+c.field, func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, "ConstructorFieldProbe.cls")
				source := "public class ConstructorFieldProbe { public static void observe() { " + c.object + " row = new " + c.object + "(" + c.field + " = 'x'); } }"
				if err := os.WriteFile(file, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{file}}, schema.Schema{Objects: []schema.Object{{Name: c.object, Partial: c.partial, Fields: []schema.Field{{Name: "Bloom__c", Type: "Text"}}}}})
				handler := NewHandler(index)
				t.Cleanup(func() { handler.parser.Close() })
				uri := uriFromPath(file)
				opened := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Text: source, Version: 1}})
				closed := handler.DidClose(DidCloseTextDocumentParams{TextDocument: TextDocumentIdentifier{URI: uri}})
				for route, notifications := range map[string][]Notification{"didOpen": opened, "didClose": closed} {
					t.Run(route, func(t *testing.T) {
						if len(notifications) != 1 {
							t.Fatalf("notifications=%+v", notifications)
						}
						payload, ok := notifications[0].Params.(PublishDiagnosticsParams)
						if !ok {
							t.Fatalf("params=%T", notifications[0].Params)
						}
						if c.reject {
							if len(payload.Diagnostics) != 1 || !strings.Contains(payload.Diagnostics[0].Message, c.field) {
								t.Fatalf("undefined field diagnostics=%+v", payload.Diagnostics)
							}
						} else if len(payload.Diagnostics) != 0 {
							t.Fatalf("accepted field diagnostics=%+v", payload.Diagnostics)
						}
					})
				}
			})
		}
	}
}

// Direct captures: constructor_field_catalog/org.tsv:90-137; see ORG-RUN.txt.
// Named native rows directly back the open-document and disk analysis routes.
// Partial metadata is a local loading control, not a Salesforce expectation.
func TestConstructorFieldCatalogLSPNativeManagedAndLookupRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/constructor_field_catalog"
	var cases []struct{ ID, Source, Object string }
	raw, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[2] != "named" {
			continue
		}
		if !strings.HasPrefix(row[1], "managed_") && !strings.HasPrefix(row[1], "meadow_") {
			continue
		}
		var body, object string
		for _, c := range cases {
			if c.ID == row[1] {
				body, object = c.Source, c.Object
			}
		}
		if body == "" {
			t.Fatal("missing native source: " + row[1])
		}
		for _, partial := range []bool{false, true} {
			state := "complete"
			if partial {
				state = "partial-control"
			}
			t.Run(strings.Join(row[:3], "/")+"/"+state, func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, "ConstructorFieldProbe.cls")
				source := "public class ConstructorFieldProbe { public static void observe() { " + body + " } }"
				if err := os.WriteFile(file, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				fields := []schema.Field{{Name: "Name", Type: "Text"}, {Name: "Parent__c", Type: "Lookup", ReferenceTo: []string{"Account"}}}
				if strings.HasPrefix(row[1], "managed_") {
					fields = []schema.Field{{Name: "Name", Type: "Text"}, {Name: "gladens004__Bloom__c", Type: "Text"}, {Name: "gladens004__Parent__c", Type: "Lookup", ReferenceTo: []string{"Account"}}}
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: []string{file}}, schema.Schema{Objects: []schema.Object{{Name: object, Partial: partial, Fields: fields}}})
				handler := NewHandler(index)
				t.Cleanup(func() { handler.parser.Close() })
				uri := uriFromPath(file)
				opened := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Text: source, Version: 1}})
				closed := handler.DidClose(DidCloseTextDocumentParams{TextDocument: TextDocumentIdentifier{URI: uri}})
				for route, notifications := range map[string][]Notification{"didOpen": opened, "didClose": closed} {
					t.Run(route, func(t *testing.T) {
						if len(notifications) != 1 {
							t.Fatalf("notifications=%+v", notifications)
						}
						payload, ok := notifications[0].Params.(PublishDiagnosticsParams)
						if !ok {
							t.Fatalf("params=%T", notifications[0].Params)
						}
						want := row[4]
						if partial || row[3] == "true" {
							want = ""
						}
						if want == "" {
							if len(payload.Diagnostics) != 0 {
								t.Fatalf("accepted field diagnostics=%+v", payload.Diagnostics)
							}
							return
						}
						if len(payload.Diagnostics) != 1 || !strings.Contains(payload.Diagnostics[0].Message, want) {
							t.Fatalf("native %q; LSP diagnostics=%+v", want, payload.Diagnostics)
						}
					})
				}
			})
		}
	}
}

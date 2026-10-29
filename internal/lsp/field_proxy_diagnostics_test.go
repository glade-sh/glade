package lsp

import (
	"encoding/csv"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fieldProxyRows(t *testing.T) [][]string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/conformance/field_proxy_diagnostics/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows=%d: %v", len(rows), err)
	}
	return rows[1:]
}

func TestFieldProxyLSPNativeDiagnosticRoutes(t *testing.T) {
	for _, row := range fieldProxyRows(t) {
		if row[1] != "isTest" {
			continue
		}
		for _, expression := range []string{"Schema.Account.fields.getMap()", "Account.fields.getMap()", "Schema.Account.fields.get('Name')", "Account.fields.get('Name')"} {
			for _, route := range []string{"named", "isTest", "trigger"} {
				t.Run(row[0]+"/"+expression+"/"+route, func(t *testing.T) {
					want := row[4]
					if strings.Contains(expression, ".get(") {
						want = strings.ReplaceAll(want, "getMap()", "get(String)")
					}
					root := t.TempDir()
					name, source := "FieldProxyProbe.cls", "public class FieldProxyProbe { public static void observe() { Object value = Schema.Account.fields.getMap(); } }"
					if route == "isTest" {
						source = "@IsTest private class FieldProxyProbe { @IsTest static void observe() { Object value = Schema.Account.fields.getMap(); } }"
					}
					if route == "trigger" {
						name = "FieldProxyProbe.trigger"
						source = "trigger FieldProxyProbe on Account (before insert) { Object value = Schema.Account.fields.getMap(); }"
					}
					source = strings.ReplaceAll(source, "Schema.Account.fields.getMap()", expression)
					path := filepath.Join(root, name)
					writeReceiverMemberFile(t, path, source)
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: []string{path}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					handler := NewHandler(index)
					defer handler.parser.Close()
					diagnostics := handler.diagnosticsForDocument(uriFromPath(path))
					if len(diagnostics) != 1 || diagnostics[0].NativeMessage != want {
						t.Fatalf("native diagnostic %q; got %+v", want, diagnostics)
					}
				})
			}
		}
	}
}

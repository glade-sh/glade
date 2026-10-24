package lsp

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSOQLFormulaWhereLSPNativeVersionRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/soql_formula_where"
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	reader := csv.NewReader(strings.NewReader(read("compile.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 7 {
		t.Fatalf("native inline rows: %d rows, %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		if row[1] != "istest" {
			continue
		}
		api, expected := row[0], row[2]
		t.Run(api+"/lsp", func(t *testing.T) {
			root := t.TempDir()
			write := func(path, body string) {
				t.Helper()
				path = filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			const base = "force-app/main/default/"
			const classPath = base + "classes/FormulaWhereStatic68.cls"
			source := read("project/" + classPath)
			write(classPath, source)
			write(classPath+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			for _, name := range []string{"Start__c", "Finish__c", "StartTime__c", "FinishTime__c"} {
				path := base + "objects/Account/fields/" + name + ".field-meta.xml"
				write(path, read("project/"+path))
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			handler := NewHandler(typesys.Build(p, s))
			t.Cleanup(handler.parser.Close)
			uri := uriFromPath(filepath.Join(root, classPath))
			assertNative := func(notifications []Notification) {
				t.Helper()
				if len(notifications) != 1 {
					t.Fatalf("missing diagnostics publication: %+v", notifications)
				}
				payload, ok := notifications[0].Params.(PublishDiagnosticsParams)
				if !ok {
					t.Fatalf("unexpected publication: %+v", notifications[0])
				}
				if strings.HasPrefix(expected, "OK|") {
					if len(payload.Diagnostics) != 0 {
						t.Fatalf("native query compiled; LSP: %+v", payload.Diagnostics)
					}
					return
				}
				for _, item := range payload.Diagnostics {
					if strings.Contains(item.Message, "FORMULA() in a SOQL WHERE clause is a Beta feature") {
						return
					}
				}
				t.Fatalf("missing native version rejection: %+v", payload.Diagnostics)
			}
			assertNative(handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: source}}))
			assertNative(handler.DidClose(DidCloseTextDocumentParams{TextDocument: TextDocumentIdentifier{URI: uri}}))
		})
	}
}

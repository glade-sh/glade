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

func TestQualifiedFieldTokensLSPNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyLSP(t, "../sema/testdata/qualified_field_tokens")
}

func runCapturedSemanticBodyLSP(t *testing.T, dir string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Body string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 1+6*len(cases) {
		t.Fatal("incomplete native matrix", err)
	}
	for _, row := range rows[1:] {
		if row[1] == "anonymous" {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			body := ""
			for _, c := range cases {
				if c.ID == row[2] {
					body = c.Body
				}
			}
			if body == "" {
				t.Fatal("missing native source")
			}
			root := t.TempDir()
			write := func(path, source string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[0]+`"}`)
			files, err := filepath.Glob(filepath.Join(dir, "classes/*.cls"))
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range files {
				raw, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				write(filepath.Join(root, "force-app/main/default/classes", filepath.Base(file)), string(raw))
			}
			for _, name := range []string{"TokenEntry__c.object-meta.xml", "Value__c.field-meta.xml"} {
				raw, err := os.ReadFile(filepath.Join(dir, "metadata", name))
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "force-app/main/default/objects/TokenEntry__c", name)
				if strings.HasSuffix(name, ".field-meta.xml") {
					path = filepath.Join(root, "force-app/main/default/objects/TokenEntry__c/fields", name)
				}
				write(path, string(raw))
			}
			source := "public class GardenBodyProbe { public static void observe() { " + body + " } }"
			if row[1] == "istest" {
				source = "@IsTest private class GardenBodyProbe { @IsTest static void observe() { " + body + " } }"
			}
			path := filepath.Join(root, "force-app/main/default/classes/GardenBodyProbe.cls")
			write(path, source)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			idx := typesys.Build(p, s)
			if idx.HasErrors() && row[3] != "rejected" {
				t.Fatal(idx.Diagnostics)
			}
			h := NewHandler(idx)
			defer h.parser.Close()
			notifications := h.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uriFromPath(path), Text: source, Version: 1}})
			n := diagnosticsCount(t, notifications)
			if (n > 0) != (row[3] == "rejected") {
				t.Fatalf("native %s (%s); LSP notifications=%+v", row[3], row[4], notifications)
			}
		})
	}
}

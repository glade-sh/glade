package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Exact named/@IsTest native sources cover disk/open/warm diagnostics. The
// handler retains a fixed semantic index: recover a malformed overlay to that
// indexed native source, then repeat with a newline-only edit.
func TestStandardConstructorCompleteNativeLSPEditRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/standard_constructor_complete/observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Object, Case, Route, Source string
		ClassName                        string `json:"class_name"`
		Compiled                         bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Route == "anonymous" {
			continue
		}
		t.Run(row.API+"/"+row.Route+"/"+row.Object+"/"+row.Case, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, row.ClassName+".cls")
			if err := os.WriteFile(path, []byte(row.Source), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path+"-meta.xml", []byte("<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>"), 0600); err != nil {
				t.Fatal(err)
			}
			p := project.Project{Root: root, SourceAPIVersion: row.API, ApexFiles: []string{path}}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			handler := NewHandler(typesys.Build(p, s))
			defer handler.parser.Close()
			uri := uriFromPath(path)
			check := func(phase string, notifications []Notification, wantError bool) {
				t.Helper()
				if len(notifications) != 1 {
					t.Fatalf("%s notifications=%v", phase, notifications)
				}
				params, ok := notifications[0].Params.(PublishDiagnosticsParams)
				if !ok {
					t.Fatalf("%s params=%T", phase, notifications[0].Params)
				}
				if !wantError {
					if len(params.Diagnostics) != 0 {
						t.Errorf("%s native accepted; diagnostics=%v", phase, params.Diagnostics)
					}
					return
				}
				if len(params.Diagnostics) != 1 || !strings.Contains(params.Diagnostics[0].Message, "SyntheticAbsentField") {
					t.Errorf("%s native missing-field rejection; diagnostics=%v", phase, params.Diagnostics)
				}
			}
			check("open", handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Text: row.Source, Version: 1}}), !row.Compiled)
			malformed, err := handler.DidChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 2}, ContentChanges: []TextDocumentContentChangeEvent{{Text: "public class " + row.ClassName + " {"}}})
			if err != nil {
				t.Fatal(err)
			}
			if diagnosticsCount(t, malformed) == 0 {
				t.Fatal("malformed edit has no syntax diagnostic")
			}
			for i, source := range []string{row.Source, row.Source + "\n"} {
				notifications, err := handler.DidChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: i + 3}, ContentChanges: []TextDocumentContentChangeEvent{{Text: source}}})
				if err != nil {
					t.Fatal(err)
				}
				check("restored-indexed-source", notifications, !row.Compiled)
			}
			check("disk", handler.DidClose(DidCloseTextDocumentParams{TextDocument: TextDocumentIdentifier{URI: uri}}), !row.Compiled)
		})
	}
}

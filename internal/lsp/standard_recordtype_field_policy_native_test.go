package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardRecordTypeFieldPolicyNativeLSPConstructorRoutes(t *testing.T) {
	const dir = "../apextest/testdata/conformance/standard_recordtype_field_policy"
	raw, err := os.ReadFile(filepath.Join(dir, "observations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		API, Profile, Route, Source, Message string
		ClassName                            string `json:"class_name"`
		Compiled                             bool
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile(filepath.Join(dir, "recordtype-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range rows {
		if row.Route != "named-constructor" {
			continue
		}
		count++
		t.Run(row.API+"/"+row.Profile+"/named-constructor", func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "config"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "config/project-scratch-def.json"), []byte(`{"edition":"Developer"}`), 0600); err != nil {
				t.Fatal(err)
			}

			path := filepath.Join(root, row.ClassName+".cls")
			if err := os.WriteFile(path, []byte(row.Source), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path+"-meta.xml", []byte("<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>"), 0600); err != nil {
				t.Fatal(err)
			}
			if row.Profile == "recordtype" {
				dir := filepath.Join(root, "objects/Account/recordTypes")
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "OwnedPolicy.recordType-meta.xml"), metadata, 0600); err != nil {
					t.Fatal(err)
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			p.SourceAPIVersion = row.API
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			handler := NewHandler(index)
			defer handler.parser.Close()
			compiled := !index.HasErrors()
			diagnostics := handler.diagnosticsForDocument(uriFromPath(path))
			for _, d := range diagnostics {
				if d.Severity == diagnostic.Error {
					compiled = false
				}
			}
			if compiled != row.Compiled {
				t.Fatalf("native compiled=%t (%s); LSP=%t: %v %v", row.Compiled, strings.TrimSpace(row.Message), compiled, index.Diagnostics, diagnostics)
			}
		})
	}
	if count != 4 {
		t.Fatal("incomplete native named constructor matrix")
	}
}

package lsp

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPlatformMemberNativeNamedLSPRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, family := range []string{"system_type_construction", "finalizer_field_dispatch", "publishing_date_arguments"} {
			t.Run(api+"/"+family, func(t *testing.T) {
				fixture := filepath.Join("..", "vm", "testdata", "conformance", family)
				native := filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"))
				rows := platformMemberNativeTable(t, filepath.Join(native, "org.tsv"))
				if rows[1][3] != "observation" || rows[2][3] != "observation" {
					t.Fatal("missing accepted anonymous and named native rows")
				}
				root := t.TempDir()
				write := func(name string, body []byte) {
					t.Helper()
					path := filepath.Join(root, name)
					if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, body, 0644); err != nil {
						t.Fatal(err)
					}
				}
				write("sfdx-project.json", []byte(fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api)))
				if shape, err := os.ReadFile(filepath.Join(fixture, "project-scratch-def.json")); err == nil {
					write("config/project-scratch-def.json", shape)
				}
				sources, err := filepath.Glob(filepath.Join(fixture, "sources", "*.cls"))
				if err != nil {
					t.Fatal(err)
				}
				for _, source := range sources {
					body, err := os.ReadFile(source)
					if err != nil {
						t.Fatal(err)
					}
					target := filepath.Join("force-app/main/default/classes", filepath.Base(source))
					write(target, body)
					write(target+"-meta.xml", []byte("<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>"))
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				s, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				check := func(name string, body []byte, wantError bool) {
					t.Helper()
					t.Run(name, func(t *testing.T) {
						target := filepath.Join("force-app/main/default/classes", name)
						path := filepath.Join(root, target)
						_, existed := os.Stat(path)
						write(target, body)
						write(target+"-meta.xml", []byte("<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>"))
						if os.IsNotExist(existed) {
							defer os.Remove(path)
							defer os.Remove(path + "-meta.xml")
						}
						p, err := project.Load(root)
						if err != nil {
							t.Fatal(err)
						}
						index := typesys.Build(p, s)
						handler := NewHandler(index)
						defer handler.parser.Close()
						notes := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uriFromPath(filepath.Join(root, "force-app/main/default/classes", name)), Version: 1, Text: string(body)}})
						var problems []Diagnostic
						for _, note := range notes {
							if payload, ok := note.Params.(PublishDiagnosticsParams); ok {
								for _, d := range payload.Diagnostics {
									if d.Severity == diagnosticSeverityError {
										problems = append(problems, d)
									}
								}
							}
						}
						if (len(problems) > 0) != wantError {
							t.Fatalf("native rejection=%v; LSP=%v", wantError, problems)
						}
					})
				}
				for _, source := range sources {
					body, _ := os.ReadFile(source)
					check(filepath.Base(source), body, false)
				}
				if family != "system_type_construction" {
					rejects := platformMemberNativeTable(t, filepath.Join(native, "named-rejections.tsv"))
					for _, row := range rejects[1:] {
						if row[0] != api || row[3] != "compile_reject" {
							t.Fatal("invalid native named rejection")
						}
						body, err := os.ReadFile(filepath.Join(fixture, "rejections", row[2]))
						if err != nil {
							t.Fatal(err)
						}
						check(row[2], body, true)
					}
				}
			})
		}
	}
}

func platformMemberNativeTable(t *testing.T, path string) [][]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 {
		t.Fatalf("native table: %s: %v", path, err)
	}
	return rows
}

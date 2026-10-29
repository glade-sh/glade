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

// LSP validates named declarations; Flow execution is covered by the runtime
// twins using the same owned native projects.
func TestFlowNativeProjectLSPRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, family := range []string{"query_conditional_record", "flow_number_text", "flow_single_filter_lookup", "flow_subflow_route_twins", "person_contact_alias", "record_type_scope", "flow_decision_criteria_native", "flow_related_fields", "generated_soql_relationships", "flow_subflow_record_lookup", "flow_subflow_record_routes"} {
			t.Run(api+"/"+family+"/lsp", func(t *testing.T) {
				source := filepath.Join("../apextest/testdata/conformance", family)
				switch family {
				case "flow_decision_criteria_native", "flow_subflow_record_routes":
					source = filepath.Join(source, "project")
				case "flow_related_fields", "generated_soql_relationships":
					source = filepath.Join(source, "native", "api"+api[:2])
				case "flow_subflow_record_lookup", "flow_subflow_route_twins":
					source = filepath.Join(source, "api"+api[:2])
				}
				root := t.TempDir()
				var classes []string
				err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() {
						if entry.Name() == "native" || strings.HasPrefix(entry.Name(), ".") {
							return filepath.SkipDir
						}
						return nil
					}
					if !strings.Contains(path, string(filepath.Separator)+"force-app"+string(filepath.Separator)) && filepath.Base(path) != "sfdx-project.json" && filepath.Base(path) != "project-scratch-def.json" {
						return nil
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					name, err := filepath.Rel(source, path)
					if err != nil {
						return err
					}
					target := filepath.Join(root, name)
					if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
						return err
					}
					if err = os.WriteFile(target, []byte(strings.ReplaceAll(string(body), "67.0", api)), 0600); err != nil {
						return err
					}
					if strings.HasSuffix(target, ".cls") {
						classes = append(classes, target)
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				if len(classes) == 0 {
					t.Fatal("no named sources selected")
				}
				checkFlowNativeProjectLSP(t, root, classes)
			})
		}
	}
}

// LSP checks named declarations for every entry pin. Runtime effects and native
// deployment rejection are checked by TestFlowCorePinsConformance; rejected
// Flow files do not belong in a compilable LSP project.
func TestFlowCorePinsLSPRoutes(t *testing.T) {
	var data struct {
		Rows []struct {
			ID    string `json:"id"`
			ByAPI map[string]struct {
				Files             map[string]string `json:"files"`
				RejectedFlowFiles []string          `json:"rejectedFlowFiles"`
			} `json:"byAPI"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("../apextest/testdata/conformance/flow_core/pins.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 22 {
		t.Fatalf("entry pins=%d; want 22", len(data.Rows))
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, row := range data.Rows {
			t.Run(api+"/"+row.ID, func(t *testing.T) {
				fixture, ok := row.ByAPI[api]
				if !ok {
					t.Fatal("missing API fixture")
				}
				root := t.TempDir()
				var classes []string
				for path, text := range fixture.Files {
					rejected := false
					for _, name := range fixture.RejectedFlowFiles {
						if path == name {
							rejected = true
						}
					}
					if rejected {
						t.Log("native metadata deployment rejection: LSP runtime cell not applicable", path)
						continue
					}
					target := filepath.Join(root, path)
					if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(target, []byte(text), 0600); err != nil {
						t.Fatal(err)
					}
					if strings.HasSuffix(path, ".cls") {
						classes = append(classes, target)
					}
				}
				if len(classes) == 0 {
					t.Fatal("no named sources selected")
				}
				checkFlowNativeProjectLSP(t, root, classes)
			})
		}
	}
}

func checkFlowNativeProjectLSP(t *testing.T, root string, classes []string) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, s)
	if index.HasErrors() {
		t.Fatal(index.Diagnostics)
	}
	handler := NewHandler(index)
	defer handler.parser.Close()
	for _, path := range classes {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		notes := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uriFromPath(path), Version: 1, Text: string(body)}})
		for _, note := range notes {
			if payload, ok := note.Params.(PublishDiagnosticsParams); ok {
				for _, d := range payload.Diagnostics {
					if d.Severity == diagnosticSeverityError {
						t.Errorf("%s: %v", filepath.Base(path), d)
					}
				}
			}
		}
	}
}

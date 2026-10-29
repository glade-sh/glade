package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStandardFeatureGateNativeLSPNamedRoutes(t *testing.T) {
	const captureRoot = "../apextest/testdata/conformance/standard_describe_profiles"
	definition, err := os.ReadFile(filepath.Join(captureRoot, "profiles/P0.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62", "67"} {
		raw, err := os.ReadFile(filepath.Join(captureRoot, "P0", "api"+api, "granted/slices.json"))
		if err != nil {
			t.Fatal(err)
		}
		var rows []struct {
			ID, API, Route, ClassName, Source, Message string
			Compiled                                   bool
		}
		if err := json.Unmarshal(raw, &rows); err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, row := range rows {
			if row.Route != "named" {
				continue
			}
			count++
			t.Run(row.API+"/named/"+row.ID, func(t *testing.T) {
				root := t.TempDir()
				// These rows describe P0's declared Developer org, not an
				// unspecified consumer whose source tree omits org evidence.
				if err := os.MkdirAll(filepath.Join(root, "config"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "config/project-scratch-def.json"), definition, 0600); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, row.ClassName+".cls")
				if err := os.WriteFile(path, []byte(row.Source), 0600); err != nil {
					t.Fatal(err)
				}
				metadata := "<ApexClass><apiVersion>" + row.API + "</apiVersion></ApexClass>"
				if err := os.WriteFile(path+"-meta.xml", []byte(metadata), 0600); err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row.API, ApexFiles: []string{path}}, schema.Schema{})
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
					t.Fatalf("native compiled=%t (%s); LSP compiled=%t: %v %v", row.Compiled, row.Message, compiled, index.Diagnostics, diagnostics)
				}
			})
		}
		if count != 21 {
			t.Fatalf("API %s: read %d named native routes, want 21", api, count)
		}
	}
}

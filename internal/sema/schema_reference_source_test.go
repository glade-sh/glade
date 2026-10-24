package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSchemaReferenceDiagnosticSourceRoutes(t *testing.T) {
	var data struct {
		Cases []struct {
			API       string            `json:"api"`
			ID        string            `json:"id"`
			Files     map[string]string `json:"files"`
			FieldPath string            `json:"fieldPath"`
		} `json:"cases"`
	}
	raw, err := os.ReadFile("testdata/schema_reference_gate/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	for _, c := range data.Cases {
		t.Run(c.API+"/"+c.ID+"/decomposed_metadata", func(t *testing.T) {
			root := t.TempDir()
			for path, source := range c.Files {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0755); err != nil {
					t.Fatal(err)
				}
				writeSemaFile(t, filepath.Join(root, path), source)
			}
			writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"base","default":true},{"path":"bad"},{"path":"loaded"}],"sourceApiVersion":"`+c.API+`"}`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			result := Analyze(typesys.Build(p, metadata))
			count := 0
			for _, d := range result.Diagnostics {
				if d.Code != "GLADESEMA003" {
					continue
				}
				count++
				if d.File != filepath.Join(root, c.FieldPath) {
					t.Errorf("lookup diagnostic lost its metadata source: file=%q want=%q message=%s", d.File, filepath.Join(root, c.FieldPath), d.Message)
				}
			}
			want := 0
			if c.ID == "missing_lookup" {
				want = 2
			}
			if count != want {
				t.Fatalf("schema diagnostics=%d want=%d: %+v", count, want, result.Diagnostics)
			}
		})
	}
}

package sema

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

// Synthetic names are used for the captured namespace/type collision; the
// result is keyed to source structure, not to any particular namespace.
func TestCapturedQualifiedTopLevelIsNotEnclosingClass(t *testing.T) {
	f, err := os.Open("testdata/enclosing_static_namespace/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 11 || strings.Join(rows[0], "|") != "case|api|route|outcome" {
		t.Fatalf("native matrix has %d rows, want 11", len(rows))
	}
	cases := map[string]bool{"qualified_valid": true, "qualified_collision": true, "qualified_missing": true, "nested_valid": true, "nested_instance": true}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[1] != "62.0" && row[1] != "67.0") || row[2] != "metadata-deploy" {
			t.Fatalf("invalid row: %v", row)
		}
		if !cases[row[0]] {
			t.Fatalf("unknown native case: %v", row)
		}
		key := row[0] + "/" + row[1]
		if seen[key] {
			t.Fatalf("duplicate row: %s", key)
		}
		seen[key] = true
		if row[3] == "" || strings.HasPrefix(row[3], "CAPTURE_ERROR") {
			t.Fatalf("invalid outcome: %v", row)
		}
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			data, err := os.ReadFile(filepath.Join("testdata/enclosing_static_namespace/cases", row[0]+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var sources map[string]string
			if err := json.Unmarshal(data, &sources); err != nil {
				t.Fatal(err)
			}
			var config struct {
				Namespace string `json:"namespace"`
			}
			data, err = os.ReadFile("testdata/enclosing_static_namespace/project.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &config); err != nil {
				t.Fatal(err)
			}
			var files []string
			for name, source := range sources {
				p := filepath.Join(root, name)
				writeSemaFile(t, p, source)
				files = append(files, p)
			}
			idx := typesys.Build(project.Project{Root: root, Namespace: config.Namespace, SourceAPIVersion: row[1], ApexFiles: files}, schema.Schema{})
			result := Analyze(idx)
			wantReject := row[3] != "accepted"
			if got := idx.HasErrors() || result.HasErrors(); got != wantReject {
				t.Fatalf("native=%q; index=%v sema=%v", row[3], idx.Diagnostics, result.Diagnostics)
			}
		})
	}
}

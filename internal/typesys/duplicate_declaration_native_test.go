package typesys

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestNativeDuplicateDeclarations(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "road255", "org.tsv"))
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
	for _, row := range rows[1:] {
		t.Run(row[0], func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, row[2]+".cls")
			writeFile(t, path, "public class "+row[2]+" { "+row[3]+" }")
			idx := Build(project.Project{Root: root, ApexFiles: []string{path}, SourceAPIVersion: row[1]}, schema.Schema{})
			duplicateError := false
			for _, d := range idx.Diagnostics {
				if d.Code == "GLADETYPE001" && d.Severity == diagnostic.Error {
					duplicateError = true
				}
			}
			if want := row[4] == "Failed"; duplicateError != want || idx.HasErrors() != want {
				t.Fatalf("duplicate error = %v, errors = %v; want %v: %v", duplicateError, idx.HasErrors(), want, idx.Diagnostics)
			}
		})
	}
}

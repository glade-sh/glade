package visualforce_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/visualforce"
)

func TestVisualforceInputHiddenColumnsNativeRows(t *testing.T) {
	const dir = "testdata/vf_inputhidden_columns"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
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
	if len(rows) != 11 {
		t.Fatalf("rows=%d, want 10", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writePageNameFixture(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			for _, suffix := range []string{".page", ".page-meta.xml"} {
				rel := filepath.Join("force-app/main/default/pages", row[1]+suffix)
				b, err := os.ReadFile(filepath.Join(dir, "project", rel))
				if err != nil {
					t.Fatal(err)
				}
				s := string(b)
				if suffix == ".page-meta.xml" {
					s = strings.ReplaceAll(s, "67.0", row[0])
				}
				writePageNameFixture(t, root, rel, s)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, rejected := visualforce.PageNamesWithDiagnostics(p, nil)
			if len(rejected) != 1 || rejected[0].Message != row[3] {
				t.Fatalf("native=%q Glade=%+v", row[3], rejected)
			}
		})
	}
}

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

func TestVisualforceEmptyIDsNativeRows(t *testing.T) {
	const dir = "testdata/vf_empty_ids"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 13 {
		t.Fatalf("native rows=%d, want 12", len(rows)-1)
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
				text := string(b)
				if suffix == ".page-meta.xml" {
					text = strings.ReplaceAll(text, "67.0", row[0])
				}
				writePageNameFixture(t, root, rel, text)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			_, err = visualforce.LoadProject(p)
			if row[2] == "reject" {
				if err == nil {
					t.Fatal("native rejected; Glade accepted")
				}
				return
			}
			if err != nil {
				t.Fatalf("native accepted: %v", err)
			}
			machine, p, index := ispickvalMachine(t, root)
			result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: index, Machine: machine, PageName: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			if result.Error != nil {
				t.Fatal(result.Error.Message)
			}
			if !strings.Contains(result.HTML, row[3]) {
				t.Errorf("native element=%q; Glade=%q", row[3], result.HTML)
			}
		})
	}
}

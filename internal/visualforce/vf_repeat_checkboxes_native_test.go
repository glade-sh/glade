package visualforce_test

import (
	"encoding/csv"
	"encoding/json"
	"github.com/glade-sh/glade/internal/visualforce"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVisualforceRepeatCheckboxNativeRows(t *testing.T) {
	const dir = "testdata/vf_repeat_checkboxes"
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
		t.Fatalf("rows=%d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writePageNameFixture(t, root, "sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			for _, rel := range []string{"classes/RepeatCapture.cls", "classes/RepeatCapture.cls-meta.xml", "pages/" + row[1] + ".page", "pages/" + row[1] + ".page-meta.xml"} {
				b, err := os.ReadFile(filepath.Join(dir, "project/force-app/main/default", rel))
				if err != nil {
					t.Fatal(err)
				}
				s := string(b)
				if strings.HasSuffix(rel, ".page-meta.xml") {
					s = strings.ReplaceAll(s, "67.0", row[0])
				}
				writePageNameFixture(t, root, filepath.Join("force-app/main/default", rel), s)
			}
			m, p, index := ispickvalMachine(t, root)
			result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: index, Machine: m, PageName: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			if result.Error != nil {
				t.Fatal(result.Error.Message)
			}
			var native []string
			if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
				t.Fatal(err)
			}
			if got := strings.Count(result.HTML, `type="checkbox"`); got != len(native) {
				t.Fatalf("checkboxes=%d native=%d", got, len(native))
			}
		})
	}
}

package visualforce_test

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/visualforce"
)

func TestVisualforceDataTableCapturedMarkup(t *testing.T) {
	const dir = "testdata/vf_datatable_capture"
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
	if len(rows) != 7 {
		t.Fatalf("native rows=%d, want six", len(rows)-1)
	}
	table := regexp.MustCompile(`(?s)<table\b.*?</table>`)
	for _, api := range []string{"59.0", "67.0"} {
		api := api
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(dir, "project"), func(path string, e fs.DirEntry, err error) error {
				if err != nil || e.IsDir() {
					return err
				}
				rel, err := filepath.Rel(filepath.Join(dir, "project"), path)
				if err != nil {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				text := string(body)
				if strings.HasSuffix(rel, ".page-meta.xml") {
					text = strings.ReplaceAll(text, "67.0", api)
				}
				writePageNameFixture(t, root, rel, text)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			machine, p, index := ispickvalMachine(t, root)
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				row := row
				t.Run(row[1], func(t *testing.T) {
					result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: index, Machine: machine, PageName: row[1]})
					if err != nil {
						t.Fatal(err)
					}
					if result.Error != nil {
						t.Fatal(result.Error.Message)
					}
					got := table.FindString(result.HTML)
					if got != row[2] {
						t.Errorf("native=%s\nGlade=%s", row[2], got)
					}
				})
			}
		})
	}
}

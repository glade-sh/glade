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

func TestVisualforceResourceUnicodeNativeRows(t *testing.T) {
	const dir = "testdata/vf_resource_unicode"
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
	if len(rows) != 23 {
		t.Fatalf("native rows=%d, want 22", len(rows)-1)
	}
	anchor := regexp.MustCompile(`<a\b[^>]*href="([^"]+)"`)
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
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				text := string(b)
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
					rendered, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: index, Machine: machine, PageName: row[1]})
					if err != nil {
						t.Fatal(err)
					}
					if rendered.Error != nil {
						t.Fatal(rendered.Error.Message)
					}
					match := anchor.FindStringSubmatch(rendered.HTML)
					if len(match) != 2 {
						t.Fatalf("missing href: %s", rendered.HTML)
					}
					want := strings.Replace(row[2], "{ORGHOST}/resource/{TS}/", "/resource/", 1)
					if match[1] != want {
						t.Errorf("native=%q Glade=%q", want, match[1])
					}
				})
			}
		})
	}
}

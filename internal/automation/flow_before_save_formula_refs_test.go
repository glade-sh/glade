package automation

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Org capture flow_assign_routes rejections (API 62.0 and 67.0, check-only
// deploys): a before-save flow that reads a formula field through $Record is
// rejected, including from a text template and a record-update field value;
// an entry filter on the field deploys, and $Record.Name controls deploy.
func TestFlowBeforeSaveFormulaFieldReferenceNative(t *testing.T) {
	dir := filepath.Join("testdata", "flow_before_save_formula_refs")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 25 || strings.Join(rows[0], ",") != "source_api,variant,deploy" {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		api, variant, want := row[0], row[1], row[2]
		t.Run(api+"/"+variant, func(t *testing.T) {
			root := t.TempDir()
			copyTree := func(source, target string, replace bool) {
				err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					name, err := filepath.Rel(source, path)
					if err != nil {
						return err
					}
					text := string(body)
					if replace {
						text = strings.ReplaceAll(text, "62.0", api)
					}
					writeWorkflowTestFile(t, filepath.Join(target, name), text)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			copyTree(filepath.Join(dir, "project"), root, true)
			src, err := os.ReadFile(filepath.Join(dir, variant+".flow-meta.xml"))
			if err != nil {
				t.Fatal(err)
			}
			writeWorkflowTestFile(t, filepath.Join(root, "force-app/main/default/flows/Fasg_Rej.flow-meta.xml"), strings.ReplaceAll(string(src), "62.0", api))
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			idx, err := LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			var rejected []string
			for _, d := range idx.Diagnostics {
				if d.Code == "GLADEAUTO004" {
					rejected = append(rejected, d.Message)
				}
			}
			got := "Succeeded"
			if len(rejected) > 0 {
				got = strings.Join(rejected, "\n")
			}
			if got != want {
				t.Fatalf("native=%q; Glade=%q", want, got)
			}
		})
	}
}

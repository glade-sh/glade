package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestAsyncInheritedStaticCallbacksProjectDeployRejectedNative(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "async_route_twins")
	file, err := os.Open(filepath.Join(dir, "deploy.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 11 {
		t.Fatalf("incomplete native rejection matrix: %v", err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, kind := range []string{"Queue", "Schedule", "Batch"} {
			t.Run(api+"/"+kind+"/inherited-static", func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				for _, suffix := range []string{"Base", "Child"} {
					name := "Static" + kind + suffix
					sourceDir := filepath.Join(dir, "project", "force-app", "main", "default", "classes")
					if suffix == "Child" {
						sourceDir = filepath.Join(dir, "rejected")
					}
					for _, ext := range []string{".cls", ".cls-meta.xml"} {
						body, err := os.ReadFile(filepath.Join(sourceDir, name+ext))
						if err != nil {
							t.Fatal(err)
						}
						writeFile(t, filepath.Join(root, "force-app", "main", "default", "classes", name+ext), strings.ReplaceAll(string(body), "62.0", api))
					}
				}
				index := loadTestIndex(t, root)
				if index.HasErrors() {
					t.Fatalf("syntax errors: %v", index.Diagnostics)
				}
				analysis := sema.Analyze(index)
				missing := map[string]bool{}
				for _, d := range analysis.Diagnostics {
					if d.Code == "GLADESEMA017" && strings.Contains(d.File, "Child.cls") {
						missing[d.Message] = true
					}
				}
				want := 0
				for _, row := range rows[1:] {
					if row[0] == api && row[1] == "Static"+kind+"Child" {
						want++
					}
				}
				if want == 0 {
					t.Fatal("native rejection row missing")
				}
				if len(missing) != want {
					t.Fatalf("native requires %d child callbacks; diagnostics=%+v", want, analysis.Diagnostics)
				}
			})
		}
	}
}

package apextest

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// TestCheckboxFormulaParentNative exercises the insert, update, and query rows
// captured in testdata/conformance/checkbox_formula_parent/org.tsv.
func TestCheckboxFormulaParentNative(t *testing.T) {
	const fixture = "testdata/conformance/checkbox_formula_parent"
	input, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	reader := csv.NewReader(input)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			files := make(map[string]string)
			base := filepath.Join(fixture, "project")
			err := filepath.WalkDir(filepath.Join(base, "force-app"), func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				if strings.Contains(entry.Name(), "CheckboxCaptureTest") {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if entry.Name() == "CheckboxCapture.trigger-meta.xml" {
					data = []byte(strings.ReplaceAll(string(data), "62.0", api))
				}
				rel, err := filepath.Rel(base, path)
				if err != nil {
					return err
				}
				files[rel] = string(data)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			name := "CheckboxCaptureTest" + strings.TrimSuffix(api, ".0")
			sourceBytes, err := os.ReadFile(filepath.Join(base, "force-app/main/default/classes", name+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			prefix, _, ok := strings.Cut(string(sourceBytes), "  System.assertEquals(")
			if !ok {
				t.Fatal("fixture assertions missing")
			}
			var assertions strings.Builder
			count := 0
			for _, row := range rows[1:] {
				if len(row) != 6 || row[0] != api || row[5] != "Pass" {
					continue
				}
				route := strings.TrimPrefix(row[1], "after-")
				key := route + ":" + row[2]
				text := "null"
				if row[4] != "null" {
					text = "'" + row[4] + "'"
				}
				fmt.Fprintf(&assertions, "System.assertEquals(%s, ((Map<String,Object>)CheckboxObserver.values.get('%s')).get('flag'), '%s:flag');\n", row[3], key, key)
				fmt.Fprintf(&assertions, "System.assertEquals(%s, ((Map<String,Object>)CheckboxObserver.values.get('%s')).get('text'), '%s:text');\n", text, key, key)
				count++
			}
			if count != 9 {
				t.Fatalf("selected %d native rows, want 9", count)
			}
			source := prefix + assertions.String() + " } }"
			index := flowCoreIndex(t, api, map[string]string{name: source}, files)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{name}, NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("route selection: %+v", run.Summary())
			}
			if result := run.Suites[0].Cases[0]; result.Status != "pass" {
				t.Fatalf("%s: %+v", result.Status, result.Problem)
			}
		})
	}
}

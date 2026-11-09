package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNoRowsShareRowsNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/no_rows_share_rows"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 3 {
		t.Fatalf("invalid oracle: %v", err)
	}
	for _, row := range rows[1:] {
		api, method, expected := row[0], row[1], row[2]
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join("testdata/conformance/no_rows_visibility/project")
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
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, pass := range []string{"cold", "warm"} {
				t.Run(pass, func(t *testing.T) {
					run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"ShareRowsProbeTest"}, SelectedMethod: "observe", Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("missing observation: %+v", run.Summary())
					}
					result := run.Suites[0].Cases[0].Problem
					_, payload, ok := strings.Cut(result.Message, "OBSERVED|"+method+"|")
					var got, want map[string]any
					if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(expected), &want) != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%s; Glade=%s; problem=%+v", expected, payload, result)
					}
				})
			}
		})
	}
}

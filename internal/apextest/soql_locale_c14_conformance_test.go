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

	"github.com/glade-sh/glade/internal/sema"
)

func TestSOQLLocaleC14NativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "soql_locale_c14")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 23 {
		t.Fatalf("invalid native observations: %v", err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				if strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, "sfdx-project.json") {
					body = []byte(strings.ReplaceAll(string(body), "62.0", api))
				}
				writeFile(t, filepath.Join(root, rel), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				if len(row) != 5 {
					t.Fatal("malformed native row")
				}
				if row[0] != api {
					continue
				}
				if seen[row[1]+"/"+row[2]] {
					t.Fatal("duplicate method")
				}
				seen[row[1]+"/"+row[2]] = true
				t.Run(row[1]+"/"+row[2], func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{row[1]}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("expected selected method: %+v", run.Summary())
					}
					result := run.Suites[0].Cases[0]
					if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Fatalf("missing capture assertion: %+v", result.Problem)
					}
					got, err := parseCaptureObservation(result.Problem.Message, row[3])
					if err != nil {
						t.Fatal(err)
					}
					var want map[string]any
					if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
						t.Fatal(err)
					}
					got = normalizeCaptureIdentities(got).(map[string]any)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%v; Glade=%v", want, got)
					}
				})
			}
			if len(seen) != 11 {
				t.Fatal("incomplete source API observations")
			}
		})
	}
}

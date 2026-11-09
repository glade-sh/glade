package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestStdlibLocaleCryptoC14NativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "stdlib_locale_crypto_c14")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 29 {
		t.Fatalf("invalid native observations: %v", err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				if len(row) != 5 {
					t.Fatal("malformed native row")
				}
				if row[0] != api {
					continue
				}
				key := row[1] + "/" + row[2]
				if seen[key] {
					t.Fatal("duplicate method")
				}
				seen[key] = true
				t.Run(key, func(t *testing.T) {
					root := t.TempDir()
					classes := filepath.Join(dir, "project", "force-app", "main", "default", "classes")
					for _, suffix := range []string{".cls", ".cls-meta.xml"} {
						name := row[1] + suffix
						body, err := os.ReadFile(filepath.Join(classes, name))
						if err != nil {
							t.Fatal(err)
						}
						if suffix == ".cls-meta.xml" {
							body = []byte(strings.ReplaceAll(string(body), "62.0", api))
						}
						writeFile(t, filepath.Join(root, "force-app", "main", "default", "classes", name), string(body))
					}
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
					index := loadTestIndex(t, root)
					analysis := sema.Analyze(index)
					if index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
					}
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
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%v; Glade=%v", want, got)
					}
				})
			}
			if len(seen) != 14 {
				t.Fatal("incomplete source API observations")
			}
		})
	}
}

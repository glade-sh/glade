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

func TestTransactionStaticDeliveryNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "transaction_static_delivery", "TransactionStaticTest", []string{"62.0", "67.0", "68.0"}, 8)
}

// Each row is the complete assertion payload observed from the retained source.
// Class and trigger metadata are pinned together for each source API.
func runNativeObservationProject(t *testing.T, family, class string, apis []string, methodsPerAPI int) {
	t.Helper()
	runNativeObservationProjectWithCarries(t, family, class, apis, methodsPerAPI, nil)
}

type nativeObservationCarry struct {
	LocalJSON string
	Reason    string
}

func runNativeObservationProjectWithCarries(t *testing.T, family, class string, apis []string, methodsPerAPI int, carries map[string]nativeObservationCarry) {
	t.Helper()
	dir := filepath.Join("testdata", "conformance", family)
	raw, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	reader.LazyQuotes = true // The final TSV column contains an unquoted JSON object.
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1+len(apis)*methodsPerAPI || strings.Join(rows[0], ",") != "source_api,method,observation" {
		t.Fatal("incomplete native observations")
	}
	for _, api := range apis {
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
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				source := strings.ReplaceAll(string(body), "62.0", api)
				writeFile(t, filepath.Join(root, name), source)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				method := row[1]
				if method == "" || seen[method] {
					t.Fatal("duplicate or empty observation")
				}
				seen[method] = true
				t.Run(method, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("expected one observation: %+v", run.Summary())
					}
					result := run.Suites[0].Cases[0]
					if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Fatalf("missing assertion transport: %+v; problem: %+v", result, result.Problem)
					}
					_, payload, found := strings.Cut(result.Problem.Message, "OBSERVED|"+method+"|")
					var got, want map[string]any
					if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
						t.Fatalf("invalid observation: %s", result.Problem.Message)
					}
					if !reflect.DeepEqual(got, want) {
						if carry, ok := carries[api+"/"+method]; ok {
							var local map[string]any
							if carry.Reason == "" || json.Unmarshal([]byte(carry.LocalJSON), &local) != nil || !reflect.DeepEqual(got, local) {
								t.Fatalf("carried observation changed: native=%s; Glade=%s", row[2], payload)
							}
							t.Logf("CARRY: %s; native=%s; Glade=%s", carry.Reason, row[2], payload)
							return
						}
						t.Fatalf("native=%s; Glade=%s", row[2], payload)
					}
					if _, carried := carries[api+"/"+method]; carried {
						t.Fatal("observation now matches native; remove its carry")
					}
				})
			}
			if len(seen) != methodsPerAPI {
				t.Fatal("missing source API observations")
			}
		})
	}
}

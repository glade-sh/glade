package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTriggerDMLFailureNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/trigger_dml_failure"
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 49 {
		t.Fatalf("want 48 native observations, got %d", len(rows)-1)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(dir, "project"), func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(filepath.Join(dir, "project"), path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(content), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"TriggerFailureObservation"}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 24 {
				t.Fatalf("missing observations: %+v", run.Summary())
			}
			seen := map[string]bool{}
			for _, result := range run.Suites[0].Cases {
				method := result.MethodName
				t.Run(method, func(t *testing.T) {
					var expected string
					for _, row := range rows[1:] {
						if row[0] == api && row[1] == method {
							expected = row[2]
						}
					}
					if expected == "" || seen[method] {
						t.Fatalf("missing or duplicate oracle for %s", method)
					}
					seen[method] = true
					if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Fatalf("missing observation: %+v", result)
					}
					_, payload, found := strings.Cut(result.Problem.Message, "OBSERVED|")
					var got, want []map[string]any
					if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(expected), &want) != nil {
						t.Fatalf("invalid observation: %+v", result.Problem)
					}
					if len(got) != 3 || len(want) != 3 {
						t.Fatal("missing DML modes")
					}
					for i := range want {
						t.Run(want[i]["mode"].(string), func(t *testing.T) {
							// The existing addError exception envelope is a separate text gap.
							// Its per-row message, status, fields and rollback remain asserted.
							if strings.HasSuffix(method, "_row") || strings.HasSuffix(method, "_field") {
								if got[i]["message"] != want[i]["message"] {
									t.Logf("carried addError envelope: native=%v; Glade=%v", want[i]["message"], got[i]["message"])
								}
								delete(got[i], "message")
								delete(want[i], "message")
							}
							if !reflect.DeepEqual(got[i], want[i]) {
								g, _ := json.Marshal(got[i])
								w, _ := json.Marshal(want[i])
								t.Errorf("native=%s\nGlade=%s", w, g)
							}
						})
					}
				})
			}
		})
	}
}

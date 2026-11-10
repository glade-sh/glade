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

// Replay the captured metadata and named test, including the native action
// input shape and explicit system-mode DML.
func TestFlowBatchFilterNativeProject(t *testing.T) {
	const fixture = "testdata/conformance/flow_followups_batch_filter"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	for _, row := range rows[1:] {
		if row[0] != row[1] {
			continue
		}
		api := row[1]
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if api == "67.0" {
					if strings.HasSuffix(rel, "flows/SyntheticFault.flow-meta.xml") {
						body, err = os.ReadFile(filepath.Join(fixture, "ceiling-flow.xml"))
					}
					if strings.HasSuffix(rel, "classes/SyntheticAction.cls-meta.xml") {
						body, err = os.ReadFile(filepath.Join(fixture, "ceiling-action-meta.xml"))
					}
					if err != nil {
						return err
					}
				}
				writeFile(t, filepath.Join(root, rel), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			class := "FaultProbe" + strings.TrimSuffix(api, ".0")
			result := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "probe", RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
			if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected selection: %+v; %s", result.Summary(), firstRunProblem(result))
			}
			problem := result.Suites[0].Cases[0].Problem
			if problem == nil {
				t.Fatal("missing capture assertion")
			}
			marker := "CAPTURE|Batch|"
			pos := strings.Index(problem.Message, marker)
			if pos < 0 {
				t.Fatalf("missing capture payload: %s: %s", problem.Type, problem.Message)
			}
			var got struct {
				Rows map[string]any `json:"rows"`
			}
			if err := json.NewDecoder(strings.NewReader(problem.Message[pos+len(marker):])).Decode(&got); err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Rows, want) {
				t.Fatalf("got %#v; native %#v", got.Rows, want)
			}
		})
	}
}

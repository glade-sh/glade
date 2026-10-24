package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestFlowLookupOutputDecisionIsTestNativeConformance(t *testing.T) {
	const directory = "testdata/conformance/flow_lookup_output_decision_native"
	capture, err := os.ReadFile(filepath.Join(directory, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capture), "native_complete=true\n") {
		t.Fatal("incomplete native capture")
	}
	raw, err := os.ReadFile(filepath.Join(directory, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(rows) != 3 || rows[0] != "source_api\troute\tobservation" {
		t.Fatal("expected both native API observations")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		columns := strings.SplitN(row, "\t", 3)
		if len(columns) != 3 || (columns[0] != "62.0" && columns[0] != "67.0") || columns[1] != "istest" || seen[columns[0]] {
			t.Fatal("invalid or duplicate native observation")
		}
		api := columns[0]
		seen[api] = true
		t.Run(api+"/istest", func(t *testing.T) {
			var want map[string]map[string]map[string]any
			if err := json.Unmarshal([]byte(columns[2]), &want); err != nil {
				t.Fatal(err)
			}
			if len(want) != 4 || len(want["Prospect"]) != 2 || len(want["Other"]) != 2 || len(want["Customer - Direct"]) != 2 || len(want["Partner"]) != 2 {
				t.Fatal("native observation must cover matched and absent lookups for four decision shapes")
			}
			root := t.TempDir()
			projectDirectory := filepath.Join(directory, "api"+strings.TrimSuffix(api, ".0"))
			err := filepath.WalkDir(projectDirectory, func(path string, entry fs.DirEntry, err error) error {
				if err == nil && entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
					return filepath.SkipDir
				}
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(projectDirectory, path)
				if err != nil {
					return err
				}
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s:%s=%x\n", api, filepath.ToSlash(name), sha256.Sum256(body))) {
					t.Fatalf("native source mismatch: %s", name)
				}
				writeFile(t, filepath.Join(root, name), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"BirchFlowDecisionTest"}, SelectedMethod: "observeDecision", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected selection: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing observation assertion: %+v; problem: %+v", result, result.Problem)
			}
			_, payload, found := strings.Cut(result.Problem.Message, "OBSERVED|lookupDecision|")
			var got map[string]map[string]map[string]any
			if !found || json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatalf("invalid Glade observation: %s", result.Problem.Message)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", columns[2], payload)
			}
		})
	}
}

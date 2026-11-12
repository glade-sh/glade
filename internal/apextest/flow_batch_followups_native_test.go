package apextest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFlowBatchFollowupsNative replays the bulk-interview captures
// that follow an action-first after-save Flow: a fault connector that runs a
// filtered record update, and an after-update trigger that edits a later
// record of the same batch. Rows are native org.tsv observations.
func TestFlowBatchFollowupsNative(t *testing.T) {
	for _, tc := range []struct {
		fixture, marker string
		cases           int
	}{
		{"flow_action_fault_update", "FLOWFAULTUPDATE|", 3},
		{"flow_batch_refire", "FLOWREFIRE|", 2},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			fixture := filepath.Join("testdata/conformance", tc.fixture)
			data, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := make(map[string]map[string]any)
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
				cells := strings.SplitN(line, "\t", 4)
				if len(cells) != 4 || cells[1] != "istest" {
					t.Fatalf("bad native row %q", line)
				}
				var value any
				if err := json.Unmarshal([]byte(cells[3]), &value); err != nil {
					t.Fatal(err)
				}
				if rows[cells[0]] == nil {
					rows[cells[0]] = map[string]any{}
				}
				rows[cells[0]][cells[2]] = value
			}
			for _, api := range []string{"62.0", "67.0"} {
				t.Run(api, func(t *testing.T) {
					if len(rows[api]) != tc.cases {
						t.Fatalf("missing native observations for %s", api)
					}
					root := t.TempDir()
					sourceRoot := filepath.Join(fixture, "project")
					err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
						if err != nil || entry.IsDir() {
							return err
						}
						body, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						rel, err := filepath.Rel(sourceRoot, path)
						if err != nil {
							return err
						}
						source := strings.ReplaceAll(string(body), "<apiVersion>62.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>")
						writeFile(t, filepath.Join(root, rel), source)
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					index := loadTestIndex(t, root)
					result := Run(index, Options{SelectedClasses: []string{"FlowBatchProbeTest"}, SelectedMethod: "observe", RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
					if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
						t.Fatalf("unexpected selection: %+v; %s", result.Summary(), firstRunProblem(result))
					}
					problem := result.Suites[0].Cases[0].Problem
					if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
						t.Fatalf("missing captured assertion: %+v", problem)
					}
					checkFlowFollowupsPayload(t, problem.Message, tc.marker, rows[api])
				})
			}
		})
	}
}

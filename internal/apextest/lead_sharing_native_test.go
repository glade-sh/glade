package apextest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFlowBatchLeadSharingNative replays testdata/conformance/lead_sharing/org.tsv in
// the named runAs route. The Private rows are keyed to the deployed Lead
// metadata (project/private-model); the scratch default rows are keyed to the
// ReadWriteTransfer model recorded in RUN-FACTS.txt, never to an omitted Glade model.
func TestFlowBatchLeadSharingNative(t *testing.T) {
	const fixture = "testdata/conformance/lead_sharing"
	data, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := make(map[string]map[string]map[string]any)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 5)
		if len(cells) != 5 || cells[1] != "istest" {
			t.Fatalf("bad native row %q", line)
		}
		var value any
		if err := json.Unmarshal([]byte(cells[4]), &value); err != nil {
			t.Fatal(err)
		}
		key := cells[0] + "/" + cells[2]
		if rows[key] == nil {
			rows[key] = map[string]map[string]any{}
		}
		if rows[key]["m"] == nil {
			rows[key]["m"] = map[string]any{}
		}
		rows[key]["m"][cells[3]] = value
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, model := range []string{"Private", "scratch_default"} {
			t.Run(api+"/"+model, func(t *testing.T) {
				want := rows[api+"/"+model]["m"]
				if len(want) != 8 {
					t.Fatalf("missing native observations for %s/%s", api, model)
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
					if strings.HasPrefix(filepath.ToSlash(rel), "private-model/") {
						rel = filepath.Join("force-app", "main", "default", strings.TrimPrefix(filepath.ToSlash(rel), "private-model/"))
						if model == "scratch_default" {
							// RUN-FACTS.txt: the fresh scratch org's Lead InternalSharingModel.
							body = []byte(strings.ReplaceAll(string(body), "<sharingModel>Private</sharingModel>", "<sharingModel>ReadWriteTransfer</sharingModel>"))
						}
					}
					source := strings.ReplaceAll(string(body), "<apiVersion>62.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>")
					writeFile(t, filepath.Join(root, rel), source)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				result := Run(index, Options{SelectedClasses: []string{"LeadSharingProbeTest"}, SelectedMethod: "observe", RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
				if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
					t.Fatalf("unexpected selection: %+v; %s", result.Summary(), firstRunProblem(result))
				}
				problem := result.Suites[0].Cases[0].Problem
				if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
					t.Fatalf("missing captured assertion: %+v", problem)
				}
				checkFlowFollowupsPayload(t, problem.Message, "LEADSHARING|", want)
			})
		}
	}
}

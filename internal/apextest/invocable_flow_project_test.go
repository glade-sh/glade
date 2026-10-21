package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Additional caller-input variations remain local expectations without native rows.
func invocableFlowProjectFixture(t *testing.T, api string) string {
	t.Helper()
	source := filepath.Join("testdata", "invocable_flow_project")
	root := t.TempDir()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == filepath.Join(source, "captured") {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := strings.ReplaceAll(string(raw), "<apiVersion>62.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>")
		if relative == "sfdx-project.json" {
			text = fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api)
		}
		writeFile(t, filepath.Join(root, relative), text)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func runInvocableFlowProjectMethods(t *testing.T, methods []string) {
	t.Helper()
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			// Use the ordinary project/index/runner path, including trigger compilation.
			index := loadTestIndex(t, invocableFlowProjectFixture(t, api))
			if index.HasErrors() {
				t.Fatalf("synthetic project index: %v", index.Diagnostics)
			}
			for _, method := range methods {
				t.Run(method, func(t *testing.T) {
					t.Log("uncaptured: needs native row for this caller-input variation; not counted as Salesforce conformance")
					run := Run(index, Options{SelectedClasses: []string{"OwnedFlowProjectTest"}, SelectedMethod: method, NoDiskCache: true})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
						t.Fatalf("Glade-side expectation; uncaptured: needs native row; summary=%#v problem=%s", summary, firstRunProblem(run))
					}
				})
			}
		})
	}
}

func TestInvocableFlowTriggerExpectationsUncaptured(t *testing.T) {
	runInvocableFlowProjectMethods(t, []string{"uncapturedChangedBeforeUpdate", "uncapturedDistinctAfterInsert"})
}

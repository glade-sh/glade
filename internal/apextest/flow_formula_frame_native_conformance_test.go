package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowFormulaFrameNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_formula_frame"
	capture, err := os.ReadFile(filepath.Join(fixture, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 {
			t.Fatalf("bad native row %q", line)
		}
		key := cells[0] + "/" + cells[1]
		if want[key] == nil {
			want[key] = map[string]any{}
		}
		var value any
		if err := json.Unmarshal([]byte(cells[3]), &value); err != nil {
			t.Fatal(err)
		}
		want[key][cells[2]] = value
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				source := strings.ReplaceAll(string(data), "62.0", api)
				pin := fmt.Sprintf("source_sha256:%s:%s=%x\n", api, filepath.ToSlash(rel), sha256.Sum256([]byte(source)))
				if !strings.Contains("\n"+string(capture), "\n"+pin) {
					return fmt.Errorf("source mismatch: %s", rel)
				}
				writeFile(t, filepath.Join(root, rel), source)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			org, err := orgFromIndexWithError(index)
			if err != nil {
				t.Fatal(err)
			}
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, route := range []string{"anonymous", "anonymous_reused_runtime"} {
				t.Run(route, func(t *testing.T) {
					source, err := os.ReadFile(filepath.Join(root, "capture.apex"))
					if err != nil {
						t.Fatal(err)
					}
					program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = runner.execute(program, func(machine *vm.VM) { machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID]) })
					var runtimeErr *vm.RuntimeError
					if !errors.As(err, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
						t.Fatalf("missing observation: %v", err)
					}
					checkFlowFollowupsPayload(t, runtimeErr.ExceptionMessage(), "FFFRAME|", want[api+"/anonymous"])
				})
			}
			for _, route := range []string{"project_runner", "project_runner_reused_index"} {
				t.Run(route, func(t *testing.T) {
					result := Run(index, Options{SelectedClasses: []string{"FFFrameProbeTest"}, SelectedMethod: "capture", RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
					if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
						t.Fatalf("selection: %+v; %s", result.Summary(), firstRunProblem(result))
					}
					problem := result.Suites[0].Cases[0].Problem
					if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
						t.Fatalf("missing observation: %+v", problem)
					}
					checkFlowFollowupsPayload(t, problem.Message, "FFFRAME|", want[api+"/istest"])
				})
			}
		})
	}
}

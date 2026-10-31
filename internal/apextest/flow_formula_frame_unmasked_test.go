package apextest

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Replay captured non-error rows independently so a partial-insert failure
// cannot mask assignment, decision or create formula differences.
func TestFlowFormulaFrameUnmaskedNativeRows(t *testing.T) {
	const fixture = "testdata/conformance/flow_formula_frame"
	data, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 {
			t.Fatalf("invalid row %q", line)
		}
		if cells[1] != "anonymous" || cells[2] == "ERROR" {
			continue
		}
		if rows[cells[0]] == nil {
			rows[cells[0]] = map[string]any{}
		}
		var value any
		if err := json.Unmarshal([]byte(cells[3]), &value); err != nil {
			t.Fatal(err)
		}
		rows[cells[0]][cells[2]] = value
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil || entry.IsDir() {
					return walkErr
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
				if filepath.Base(path) == "FFFrameProbe.cls" {
					const all = "'ASSIGN','DECIDE','CREATE','ERROR','CONTROL'"
					if strings.Count(source, all) != 1 {
						t.Fatal("captured probe case list changed")
					}
					source = strings.Replace(source, all, "'ASSIGN','DECIDE','CREATE','CONTROL'", 1)
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
			if len(rows[api]) != 4 {
				t.Fatalf("expected four captured rows, got %d", len(rows[api]))
			}
			checkFlowFollowupsPayload(t, runtimeErr.ExceptionMessage(), "FFFRAME|", rows[api])
		})
	}
}

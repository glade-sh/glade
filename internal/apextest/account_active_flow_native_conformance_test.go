package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestAccountActiveFlowNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/account_active_flow"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	anonymous, shape := read("anonymous.apex"), read("scratch.json")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), shape)
			sources := map[string]string{"anonymous.apex": anonymous, "scratch.json": shape}
			for _, directory := range []string{"classes", "flows"} {
				err := filepath.WalkDir(filepath.Join(fixture, version, directory), func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					name, err := filepath.Rel(filepath.Join(fixture, version), path)
					if err != nil {
						return err
					}
					sources[name] = read(filepath.Join(version, name))
					writeFile(t, filepath.Join(root, "force-app/main/default", name), sources[name])
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					capture := filepath.Join(version, "native", route)
					run := read(filepath.Join(capture, "RUN-FACTS.txt"))
					for _, fact := range []string{"api=" + api, "context=" + route, "rows=5", "native_complete=true", "deployment_success=true"} {
						if !strings.Contains("\n"+run, "\n"+fact+"\n") {
							t.Fatalf("incomplete capture: %s", fact)
						}
					}
					for name, source := range sources {
						if !strings.Contains(run, fmt.Sprintf("source_sha256:%s=%x\n", name, sha256.Sum256([]byte(source)))) {
							t.Fatalf("capture source changed: %s", name)
						}
					}
					rows := strings.Split(strings.TrimSpace(read(filepath.Join(capture, "org.tsv"))), "\n")
					if len(rows) != 6 || rows[0] != "id\tgroup\troute\tstatus\tvalue" {
						t.Fatal("incomplete native rows")
					}
					var message string
					if route == "anonymous" {
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						program, err := vm.CompileAnonymousWithOptions(anonymous, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						var observation *vm.RuntimeError
						if !errors.As(err, &observation) || flowRefsExceptionType(observation.Type) != "System.AssertException" {
							t.Fatalf("missing observation: %v", err)
						}
						message = observation.ExceptionMessage()
					} else {
						run := Run(index, Options{SelectedClasses: []string{"AccountActiveProbeTest"}, SelectedMethod: "observeActive", NoDiskCache: true, Parallelism: 1})
						if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("unexpected test selection: %+v", run.Summary())
						}
						observation := run.Suites[0].Cases[0]
						if observation.Problem == nil || flowRefsExceptionType(observation.Problem.Type) != "System.AssertException" {
							t.Fatalf("missing observation: %+v", observation)
						}
						message = observation.Problem.Message
					}
					_, payload, found := strings.Cut(message, "ACTIVE_RESULT|")
					var got map[string]map[string]any
					if !found || json.Unmarshal([]byte(payload), &got) != nil || len(got) != 5 {
						t.Fatalf("invalid observation: %s", message)
					}
					seen := map[string]bool{}
					for _, line := range rows[1:] {
						row := strings.SplitN(line, "\t", 5)
						if len(row) != 5 || seen[row[0]] || row[1] != "ACCOUNT_ACTIVE_FLOW" || row[2] != route || row[3] != "runtime" {
							t.Fatalf("invalid native row: %s", line)
						}
						seen[row[0]] = true
						t.Run(row[0], func(t *testing.T) {
							var want map[string]any
							if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(got[row[0]], want) {
								t.Fatalf("native %s; Glade %s", row[4], mustMarshalActiveObservation(t, got[row[0]]))
							}
						})
					}
				})
			}
		})
	}
}

func mustMarshalActiveObservation(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

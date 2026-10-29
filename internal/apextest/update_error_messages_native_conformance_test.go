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

// These owned fixtures compare the complete update exception/SaveResult shape,
// including the first failing bulk index and rollback, against API 62/67.
func TestUpdateErrorMessagesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/update_error_messages"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native fixture rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					capture := filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), route)
					body, err := os.ReadFile(filepath.Join(capture, "org.tsv"))
					if err != nil {
						t.Fatal(err)
					}
					lines := strings.Split(strings.TrimSpace(string(body)), "\n")
					if len(lines) != 57 || lines[0] != "id\tvalue" {
						t.Fatal("incomplete oracle")
					}
					facts, err := os.ReadFile(filepath.Join(capture, "RUN-FACTS.txt"))
					if err != nil {
						t.Fatal(err)
					}
					for _, fact := range []string{"api=" + api, "context=" + route, "rows=56", "native_complete=true", "deployment_success=true"} {
						if !strings.Contains("\n"+string(facts), "\n"+fact+"\n") {
							t.Fatalf("missing capture fact %s", fact)
						}
					}
					if err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry fs.DirEntry, err error) error {
						if err != nil || entry.IsDir() {
							return err
						}
						source, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						relative, err := filepath.Rel(filepath.Join(fixture, "project"), path)
						if err != nil {
							return err
						}
						source = []byte(strings.ReplaceAll(string(source), "62.0", api))
						fact := fmt.Sprintf("source_sha256:%s=%x\n", relative, sha256.Sum256(source))
						if !strings.Contains(string(facts), fact) {
							t.Fatalf("capture source changed: %s", relative)
						}
						return nil
					}); err != nil {
						t.Fatal(err)
					}
					want := map[string]any{}
					for _, line := range lines[1:] {
						id, payload, found := strings.Cut(line, "\t")
						if !found || want[id] != nil {
							t.Fatalf("invalid native row %s", line)
						}
						var value any
						if err := json.Unmarshal([]byte(payload), &value); err != nil {
							t.Fatal(err)
						}
						want[id] = value
					}
					check := func(t *testing.T, id, message string) {
						t.Helper()
						_, payload, found := strings.Cut(message, "OBSERVED|")
						var got any
						if !found || json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatalf("missing observation: %s", message)
						}
						if !reflect.DeepEqual(got, want[id]) {
							expected, _ := json.Marshal(want[id])
							t.Errorf("%s native=%s\nGlade=%s", id, expected, payload)
						}
					}
					if route == "anonymous" {
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						triggers, triggerErrors := compileProjectTriggers(index)
						if len(triggerErrors) != 0 {
							t.Fatal(triggerErrors)
						}
						for _, trigger := range triggers {
							if err := runner.base.RegisterTrigger(trigger); err != nil {
								t.Fatal(err)
							}
						}
						// Reuse the frozen project runtime for every independent row.
						for _, line := range lines[1:] {
							id, _, _ := strings.Cut(line, "\t")
							t.Run(id, func(t *testing.T) {
								parts := strings.Split(id, "_")
								source := fmt.Sprintf("System.assert(false, 'OBSERVED|' + JSON.serialize(UpdateMessageProbe.observe('%s','%s',%t)));", parts[0], parts[2], parts[1] == "bulk")
								program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								_, err = runner.newMachine().Execute(program)
								var fault *vm.RuntimeError
								if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
									t.Fatalf("missing observation: %v", err)
								}
								check(t, id, fault.ExceptionMessage())
							})
						}
					} else {
						// The second run exercises reuse of the same project's runtime.
						for _, mode := range []string{"cold", "warm"} {
							t.Run(mode, func(t *testing.T) {
								run := Run(index, Options{SelectedClasses: []string{"UpdateMessageObservation"}, NoDiskCache: true, Parallelism: 1})
								if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 56 {
									t.Fatalf("missing cases: %+v", run.Summary())
								}
								for _, result := range run.Suites[0].Cases {
									t.Run(result.MethodName, func(t *testing.T) {
										if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
											t.Fatalf("missing observation: %+v", result)
										}
										check(t, result.MethodName, result.Problem.Message)
									})
								}
							})
						}
					}
				})
			}
		})
	}
}

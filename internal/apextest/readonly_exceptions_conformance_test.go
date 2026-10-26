package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestReadOnlyExceptionsNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/readonly_exceptions"
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			read := func(path string) []byte {
				t.Helper()
				body, err := os.ReadFile(filepath.Join(fixture, path))
				if err != nil {
					t.Fatal(err)
				}
				return body
			}
			var capture struct {
				SourceAPI              string
				DeploymentSuccess      bool
				SourceSHA256           map[string]string
				Methods, ReadOnlyCells int
			}
			if err := json.Unmarshal(read("native/api"+api+"/RUN-FACTS.txt"), &capture); err != nil {
				t.Fatal(err)
			}
			if capture.SourceAPI != api+".0" || !capture.DeploymentSuccess || capture.Methods != 15 || capture.ReadOnlyCells != 40 || len(capture.SourceSHA256) != 7 {
				t.Fatal("incomplete native capture")
			}
			root := t.TempDir()
			for name, digest := range capture.SourceSHA256 {
				body := read("api" + api + "/" + name)
				if fmt.Sprintf("%x", sha256.Sum256(body)) != digest {
					t.Fatalf("captured source changed: %s", name)
				}
				writeFile(t, filepath.Join(root, name), string(body))
			}
			index := loadTestIndex(t, root)
			t.Run("compilation", func(t *testing.T) {
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native source compiled; local diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
			})
			cases := Discover(index, Options{SelectedClasses: []string{"CedarReadOnlyTest"}})
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			triggers, failures := compileProjectTriggers(index)
			if len(failures) != 0 {
				t.Fatal(failures)
			}
			for _, trigger := range triggers {
				if err := runner.base.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
			}
			rows := strings.Split(strings.TrimSpace(string(read("native/api"+api+"/org.tsv"))), "\n")
			if len(rows) != 16 || rows[0] != "method\tresult" || len(cases) != 15 {
				t.Fatal("incomplete native method table")
			}
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				name, payload, ok := strings.Cut(row, "\t")
				if !ok || seen[name] {
					t.Fatal("invalid native row")
				}
				seen[name] = true
				t.Run(name, func(t *testing.T) {
					var want struct {
						Outcome, Type, Message string
						Value                  any
					}
					if err := json.Unmarshal([]byte(payload), &want); err != nil {
						t.Fatal(err)
					}
					if name == "observeReadOnly" {
						cells, ok := want.Value.(map[string]any)
						if !ok || len(cells) != capture.ReadOnlyCells {
							t.Fatal("incomplete read-only matrix")
						}
					}
					for _, c := range cases {
						if c.MethodName != name {
							continue
						}
						key := testCaseKey(c)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, execErr := machine.ExecuteInClass(programs[key], c.ClassName)
						var fault *vm.RuntimeError
						if !errors.As(execErr, &fault) {
							t.Fatalf("missing terminal observation: %v", execErr)
						}
						kind := fault.Type
						if !strings.HasPrefix(kind, "System.") {
							kind = "System." + kind
						}
						if want.Outcome == "uncaught" {
							if kind != want.Type || fault.ExceptionMessage() != want.Message {
								t.Fatalf("native=%s: %s; local=%s: %s", want.Type, want.Message, kind, fault.ExceptionMessage())
							}
						} else {
							_, observed, found := strings.Cut(fault.ExceptionMessage(), "OBSERVED|")
							if kind != "System.AssertException" || !found {
								t.Fatalf("native observation missing: %s: %s", kind, fault.ExceptionMessage())
							}
							var got any = observed
							if name == "observeReadOnly" {
								if err := json.Unmarshal([]byte(observed), &got); err != nil {
									t.Fatal(err)
								}
							}
							if !reflect.DeepEqual(got, want.Value) {
								t.Fatalf("native=%v; local=%v", want.Value, got)
							}
						}
						return
					}
					t.Fatalf("native method not found: %s", name)
				})
			}
		})
	}
}

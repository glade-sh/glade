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
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestApprovalSubmissionNativeConformance(t *testing.T) {
	t.Run("single_request", func(t *testing.T) {
		testApprovalSubmissionNativeConformance(t, "testdata/conformance/approval_submission", []string{"missing", "inactive", "no_applicable", "pending", "null_id", "success"}, 31)
	})
	t.Run("mixed_requests", func(t *testing.T) {
		testApprovalSubmissionNativeConformance(t, "testdata/conformance/approval_submission/mixed", []string{"mixed_late", "mixed_early", "mixed_success", "mixed_workitems", "mixed_reject", "single_approve", "single_reject", "single_invalid"}, 48)
	})
}

func testApprovalSubmissionNativeConformance(t *testing.T, fixture string, shapes []string, rowCount int) {
	t.Helper()
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			sources := map[string]string{}
			err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				name, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				if strings.HasPrefix(name, ".sf/") || strings.HasPrefix(name, ".sfdx/") {
					return nil
				}
				source := read(filepath.Join("project", name))
				if strings.HasSuffix(name, "-meta.xml") || name == "sfdx-project.json" {
					source = strings.ReplaceAll(source, "62.0", api)
				}
				sources[name] = source
				writeFile(t, filepath.Join(root, name), source)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source accepted; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			// The runtime consumes ProcessDefinition rows. Approval metadata loading
			// is outside this test; seed the two definitions deployed natively.
			storage.EnsureStandardObject(&org, "ProcessDefinition")
			definitions := org.Objects["ProcessDefinition"]
			for i, state := range []string{"Active", "Inactive"} {
				id := storage.ID(fmt.Sprintf("04a00000000000%dAAA", i+1))
				definitions.Records[id] = storage.Record{ID: id, Object: "ProcessDefinition", Fields: map[string]storage.Value{
					"Id": storage.IDValue(id), "DeveloperName": storage.StringValue("Owned_" + state),
					"State": storage.StringValue(state), "TableEnumOrId": storage.StringValue("Account"), "Type": storage.StringValue("Approval"),
				}}
			}
			org.Objects["ProcessDefinition"] = definitions
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					capture := "api" + strings.TrimSuffix(api, ".0") + "/" + route
					run := read(capture + "/RUN-FACTS.txt")
					for _, fact := range []string{"api=" + api, "context=" + route, fmt.Sprintf("rows=%d", rowCount), "native_complete=true", "deployment_success=true"} {
						if !strings.Contains("\n"+run, "\n"+fact+"\n") {
							t.Fatalf("incomplete capture: %s", fact)
						}
					}
					for name, source := range sources {
						if !strings.Contains(run, fmt.Sprintf("source_sha256:%s=%x\n", name, sha256.Sum256([]byte(source)))) {
							t.Fatalf("capture source changed: %s", name)
						}
					}
					rows := strings.Split(strings.TrimSpace(read(capture+"/org.tsv")), "\n")
					if len(rows) != rowCount+1 || rows[0] != "case\tobservation" {
						t.Fatal("incomplete native matrix")
					}
					expected := map[string]any{}
					for _, line := range rows[1:] {
						key, value, ok := strings.Cut(line, "\t")
						if !ok || expected[key] != nil {
							t.Fatalf("invalid row: %s", line)
						}
						var observation any
						if err := json.Unmarshal([]byte(value), &observation); err != nil {
							t.Fatal(err)
						}
						expected[key] = observation
					}
					for _, shape := range shapes {
						t.Run(shape, func(t *testing.T) {
							source := read("anonymous.apex")
							call := "ApprovalSubmissionProbe.observeGroup('" + shape + "')"
							if shape == "success" {
								call = "ApprovalSubmissionProbe.success()"
							}
							source = "System.assert(false, 'APPROVAL_ROWS|' + JSON.serialize(" + call + "));"
							if route == "istest" {
								source = "ApprovalSubmissionProbeTest." + shape + "();"
							}
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							machine := runner.newMachine()
							if route == "istest" {
								machine.EnableTestContext()
								if err := RegisterTestMethodForRequest(machine, index, "ApprovalSubmissionProbeTest", shape); err != nil {
									t.Fatal(err)
								}
							}
							if route == "istest" {
								_, err = machine.ExecuteInClass(program, "ApprovalSubmissionProbeTest")
							} else {
								_, err = machine.Execute(program)
							}
							var observation *vm.RuntimeError
							if !errors.As(err, &observation) || !strings.HasSuffix(observation.Type, "AssertException") {
								t.Fatalf("missing observation: %v", err)
							}
							_, payload, ok := strings.Cut(observation.ExceptionMessage(), "APPROVAL_ROWS|")
							var got map[string]any
							if !ok || json.Unmarshal([]byte(payload), &got) != nil {
								t.Fatalf("invalid observation: %v", err)
							}
							count := 6
							if shape == "success" {
								count = 1
							}
							if len(got) != count {
								t.Fatalf("observations=%d, want %d", len(got), count)
							}
							for key, actual := range got {
								t.Run(key, func(t *testing.T) {
									want, ok := expected[key]
									if !ok {
										t.Fatalf("uncaptured observation: %s", key)
									}
									if !reflect.DeepEqual(actual, want) {
										native, _ := json.Marshal(want)
										local, _ := json.Marshal(actual)
										t.Fatalf("native=%s; Glade=%s", native, local)
									}
								})
							}
						})
					}
				})
			}
		})
	}
}

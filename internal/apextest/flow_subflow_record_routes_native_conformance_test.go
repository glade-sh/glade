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

func TestFlowSubflowRecordRoutesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_subflow_record_routes"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	anonymous := read("anonymous.apex")
	routes := []string{"after-flow", "before-flow", "before-invocable", "after-invocable", "before-interview", "after-interview", "invocable", "interview"}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			// Native rejects a Subflow in a before-save Flow. The accepted twin
			// inlines its lookup; before-trigger Apex calls still use the subflow.
			var rejection struct {
				Success  bool                                               `json:"success"`
				Failures []struct{ Component, Problem, ProblemType string } `json:"failures"`
			}
			if err := json.Unmarshal([]byte(read(version+"/rejected.json")), &rejection); err != nil {
				t.Fatal(err)
			}
			if rejection.Success || len(rejection.Failures) != 1 || rejection.Failures[0].Component != "CedarBeforeSubflow" || !strings.Contains(rejection.Failures[0].Problem, "Subflow elements") {
				t.Fatalf("missing native before-save restriction: %+v", rejection)
			}
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
				body := strings.ReplaceAll(read("project/"+filepath.ToSlash(name)), "62.0", api)
				sources[filepath.ToSlash(name)] = body
				writeFile(t, filepath.Join(root, name), body)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(runtime.Triggers) != 4 || len(org.Metadata.Flows) != 2 || len(org.Objects["Contact"].Definition.FlowRules) != 2 {
				t.Fatal("expected four trigger entries, two autolaunched flows and two record-triggered flows")
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			for _, context := range []string{"anonymous", "istest"} {
				t.Run(context, func(t *testing.T) {
					capture := read(version + "/" + context + "/RUN-FACTS.txt")
					for key, value := range map[string]string{"api": api, "context": context, "rows": "16", "native_complete": "true", "deployment_success": "true"} {
						if !strings.Contains("\n"+capture, "\n"+key+"="+value+"\n") {
							t.Fatalf("missing native fact %s", key)
						}
					}
					for path, source := range sources {
						if !strings.Contains(capture, fmt.Sprintf("source_sha256:%s=%x\n", path, sha256.Sum256([]byte(source)))) {
							t.Fatalf("native source changed: %s", path)
						}
					}
					if !strings.Contains(capture, fmt.Sprintf("anonymous_source_sha256=%x\n", sha256.Sum256([]byte(anonymous)))) {
						t.Fatal("native anonymous source changed")
					}
					rejected := strings.ReplaceAll(read("rejected/CedarBeforeSubflow.flow-meta.xml"), "62.0", api)
					if !strings.Contains(capture, fmt.Sprintf("rejected_source_sha256=%x\n", sha256.Sum256([]byte(rejected)))) {
						t.Fatal("native rejected before-save source changed")
					}
					lines := strings.Split(strings.TrimSpace(read(version+"/"+context+"/org.tsv")), "\n")
					if len(lines) != 17 || lines[0] != "route\toperation\tobservation" {
						t.Fatal("incomplete native route matrix")
					}
					observations := map[string]string{}
					for _, line := range lines[1:] {
						parts := strings.SplitN(line, "\t", 3)
						if len(parts) != 3 || observations[parts[0]+"/"+parts[1]] != "" {
							t.Fatalf("invalid native row: %s", line)
						}
						observations[parts[0]+"/"+parts[1]] = parts[2]
					}
					for _, route := range routes {
						for _, operation := range []string{"insert", "update"} {
							t.Run(route+"/"+operation, func(t *testing.T) {
								wantJSON := observations[route+"/"+operation]
								var want, got map[string]any
								if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
									t.Fatal(err)
								}
								var baseline struct{ Description string }
								if err := json.Unmarshal([]byte(observations["after-flow/"+operation]), &baseline); err != nil || baseline.Description == "" || want["description"] != baseline.Description {
									t.Fatal("native routes must produce the same stored description")
								}
								var message string
								if context == "anonymous" {
									code := strings.NewReplacer("ROUTE", route, "OPERATION", operation).Replace(anonymous)
									analysis := sema.AnalyzeAnonymous(index, code, api)
									program, err := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
									if err != nil || analysis.HasErrors() {
										t.Fatalf("anonymous compile: %v %v", err, analysis.Diagnostics)
									}
									_, err = base.execute(program)
									var runtimeErr *vm.RuntimeError
									if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
										t.Fatalf("native=%s; Glade error=%v", wantJSON, err)
									}
									message = runtimeErr.ExceptionMessage()
								} else {
									method := ""
									for _, word := range strings.Split(route+"-"+operation, "-") {
										method += strings.ToUpper(word[:1]) + word[1:]
									}
									run := Run(index, Options{SelectedClasses: []string{"CedarRouteProbeTest"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
									if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
										t.Fatalf("unexpected named selection: %+v", run.Summary())
									}
									result := run.Suites[0].Cases[0]
									if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
										t.Fatalf("native=%s; Glade problem=%+v", wantJSON, result.Problem)
									}
									message = result.Problem.Message
								}
								_, payload, found := strings.Cut(message, "CEDAR_RESULT|")
								if !found || json.Unmarshal([]byte(payload), &got) != nil {
									t.Fatalf("native=%s; Glade observation=%s", wantJSON, message)
								}
								if !reflect.DeepEqual(got, want) {
									t.Fatalf("native=%s; Glade=%s", wantJSON, payload)
								}
							})
						}
					}
				})
			}
		})
	}
}

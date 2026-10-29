package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

type innerReceiverCase struct {
	ID, Body, AnonymousSource, TestClass, TestMethod, TestSource string
	Classes, SetupClasses                                        map[string]string
	RunnerReject                                                 bool
	Captures                                                     []struct {
		API, Route, Outcome, NativeMessage string
		Compiled, Success                  bool
	}
}

// The native matrix pins receiver binding, compile acceptance, and runtime
// observations. A rejection must reproduce a captured root diagnostic, rather
// than pass because of an unrelated error elsewhere in the helper project.
func TestInnerTypeReceiverNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/inner_type_receivers"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []innerReceiverCase
	if err := json.Unmarshal(raw, &cases); err != nil || len(cases) != 54 {
		t.Fatalf("native cases: count=%d err=%v", len(cases), err)
	}
	byID := map[string]innerReceiverCase{}
	expected := map[string]string{}
	// Replay the same deployed helper project, including foreign inner names.
	projects := map[string]map[string]string{"62.0": {}, "67.0": {}}
	directSetup := map[string]string{}
	for _, c := range cases {
		if c.ID == "" || byID[c.ID].ID != "" || len(c.Captures) != 4 {
			t.Fatalf("incomplete or duplicate capture: %s", c.ID)
		}
		byID[c.ID] = c
		for name, source := range c.SetupClasses {
			directSetup[name] = source
		}
		for _, r := range c.Captures {
			if projects[r.API] == nil || (r.Route != "anonymous" && r.Route != "isTest") {
				t.Fatalf("invalid native cell: %s/%s/%s", c.ID, r.API, r.Route)
			}
			key := c.ID + "/" + r.API + "/" + r.Route
			if _, exists := expected[key]; exists {
				t.Fatalf("duplicate native cell: %s", key)
			}
			expected[key] = strings.Join([]string{strconv.FormatBool(r.Compiled), strconv.FormatBool(r.Success), r.Outcome}, "\t")
			if r.Route == "isTest" && r.Compiled {
				for name, source := range c.Classes {
					projects[r.API][name] = source
				}
			}
		}
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 217 || strings.Join(rows[0], ",") != "id,api,route,compiled,success,outcome" {
		t.Fatalf("incomplete native table: rows=%d err=%v", len(rows), err)
	}
	seen := map[string]bool{}
	for i, row := range rows[1:] {
		if len(row) != 6 {
			t.Fatalf("invalid native row: %v", row)
		}
		key := strings.Join(row[:3], "/")
		if seen[key] || expected[key] != strings.Join(row[3:], "\t") {
			t.Fatalf("native table differs from capture: %s", key)
		}
		seen[key] = true
		c := byID[row[0]]
		t.Run(key, func(t *testing.T) {
			t.Logf("native: %s/org.tsv:%d compiled=%s success=%s outcome=%q", dir, i+2, row[3], row[4], row[5])
			files := map[string]string{}
			for name, source := range projects[row[1]] {
				files[name] = source
			}
			if c.RunnerReject {
				for name, source := range directSetup {
					files[name] = source
				}
			}
			if row[2] == "isTest" {
				for name, source := range c.Classes {
					files[name] = source
				}
				files[c.TestClass] = c.TestSource
			}
			index := nativeBoundaryIndex(t, row[1], files, nil)
			var prepared innerShapeAnonymous
			diagnostics := append([]diagnostic.Diagnostic(nil), index.Diagnostics...)
			if row[2] == "anonymous" {
				body := c.Body
				if c.AnonymousSource != "" {
					body = c.AnonymousSource
				}
				prepared = innerShapePrepareAnonymous(t, row[1], index, body)
				diagnostics = append(diagnostics, prepared.diagnostics...)
			} else {
				diagnostics = append(diagnostics, sema.Analyze(index).Diagnostics...)
			}
			var problems []string
			for _, d := range diagnostics {
				if d.Severity == diagnostic.Error {
					message := d.NativeMessage
					if message == "" {
						message = d.Message
					}
					problems = append(problems, message)
				}
			}
			compiled := len(problems) == 0
			if compiled != (row[3] == "true") {
				t.Fatalf("native compiled=%s; glade compiled=%t diagnostics=%v", row[3], compiled, problems)
			}
			if !compiled {
				matchedRoot := false
				for _, native := range strings.Split(row[5], " | ") {
					for _, problem := range problems {
						matchedRoot = matchedRoot || strings.TrimSpace(native) == strings.TrimSpace(problem)
					}
				}
				if !matchedRoot || row[4] != "false" {
					t.Fatalf("native rejection=%q; glade diagnostics=%v", row[5], problems)
				}
				if row[2] == "isTest" {
					run := Run(index, Options{SelectedClasses: []string{c.TestClass}, SelectedMethod: c.TestMethod, NoDiskCache: true, Parallelism: 1})
					if s := run.Summary(); c.RunnerReject && (s.Total != 1 || s.CompileErrors != 1) {
						t.Fatalf("native rejected test deployment; project runner=%+v", s)
					}
					// Earlier deployment-only wrappers are independent of the bad
					// helper. The runner excludes that helper without rejecting an
					// unrelated selected test.
					if !c.RunnerReject {
						excludedHelper := false
						for _, exclusion := range run.ExcludedClasses {
							_, owned := c.Classes[exclusion.Root]
							excludedHelper = excludedHelper || owned
						}
						if !excludedHelper {
							t.Fatalf("native rejected helper was not excluded: %+v", run.ExcludedClasses)
						}
					}
				}
				return
			}
			if row[2] == "isTest" {
				for _, temperature := range []string{"cold", "warm"} {
					t.Run(temperature, func(t *testing.T) {
						run := Run(index, Options{SelectedClasses: []string{c.TestClass}, SelectedMethod: c.TestMethod, NoDiskCache: true, Parallelism: 1})
						if s := run.Summary(); s.Total != 1 {
							t.Fatalf("test discovery: %+v", s)
						}
						success, outcome := false, ""
						for _, suite := range run.Suites {
							for _, result := range suite.Cases {
								if result.Problem != nil {
									if _, observation, ok := strings.Cut(result.Problem.Message, "OBS|"); ok && strings.HasSuffix(result.Problem.Type, "AssertException") {
										success, outcome = true, observation
									} else {
										outcome = result.Problem.Type + ": " + result.Problem.Message
									}
								} else if result.Status != testreport.StatusPass {
									outcome = fmt.Sprintf("status: %s", result.Status)
								}
							}
						}
						if success != (row[4] == "true") || !innerShapeOutcomesEqual(outcome, row[5]) {
							t.Fatalf("native success=%s outcome=%q; glade success=%t outcome=%q", row[4], row[5], success, outcome)
						}
					})
				}
				return
			}
			program, err := vm.CompileAnonymousWithOptions(prepared.body, vm.CompileOptions{APIVersion: row[1], ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(prepared.merged, prepared.body, row[1])})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(prepared.merged)
			org.APIVersion = row[1]
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if len(prepared.transient.Types) > 0 {
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(prepared.transient, nil)
				if err != nil {
					t.Fatal(err)
				}
				runner = newConformanceRunner(t, prepared.transient, conformanceRunnerOptions{Base: runner, RequestRuntime: &runtime, Org: &org})
			}
			for _, temperature := range []string{"cold", "warm"} {
				t.Run(temperature, func(t *testing.T) {
					result, err := runner.execute(program)
					success, outcome := err == nil, ""
					if err != nil {
						outcome = err.Error()
					} else {
						for _, line := range result.Debug {
							if _, observation, ok := strings.Cut(line, "OBS|"); ok {
								outcome = observation
							}
						}
					}
					if success != (row[4] == "true") || !innerShapeOutcomesEqual(outcome, row[5]) {
						t.Fatalf("native success=%s outcome=%q; glade success=%t outcome=%q", row[4], row[5], success, outcome)
					}
				})
			}
		})
	}
}

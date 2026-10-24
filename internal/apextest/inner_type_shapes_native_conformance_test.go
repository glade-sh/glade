package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type innerShapeCase struct {
	ID                          string
	Classes, TestClasses        map[string]string
	Body, TestClass, TestMethod string
	ExpectedTests               int
	Routes                      []string
	CompileOnly                 bool
	Captures                    []struct {
		API, Route, Outcome string
		Compiled, Success   bool
	}
}

func TestInnerTypeShapesNativeConformance(t *testing.T) {
	runInnerShapeNativeRows(t, "testdata/conformance/inner_type_shapes", 20, 84)
}

func TestInnerTypeScopeRestoredNativeConformance(t *testing.T) {
	runInnerShapeNativeRows(t, "../sema/testdata/inner_type_scope", 17, 34)
}

// Each API and entry route replays the captured body independently. Test wrappers
// retain the selected native method; unrelated observation methods are omitted.
func runInnerShapeNativeRows(t *testing.T, dir string, wantCases, wantRows int) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []innerShapeCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != wantCases {
		t.Fatalf("native case count: got %d, want %d", len(cases), wantCases)
	}
	byID := map[string]innerShapeCase{}
	expected := map[string]string{}
	for _, c := range cases {
		if c.ID == "" || byID[c.ID].ID != "" || len(c.Captures) != 2*len(c.Routes) {
			t.Fatalf("incomplete source matrix: %s", c.ID)
		}
		byID[c.ID] = c
		for _, row := range c.Captures {
			key := c.ID + "/" + row.API + "/" + row.Route
			if (row.API != "62.0" && row.API != "67.0") || !slices.Contains(c.Routes, row.Route) {
				t.Fatalf("invalid source cell %s", key)
			}
			if _, ok := expected[key]; ok {
				t.Fatalf("duplicate source cell %s", key)
			}
			expected[key] = strings.Join([]string{strconv.FormatBool(row.Compiled), strconv.FormatBool(row.Success), row.Outcome}, "\t")
		}
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != wantRows+1 || len(expected) != wantRows || strings.Join(rows[0], ",") != "id,api,route,compiled,success,outcome" {
		t.Fatal("incomplete native table")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 6 {
			t.Fatalf("invalid native row %v", row)
		}
		key := strings.Join(row[:3], "/")
		if seen[key] || expected[key] != strings.Join(row[3:], "\t") {
			t.Fatalf("native table differs from captured cell %s", key)
		}
		seen[key] = true
		c := byID[row[0]]
		t.Run(key, func(t *testing.T) {
			files := map[string]string{}
			sources := c.Classes
			if row[2] == "isTest" && len(c.TestClasses) > 0 {
				sources = c.TestClasses
			}
			if c.CompileOnly && row[2] == "anonymous" {
				sources = nil
			}
			for name, source := range sources {
				files[name] = source
			}
			index := nativeBoundaryIndex(t, row[1], files, nil)
			analysis := sema.Analyze(index)
			diagnostics := append([]diagnostic.Diagnostic{}, index.Diagnostics...)
			var prepared innerShapeAnonymous
			if row[2] == "anonymous" {
				// The request does not run named semantic analysis over the project;
				// only the transient declarations and the body are analyzed.
				prepared = innerShapePrepareAnonymous(t, row[1], index, c.Body)
				analysis = sema.Result{Diagnostics: prepared.diagnostics}
			}
			diagnostics = append(diagnostics, analysis.Diagnostics...)
			var problems []string
			for _, d := range diagnostics {
				if d.Severity != diagnostic.Error {
					continue
				}
				message := d.NativeMessage
				if message == "" {
					message = d.Message
				}
				problems = append(problems, message)
			}
			compiled := len(problems) == 0
			outcome := strings.Join(problems, " | ")
			success := false
			if compiled && row[2] == "named" {
				outcome = "Pass"
			}
			if compiled && !c.CompileOnly {
				if row[2] == "named" {
					outcome = "Pass"
				} else if row[2] == "isTest" {
					run := Run(index, Options{SelectedClasses: []string{c.TestClass}, SelectedMethod: c.TestMethod, NoDiskCache: true, Parallelism: 1})
					want := c.ExpectedTests
					if want == 0 {
						want = 1
					}
					if run.Summary().Total != want {
						outcome = fmt.Sprintf("test discovery: %+v", run.Summary())
					} else if run.Summary().Passed == want {
						success = true
						outcome = "Pass"
					} else {
						for _, suite := range run.Suites {
							for _, result := range suite.Cases {
								if result.Problem != nil {
									if _, observation, ok := strings.Cut(result.Problem.Message, "OBS|"); ok && strings.HasSuffix(result.Problem.Type, "AssertException") {
										success = true
										outcome = observation
									} else {
										outcome = result.Problem.Type + ": " + result.Problem.Message
									}
								} else if result.Status != testreport.StatusPass {
									outcome = fmt.Sprintf("test status: %s", result.Status)
								}
							}
						}
					}
				} else {
					program, err := vm.CompileAnonymousWithOptions(prepared.body, vm.CompileOptions{APIVersion: row[1], ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(prepared.merged, prepared.body, row[1])})
					if err != nil {
						compiled = false
						outcome = err.Error()
					} else {
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
						result, err := runner.execute(program)
						if err != nil {
							outcome = err.Error()
						} else {
							success = true
							outcome = "Pass"
							for _, line := range result.Debug {
								if _, observation, ok := strings.Cut(line, "OBS|"); ok {
									outcome = observation
								}
							}
						}
					}
				}
			}
			// A rejected test declaration must also be rejected through the runner.
			// The native deployment rows preserve that @IsTest entry route.
			if c.CompileOnly && row[2] == "isTest" {
				run := Run(index, Options{SelectedClasses: []string{c.TestClass}, SelectedMethod: c.TestMethod, NoDiskCache: true, Parallelism: 1})
				want := c.ExpectedTests
				if want == 0 {
					want = 1
				}
				if summary := run.Summary(); summary.Total != want || summary.CompileErrors != want {
					t.Errorf("native rejected test deployment; runner summary=%+v suites=%+v", summary, run.Suites)
				}
			}
			// Named deployment observations carry no runtime assertion.
			wantSuccess := row[4] == "true"
			match := compiled == (row[3] == "true") && success == wantSuccess && innerShapeOutcomesEqual(outcome, row[5])
			t.Logf("native compiled=%s success=%s outcome=%q; tip compiled=%t success=%t outcome=%q; match=%t", row[3], row[4], row[5], compiled, success, outcome, match)
			if !match {
				t.Errorf("native / tip mismatch")
			}
		})
	}
}

func innerShapeOutcomesEqual(got, want string) bool {
	var g, w any
	if json.Unmarshal([]byte(got), &g) == nil && json.Unmarshal([]byte(want), &w) == nil {
		return reflect.DeepEqual(g, w)
	}
	return got == want
}

type innerShapeAnonymous struct {
	body        string
	transient   typesys.Index
	merged      typesys.Index
	diagnostics []diagnostic.Diagnostic
}

// innerShapePrepareAnonymous follows the anonymous request path in
// internal/gladecli/anonymous_declarations.go and cli.go: top-level declarations
// move to a transient index and are blanked from the body, declaration
// diagnostics stop the request before body analysis, and the body is analyzed
// against the project index merged with the transient types.
func innerShapePrepareAnonymous(t *testing.T, api string, project typesys.Index, source string) innerShapeAnonymous {
	t.Helper()
	prepared := innerShapeAnonymous{body: source, merged: project}
	parsed := apexast.ParseSource("__glade_anonymous.cls", source)
	declarations := nativeBoundaryAnonymousDeclarations(t, source)
	if len(declarations) > 0 {
		for _, first := range parsed.Diagnostics {
			if first.Code != "APEXPARSE002" && first.Code != "APEXPARSE003" {
				prepared.diagnostics = []diagnostic.Diagnostic{first}
				return prepared
			}
		}
		body := []byte(source)
		for _, decl := range parsed.Declarations {
			if decl.Kind != apexast.DeclarationClass && decl.Kind != apexast.DeclarationInterface && decl.Kind != apexast.DeclarationEnum {
				continue
			}
			for i := decl.Range.Start.Offset; i < decl.Range.End.Offset; i++ {
				if body[i] != '\n' && body[i] != '\r' {
					body[i] = ' '
				}
			}
		}
		prepared.body = string(body)
		prepared.transient = sema.WithAnonymousDeclarationContext(nativeBoundaryIndex(t, api, declarations, nil))
		for _, item := range sema.NativeLifecycleDiagnostics(prepared.transient, prepared.transient.Diagnostics) {
			if item.Severity == diagnostic.Error {
				// The request reports only the first declaration error.
				prepared.diagnostics = []diagnostic.Diagnostic{item}
				break
			}
		}
		prepared.merged.Types = append(append([]typesys.TypeSymbol(nil), project.Types...), prepared.transient.Types...)
		if len(prepared.diagnostics) > 0 {
			return prepared
		}
		if analysis := sema.AnalyzeAnonymousDeclarationsInContext(prepared.merged, prepared.transient); analysis.HasErrors() {
			prepared.diagnostics = analysis.Diagnostics[:1]
			return prepared
		}
	}
	prepared.diagnostics = sema.AnalyzeAnonymous(prepared.merged, prepared.body, api).Diagnostics
	return prepared
}

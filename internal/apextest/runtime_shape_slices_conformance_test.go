package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type runtimeShapeCarry struct {
	API    string   `json:"source_api"`
	Route  string   `json:"route"`
	Method string   `json:"method"`
	Owner  string   `json:"owner"`
	Reason string   `json:"reason"`
	Fields []string `json:"fields"`
}

func runtimeShapeObservationsEqual(got, want map[string]any, carry *runtimeShapeCarry) bool {
	if carry != nil {
		if len(carry.Fields) == 0 {
			return false
		}
		got, want = maps.Clone(got), maps.Clone(want)
		seen := map[string]bool{}
		for _, field := range carry.Fields {
			gotValue, gotPresent := got[field]
			wantValue, wantPresent := want[field]
			if strings.TrimSpace(field) == "" || seen[field] || (gotPresent == wantPresent && reflect.DeepEqual(gotValue, wantValue)) {
				return false
			}
			seen[field] = true
			delete(got, field)
			delete(want, field)
		}
	}
	return reflect.DeepEqual(got, want)
}

// Compare native observations on each execution route. Explicit known-gap
// carries retain their native result and report the current local observation.
func TestRuntimeShapeSlicesNativeConformance(t *testing.T) {
	runRuntimeShapeSlicesNativeConformance(t, nil)
}

func runRuntimeShapeSlicesNativeConformance(t *testing.T, mutate func(string, map[string]any)) {
	runRuntimeShapeFixture(t, "testdata/conformance/runtime_shape_slices", "RuntimeShapeProbe", "RuntimeShapeTest", 32, 0, mutate)
}

func runRuntimeShapeFixture(t *testing.T, dir, probeClass, testClass string, perAPIRows, carryCount int, mutate func(string, map[string]any)) {
	t.Helper()
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2*perAPIRows+1 || strings.Join(rows[0], ",") != "source_api,route,method,observation" {
		t.Fatal("incomplete native matrix")
	}
	nativeRoutes := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatal("malformed native row")
		}
		nativeRoutes[row[0]+"/"+row[1]+"/"+row[2]] = true
		if row[1] == "isTest" {
			nativeRoutes[row[0]+"/namedVM/"+row[2]] = true
		}
	}
	var data struct {
		Carries []runtimeShapeCarry `json:"carries"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "carries.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Carries) != carryCount {
		t.Fatal("incomplete known-gap carry matrix")
	}
	carries := map[string]runtimeShapeCarry{}
	for _, item := range data.Carries {
		key := item.API + "/" + item.Route + "/" + item.Method
		if _, exists := carries[key]; exists || !nativeRoutes[key] || strings.TrimSpace(item.Owner) == "" || strings.TrimSpace(item.Reason) == "" || len(item.Fields) == 0 {
			t.Fatalf("invalid or duplicate known-gap carry: %+v", item)
		}
		carries[key] = item
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
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
				t.Fatalf("native deployed; named diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			request := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			named := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				if len(row) != 4 {
					t.Fatal("malformed native row")
				}
				if row[0] != api {
					continue
				}
				route, method := row[1], row[2]
				key := route + "/" + method
				if seen[key] || (route != "anonymous" && route != "isTest") {
					t.Fatal("invalid or duplicate route")
				}
				seen[key] = true
				routes := []string{route}
				if route == "isTest" {
					routes = append(routes, "namedVM")
				}
				for _, execution := range routes {
					t.Run(execution+"/"+method, func(t *testing.T) {
						marker := "OBSERVED|" + method + "|"
						message := ""
						switch execution {
						case "anonymous":
							source := "Savepoint point=Database.setSavepoint();System.debug('" + marker + "'+JSON.serialize(" + probeClass + ".observe('" + method + "')));Database.rollback(point);"
							if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
								t.Fatal(analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							result, err := request.execute(program)
							if err != nil {
								t.Fatal(err)
							}
							for _, line := range result.Debug {
								if strings.HasPrefix(line, marker) {
									if message != "" {
										t.Fatal("duplicate payload")
									}
									message = line
								}
							}
						case "isTest":
							run := Run(index, Options{SelectedClasses: []string{testClass}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
							if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
								t.Fatalf("observation discovery: %+v", run.Summary())
							}
							result := run.Suites[0].Cases[0]
							if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("missing assertion transport: %+v", result.Problem)
							}
							message = result.Problem.Message
						case "namedVM":
							cases := Discover(index, Options{SelectedClasses: []string{testClass}, SelectedMethod: method})
							if len(cases) != 1 {
								t.Fatal("missing direct observer")
							}
							methods, methodErrors := compileTestMethods(cases)
							programs, programErrors := compileTestInvokePrograms(cases)
							key := testCaseKey(cases[0])
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
							}
							machine := named.newMachine()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							machine.EnableTestContext()
							_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
							var issue *vm.RuntimeError
							if !errors.As(err, &issue) || issue.Type != "System.AssertException" {
								t.Fatalf("missing direct observation: %v", err)
							}
							message = issue.ExceptionMessage()
						}
						_, payload, ok := strings.Cut(message, marker)
						var got, want map[string]any
						if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil {
							t.Fatalf("invalid observation: %s", message)
						}
						if mutate != nil {
							mutate(method, got)
						}
						var carry *runtimeShapeCarry
						if item, carried := carries[api+"/"+execution+"/"+method]; carried {
							carry = &item
						}
						if !runtimeShapeObservationsEqual(got, want, carry) {
							t.Fatalf("native=%s; Glade=%v; carry=%+v", row[3], got, carry)
						}
						if carry == nil {
							t.Logf("MATCH native=%s", row[3])
						} else {
							t.Logf("CARRIED fields=%v; owner=%s; reason=%s; native=%s; Glade=%s", carry.Fields, carry.Owner, carry.Reason, row[3], payload)
						}
					})
				}
			}
			if len(seen) != perAPIRows {
				t.Fatalf("native row count=%d", len(seen))
			}
		})
	}
}

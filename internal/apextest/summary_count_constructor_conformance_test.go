package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type summaryCountConstructorCell struct {
	Status                string            `json:"status"`
	Reason                string            `json:"reason"`
	Native                map[string]any    `json:"native"`
	Expected              map[string]any    `json:"expected"`
	CarriedFields         map[string]string `json:"carriedFields"`
	OriginalUncaughtQuery map[string]string `json:"originalUncaughtQuery"`
}

type summaryCountConstructorRow struct {
	ID        string                                 `json:"id"`
	ClassName string                                 `json:"className"`
	Method    string                                 `json:"method"`
	Marker    string                                 `json:"marker"`
	Cells     map[string]summaryCountConstructorCell `json:"cells"`
}

func TestSummaryCountConstructorConformance(t *testing.T) {
	var data struct {
		APIVersions      []string                     `json:"apiVersions"`
		Routes           []string                     `json:"routes"`
		ProjectSourceAPI string                       `json:"projectSourceAPI"`
		FixedSourcePins  map[string]int               `json:"fixedSourcePins"`
		Files            map[string]string            `json:"files"`
		FilesByAPI       map[string]map[string]string `json:"filesByAPI"`
		Rows             []summaryCountConstructorRow `json:"rows"`
		Controls         []struct {
			API       string            `json:"api"`
			ClassName string            `json:"className"`
			Method    string            `json:"method"`
			Status    string            `json:"status"`
			Native    map[string]string `json:"native"`
			Reason    string            `json:"reason"`
		} `json:"controls"`
	}
	raw, err := os.ReadFile("testdata/conformance/summary_count_constructor_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || data.ProjectSourceAPI != "61.0" || len(data.Rows) != 4 {
		t.Fatal("incomplete constructor observation axes")
	}
	if !reflect.DeepEqual(data.FixedSourcePins, map[string]int{"LegacySummaryObservation": 26, "SummaryFactoryAdapter": 29, "SummaryRecordFactory": 31, "SummaryInsertService": 34}) {
		t.Fatal("legacy source pins changed")
	}
	observed, carried := 0, 0
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{}
			for path, text := range data.Files {
				files[path] = strings.ReplaceAll(text, "{{API}}", api)
			}
			for path, text := range data.FilesByAPI[api] {
				files[path] = text
			}
			for path, text := range files {
				writeFile(t, filepath.Join(root, path), text)
			}
			index := loadTestIndex(t, root)
			if index.Project.SourceAPIVersion != data.ProjectSourceAPI {
				t.Fatal("project source API changed")
			}
			for _, typ := range index.Types {
				if !strings.HasPrefix(typ.File, root) || typ.NestingDepth != 0 {
					continue
				}
				want := api
				if fixed, ok := data.FixedSourcePins[typ.Name]; ok {
					want = strconv.Itoa(fixed) + ".0"
				}
				if typ.EffectiveAPIVersion != want {
					t.Fatalf("%s source API=%s, want %s", typ.Name, typ.EffectiveAPIVersion, want)
				}
			}
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native fixture accepted; Glade=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, RequestRuntime: &runtime})
			execute := func(class, method string) error {
				cases := Discover(index, Options{SelectedClasses: []string{class}, SelectedMethod: method})
				if len(cases) != 1 {
					t.Fatalf("missing named observation %s.%s", class, method)
				}
				methods, methodErrs := compileTestMethodsForIndex(&index, cases)
				programs, errs := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrs[key] != nil || errs[key] != nil {
					t.Fatalf("named compilation: %v %v", methodErrs[key], errs[key])
				}
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
				return execErr
			}
			for _, row := range data.Rows {
				if len(row.Cells) != 6 {
					t.Fatal("incomplete native matrix")
				}
				for _, route := range data.Routes {
					t.Run(route+"/"+row.ID, func(t *testing.T) {
						cell, ok := row.Cells[api+"/"+route]
						if !ok || len(cell.Native) == 0 {
							t.Fatal("missing native cell")
						}
						if cell.Status == "NOT_RUN" {
							carried++
							if cell.Reason == "" || len(cell.Expected) != 0 {
								t.Fatal("unobserved route was asserted")
							}
							t.Logf("carried: %s; native=%v", cell.Reason, cell.Native)
							return
						}
						if route != "istest" || cell.Status != "runtime" || len(cell.Expected) == 0 {
							t.Fatal("invalid observed named cell")
						}
						observed++
						for field, reason := range cell.CarriedFields {
							value, present := cell.Native[field]
							_, asserted := cell.Expected[field]
							if !present || reason == "" || asserted {
								t.Fatal("carry lacks separate native evidence")
							}
							t.Logf("carried %s: %s; native=%v", field, reason, value)
						}
						if len(cell.OriginalUncaughtQuery) > 0 {
							if cell.OriginalUncaughtQuery["message"] == "" || cell.OriginalUncaughtQuery["reason"] == "" {
								t.Fatal("incomplete original query carry")
							}
							t.Logf("original observer carry: %v", cell.OriginalUncaughtQuery)
						}
						execErr := execute(row.ClassName, row.Method)
						var runtimeErr *vm.RuntimeError
						if !errors.As(execErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
							t.Fatalf("missing observation marker: %v", execErr)
						}
						_, payload, ok := strings.Cut(runtimeErr.ExceptionMessage(), row.Marker)
						if !ok {
							t.Fatal(execErr)
						}
						var got map[string]any
						if err := json.Unmarshal([]byte(payload), &got); err != nil {
							t.Fatal(err)
						}
						for field, want := range cell.Expected {
							actual, present := got[field]
							native, observed := cell.Native[field]
							if !present || !observed || !reflect.DeepEqual(actual, want) || !reflect.DeepEqual(native, want) {
								t.Errorf("%s: native=%v expected=%v Glade=%v", field, native, want, actual)
							}
						}
					})
				}
			}
			for _, control := range data.Controls {
				if control.API != api {
					continue
				}
				t.Run("control/"+control.Method, func(t *testing.T) {
					switch control.Status {
					case "carry":
						if control.Reason == "" || control.Native["message"] == "" {
							t.Fatal("control carry lacks native evidence")
						}
						t.Logf("carried control: %s; native=%v", control.Reason, control.Native)
					case "pass":
						if control.Native["result"] != "pass" {
							t.Fatal("control lacks native success")
						}
						if err := execute(control.ClassName, control.Method); err != nil {
							t.Fatal(err)
						}
					default:
						t.Fatal("unknown control status")
					}
				})
			}
		})
	}
	if observed != 12 || carried != 12 {
		t.Fatalf("observation counts changed: %d observed, %d carried", observed, carried)
	}
}

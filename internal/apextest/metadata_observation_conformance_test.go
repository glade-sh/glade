package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type metadataObservationCell struct {
	Status        string            `json:"status"`
	Reason        string            `json:"reason"`
	Native        map[string]any    `json:"native"`
	Expected      map[string]any    `json:"expected"`
	CarriedFields map[string]string `json:"carriedFields"`
}

type metadataObservationRow struct {
	ID             string                             `json:"id"`
	Code           string                             `json:"code"`
	MetadataObject string                             `json:"metadataObject"`
	Cells          map[string]metadataObservationCell `json:"cells"`
}

// The exports retain incomplete native cells and org-inventory facets as
// carries. Only observed, fixture-independent values become runtime assertions.
func runMetadataObservationConformance(t *testing.T, file string, runtimeCells, metadataCells, carriedCells, excludedCells int) {
	t.Helper()
	var data struct {
		APIVersions []string                 `json:"apiVersions"`
		Routes      []string                 `json:"routes"`
		Files       map[string]string        `json:"files"`
		Rows        []metadataObservationRow `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/" + file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" {
		t.Fatal("incomplete observation axes")
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if seen[row.ID] || row.ID == "" || len(row.Cells) != 6 {
			t.Fatal("missing or duplicate native shape")
		}
		seen[row.ID] = true
		for _, api := range data.APIVersions {
			for _, route := range data.Routes {
				cell, ok := row.Cells[api+"/"+route]
				if !ok || len(cell.Native) == 0 {
					t.Fatal("missing native cell")
				}
				if (cell.Status == "runtime" || cell.Status == "metadata") && len(cell.Expected) == 0 {
					t.Fatal("observed cell has no asserted projection")
				}
				counts[cell.Status]++
				for field, reason := range cell.CarriedFields {
					if _, ok := cell.Native[field]; !ok || reason == "" {
						t.Fatal("carry lacks native evidence or reason")
					}
					if _, asserted := cell.Expected[field]; asserted {
						t.Fatal("carried native facet was also asserted")
					}
					t.Logf("carried facet %s/%s/%s/%s: %s", api, route, row.ID, field, reason)
				}
			}
		}
	}
	if counts["runtime"] != runtimeCells || counts["metadata"] != metadataCells || counts["NOT_RUN"]+counts["FIXTURE_ERROR"] != carriedCells || counts["NOT_EXPRESSIBLE"] != excludedCells {
		t.Fatalf("native matrix changed: %v", counts)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			files := map[string]string{}
			for path, text := range data.Files {
				files[path] = strings.ReplaceAll(text, "{{API}}", api)
			}
			class := "@IsTest private class MetadataObservation {"
			for n, row := range data.Rows {
				if row.Cells[api+"/istest"].Status == "runtime" {
					class += fmt.Sprintf("@IsTest static void observe%d(){%s}", n, metadataObservationBody(row.Code))
				}
			}
			class += "}"
			index := userModeInaccessibleIndex(t, api, userModeInaccessibleFixture{Metadata: files}, map[string]string{"MetadataObservation": class})
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native fixture accepted; Glade=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, RequestRuntime: &runtime})
			for n, row := range data.Rows {
				for _, route := range data.Routes {
					t.Run(route+"/"+row.ID, func(t *testing.T) {
						cell := row.Cells[api+"/"+route]
						switch cell.Status {
						case "metadata":
							got := org.Objects[row.MetadataObject].Definition.Metadata["publishBehavior"]
							if got != cell.Expected["publishBehavior"] || got != cell.Native["publishBehavior"] {
								t.Fatalf("loader projection %q differs from retrieval %v", got, cell.Native)
							}
							return
						case "NOT_RUN", "FIXTURE_ERROR", "NOT_EXPRESSIBLE":
							if cell.Reason == "" || len(cell.Expected) != 0 {
								t.Fatal("non-answer asserted or missing carry reason")
							}
							t.Logf("%s: %s; native=%v", cell.Status, cell.Reason, cell.Native)
							return
						case "runtime":
						default:
							t.Fatalf("unknown native status %q", cell.Status)
						}
						var execErr error
						if route == "anonymous" {
							body := metadataObservationBody(row.Code)
							checked := sema.AnalyzeAnonymous(index, body, api)
							program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
							if err != nil || checked.HasErrors() {
								t.Fatalf("native source accepted; Glade=%v %v", err, checked.Diagnostics)
							}
							_, execErr = runner.execute(program)
						} else {
							cases := Discover(index, Options{SelectedClasses: []string{"MetadataObservation"}, SelectedMethod: fmt.Sprintf("observe%d", n)})
							if len(cases) != 1 {
								t.Fatal("missing named observation")
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
							_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
						}
						metadataObservationValue(t, cell, execErr)
					})
				}
			}
		})
	}
}

func metadataObservationBody(code string) string {
	return "Map<String,Object> p=new Map<String,Object>();" + code + "System.assert(false,'METADATA_OBSERVATION|'+JSON.serialize(p));"
}

func metadataObservationValue(t *testing.T, cell metadataObservationCell, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
		t.Fatalf("missing observation marker: %v", execErr)
	}
	_, payload, ok := strings.Cut(runtimeErr.ExceptionMessage(), "METADATA_OBSERVATION|")
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
		if want == "<PAGE_ID>" {
			for _, value := range []any{actual, native} {
				id, ok := value.(string)
				if !ok || len(id) != 15 || !strings.HasPrefix(id, "066") {
					t.Fatalf("invalid observed page identity: %v", value)
				}
			}
		} else if !present || !observed || !reflect.DeepEqual(actual, want) || !reflect.DeepEqual(native, want) {
			t.Fatalf("%s: native=%v expected=%v Glade=%v", field, native, want, actual)
		}
	}
}

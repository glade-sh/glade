package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

type hostedPageContentCase struct {
	id, api, operation, method, decision string
}

type hostedPageContentOracle struct {
	id, api, operation                string
	returned, returnType, returnValue string
	exceptionType, exceptionMessage   string
	callouts, queries, dml, emails    string
	cpuMsDelta, heapBytesDelta        string
}

func TestHostedPageContentIsTestNativeRoutes(t *testing.T) {
	root := filepath.Join("testdata", "conformance", "hosted_page_content_routes")
	cases := readHostedPageContentCases(t, filepath.Join(root, "cases.tsv"))
	oracles := readHostedPageContentOracles(t, filepath.Join(root, "org.tsv"))
	if len(cases) != 24 || len(oracles) != len(cases) {
		t.Fatalf("cases=%d native rows=%d, want twenty-four matched rows", len(cases), len(oracles))
	}

	seen := map[string]bool{}
	for _, cell := range cases {
		cell := cell
		if seen[cell.id] {
			t.Fatalf("duplicate case %s", cell.id)
		}
		seen[cell.id] = true
		oracle, ok := oracles[cell.id]
		if !ok || oracle.api != cell.api || oracle.operation != cell.operation {
			t.Fatalf("native row does not match case %s", cell.id)
		}
		t.Run("isTest_vm/"+cell.id, func(t *testing.T) {
			apiRoot := filepath.Join(root, "api"+strings.TrimSuffix(cell.api, ".0"))
			index := loadTestIndex(t, apiRoot)
			org := orgFromIndex(index)
			org.APIVersion = cell.api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			machine.EnableTestContext()
			visualforceProject, err := project.Load(apiRoot)
			if err != nil {
				t.Fatal(err)
			}
			visualforce.SetVMRenderEnvironment(machine, visualforceProject)
			t.Cleanup(func() { visualforce.ClearVMRenderEnvironment(machine) })

			tests := Discover(index, Options{SelectedClasses: []string{hostedPageContentProbeClassName(cell.api)}, SelectedMethod: cell.method})
			if len(tests) != 1 {
				t.Fatalf("discovered tests=%d for %s", len(tests), cell.method)
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, tests)
			invokes, invokeErrors := compileTestInvokePrograms(tests)
			key := testCaseKey(tests[0])
			if methodErrors[key] != nil || invokeErrors[key] != nil {
				t.Fatalf("compile test method=%v invoke=%v", methodErrors[key], invokeErrors[key])
			}
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			_, execErr := machine.ExecuteTestInClass(invokes[key], hostedPageContentProbeClassName(cell.api))
			if cell.decision == "unsupported_exact_hosted_pdf" {
				if oracle.returned != "true" || oracle.returnType != "Blob" || oracle.returnValue == "" {
					t.Fatalf("native PDF capture is incomplete: %+v", oracle)
				}
				got := hostedPageContentCapture(t, cell.method, execErr)
				if got["returned"] != true || got["returnType"] != "Blob" || got["exceptionType"] != nil {
					t.Fatalf("local PDF stand-in did not return a Blob: %v", got)
				}
				size, ok := got["returnValue"].(string)
				if !ok || size == "" || size == "0" {
					t.Fatalf("empty local PDF stand-in: %v", got)
				}
				t.Logf("native PDF size=%s; local stand-in size=%s; renderer fidelity is outside the local contract", oracle.returnValue, size)
				return
			}
			got := hostedPageContentCapture(t, cell.method, execErr)
			want := hostedPageContentExpected(oracle)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v local=%v", want, got)
			}
		})
	}
}

// The project runner loads the same deployed classes, including the helper that
// accepts PageReference arguments, before invoking each captured Apex test.
func TestHostedBoundaryContentProjectIsTestRoutes(t *testing.T) {
	root := filepath.Join("testdata", "conformance", "hosted_page_content_routes")
	cases := readHostedPageContentCases(t, filepath.Join(root, "cases.tsv"))
	oracles := readHostedPageContentOracles(t, filepath.Join(root, "org.tsv"))
	for _, cell := range cases {
		if !strings.HasPrefix(cell.method, "route_") {
			continue
		}
		t.Run("isTest_project/"+cell.id, func(t *testing.T) {
			apiRoot := filepath.Join(root, "api"+strings.TrimSuffix(cell.api, ".0"))
			run := Run(loadTestIndex(t, apiRoot), Options{
				SelectedClasses: []string{hostedPageContentProbeClassName(cell.api)}, SelectedMethod: cell.method,
				NoDiskCache: true, Parallelism: 1,
			})
			if summary := run.Summary(); summary.Total != 1 || summary.Failed != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("want one native capture assertion; summary=%+v suites=%+v", summary, run.Suites)
			}
			result := run.Suites[0].Cases[0]
			if result.Status != testreport.StatusFail || result.Problem == nil {
				t.Fatalf("missing native capture assertion: %+v", result)
			}
			got := hostedPageContentCaptureMessage(t, cell.method, result.Problem.Message)
			want := hostedPageContentExpected(oracles[cell.id])
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v project=%v", want, got)
			}
		})
	}
}

func hostedPageContentProbeClassName(api string) string {
	return "HostedBoundaryProbe" + strings.TrimSuffix(api, ".0")
}

func readHostedPageContentCases(t *testing.T, path string) []hostedPageContentCase {
	t.Helper()
	rows := readHostedPageContentTable(t, path)
	result := make([]hostedPageContentCase, 0, len(rows))
	for _, row := range rows {
		result = append(result, hostedPageContentCase{row["case_id"], row["api_version"], row["operation"], row["test_method"], row["local_decision"]})
	}
	return result
}

func readHostedPageContentOracles(t *testing.T, path string) map[string]hostedPageContentOracle {
	t.Helper()
	rows := readHostedPageContentTable(t, path)
	result := make(map[string]hostedPageContentOracle, len(rows))
	for _, row := range rows {
		cell := hostedPageContentOracle{row["case_id"], row["api_version"], row["operation"], row["returned"], row["return_type"], row["return_value"], row["exception_type"], row["exception_message"], row["callouts"], row["queries"], row["dml"], row["emails"], row["cpu_ms_delta"], row["heap_bytes_delta"]}
		if cell.id == "" || cell.cpuMsDelta == "" || cell.heapBytesDelta == "" {
			t.Fatalf("incomplete native row in %s", path)
		}
		if _, exists := result[cell.id]; exists {
			t.Fatalf("duplicate native row %s", cell.id)
		}
		result[cell.id] = cell
	}
	return result
}

func readHostedPageContentTable(t *testing.T, path string) []map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		t.Fatalf("read hosted-boundary table %s: %v", path, err)
	}
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		if len(record) != len(records[0]) {
			t.Fatalf("malformed row in %s", path)
		}
		row := make(map[string]string, len(record))
		for i, key := range records[0] {
			row[key] = record[i]
		}
		rows = append(rows, row)
	}
	return rows
}

func hostedPageContentCapture(t *testing.T, method string, execErr error) map[string]any {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) {
		t.Fatalf("missing Apex test capture: %v", execErr)
	}
	return hostedPageContentCaptureMessage(t, method, runtimeErr.ExceptionMessage())
}

func hostedPageContentCaptureMessage(t *testing.T, method, message string) map[string]any {
	t.Helper()
	_, payload, ok := strings.Cut(message, "HB|"+method+"|")
	if !ok {
		t.Fatalf("missing capture marker in %q", message)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		t.Fatalf("decode capture: %v", err)
	}
	delete(result, "cpuMs")
	delete(result, "heapBytes")
	if exceptionType, ok := result["exceptionType"].(string); ok {
		result["exceptionType"] = strings.TrimPrefix(exceptionType, "System.")
	}
	return result
}

func hostedPageContentExpected(row hostedPageContentOracle) map[string]any {
	return map[string]any{
		"returned":         row.returned == "true",
		"returnType":       hostedPageContentOptional(row.returnType),
		"returnValue":      hostedPageContentOptional(row.returnValue),
		"exceptionType":    hostedPageContentOptional(strings.TrimPrefix(row.exceptionType, "System.")),
		"exceptionMessage": hostedPageContentOptional(row.exceptionMessage),
		"callouts":         hostedPageContentNumber(row.callouts),
		"queries":          hostedPageContentNumber(row.queries),
		"dml":              hostedPageContentNumber(row.dml),
		"emails":           hostedPageContentNumber(row.emails),
	}
}

func hostedPageContentOptional(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func hostedPageContentNumber(value string) float64 {
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		panic(fmt.Sprintf("invalid native counter %q", value))
	}
	return number
}

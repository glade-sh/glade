package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

type interfaceDispatchNative struct {
	nativeObservationExpected
	Diagnostics         []nativeBoundaryDiagnostic `json:"diagnostics"`
	API, Route, Context string
}

type interfaceDispatchCell struct {
	ID                           string                  `json:"id"`
	Classes                      map[string]string       `json:"classes"`
	Objects                      map[string]string       `json:"objects"`
	TestClass                    string                  `json:"testClass"`
	Native                       interfaceDispatchNative `json:"native"`
	Comparison                   string                  `json:"comparison,omitempty"`
	CarryReason                  string                  `json:"carryReason,omitempty"`
	PrimaryClass, PrimaryProblem string
	ProviderNamespace            string
	ProviderClasses              map[string]string
	ProviderObjects              map[string]string
}

// Captured named requests remain separate API and namespace observations.
// Other routes and namespace pairs retain explicit unanswered cells.
func TestInterfaceDispatchNamespaceOrgConformance(t *testing.T) {
	var data struct {
		Scope               string
		PendingSubjectCells int
		PendingControlCells int
		Captures            []struct {
			Namespace, API, Route, Context                 string
			Complete                                       bool
			SubjectMatched, ControlMatched, ControlPartial int
			Rows                                           []interfaceDispatchCell
		}
		Carries []struct {
			ID, API, Route, Context, Status, Reason string
			Native                                  json.RawMessage
		}
	}
	raw, err := os.ReadFile("testdata/conformance/interface_dispatch_namespace.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Scope != "supplemental same-namespace twin" || len(data.Captures) != 3 || data.PendingSubjectCells != 98 || data.PendingControlCells != 0 || len(data.Carries) != 98 {
		t.Fatal("invalid native matrix")
	}
	seen := map[string]bool{}
	for _, capture := range data.Captures {
		if (capture.API != "62.0" && capture.API != "67.0" && capture.API != "68.0") || capture.Namespace != "gladens004" || capture.Route != "istest" || capture.Context != "single" || !capture.Complete || capture.SubjectMatched != 20 || capture.ControlMatched != 3 || capture.ControlPartial != 1 || len(capture.Rows) != 24 {
			t.Fatal("invalid captured API/namespace scope")
		}
		for _, cell := range capture.Rows {
			key := strings.Join([]string{cell.ID, capture.API, capture.Route, capture.Context}, "/")
			if seen[key] || cell.ID == "" || cell.TestClass == "" || cell.Native.API != capture.API || cell.Native.Route != capture.Route || cell.Native.Context != capture.Context {
				t.Fatal("invalid or repeated native cell")
			}
			seen[key] = true
			if cell.Comparison != "" && (cell.ID != "CTRL_COMPILE" || cell.Comparison != "rejectionOnly" || cell.CarryReason == "") {
				t.Fatal("unexpected partial native comparison")
			}
			t.Run(key, func(t *testing.T) { interfaceDispatchNamespaceCell(t, capture.Namespace, capture.API, cell) })
		}
		for _, id := range []string{"CTRL_POS", "CTRL_CAUGHT", "CTRL_UNCAUGHT", "CTRL_COMPILE"} {
			if !seen[id+"/"+capture.API+"/istest/single"] {
				t.Fatalf("native control omitted: %s/%s", id, capture.API)
			}
		}
	}
	interfaceDispatchNamespaceCross62(t, seen)
	for _, cell := range data.Carries {
		key := strings.Join([]string{cell.ID, cell.API, cell.Route, cell.Context}, "/")
		fixtureError := cell.ID == "V19" && cell.API == "62.0" && cell.Route == "istest" && cell.Context == "cross" && cell.Status == "FIXTURE_ERROR"
		if seen[key] || cell.Reason == "" || (!fixtureError && cell.Status != "PENDING") || len(cell.Native) != 0 {
			t.Fatalf("invalid unanswered cell: %s", key)
		}
		seen[key] = true
		t.Logf("carried %s: %s", key, cell.Reason)
	}
	// Validate coverage without executing unanswered subject cells.
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for i := 1; i <= 20; i++ {
			for _, axis := range []string{"istest/single", "istest/cross", "anonymous/cross"} {
				// The private inner implementation has only a named fixture.
				if i == 13 && axis == "anonymous/cross" {
					continue
				}
				key := fmt.Sprintf("V%02d/%s/%s", i, api, axis)
				if !seen[key] {
					t.Fatalf("subject cell omitted: %s", key)
				}
			}
		}
	}
	t.Log("same-namespace observations unchanged; CD136 adds 27 observed cross-62 cells, with V19 fixture error and 98 subject carries; no complete cross-namespace matrix or cleanup claim")
}

func interfaceDispatchNamespaceCell(t *testing.T, namespace, api string, cell interfaceDispatchCell) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, namespace, api))
	for name, code := range cell.Classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, code)
		writeFile(t, path+"-meta.xml", fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
	}
	for name, xml := range cell.Objects {
		writeFile(t, filepath.Join(root, "force-app/main/default/objects", name, name+".object-meta.xml"), xml)
	}
	if cell.ProviderNamespace != "" {
		provider := filepath.Join(root, "provider")
		writeFile(t, filepath.Join(provider, "sfdx-project.json"), fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, cell.ProviderNamespace, api))
		for name, code := range cell.ProviderClasses {
			path := filepath.Join(provider, "force-app/main/default/classes", name+".cls")
			writeFile(t, path, code)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
		}
		for name, xml := range cell.ProviderObjects {
			writeFile(t, filepath.Join(provider, "force-app/main/default/objects", name, name+".object-meta.xml"), xml)
		}
		writeFile(t, filepath.Join(root, "glade.yml"), "project:\n  managedPackageDependencies: [\""+cell.ProviderNamespace+":provider\"]\n")
	}
	index := loadTestIndex(t, root)
	diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), sema.Analyze(index).Diagnostics...)
	if cell.Native.Outcome == "compile-rejected" {
		if len(cell.Native.Diagnostics) == 0 || cell.PrimaryClass == "" || cell.PrimaryProblem == "" {
			t.Fatal("captured primary rejection required")
		}
		primary := ""
		for _, item := range diagnostics {
			if item.Severity == diagnostic.Error && strings.TrimSuffix(filepath.Base(item.File), ".cls") == cell.PrimaryClass {
				primary = item.NativeMessage
				if primary == "" {
					primary = item.Message
				}
				break
			}
		}
		if cell.Comparison == "rejectionOnly" {
			missingType := strings.TrimPrefix(cell.PrimaryProblem, "Invalid type: ")
			if missingType == cell.PrimaryProblem || missingType == "" || !strings.Contains(primary, missingType) || cell.CarryReason == "" {
				t.Fatal("captured rejection needs a local rejection and explicit wording carry")
			}
			t.Logf("diagnostic wording carried: %s; native=%q local=%q", cell.CarryReason, cell.PrimaryProblem, primary)
		} else if primary != cell.PrimaryProblem {
			t.Fatalf("native rejection %q; local %q; diagnostics=%v", cell.PrimaryProblem, primary, diagnostics)
		}
		return
	}
	if nativeObservationHasErrors(diagnostics) {
		t.Fatalf("native compilation accepted; local diagnostics=%v", diagnostics)
	}
	cases := Discover(index, Options{SelectedClasses: []string{cell.TestClass}, SelectedMethod: "observe"})
	if len(cases) != 1 {
		t.Fatalf("named observation discovery: %d", len(cases))
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
	}
	org := orgFromIndex(index)
	org.APIVersion = api
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	machine := runner.newMachine()
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
	if cell.Native.Outcome == "uncaught_test_failure" {
		if len(cell.Native.TestFailures) != 1 {
			t.Fatal("one native uncaught failure required")
		}
		failure := cell.Native.TestFailures[0]
		var runtimeErr *vm.RuntimeError
		// The caught control checks getTypeName(). RuntimeError stores the
		// built-in exception's short name; preserve its native qualified identity.
		if !errors.As(execErr, &runtimeErr) || failure.ExceptionType != "System.MathException" || (runtimeErr.Type != failure.ExceptionType && runtimeErr.Type != "MathException") || failure.Message != failure.ExceptionType+": "+runtimeErr.ExceptionMessage() {
			t.Fatalf("native uncaught failure=%+v; local=%v", failure, execErr)
		}
		return
	}
	source := nativeObservationRoute{nativeBoundaryRoute: nativeBoundaryRoute{ID: cell.ID}}
	nativeObservationValue(t, source, "isTest", cell.Native.nativeObservationExpected, execErr, machine.Org)
}

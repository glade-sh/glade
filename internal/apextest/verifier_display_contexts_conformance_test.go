package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type displayContextRow struct {
	ID, Group, APIVersion, Namespace, Route                 string
	Expression, SourceSHA256, ExpressionSHA256, CarryReason string
	Body, BodySHA256                                        string
	Helpers, HelperSHA256                                   map[string]string
	Native                                                  overrideConversionNative
}

// Each helper variant is independent. Plain anonymous/test and owner namespace
// test observations are all retained; namespace anonymous was not admitted.
func TestVerifierDisplayContextsOrgConformance(t *testing.T) {
	rows := readDisplayContextCapture(t)
	groups := map[string][]displayContextRow{}
	for _, row := range rows {
		helpers, err := json.Marshal(row.Helpers)
		if err != nil {
			t.Fatal(err)
		}
		key := row.APIVersion + "/" + row.Namespace + "/" + fmt.Sprintf("%x", sha256.Sum256(helpers))
		groups[key] = append(groups[key], row)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	counts := map[string]*conformanceCounts{}
	for _, key := range keys {
		group := groups[key]
		index, runner := displayContextRunner(t, group)
		for _, row := range group {
			if counts[row.APIVersion] == nil {
				counts[row.APIVersion] = newConformanceCountLog(t, "VerifierDisplayContexts", row.APIVersion)
			}
			kind := "exact"
			if row.CarryReason != "" {
				kind = "carried"
			}
			name := row.APIVersion + "/" + row.ID + "/" + row.Route
			counts[row.APIVersion].RunKind(t, row.Namespace+"/"+row.Route, row.ID+"/"+row.Route, name, kind, func(t *testing.T) {
				if row.CarryReason != "" {
					t.Logf("carry: %s; native=%+v", row.CarryReason, row.Native)
					return
				}
				if row.Native.Outcome == "compile-rejected" {
					displayContextRejection(t, index, row)
					return
				}
				err := executeDisplayContext(t, index, runner, row)
				assertOverrideConversionValue(t, row.ID, row.Native, err)
			})
		}
	}
}

func readDisplayContextCapture(t *testing.T) []displayContextRow {
	t.Helper()
	var data struct {
		APIVersions      []string
		ExcludedContexts []struct{ Context, Route, Reason string }
		Rows             []displayContextRow
	}
	raw, err := os.ReadFile("testdata/conformance/verifier_display_contexts.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 666 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || len(data.ExcludedContexts) != 1 {
		t.Fatal("incomplete admitted display matrix")
	}
	excluded := data.ExcludedContexts[0]
	if excluded.Context != "namespace" || excluded.Route != "anonymous" || excluded.Reason == "" {
		t.Fatal("excluded named-only boundary missing")
	}
	t.Logf("excluded namespace/anonymous: %s", excluded.Reason)
	seen, sets := map[string]bool{}, map[string]int{}
	for _, row := range data.Rows {
		key := row.APIVersion + "/" + row.ID + "/" + row.Route
		if seen[key] || row.Expression == "" || row.Native.Outcome == "" || len(row.SourceSHA256) != 64 || len(row.Helpers) != len(row.HelperSHA256) {
			t.Fatal("duplicate/incomplete native row")
		}
		seen[key] = true
		// Native source hashes precede run-specific sink-token binding. Retain
		// them separately from the exact rendered expression used for replay.
		if fmt.Sprintf("%x", sha256.Sum256([]byte(row.Expression))) != row.ExpressionSHA256 {
			t.Fatal("rendered native expression changed")
		}
		if row.Body == "" || fmt.Sprintf("%x", sha256.Sum256([]byte(row.Body))) != row.BodySHA256 {
			t.Fatal("captured request body changed")
		}
		if row.Namespace != "" && (row.Namespace != "gladens001" || row.Route != "istest") {
			t.Fatal("uncaptured namespace route")
		}
		sets[row.APIVersion+"/"+row.Namespace+"/"+row.Route]++
		for name, source := range row.Helpers {
			if fmt.Sprintf("%x", sha256.Sum256([]byte(source))) != row.HelperSHA256[name] {
				t.Fatal("native helper changed")
			}
		}
		if row.CarryReason != "" && !displayContextCarryAllowed(row) {
			t.Fatal("unsettled cell lacks native evidence")
		}
	}
	for _, api := range data.APIVersions {
		for _, context := range []string{"/anonymous", "/istest", "gladens001/istest"} {
			if sets[api+"/"+context] != 74 {
				t.Fatal("missing independent admitted cells")
			}
		}
	}
	return data.Rows
}

func displayContextCarryAllowed(row displayContextRow) bool {
	if strings.HasSuffix(row.ID, "CTRL_UNCAUGHT") {
		return row.Native.ExceptionType == "System.MathException" && row.Native.ExceptionMessage == "System.MathException: Divide by 0"
	}
	return false
}

func displayContextSource(row displayContextRow) string {
	return row.Body
}

func displayContextIndex(t *testing.T, row displayContextRow, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, row.Namespace, row.APIVersion))
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row.APIVersion+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, root)
}

func displayContextRunner(t *testing.T, group []displayContextRow) (typesys.Index, *conformanceRunner) {
	t.Helper()
	classes := make(map[string]string, len(group[0].Helpers)+1)
	for name, source := range group[0].Helpers {
		classes[name] = source
	}
	var methods strings.Builder
	for _, row := range group {
		if row.Route == "istest" && row.Native.Outcome != "compile-rejected" && row.CarryReason == "" {
			fmt.Fprintf(&methods, "@IsTest static void %s(){%s}\n", row.ID, displayContextSource(row))
		}
	}
	classes["DisplayContextObservation"] = "@IsTest private class DisplayContextObservation {" + methods.String() + "}"
	index := displayContextIndex(t, group[0], classes)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native helpers accepted; local=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.Namespace, org.APIVersion = group[0].Namespace, group[0].APIVersion
	return index, newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
}

func executeDisplayContext(t *testing.T, index typesys.Index, runner *conformanceRunner, row displayContextRow) error {
	t.Helper()
	if row.Route == "anonymous" {
		source := displayContextSource(row)
		if analysis := sema.AnalyzeAnonymous(index, source, row.APIVersion); analysis.HasErrors() {
			t.Fatal(analysis.Diagnostics)
		}
		program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: row.APIVersion})
		if err != nil {
			t.Fatal(err)
		}
		_, err = runner.execute(program)
		return err
	}
	cases := Discover(index, Options{SelectedClasses: []string{"DisplayContextObservation"}, SelectedMethod: row.ID})
	if len(cases) != 1 {
		t.Fatalf("named discovery=%d", len(cases))
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("named compile=%v %v", methodErrors[key], programErrors[key])
	}
	machine := runner.newMachine()
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
	return err
}

func displayContextRejection(t *testing.T, index typesys.Index, row displayContextRow) {
	t.Helper()
	var problem string
	if row.Route == "anonymous" {
		var native struct{ CompileProblem string }
		if err := json.Unmarshal(row.Native.Diagnostics, &native); err != nil {
			t.Fatal(err)
		}
		problem = native.CompileProblem
	} else {
		var native []nativeBoundaryDiagnostic
		if err := json.Unmarshal(row.Native.Diagnostics, &native); err != nil || len(native) == 0 {
			t.Fatal("missing native compiler diagnostic")
		}
		problem = native[0].Problem
	}
	if problem == "" {
		t.Fatal("empty native compiler message")
	}
	observation := nativeBoundaryRoute{NativeDiagnostics: []nativeBoundaryDiagnostic{{Problem: problem}}}
	if row.Route == "anonymous" {
		nativeBoundaryReject(t, observation, row.APIVersion, sema.AnalyzeAnonymous(index, displayContextSource(row), row.APIVersion).Diagnostics, nil)
		return
	}
	classes := make(map[string]string, len(row.Helpers)+1)
	for name, source := range row.Helpers {
		classes[name] = source
	}
	classes["DisplayCompileObservation"] = "@IsTest private class DisplayCompileObservation {@IsTest static void observe(){" + displayContextSource(row) + "}}"
	control := displayContextIndex(t, row, classes)
	nativeBoundaryReject(t, observation, row.APIVersion, sema.Analyze(control).Diagnostics, nil)
}

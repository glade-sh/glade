package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type jsonDateNullNativeCapture struct {
	Context, Kind, Route, NamespaceContext, SourceAPI, CallerAPI, TransportAPI, TestClass, Marker string
	Deployment                                                                                    struct {
		ID, Status                                                                      string
		Done, CheckOnly                                                                 bool
		NumberComponentErrors, NumberTestErrors, NumberTestsCompleted, NumberTestsTotal int
	}
	Execution struct {
		Status, ExitCode                 int
		Success, Compiled                bool
		CompileProblem, ExceptionMessage string
		ParseErrors                      []any
	}
	Sources          map[string]struct{ Source, SHA256 string }
	Source           struct{ Source, SHA256 string }
	RawSHA256        map[string]string
	ParserCorrection *struct {
		Reason                                    string
		NativeRunRepeated, RawDeploymentUnchanged bool
		InitialParserSHA256                       map[string]string
	}
	Rows []struct {
		ID, Classification string
		Native             map[string]any
		MatchedPaths       []string
		CarriedPaths       map[string]string
		NativeFailure      struct{ Message string }
	}
}

func TestJSONDateNullNativeConformance(t *testing.T) {
	var data struct {
		APIs            []string `json:"apiVersions"`
		Routes          []string
		Namespace       string         `json:"namespaceContext"`
		Known           int            `json:"knownRows"`
		Pending         []string       `json:"pendingCoverage"`
		Classifications map[string]int `json:"rowClassifications"`
		MatchedPaths    int            `json:"matchedPathCount"`
		CarriedPaths    int            `json:"carriedPathCount"`
		Captures        []jsonDateNullNativeCapture
	}
	raw, err := os.ReadFile("testdata/conformance/json_date_null_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIs, []string{"62.0", "63.0", "67.0"}) || !reflect.DeepEqual(data.Routes, []string{"isTest", "anonymous"}) || data.Namespace != "none" || data.Known != 81 || len(data.Captures) != 9 || len(data.Pending) == 0 || !reflect.DeepEqual(data.Classifications, map[string]int{"full": 66, "partial": 12, "carried": 3}) || data.MatchedPaths != 93 || data.CarriedPaths != 21 {
		t.Fatal("Date-null native scope differs")
	}
	expected := map[string]string{}
	for _, api := range []string{"62", "63", "67"} {
		for _, prefix := range []string{"api", "origin-input-api", "anonymous-api"} {
			expected[prefix+api] = api + ".0"
		}
	}
	classes := map[string]int{}
	rows, matchedPaths, carriedPaths := 0, 0, 0
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if expected[capture.Context] != capture.SourceAPI || capture.CallerAPI != capture.SourceAPI || capture.NamespaceContext != "none" {
				t.Fatal("capture source/caller scope differs")
			}
			delete(expected, capture.Context)
			if strings.HasPrefix(capture.Context, "anonymous-") != (capture.Kind == "anonymous") {
				t.Fatal("capture route kind differs")
			}
			actual := jsonDateNullNativeReplay(t, capture)
			if len(actual) != len(capture.Rows) {
				t.Fatal("observation count differs")
			}
			seen := map[string]bool{}
			for _, row := range capture.Rows {
				t.Run(row.ID, func(t *testing.T) {
					if seen[row.ID] || len(row.MatchedPaths) == 0 {
						t.Fatal("duplicate/unsupported native row")
					}
					seen[row.ID] = true
					rows++
					matchedPaths += len(row.MatchedPaths)
					carriedPaths += len(row.CarriedPaths)
					classes[row.Classification]++
					got, exists := actual[row.ID]
					if !exists {
						t.Fatal("native method/observation missing")
					}
					if row.Classification == "full" && (len(row.MatchedPaths) != 1 || row.MatchedPaths[0] != "" || len(row.CarriedPaths) != 0) {
						t.Fatal("full row must match complete payload")
					}
					if row.Classification == "carried" && (capture.Kind != "anonymous" || row.Native["label"] != "rootDatetimeNull" || len(row.MatchedPaths) != 1 || row.MatchedPaths[0] != "/label" || len(row.CarriedPaths) != 2) {
						t.Fatal("unbounded full-behavior carry")
					}
					for _, path := range row.MatchedPaths {
						want, ok := jsonShortYearDateFacet(row.Native, path)
						value, actualOK := jsonShortYearDateFacet(got, path)
						if !ok || !actualOK || !reflect.DeepEqual(value, want) {
							t.Errorf("matched %q: native=%v; Glade=%v", path, want, value)
						}
					}
					for path, reason := range row.CarriedPaths {
						value, ok := jsonShortYearDateFacet(row.Native, path)
						if !ok || reason == "" {
							t.Fatalf("carry lacks native value/reason: %s", path)
						}
						t.Logf("carried %s: native=%v; %s", path, value, reason)
					}
				})
			}
		})
	}
	if rows != data.Known || len(expected) != 0 || !reflect.DeepEqual(classes, data.Classifications) || matchedPaths != data.MatchedPaths || carriedPaths != data.CarriedPaths {
		t.Fatal("native row/facet coverage differs")
	}
}

func jsonDateNullNativeReplay(t *testing.T, capture jsonDateNullNativeCapture) map[string]map[string]any {
	t.Helper()
	decode := func(message string) map[string]any {
		t.Helper()
		_, payload, ok := strings.Cut(message, capture.Marker)
		if !ok {
			t.Fatalf("missing captured marker: %s", message)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(payload), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	rawNames := []string{"org.tsv", "errors.json", "ORG-RUN.txt"}
	if capture.Kind == "anonymous" {
		rawNames = append(rawNames, "run.json", "source.apex")
	} else {
		rawNames = append(rawNames, "deploy.json")
	}
	for _, name := range rawNames {
		sum, err := hex.DecodeString(capture.RawSHA256[name])
		if err != nil || len(sum) != sha256.Size {
			t.Fatalf("missing raw digest %s", name)
		}
	}
	actual := map[string]map[string]any{}
	if capture.Kind == "anonymous" {
		if capture.Route != "anonymous" || capture.TransportAPI != capture.SourceAPI || capture.Marker != "OWNED_DATE_NULL_ANONYMOUS|" || len(capture.Rows) != 17 {
			t.Fatal("anonymous scope differs")
		}
		e := capture.Execution
		if e.Status != 0 || e.ExitCode != 0 || !e.Success || !e.Compiled || e.CompileProblem != "" || e.ExceptionMessage != "" || len(e.ParseErrors) != 0 {
			t.Fatal("incomplete native anonymous execution")
		}
		sum := sha256.Sum256([]byte(capture.Source.Source))
		if capture.Source.Source == "" || hex.EncodeToString(sum[:]) != capture.Source.SHA256 {
			t.Fatal("anonymous source digest differs")
		}
		analysis := sema.AnalyzeAnonymous(typesys.Index{}, capture.Source.Source, capture.SourceAPI)
		if analysis.HasErrors() {
			t.Fatalf("native compiled; Glade diagnostics=%v", analysis.Diagnostics)
		}
		program, err := vm.CompileAnonymousWithOptions(capture.Source.Source, vm.CompileOptions{APIVersion: capture.SourceAPI})
		if err != nil {
			t.Fatal(err)
		}
		runner := newConformanceRunner(t, typesys.Index{}, conformanceRunnerOptions{})
		result, err := runner.execute(program)
		if err != nil {
			t.Fatal(err)
		}
		for _, message := range result.Debug {
			if strings.HasPrefix(message, capture.Marker) {
				actual[strconv.Itoa(len(actual))] = decode(message)
			}
		}
		return actual
	}
	if capture.Kind != "named" || capture.Route != "isTest" || capture.TransportAPI != "62.0" {
		t.Fatal("named scope differs")
	}
	wantClass, wantMarker, wantRows := "OwnedDateNullBoundary", "OWNED_DATE_NULL_BOUNDARY|", 6
	if strings.HasPrefix(capture.Context, "origin-input-") {
		wantClass, wantMarker, wantRows = "OwnedDateNullInputOrigins", "OWNED_DATE_NULL_INPUT_ORIGINS|", 4
	}
	if capture.TestClass != wantClass || capture.Marker != wantMarker || len(capture.Rows) != wantRows {
		t.Fatal("named fixture shape differs")
	}
	d := capture.Deployment
	if d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestsCompleted != 0 || d.NumberTestErrors != d.NumberTestsTotal || d.NumberTestsTotal != len(capture.Rows) {
		t.Fatal("incomplete native named deployment")
	}
	if capture.Context == "origin-input-api63" {
		c := capture.ParserCorrection
		if c == nil || c.Reason == "" || c.NativeRunRepeated || !c.RawDeploymentUnchanged || len(c.InitialParserSHA256) != 3 {
			t.Fatal("original marker-parser carry missing")
		}
		for _, name := range []string{"org.tsv", "errors.json", "digests.json"} {
			sum, err := hex.DecodeString(c.InitialParserSHA256[name])
			if err != nil || len(sum) != sha256.Size {
				t.Fatal("initial parser digest differs")
			}
		}
	} else if capture.ParserCorrection != nil {
		t.Fatal("unexpected parser carry context")
	}
	root := t.TempDir()
	for name, source := range capture.Sources {
		sum := sha256.Sum256([]byte(source.Source))
		if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 {
			t.Fatal("named source digest differs")
		}
		writeFile(t, filepath.Join(root, name), source.Source)
	}
	var project struct {
		API string `json:"sourceApiVersion"`
	}
	if err := json.Unmarshal([]byte(capture.Sources["sfdx-project.json"].Source), &project); err != nil || project.API != capture.SourceAPI {
		t.Fatal("named project pin differs")
	}
	var metadata struct {
		API string `xml:"apiVersion"`
	}
	if err := xml.Unmarshal([]byte(capture.Sources["force-app/main/default/classes/"+capture.TestClass+".cls-meta.xml"].Source), &metadata); err != nil || metadata.API != capture.SourceAPI {
		t.Fatal("named source pin differs")
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native compiled; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = capture.SourceAPI
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
	if len(cases) != len(capture.Rows) {
		t.Fatal("named discovery differs")
	}
	methods, methodErrors := compileTestMethodsForIndex(&index, cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	for _, row := range capture.Rows {
		if !reflect.DeepEqual(decode(row.NativeFailure.Message), row.Native) {
			t.Fatal("native assertion envelope differs")
		}
		key := ""
		for _, c := range cases {
			if c.MethodName == row.ID {
				key = testCaseKey(c)
			}
		}
		if key == "" || methodErrors[key] != nil || programErrors[key] != nil {
			t.Fatal("named compilation differs")
		}
		machine := runner.newMachine()
		if err := machine.RegisterMethod(methods[key]); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		_, execErr := machine.ExecuteInClass(programs[key], capture.TestClass)
		var runtimeErr *vm.RuntimeError
		if !errors.As(execErr, &runtimeErr) || genericInterfaceCastExceptionType(runtimeErr.Type) != "System.AssertException" {
			t.Fatalf("observation assertion missing: %v", execErr)
		}
		actual[row.ID] = decode(runtimeErr.Message)
	}
	return actual
}

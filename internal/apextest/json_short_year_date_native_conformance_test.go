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

func TestJSONShortYearDateNativeConformance(t *testing.T) {
	var data struct {
		APIs              []string `json:"apiVersions"`
		Route             string
		TransportAPI      string   `json:"transportApi"`
		Namespace         string   `json:"namespaceContext"`
		Known             int      `json:"knownRows"`
		Pending           []string `json:"pendingCoverage"`
		AnonymousRows     int      `json:"anonymousRows"`
		AnonymousCaptures []jsonShortYearDateAnonymousCapture
		Captures          []struct {
			Context, SourceAPI, TestClass, Marker string
			Deployment                            struct {
				ID, Status                              string
				Done, CheckOnly                         bool
				NumberComponentErrors, NumberTestErrors int
				NumberTestsCompleted, NumberTestsTotal  int
			}
			RawSHA256 map[string]string
			Sources   map[string]struct{ Source, SHA256 string }
			Rows      []struct {
				Method        string
				NativeFailure struct{ Message string }
				Native        map[string]any
				MatchedPaths  []string
				CarriedPaths  map[string]string
			}
		}
	}
	// These spellings are explicit because transport/source scope must not be
	// inferred from a capture directory or from the current runner's API.
	raw, err := os.ReadFile("testdata/conformance/json_short_year_date_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIs, []string{"62.0", "63.0", "67.0"}) || data.Route != "isTest" || data.TransportAPI != "62.0" || data.Namespace != "none" || data.Known != 13 || len(data.Captures) != 4 || len(data.Pending) == 0 {
		t.Fatal("native capture scope differs")
	}
	expectedAPIs := map[string]string{"api62": "62.0", "api63": "63.0", "api67": "67.0", "equality-api63": "63.0"}
	seen, rows := map[string]bool{}, 0
	for _, capture := range data.Captures {
		t.Run(capture.Context, func(t *testing.T) {
			if expectedAPIs[capture.Context] != capture.SourceAPI {
				t.Fatal("capture source scope differs")
			}
			delete(expectedAPIs, capture.Context)
			d := capture.Deployment
			if d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestsCompleted != 0 || d.NumberTestErrors != d.NumberTestsTotal || d.NumberTestsTotal != len(capture.Rows) {
				t.Fatal("incomplete native deployment")
			}
			for _, name := range []string{"org.tsv", "deploy.json", "ORG-RUN.txt", "errors.json"} {
				sum, err := hex.DecodeString(capture.RawSHA256[name])
				if err != nil || len(sum) != sha256.Size {
					t.Fatalf("missing raw digest %s", name)
				}
			}
			root := t.TempDir()
			for name, source := range capture.Sources {
				sum := sha256.Sum256([]byte(source.Source))
				if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 {
					t.Fatal("captured source differs")
				}
				writeFile(t, filepath.Join(root, name), source.Source)
			}
			var metadata struct {
				API string `xml:"apiVersion"`
			}
			if err := xml.Unmarshal([]byte(capture.Sources["force-app/main/default/classes/"+capture.TestClass+".cls-meta.xml"].Source), &metadata); err != nil || metadata.API != capture.SourceAPI {
				t.Fatal("named source API differs")
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
				t.Fatal("native method discovery differs")
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			for _, row := range capture.Rows {
				t.Run(row.Method, func(t *testing.T) {
					identity := capture.Context + "/" + row.Method
					if seen[identity] || len(row.MatchedPaths) == 0 {
						t.Fatal("duplicate or unsupported native row")
					}
					seen[identity] = true
					rows++
					decode := func(message string) map[string]any {
						t.Helper()
						_, text, ok := strings.Cut(message, capture.Marker)
						if !ok {
							t.Fatalf("missing observation marker: %s", message)
						}
						var value map[string]any
						if err := json.Unmarshal([]byte(text), &value); err != nil {
							t.Fatal(err)
						}
						return value
					}
					if !reflect.DeepEqual(decode(row.NativeFailure.Message), row.Native) {
						t.Fatal("native envelope differs")
					}
					key := ""
					for _, c := range cases {
						if c.MethodName == row.Method {
							key = testCaseKey(c)
						}
					}
					if key == "" || methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
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
					got := decode(runtimeErr.Message)
					for _, path := range row.MatchedPaths {
						wantFacet, ok := jsonShortYearDateFacet(row.Native, path)
						gotFacet, actualOK := jsonShortYearDateFacet(got, path)
						if !ok || !actualOK || !reflect.DeepEqual(gotFacet, wantFacet) {
							t.Errorf("matched facet %q: got=%v want=%v", path, gotFacet, wantFacet)
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
	if rows != data.Known || len(expectedAPIs) != 0 {
		t.Fatalf("matched scoped rows=%d want=%d", rows, data.Known)
	}
	if data.AnonymousRows != 21 || len(data.AnonymousCaptures) != 3 {
		t.Fatal("anonymous scope coverage differs")
	}
	expectedAnonymous := map[string]string{"anonymous-api62": "62.0", "anonymous-api63": "63.0", "anonymous-api67": "67.0"}
	anonymousRows := 0
	for _, capture := range data.AnonymousCaptures {
		t.Run(capture.Context, func(t *testing.T) {
			if expectedAnonymous[capture.Context] != capture.SourceAPI || capture.TransportAPI != capture.SourceAPI || capture.Route != "anonymous" || capture.NamespaceContext != "none" || capture.Marker != "OWNED_JSON_DATE_OBSERVATION|" || len(capture.Rows) != 7 {
				t.Fatal("anonymous source/transport scope differs")
			}
			delete(expectedAnonymous, capture.Context)
			e := capture.Execution
			if e.Status != 0 || e.ExitCode != 0 || !e.Success || !e.Compiled || e.CompileProblem != "" || e.ExceptionMessage != "" || len(e.ParseErrors) != 0 {
				t.Fatal("anonymous native execution incomplete")
			}
			for _, name := range []string{"org.tsv", "run.json", "source.apex", "ORG-RUN.txt", "errors.json"} {
				sum, err := hex.DecodeString(capture.RawSHA256[name])
				if err != nil || len(sum) != sha256.Size {
					t.Fatalf("missing anonymous raw digest %s", name)
				}
			}
			if capture.Context == "anonymous-api63" {
				c := capture.ParserCorrection
				if c == nil || c.Reason == "" || c.NativeRunRepeated || !c.RawRunUnchanged || len(c.InitialParserSHA256) != 3 {
					t.Fatal("original anonymous parser carry missing")
				}
				for _, name := range []string{"org.tsv", "errors.json", "digests.json"} {
					sum, err := hex.DecodeString(c.InitialParserSHA256[name])
					if err != nil || len(sum) != sha256.Size {
						t.Fatalf("missing initial-parser digest %s", name)
					}
				}
			} else if capture.ParserCorrection != nil {
				t.Fatal("unexpected parser carry scope")
			}
			sum := sha256.Sum256([]byte(capture.Source.Source))
			if capture.Source.Source == "" || hex.EncodeToString(sum[:]) != capture.Source.SHA256 {
				t.Fatal("anonymous source digest differs")
			}
			analysis := sema.AnalyzeAnonymous(typesys.Index{}, capture.Source.Source, capture.SourceAPI)
			if analysis.HasErrors() {
				t.Fatalf("native anonymous compiled; Glade diagnostics=%v", analysis.Diagnostics)
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
			var actual []map[string]any
			for _, message := range result.Debug {
				if payload, ok := strings.CutPrefix(message, capture.Marker); ok {
					var row map[string]any
					if err := json.Unmarshal([]byte(payload), &row); err != nil {
						t.Fatal(err)
					}
					actual = append(actual, row)
				}
			}
			if len(actual) != len(capture.Rows) {
				t.Fatalf("anonymous observations=%d want=%d", len(actual), len(capture.Rows))
			}
			classes := []string{"partial", "partial", "partial", "partial", "full", "partial", "full"}
			for i, row := range capture.Rows {
				t.Run(row.ID, func(t *testing.T) {
					anonymousRows++
					if row.ID != strconv.Itoa(i) || row.Classification != classes[i] || len(row.MatchedPaths) == 0 {
						t.Fatal("anonymous row classification differs")
					}

					for _, path := range row.MatchedPaths {
						want, ok := jsonShortYearDateFacet(row.Native, path)
						got, actualOK := jsonShortYearDateFacet(actual[i], path)
						if !ok || !actualOK || !reflect.DeepEqual(got, want) {
							t.Errorf("matched anonymous facet %q: got=%v want=%v", path, got, want)
						}
					}
					for path, reason := range row.CarriedPaths {
						value, ok := jsonShortYearDateFacet(row.Native, path)
						if !ok || reason == "" {
							t.Fatalf("anonymous carry lacks native value/reason: %s", path)
						}
						t.Logf("carried %s: native=%v; %s", path, value, reason)
					}
				})
			}
		})
	}
	if anonymousRows != data.AnonymousRows || len(expectedAnonymous) != 0 {
		t.Fatalf("anonymous rows=%d want=%d", anonymousRows, data.AnonymousRows)
	}
}

func jsonShortYearDateFacet(value any, path string) (any, bool) {
	if path == "" {
		return value, true
	}
	if !strings.HasPrefix(path, "/") {
		return nil, false
	}
	for _, part := range strings.Split(path[1:], "/") {
		switch current := value.(type) {
		case map[string]any:
			var ok bool
			value, ok = current[part]
			if !ok {
				return nil, false
			}
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(current) {
				return nil, false
			}
			value = current[i]
		default:
			return nil, false
		}
	}
	return value, true
}

// Anonymous transport observations stay separate from named helper/caller rows.
// Root scalar-null rows now assert their complete native JSONException payload.
type jsonShortYearDateAnonymousCapture struct {
	Context, SourceAPI, TransportAPI, Route, NamespaceContext, Marker string
	Execution                                                         struct {
		Status, ExitCode                 int
		Success, Compiled                bool
		CompileProblem, ExceptionMessage string
		ParseErrors                      []any
	}
	Source           struct{ Source, SHA256 string }
	RawSHA256        map[string]string
	ParserCorrection *struct {
		Reason                             string
		InitialParserSHA256                map[string]string
		NativeRunRepeated, RawRunUnchanged bool
	}
	Rows []struct {
		ID, Classification string
		Native             map[string]any
		MatchedPaths       []string
		CarriedPaths       map[string]string
	}
}

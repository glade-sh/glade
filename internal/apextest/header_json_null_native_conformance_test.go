package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestHeaderJSONNullNativeConformance(t *testing.T) {
	var data struct {
		APIs                                                              []string `json:"apiVersions"`
		NamedTransportAPI, NamedProjectAPI, NamespaceContext, OrgIDSHA256 string
		KnownCells, NamedCells, AnonymousCells                            int
		Pending                                                           []string `json:"pendingCoverage"`
		NamedCaptures                                                     []struct {
			Context, SourceAPI, TestClass, Marker string
			Deployment                            struct {
				ID, Status                                                                      string
				Done, CheckOnly                                                                 bool
				NumberComponentErrors, NumberTestErrors, NumberTestsCompleted, NumberTestsTotal int
			}
			RawSHA256 map[string]string
			Sources   map[string]struct{ Source, SHA256 string }
			Rows      []struct {
				Method        string
				Native        map[string]any
				NativeFailure struct{ Message string }
			}
		}
		AnonymousCaptures []struct {
			Context, SourceAPI, TransportAPI, Namespace, Marker string
			Execution                                           struct {
				Status                           int
				Success, Compiled                bool
				CompileProblem, ExceptionMessage string
			}
			RawSHA256 map[string]string
			Source    struct{ Source, SHA256 string }
			Native    struct{ Cells []map[string]any }
		}
	}
	raw, err := os.ReadFile("testdata/conformance/header_json_null_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.APIs, []string{"62.0", "67.0", "68.0"}) || data.NamedTransportAPI != "62.0" || data.NamedProjectAPI != "62.0" || data.NamespaceContext != "none" || data.KnownCells != 204 || data.NamedCells != 102 || data.AnonymousCells != 102 || len(data.NamedCaptures) != 3 || len(data.AnonymousCaptures) != 3 || len(data.Pending) == 0 {
		t.Fatal("native isolation scope differs")
	}
	if data.OrgIDSHA256 != "65614fb75c7a440ea1f5a5fdb1cd14b0ce5081d7888d05d063bf5c57ecc9d931" {
		t.Fatal("native org context differs")
	}
	decode := func(t *testing.T, message, marker string) map[string]any {
		t.Helper()
		_, payload, ok := strings.Cut(message, marker)
		if !ok {
			t.Fatalf("native marker missing: %s", message)
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	checkDigests := func(t *testing.T, digests map[string]string, names ...string) {
		t.Helper()
		for _, name := range names {
			sum, err := hex.DecodeString(digests[name])
			if err != nil || len(sum) != sha256.Size {
				t.Fatalf("missing native digest %s", name)
			}
		}
	}
	cellNames := []string{"baseline", "omitted", "rootNull", "priorComposite", "allNull", "null_entityName", "null_changeType", "null_changeOrigin", "null_transactionKey", "null_sequenceNumber", "null_commitTimestamp", "null_commitUser", "null_commitNumber", "null_recordIds", "null_changedFields", "null_nulledFields", "null_diffFields"}
	checkCell := func(t *testing.T, row map[string]any, seen map[string]bool) string {
		t.Helper()
		cell, ok := row["cell"].(string)
		if !ok {
			t.Fatal("native cell name missing")
		}
		strict, ok := row["strict"].(bool)
		if !ok {
			t.Fatal("native strict mode missing")
		}
		known := false
		for _, name := range cellNames {
			known = known || name == cell
		}
		key := fmt.Sprintf("%t/%s", strict, cell)
		if !known || seen[key] || len(row) != 4 {
			t.Fatalf("duplicate/unknown native cell %s", key)
		}
		seen[key] = true
		phase := "complete"
		if cell == "null_recordIds" || cell == "priorComposite" || cell == "allNull" {
			phase = "deserialize"
		}
		if row["phase"] != phase {
			t.Fatalf("native phase differs for %s", key)
		}
		return key
	}
	namedRows, anonymousRows := 0, 0
	expectedNamed := map[string]string{"api62-named": "62.0", "api67-named": "67.0", "api68-named": "68.0"}
	for _, capture := range data.NamedCaptures {
		t.Run(capture.Context, func(t *testing.T) {
			if expectedNamed[capture.Context] != capture.SourceAPI || capture.TestClass != "OwnedHeaderNullIsolation" || capture.Marker != "HEADER_NULL_OBSERVATION|" {
				t.Fatal("named context differs")
			}
			delete(expectedNamed, capture.Context)
			d := capture.Deployment
			if d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestsCompleted != 0 || d.NumberTestErrors != 34 || d.NumberTestsTotal != 34 || len(capture.Rows) != 34 {
				t.Fatal("native named deployment incomplete")
			}
			checkDigests(t, capture.RawSHA256, "org.tsv", "deploy.json", "ORG-RUN.txt", "errors.json")
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
				t.Fatal("native class source pin differs")
			}
			var project struct {
				API string `json:"sourceApiVersion"`
			}
			if err := json.Unmarshal([]byte(capture.Sources["sfdx-project.json"].Source), &project); err != nil || project.API != data.NamedProjectAPI {
				t.Fatal("native project pin differs")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = data.NamedProjectAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
			if len(cases) != 34 {
				t.Fatal("native method discovery differs")
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			seen := map[string]bool{}
			for _, row := range capture.Rows {
				t.Run(row.Method, func(t *testing.T) {
					checkCell(t, row.Native, seen)
					namedRows++
					mode := "plain"
					if row.Native["strict"].(bool) {
						mode = "strict"
					}
					if row.Method != mode+"_"+row.Native["cell"].(string) {
						t.Fatal("native method/cell identity differs")
					}
					if !reflect.DeepEqual(row.Native, decode(t, row.NativeFailure.Message, capture.Marker)) {
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
						t.Fatalf("native observation assertion missing: %v", execErr)
					}
					if got := decode(t, runtimeErr.ExceptionMessage(), capture.Marker); !reflect.DeepEqual(got, row.Native) {
						t.Fatalf("native %v; Glade %v", row.Native, got)
					}
				})
			}
			if len(seen) != 34 {
				t.Fatal("native cell coverage differs")
			}
		})
	}
	expectedAnonymous := map[string]string{"anonymous-api62": "62.0", "anonymous-api67": "67.0", "anonymous-api68": "68.0"}
	for _, capture := range data.AnonymousCaptures {
		t.Run(capture.Context, func(t *testing.T) {
			if expectedAnonymous[capture.Context] != capture.SourceAPI || capture.TransportAPI != capture.SourceAPI || capture.Namespace != "none" || capture.Marker != "HEADER_NULL_ANONYMOUS|" {
				t.Fatal("anonymous context differs")
			}
			delete(expectedAnonymous, capture.Context)
			e := capture.Execution
			if e.Status != 0 || !e.Success || !e.Compiled || e.CompileProblem != "" || e.ExceptionMessage != "" || len(capture.Native.Cells) != 34 {
				t.Fatal("native anonymous execution incomplete")
			}
			checkDigests(t, capture.RawSHA256, "org.tsv", "run.json", "ORG-RUN.txt", "errors.json", "source.apex")
			sum := sha256.Sum256([]byte(capture.Source.Source))
			if capture.Source.Source == "" || hex.EncodeToString(sum[:]) != capture.Source.SHA256 || capture.RawSHA256["source.apex"] != capture.Source.SHA256 {
				t.Fatal("anonymous source differs")
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
			var envelopes []map[string]any
			for _, message := range result.Debug {
				if strings.HasPrefix(message, capture.Marker) {
					envelopes = append(envelopes, decode(t, message, capture.Marker))
				}
			}
			if len(envelopes) != 1 || len(envelopes[0]) != 1 {
				t.Fatal("anonymous envelope count/shape differs")
			}
			actual, ok := envelopes[0]["cells"].([]any)
			if !ok || len(actual) != 34 {
				t.Fatal("anonymous cell count differs")
			}
			seen := map[string]bool{}
			for i, row := range capture.Native.Cells {
				t.Run(fmt.Sprintf("%t/%s", row["strict"], row["cell"]), func(t *testing.T) {
					checkCell(t, row, seen)
					anonymousRows++
					if !reflect.DeepEqual(actual[i], row) {
						t.Fatalf("native %v; Glade %v", row, actual[i])
					}
				})
			}
			if len(seen) != 34 {
				t.Fatal("anonymous native cell coverage differs")
			}
		})
	}
	if namedRows != data.NamedCells || anonymousRows != data.AnonymousCells || namedRows+anonymousRows != data.KnownCells || len(expectedNamed) != 0 || len(expectedAnonymous) != 0 {
		t.Fatal("native isolation coverage differs")
	}
}

package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const eventRelationshipNullFixture = "testdata/conformance/event_relationship_null"

type eventRelationshipNullData struct {
	APIVersions, Routes, RowIDs                                 []string
	ShapeCount, NativeObservationCount, CarriedObservationCount int
	Files                                                       map[string]string
}

// The exact owned callers retain their capture transports: anonymous debug and
// a named @IsTest terminal assertion. Every value is compared with its own
// native TSV cell; the terminal assertion is not a behavioral failure claim.
func TestEventRelationshipNullNativeConformance(t *testing.T) {
	data, sources, answers := loadEventRelationshipNullEvidence(t)
	classes := map[string]string{
		"MeadowEventTypeProbe":     string(sources["sources/MeadowEventTypeProbe.cls"]),
		"MeadowEventTypeProbeTest": string(sources["sources/MeadowEventTypeProbeTest.cls"]),
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "EventRelationshipNull", api)
			index := nativeBoundaryIndex(t, api, classes, nil)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"MeadowEventTypeProbeTest"}, SelectedMethod: "observeRows"})
			if len(cases) != 1 {
				t.Fatalf("expected captured named method, got %d", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, route := range data.Routes {
				t.Run(route, func(t *testing.T) {
					machine := base.newMachine()
					payload := ""
					if route == "anonymous" {
						source := string(sources["sources/anonymous.apex"])
						analysis := sema.AnalyzeAnonymous(index, source, api)
						program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if analysis.HasErrors() || compileErr != nil {
							t.Fatalf("native anonymous accepted: %v %v", analysis.Diagnostics, compileErr)
						}
						result, err := machine.Execute(program)
						if err != nil {
							t.Fatal(err)
						}
						markers := 0
						for _, line := range result.Debug {
							if strings.HasPrefix(line, "PROBE|") {
								markers++
								payload = strings.TrimPrefix(line, "PROBE|")
							}
						}
						if markers != 1 {
							t.Fatalf("expected one captured debug payload, got %d", markers)
						}
					} else {
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteTestInClass(programs[key], cases[0].ClassName)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
							t.Fatalf("missing captured assertion transport: %v", err)
						}
						message := runtimeErr.ExceptionMessage()
						var found bool
						_, payload, found = strings.Cut(message, "PROBE|")
						if !found || strings.Count(message, "PROBE|") != 1 {
							t.Fatalf("invalid captured assertion transport: %s", message)
						}
					}
					var observed map[string]json.RawMessage
					if err := json.Unmarshal([]byte(payload), &observed); err != nil || len(observed) != data.ShapeCount {
						t.Fatalf("invalid observed payload: %s (%v)", payload, err)
					}
					countRoute := route
					if route == "istest" {
						countRoute = "isTest"
					}
					for _, id := range data.RowIDs {
						counts.Run(t, countRoute, id, id, func(t *testing.T) {
							value, found := observed[id]
							if !found {
								t.Fatal("missing observed row")
							}
							want := answers[api+"/"+route][id]
							if eventRelationshipNullScalar(t, id, value) != eventRelationshipNullScalar(t, id, want) {
								t.Fatalf("native %s; Glade %s", want, value)
							}
						})
					}
				})
			}
		})
	}
}

func loadEventRelationshipNullEvidence(t *testing.T) (eventRelationshipNullData, map[string][]byte, map[string]map[string]json.RawMessage) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(eventRelationshipNullFixture, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var data eventRelationshipNullData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, 12)
	for i := 1; i <= 9; i++ {
		ids = append(ids, fmt.Sprintf("EN%02d", i))
	}
	for i := 1; i <= 3; i++ {
		ids = append(ids, fmt.Sprintf("EC%02d", i))
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || strings.Join(data.RowIDs, ",") != strings.Join(ids, ",") || data.ShapeCount != 12 || data.NativeObservationCount != 72 || data.CarriedObservationCount != 0 || len(data.Files) != 15 {
		t.Fatal("incomplete native Event relationship matrix")
	}
	files := make(map[string][]byte, len(data.Files))
	read := func(path string) []byte {
		t.Helper()
		wantHash, ok := data.Files[path]
		if !ok {
			t.Fatalf("missing capture/source file: %s", path)
		}
		raw, err := os.ReadFile(filepath.Join(eventRelationshipNullFixture, path))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != wantHash {
			t.Fatalf("changed native capture/source: %s", path)
		}
		files[path] = raw
		return raw
	}
	sourceKeys := map[string]string{"helper": "sources/MeadowEventTypeProbe.cls", "wrapper": "sources/MeadowEventTypeProbeTest.cls", "anonymous": "sources/anonymous.apex"}
	for _, path := range sourceKeys {
		read(path)
	}
	answers := make(map[string]map[string]json.RawMessage)
	for _, api := range data.APIVersions {
		for _, route := range data.Routes {
			root := "native/api" + strings.TrimSuffix(api, ".0") + "/" + route + "/"
			run := make(map[string]string)
			for _, line := range strings.Split(strings.TrimSuffix(string(read(root+"RUN-FACTS.txt")), "\n"), "\n") {
				key, value, found := strings.Cut(line, "=")
				if !found || key == "" || run[key] != "" {
					t.Fatal("invalid/repeated native run fact")
				}
				run[key] = value
			}
			if run["api"] != api || run["context"] != route || run["rows"] != "12" || run["native_complete"] != "true" || run["deployment_success"] != "true" || run["helper_class_api"] != api || run["wrapper_class_api"] != api || run["captured_utc"] == "" || run["transport"] == "" {
				t.Fatal("incomplete native capture facts")
			}
			sourceHash, err := hex.DecodeString(run["helper_source_sha256"])
			if err != nil || len(sourceHash) != sha256.Size {
				t.Fatal("invalid native helper source hash")
			}
			for name, path := range sourceKeys {
				if run[name+"_source_sha256"] != data.Files[path] {
					t.Fatalf("capture detached from %s source", name)
				}
			}
			lines := strings.Split(strings.TrimSuffix(string(read(root+"org.tsv")), "\n"), "\n")
			if len(lines) != 13 || lines[0] != "id\tgroup\troute\tstatus\tvalue" {
				t.Fatal("incomplete native TSV")
			}
			values := make(map[string]json.RawMessage)
			for i, line := range lines[1:] {
				fields := strings.SplitN(line, "\t", 5)
				group := "PREDICATE"
				if strings.HasPrefix(ids[i], "EC") {
					group = "CONTROL"
				}
				if len(fields) != 5 || fields[0] != ids[i] || fields[1] != group || fields[2] != route || fields[3] != "runtime" {
					t.Fatal("invalid native row identity/context")
				}
				var envelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(fields[4]), &envelope); err != nil || len(envelope) != 2 || string(envelope["outcome"]) != `"runtime"` {
					t.Fatal("invalid native row payload")
				}
				value, ok := envelope["value"]
				if !ok {
					t.Fatal("missing native value")
				}
				eventRelationshipNullScalar(t, fields[0], value)
				values[fields[0]] = value
			}
			answers[api+"/"+route] = values
		}
	}
	if len(files) != len(data.Files) || len(answers) != 6 {
		t.Fatal("unexpected or incomplete capture file matrix")
	}
	return data, files, answers
}

func eventRelationshipNullScalar(t *testing.T, id string, raw json.RawMessage) any {
	t.Helper()
	if id == "EC02" {
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil || string(raw) == "null" {
			t.Fatalf("%s requires captured Boolean value: %s", id, raw)
		}
		return value
	}
	value, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil {
		t.Fatalf("%s requires captured integer count: %s", id, raw)
	}
	return value
}

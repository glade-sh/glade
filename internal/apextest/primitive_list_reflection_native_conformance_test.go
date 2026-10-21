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
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Replay the owned named-test sources, retaining helper and caller metadata
// independently. The intentional assertion transports observations rather than
// asserting an expected native result inside the Apex fixture.
func TestPrimitiveListReflectionNativeConformance(t *testing.T) {
	var data struct {
		Marker          string `json:"marker"`
		NativeRowCount  int    `json:"nativeRowCount"`
		UniqueRowCount  int    `json:"uniqueRowCount"`
		PendingCoverage struct {
			Status               string   `json:"status"`
			AdditionalCallerAPIs []string `json:"additionalCallerApis"`
			AdditionalRoutes     []string `json:"additionalRoutes"`
			AdditionalHelperAPIs []string `json:"additionalHelperApis"`
		} `json:"pendingCoverage"`
		Captures []struct {
			CallerAPI        string `json:"callerApi"`
			HelperAPI        string `json:"helperApi"`
			TransportAPI     string `json:"transportApi"`
			Route            string `json:"route"`
			NamespaceContext string `json:"namespaceContext"`
			ClassName        string `json:"className"`
			HelperName       string `json:"helperName"`
			Sources          map[string]struct {
				Source string `json:"source"`
				SHA256 string `json:"sha256"`
			} `json:"sources"`
			Deployment struct {
				ID              string `json:"id"`
				CheckOnly       bool   `json:"checkOnly"`
				Done            bool   `json:"done"`
				Status          string `json:"status"`
				ComponentErrors int    `json:"numberComponentErrors"`
				TestErrors      int    `json:"numberTestErrors"`
				TestsCompleted  int    `json:"numberTestsCompleted"`
				TestsTotal      int    `json:"numberTestsTotal"`
			} `json:"deployment"`
			MetadataUnchanged bool              `json:"metadataUnchanged"`
			RawSHA256         map[string]string `json:"rawSha256"`
			Rows              []struct {
				Method        string `json:"method"`
				NativeFailure struct {
					Name       string `json:"name"`
					MethodName string `json:"methodName"`
					Message    string `json:"message"`
				} `json:"nativeFailure"`
				Native json.RawMessage `json:"native"`
			} `json:"rows"`
		} `json:"captures"`
	}
	raw, err := os.ReadFile("testdata/conformance/primitive_list_reflection_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Marker != "PRIMITIVE_LIST_OBSERVATION" || len(data.Captures) == 0 {
		t.Fatal("missing native observations")
	}
	observation := func(t *testing.T, message string) map[string]any {
		t.Helper()
		prefix, payload, ok := strings.Cut(message, ", Actual: ")
		if !ok || strings.Count(prefix, data.Marker) != 1 {
			t.Fatalf("observation envelope differs: %s", message)
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			t.Fatalf("invalid observation JSON: %v", err)
		}
		if len(value) == 0 {
			t.Fatal("empty observation")
		}
		return value
	}
	if data.PendingCoverage.Status != "PENDING-ROWS" || strings.Join(data.PendingCoverage.AdditionalCallerAPIs, ",") != "67.0,68.0" || strings.Join(data.PendingCoverage.AdditionalRoutes, ",") != "anonymous" || strings.Join(data.PendingCoverage.AdditionalHelperAPIs, ",") != "61.0,62.0,67.0,68.0" {
		t.Fatal("unobserved coverage metadata differs")
	}
	rows := 0
	unique := map[string]map[string]any{}
	seenCaptures := map[string]bool{}
	for _, capture := range data.Captures {
		captureName := capture.Deployment.ID
		if seenCaptures[captureName] {
			t.Fatal("duplicate native capture")
		}
		seenCaptures[captureName] = true
		if capture.Route != "isTest" || capture.NamespaceContext != "none" || capture.TransportAPI != capture.CallerAPI || capture.HelperAPI != "54.0" || capture.CallerAPI != "62.0" || !capture.MetadataUnchanged {
			t.Fatal("native execution context differs")
		}
		deployment := capture.Deployment
		if deployment.ID == "" || !deployment.CheckOnly || !deployment.Done || deployment.Status != "Failed" || deployment.ComponentErrors != 0 || deployment.TestsCompleted != 0 || deployment.TestErrors != deployment.TestsTotal || deployment.TestsTotal != len(capture.Rows) || (len(capture.Rows) != 6 && len(capture.Rows) != 17) {
			t.Fatal("incomplete native assertion transport")
		}
		for _, name := range []string{"deploy.json", "org.tsv", "SOURCE-DIGESTS.json", "RAW-DIGESTS.json", "ORG-RUN.txt", "errors.json"} {
			hash, err := hex.DecodeString(capture.RawSHA256[name])
			if err != nil || len(hash) != sha256.Size {
				t.Fatalf("missing raw provenance: %s", name)
			}
		}
		t.Run(captureName, func(t *testing.T) {
			root := t.TempDir()
			for name, source := range capture.Sources {
				if filepath.IsAbs(name) || strings.Contains(name, "..") || name == "" {
					t.Fatal("invalid captured source path")
				}
				sum := sha256.Sum256([]byte(source.Source))
				if source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 {
					t.Fatalf("captured source changed: %s", name)
				}
				writeFile(t, filepath.Join(root, name), source.Source)
			}
			var projectMetadata struct {
				API string `json:"sourceApiVersion"`
			}
			if err := json.Unmarshal([]byte(capture.Sources["sfdx-project.json"].Source), &projectMetadata); err != nil || projectMetadata.API != capture.CallerAPI {
				t.Fatal("project source API differs")
			}
			for name, api := range map[string]string{capture.ClassName: capture.CallerAPI, capture.HelperName: capture.HelperAPI} {
				path := "force-app/main/default/classes/" + name + ".cls"
				var metadata struct {
					API string `xml:"apiVersion"`
				}
				if capture.Sources[path].Source == "" {
					t.Fatal("missing captured class")
				}
				if err := xml.Unmarshal([]byte(capture.Sources[path+"-meta.xml"].Source), &metadata); err != nil || metadata.API != api {
					t.Fatalf("class metadata differs: %s", name)
				}
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native fixture compiled; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = capture.CallerAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: []string{capture.ClassName}})
			if len(cases) != len(capture.Rows) {
				t.Fatal("named observation discovery differs")
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			seenMethods := map[string]bool{}
			for _, row := range capture.Rows {
				rows++
				t.Run(row.Method, func(t *testing.T) {
					if row.Method == "" || seenMethods[row.Method] || row.NativeFailure.Name != capture.ClassName || row.NativeFailure.MethodName != row.Method {
						t.Fatal("native method identity differs")
					}
					seenMethods[row.Method] = true
					var want map[string]any
					if err := json.Unmarshal(row.Native, &want); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(want, observation(t, row.NativeFailure.Message)) {
						t.Fatal("native payload differs from retained failure envelope")
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
					_, execErr := machine.ExecuteInClass(programs[key], capture.ClassName)
					var runtimeErr *vm.RuntimeError
					if !errors.As(execErr, &runtimeErr) || genericInterfaceCastExceptionType(runtimeErr.Type) != "System.AssertException" {
						t.Fatalf("observation assertion missing: %v", execErr)
					}
					got := observation(t, runtimeErr.ExceptionMessage())
					if previous, exists := unique[row.Method]; exists {
						if row.Method != "PL_STRING_CONTROL" && row.Method != "PL_INTEGER_CONTROL" && row.Method != "PL_RECORD_CONTROL" {
							t.Fatal("unexpected duplicate native row")
						}
						if !reflect.DeepEqual(previous, want) {
							t.Fatal("duplicated control payload differs")
						}
					} else {
						unique[row.Method] = want
					}

					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native stages %s; Glade stages %s", row.Native, runtimeErr.ExceptionMessage())
					}
				})
			}
		})
	}
	if rows != 23 || rows != data.NativeRowCount || len(unique) != 20 || len(unique) != data.UniqueRowCount || len(data.Captures) != 2 {
		t.Fatalf("native coverage differs: observations=%d unique=%d", rows, len(unique))
	}
}

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

func TestRawListAliasNativeConformance(t *testing.T) {
	var data struct {
		Helper    string `json:"sourceHelperApi"`
		Caller    string `json:"sourceCallerApi"`
		Transport string `json:"transportApi"`
		Route     string `json:"route"`
		Namespace string `json:"namespaceContext"`
		Known     int    `json:"knownRowCount"`
		Pending   string `json:"pendingCoverage"`
		Setup     struct {
			ID                                 string `json:"deploymentId"`
			CheckOnly, Done, MetadataUnchanged bool
			Reason                             string
			ComponentFailures                  []json.RawMessage
		} `json:"setupRejected"`
		Captures []struct {
			Deployment struct {
				ID              string
				Done, CheckOnly bool
				Status          string
				Errors          int `json:"numberComponentErrors"`
				TestErrors      int `json:"numberTestErrors"`
				Completed       int `json:"numberTestsCompleted"`
				Total           int `json:"numberTestsTotal"`
			}
			Sources           map[string]struct{ Source, SHA256 string }
			MetadataUnchanged bool
			RawSHA256         map[string]string
			Rows              []struct {
				Method        string
				NativeFailure struct{ Message string }
				Native        json.RawMessage
			}
			ExcludedRows []struct {
				Method, Reason string
				Native         json.RawMessage
			}
		}
	}
	raw, err := os.ReadFile("testdata/conformance/raw_list_alias_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Helper != "61.0" || data.Caller != "61.0" || data.Transport != "61.0" || data.Route != "isTest" || data.Namespace != "none" || data.Known != 3 || data.Pending == "" || len(data.Captures) != 2 {
		t.Fatal("native scope differs")
	}
	if data.Setup.ID == "" || !data.Setup.CheckOnly || !data.Setup.Done || !data.Setup.MetadataUnchanged || data.Setup.Reason == "" || len(data.Setup.ComponentFailures) == 0 {
		t.Fatal("compile-rejected setup carry missing")
	}
	decode := func(t *testing.T, message string) map[string]any {
		t.Helper()
		prefix, payload, ok := strings.Cut(message, ", Actual: ")
		if !ok || !strings.Contains(prefix, "RAW_LIST_OBSERVATION") {
			t.Fatalf("observation transport: %s", message)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(payload), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	seen := map[string]bool{}
	excluded := 0
	for _, capture := range data.Captures {
		d := capture.Deployment
		if d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.Errors != 0 || d.Completed != 0 || d.TestErrors != d.Total || d.Total != len(capture.Rows)+len(capture.ExcludedRows) || !capture.MetadataUnchanged {
			t.Fatal("incomplete native deployment")
		}
		for _, name := range []string{"deploy.json", "org.tsv", "SOURCE-DIGESTS.json", "RAW-DIGESTS.json", "ORG-RUN.txt", "errors.json"} {
			hash, err := hex.DecodeString(capture.RawSHA256[name])
			if err != nil || len(hash) != sha256.Size {
				t.Fatalf("missing raw digest %s", name)
			}
		}
		for _, row := range capture.ExcludedRows {
			if row.Method != "PARSED_LIST_OBJECT" || row.Reason == "" || len(row.Native) == 0 {
				t.Fatal("excluded fixture-error row missing")
			}
			excluded++
		}
		t.Run(d.ID, func(t *testing.T) {
			root := t.TempDir()
			for name, source := range capture.Sources {
				sum := sha256.Sum256([]byte(source.Source))
				if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 {
					t.Fatal("captured source differs")
				}
				writeFile(t, filepath.Join(root, name), source.Source)
			}
			var project struct {
				API string `json:"sourceApiVersion"`
			}
			if err := json.Unmarshal([]byte(capture.Sources["sfdx-project.json"].Source), &project); err != nil || project.API != data.Caller {
				t.Fatal("project source pin differs")
			}
			for _, name := range []string{"RawListView61", "RawListObservation61"} {
				var metadata struct {
					API string `xml:"apiVersion"`
				}
				if err := xml.Unmarshal([]byte(capture.Sources["force-app/main/default/classes/"+name+".cls-meta.xml"].Source), &metadata); err != nil || metadata.API != data.Helper {
					t.Fatal("class source pin differs")
				}
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = data.Caller
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: []string{"RawListObservation61"}})
			if len(cases) != d.Total {
				t.Fatal("native method discovery differs")
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			for _, row := range capture.Rows {
				t.Run(row.Method, func(t *testing.T) {
					if seen[row.Method] {
						t.Fatal("duplicate supported row")
					}
					seen[row.Method] = true
					var want map[string]any
					if err := json.Unmarshal(row.Native, &want); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(want, decode(t, row.NativeFailure.Message)) {
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
					_, execErr := machine.ExecuteInClass(programs[key], "RawListObservation61")
					var runtimeErr *vm.RuntimeError
					if !errors.As(execErr, &runtimeErr) || genericInterfaceCastExceptionType(runtimeErr.Type) != "System.AssertException" {
						t.Fatalf("observation assertion missing: %v", execErr)
					}
					if got := decode(t, runtimeErr.ExceptionMessage()); !reflect.DeepEqual(got, want) {
						t.Fatalf("native %s; Glade %s", row.Native, runtimeErr.ExceptionMessage())
					}
				})
			}
		})
	}
	if len(seen) != 3 || !seen["RAW_LIST_OBJECT"] || !seen["PARSED_LIST_OBJECT"] || !seen["OTHER_TARGETS"] || excluded != 1 {
		t.Fatal("supported/carry row coverage differs")
	}
}

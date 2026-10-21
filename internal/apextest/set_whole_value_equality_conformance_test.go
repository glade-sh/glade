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

func TestWholeSetValueEqualityNativeConformance(t *testing.T) {
	var data struct {
		SourceAPIs                                                         map[string]string `json:"sourceApis"`
		ProjectAPI, ProjectNamespace, TransportAPI, Route, PendingCoverage string
		KnownRowCount                                                      int
		Deployment                                                         struct {
			ID, Status                                                                      string
			Done, CheckOnly                                                                 bool
			NumberComponentErrors, NumberTestErrors, NumberTestsCompleted, NumberTestsTotal int
		}
		MetadataUnchanged bool
		RawSHA256         map[string]string
		Sources           map[string]struct{ Source, SHA256 string }
		Rows              []struct {
			Method, NativeFailureMessage string
			Native                       json.RawMessage
		}
	}
	raw, err := os.ReadFile("testdata/conformance/set_whole_value_equality.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data.SourceAPIs, map[string]string{"CensusKey": "50.0", "LookupPort": "48.0", "EqualityProvider": "51.0", "SetProviderProof": "51.0"}) || data.ProjectAPI != "51.0" || data.ProjectNamespace != "probe" || data.TransportAPI != "62.0" || data.Route != "isTest" || data.KnownRowCount != 4 || data.PendingCoverage == "" {
		t.Fatal("captured source/route scope differs")
	}
	d := data.Deployment
	if d.ID == "" || !d.Done || !d.CheckOnly || d.Status != "Failed" || d.NumberComponentErrors != 0 || d.NumberTestErrors != 4 || d.NumberTestsCompleted != 0 || d.NumberTestsTotal != 4 || len(data.Rows) != 4 || !data.MetadataUnchanged {
		t.Fatal("incomplete native observation transport")
	}
	for _, name := range []string{"deploy.json", "org.tsv", "SOURCE-DIGESTS.json", "RAW-DIGESTS.json", "ORG-RUN.txt", "errors.json"} {
		hash, err := hex.DecodeString(data.RawSHA256[name])
		if err != nil || len(hash) != sha256.Size {
			t.Fatalf("missing raw digest %s", name)
		}
	}
	root := t.TempDir()
	for name, source := range data.Sources {
		sum := sha256.Sum256([]byte(source.Source))
		if filepath.IsAbs(name) || strings.Contains(name, "..") || source.Source == "" || hex.EncodeToString(sum[:]) != source.SHA256 {
			t.Fatal("captured source differs")
		}
		writeFile(t, filepath.Join(root, name), source.Source)
	}
	var project struct{ SourceAPIVersion, Namespace string }
	if err := json.Unmarshal([]byte(data.Sources["sfdx-project.json"].Source), &project); err != nil || project.SourceAPIVersion != data.ProjectAPI || project.Namespace != data.ProjectNamespace {
		t.Fatal("project scope differs")
	}
	for name, api := range data.SourceAPIs {
		var metadata struct {
			API string `xml:"apiVersion"`
		}
		if err := xml.Unmarshal([]byte(data.Sources["force-app/main/default/classes/"+name+".cls-meta.xml"].Source), &metadata); err != nil || metadata.API != api {
			t.Fatal("class source pin differs")
		}
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native compiled; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = data.ProjectAPI
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	cases := Discover(index, Options{SelectedClasses: []string{"SetProviderProof"}})
	if len(cases) != 4 {
		t.Fatal("observation methods differ")
	}
	methods, methodErrors := compileTestMethodsForIndex(&index, cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	decode := func(t *testing.T, message string) map[string]any {
		t.Helper()
		prefix, payload, ok := strings.Cut(message, ", Actual: ")
		if !ok || !strings.Contains(prefix, "SET_EQ_OBSERVATION") {
			t.Fatalf("observation envelope differs: %s", message)
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		t.Run(row.Method, func(t *testing.T) {
			if seen[row.Method] {
				t.Fatal("duplicate observation")
			}
			seen[row.Method] = true
			var want map[string]any
			if err := json.Unmarshal(row.Native, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(want, decode(t, row.NativeFailureMessage)) {
				t.Fatal("native payload/envelope differ")
			}
			key := ""
			class := ""
			for _, c := range cases {
				if c.MethodName == row.Method {
					key = testCaseKey(c)
					class = c.ClassName
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
			_, execErr := machine.ExecuteInClass(programs[key], class)
			var runtimeErr *vm.RuntimeError
			if !errors.As(execErr, &runtimeErr) || stubNamespaceExceptionType(runtimeErr.Type) != "System.AssertException" {
				t.Fatalf("observation assertion missing: %v", execErr)
			}
			if got := decode(t, runtimeErr.ExceptionMessage()); !reflect.DeepEqual(got, want) {
				t.Fatalf("native %s; Glade %s", row.Native, runtimeErr.ExceptionMessage())
			}
		})
	}
	for _, name := range []string{"freshEquivalent", "sameInstance", "distinctValue", "providerEquivalent"} {
		if !seen[name] {
			t.Fatalf("missing observation %s", name)
		}
	}
}

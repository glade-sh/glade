package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStaticResourceNamedResponseOrgConformance(t *testing.T) {
	var data struct {
		APIs                  []string
		Route, ResourceName   string
		Resource              string
		Classes, SourceSHA256 map[string]string
		Cells                 []struct {
			API, ClassName string
			Native         json.RawMessage
		}
		Carries []struct {
			Count  int
			Reason string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/static_resource_named_response/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIs, ",") != "62.0,66.0,67.0,68.0" || data.Route != "istest" || len(data.Cells) != 16 || len(data.Classes) != 4 {
		t.Fatal("incomplete captured response cells")
	}
	for name, source := range data.Classes {
		sum := sha256.Sum256([]byte(source))
		if hex.EncodeToString(sum[:]) != data.SourceSHA256[name] {
			t.Fatalf("captured source changed: %s", name)
		}
	}
	for _, carry := range data.Carries {
		if carry.Count != 12 || carry.Reason == "" {
			t.Fatal("missing anonymous carry reason")
		}
		t.Logf("%d native cells carried: %s", carry.Count, carry.Reason)
	}
	for _, cell := range data.Cells {
		t.Run(cell.API+"/"+cell.ClassName, func(t *testing.T) {
			index := nativeBoundaryIndex(t, cell.API, data.Classes, nil)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("captured source rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{cell.ClassName}})
			if len(cases) != 1 {
				t.Fatal("expected one observation")
			}
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatalf("compilation: %v %v", methodErrors[key], programErrors[key])
			}
			org := orgFromIndex(index)
			org.Metadata.StaticResources = []storage.StaticResourceMetadata{{Name: data.ResourceName, Content: data.Resource, ContentType: "text/plain"}}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			machine.EnableTestContext()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			_, execErr := machine.ExecuteInClass(programs[key], cell.ClassName)
			var fault *vm.RuntimeError
			if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
				t.Fatalf("missing native observation: %v", execErr)
			}
			_, text, ok := strings.Cut(fault.ExceptionMessage(), "RESOURCE_REPLY_OBSERVATION|")
			if !ok {
				t.Fatalf("missing native payload: %v", execErr)
			}
			var got, want any
			if err := json.Unmarshal([]byte(text), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(cell.Native, &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s\nlocal=%s", cell.Native, text)
			}
		})
	}
}

package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Legal Apex observations are separate from duplicate registration and
// dependency-overlay loader controls: native Apex has no such source inventory.
func TestConstructorInvocationNativeConformance(t *testing.T) {
	var data struct {
		NativeFilesSHA256 map[string]string `json:"nativeFilesSha256"`
		Rows              []struct {
			API, Route, ID, Method, Anonymous string
			Classes                           map[string]string
			TestClass                         string
			Native                            json.RawMessage
		}
		Rejections []struct {
			API, Message string
			Source       string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/constructor_invocation_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 18 || len(data.Rejections) != 3 {
		t.Fatal("incomplete constructor observations")
	}
	if len(data.NativeFilesSHA256) == 0 {
		t.Fatal("missing native evidence binding")
	}
	for name, expected := range data.NativeFilesSHA256 {
		raw, err := os.ReadFile("testdata/native-capture/44626d86c3f1/" + name)
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		if hex.EncodeToString(digest[:]) != expected {
			t.Fatalf("native evidence changed: %s", name)
		}
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		key := row.API + "/" + row.Route + "/" + row.ID
		t.Run(key, func(t *testing.T) {
			if seen[key] {
				t.Fatal("duplicate cell")
			}
			seen[key] = true
			if row.API != "62.0" && row.API != "67.0" && row.API != "68.0" {
				t.Fatal("unobserved API")
			}
			if row.Route == "isTest" {
				source := nativeBoundaryRoute{ID: row.ID, Classes: row.Classes, Tests: []string{row.TestClass}, TestMethod: row.Method}
				residualObservationObserveExact(t, row.API, row.Route, source, nativeObservationExpected{Outcome: "runtime", Value: row.Native})
				return
			}
			if row.Route != "anonymous" {
				t.Fatal("unobserved route")
			}
			index := nativeBoundaryIndex(t, row.API, row.Classes, nil)
			analysis := sema.Analyze(index)
			anonymous := sema.AnalyzeAnonymous(index, row.Anonymous, row.API)
			if index.HasErrors() || analysis.HasErrors() || anonymous.HasErrors() {
				t.Fatalf("native accepted; diagnostics=%v %v %v", index.Diagnostics, analysis.Diagnostics, anonymous.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: row.API})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			marker := "NATIVE_RESULT|" + row.ID + "|"
			var payload string
			for _, line := range result.Debug {
				if strings.HasPrefix(line, marker) {
					if payload != "" {
						t.Fatal("duplicate observation")
					}
					payload = strings.TrimPrefix(line, marker)
				}
			}
			var got, want map[string]any
			if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal(row.Native, &want) != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s local=%s", row.Native, payload)
			}
		})
	}
	for _, row := range data.Rejections {
		// A deployment rejection is evidence about an invalid declaration, not
		// evidence for choosing one of two VM-registered source implementations.
		if row.Source == "" || !strings.Contains(row.Message, "Method already defined: <init>") {
			t.Fatal("missing duplicate declaration rejection")
		}
		t.Logf("native API%s duplicate declaration rejected: %s", row.API, row.Message)
	}
}

package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCreatedDateTieNativeConformance(t *testing.T) {
	const root = "testdata/conformance/created_date_ties"
	const class = "CedarRenewalProbeTest"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	source := read("sources/" + class + ".cls")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			path := "native/api" + strings.TrimSuffix(api, ".0") + "/"
			capture := read(path + "RUN-FACTS.txt")
			for _, fact := range []string{"api=" + api, "class_api=" + api, "context=istest", "rows=10", "native_complete=true", "deployment_success=true", fmt.Sprintf("source_sha256=%x", sha256.Sum256([]byte(source)))} {
				if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
					t.Fatal("missing native capture fact: ", fact)
				}
			}
			rows := strings.Split(strings.TrimSpace(read(path+"org.tsv")), "\n")
			if len(rows) != 11 || rows[0] != "id\tvalue" {
				t.Fatal("incomplete native rows")
			}
			native := map[string]any{}
			for _, row := range rows[1:] {
				id, payload, ok := strings.Cut(row, "\t")
				var value any
				if !ok || json.Unmarshal([]byte(payload), &value) != nil {
					t.Fatal("invalid native row: ", row)
				}
				if _, duplicate := native[id]; duplicate {
					t.Fatal("duplicate native row: ", id)
				}
				native[id] = value
			}
			var assumption struct{ Outcome, Message string }
			if err := json.Unmarshal([]byte(read(path+"assumption.json")), &assumption); err != nil {
				t.Fatal(err)
			}
			if assumption.Outcome != "Fail" || !strings.Contains(assumption.Message, "Expected: 2, Actual: 0") {
				t.Fatal("missing native failure of newest-row assumption")
			}
			index := nativeBoundaryIndex(t, api, map[string]string{class: source}, nil)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepts declarations: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			execute := func(method string) *vm.RuntimeError {
				t.Helper()
				cases := Discover(index, Options{SelectedClasses: []string{class}, SelectedMethod: method})
				if len(cases) != 1 {
					t.Fatal("unexpected selected method count: ", len(cases))
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatal(methodErrors[key], programErrors[key])
				}
				machine := runner.newMachine()
				machine.EnableTestContext()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				_, err := machine.ExecuteTestInClass(programs[key], class)
				if err == nil {
					return nil
				}
				var runtimeErr *vm.RuntimeError
				if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
					t.Fatalf("unexpected execution error: %v", err)
				}
				return runtimeErr
			}
			observed := execute("observeRows")
			if observed == nil {
				t.Fatal("missing observation transport")
			}
			_, payload, found := strings.Cut(observed.ExceptionMessage(), "PROBE|")
			got := map[string]any{}
			if !found || json.Unmarshal([]byte(payload), &got) != nil || len(got) != len(native) {
				t.Fatal("invalid observation: ", observed.ExceptionMessage())
			}
			// Timing and the relative order of equal sort keys are observations,
			// not guarantees. Assert the stable rows and each selection's counts.
			for _, id := range []string{"tied_dates_equal", "tied_selected_processed", "explicit_renewal_count", "renewal_only_lines", "renewal_only_count"} {
				if got[id] != native[id] {
					t.Errorf("%s: native %v; Glade %v", id, native[id], got[id])
				}
			}
			for _, values := range []map[string]any{native, got} {
				for _, prefix := range []string{"natural", "tied"} {
					want := float64(2)
					if values[prefix+"_selected_original"] == true {
						want = 0
					}
					if values[prefix+"_selected_count"] != want {
						t.Errorf("%s selection/count disagree: %v", prefix, values)
					}
				}
			}
			failure := execute("assumesNewest")
			if got["tied_selected_count"] == float64(0) {
				if failure == nil || !strings.Contains(failure.ExceptionMessage(), "Expected: 2, Actual: 0") {
					t.Fatal("missing corresponding assertion failure: ", failure)
				}
				t.Log("unfixed runtime assumesNewest FAIL:", failure.ExceptionMessage())
			} else if failure != nil {
				t.Fatal("renewal selection should satisfy the assertion: ", failure)
			}
			t.Log("native assumesNewest FAIL:", assumption.Message)
		})
	}
}

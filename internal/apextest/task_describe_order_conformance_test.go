package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestTaskDescribeOrderNative65Conformance(t *testing.T) {
	var data struct {
		APIVersion       string                         `json:"apiVersion"`
		ObservationCount int                            `json:"observationCount"`
		Sources          map[string]nativeBoundaryRoute `json:"sources"`
		NativeRows       []struct {
			APIVersion string         `json:"apiVersion"`
			Context    string         `json:"context"`
			ID         string         `json:"id"`
			Value      map[string]any `json:"value"`
		} `json:"nativeRows"`
		Carries []struct {
			Cell        string          `json:"cell"`
			Reason      string          `json:"reason"`
			NativeValue json.RawMessage `json:"nativeValue"`
		} `json:"carries"`
	}
	raw, err := os.ReadFile("testdata/conformance/task_describe_order65.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.APIVersion != "65.0" || data.ObservationCount != 4 || len(data.NativeRows) != 4 || len(data.Carries) != 4 {
		t.Fatal("incomplete native Task traversal packet")
	}
	for _, carry := range data.Carries {
		if carry.Cell == "" || carry.Reason == "" || len(carry.NativeValue) == 0 {
			t.Fatal("carry needs a reason and retained native value")
		}
		t.Logf("carried %s: %s; native=%s", carry.Cell, carry.Reason, carry.NativeValue)
	}
	for _, row := range data.NativeRows {
		t.Run(row.Context+"/"+row.ID, func(t *testing.T) {
			if row.APIVersion != data.APIVersion || (row.Context != "anonymous" && row.Context != "isTest") {
				t.Fatal("invalid native route")
			}
			source := data.Sources[row.ID]
			nativeBoundaryCheckSources(t, source)
			index := nativeBoundaryIndex(t, data.APIVersion, source.Classes, nil)
			org := orgFromIndex(index)
			// The captured source is API65. REST schema availability remains
			// independent of that source-level ordering observation.
			org.APIVersion = "68.0"
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			var observedText string
			marker := "NATIVE_RESULT|" + row.ID + "|"
			if row.Context == "anonymous" {
				analysis := sema.AnalyzeAnonymous(index, source.Anonymous, data.APIVersion)
				if analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: data.APIVersion})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if strings.HasPrefix(line, marker) {
						if observedText != "" {
							t.Fatal("duplicate observation")
						}
						observedText = strings.TrimPrefix(line, marker)
					}
				}
			} else {
				analysis := sema.Analyze(index)
				if analysis.HasErrors() || index.HasErrors() {
					t.Fatalf("named diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: source.TestMethod})
				if len(cases) != 1 {
					t.Fatalf("observer cases: %d", len(cases))
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("observer compile: %v %v", methodErrors[key], programErrors[key])
				}
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
				var failure *vm.RuntimeError
				if !errors.As(execErr, &failure) || failure.Type != "System.AssertException" {
					t.Fatalf("missing observation: %v", execErr)
				}
				_, observedText, _ = strings.Cut(failure.ExceptionMessage(), marker)
			}
			var observed map[string]any
			if err := json.Unmarshal([]byte(observedText), &observed); err != nil {
				t.Fatalf("missing or invalid observation: %q", observedText)
			}
			if row.ID == "T1" {
				// Target availability is carried independently, retaining all
				// other exact token metadata and traversal facts.
				for _, value := range []map[string]any{row.Value, observed} {
					tokens := value["tokens"].(map[string]any)
					delete(tokens["WhatId"].(map[string]any), "referenceTo")
				}
				captured := map[any]bool{}
				for _, value := range row.Value["references"].([]any) {
					captured[value.(map[string]any)["field"]] = true
				}
				common := []any{}
				for _, value := range observed["references"].([]any) {
					if captured[value.(map[string]any)["field"]] {
						common = append(common, value)
					} else {
						t.Logf("carried catalog-only reference metadata: %v", value)
					}
				}
				observed["references"] = common
			} else if row.ID == "T2" {
				// Compare the captured sequence on fields actually available
				// in both schemas. Type route availability is an explicit carry;
				// its ordinal is asserted wherever both schemas expose it.
				native := row.Value["fieldNames"].([]any)
				local := observed["fieldNames"].([]any)
				filter := func(values, available []any) []any {
					set := map[any]bool{}
					for _, name := range available {
						set[name] = true
					}
					out := []any{}
					for _, name := range values {
						if set[name] {
							out = append(out, name)
						}
					}
					return out
				}
				row.Value = map[string]any{"fieldNames": filter(native, local)}
				observed = map[string]any{"fieldNames": filter(local, native)}
			} else {
				t.Fatalf("unknown row: %s", row.ID)
			}
			if !reflect.DeepEqual(row.Value, observed) {
				t.Fatalf("native common cells=%v; local=%v", row.Value, observed)
			}
		})
	}
}

package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Native source, request and first-describe controls are in
// ../dml/testdata/locale_sid_key/cross-version/org.tsv.
func TestNativeLocaleDescribeMixedRoutes(t *testing.T) {
	fixture := filepath.Join("..", "dml", "testdata", "locale_sid_key", "cross-version")
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 27 || !reflect.DeepEqual(rows[0], []string{"route", "transport_api", "first_source_api", "second_source_api", "observation"}) {
		t.Fatalf("unexpected cross-version fixture shape: %d rows", len(rows))
	}
	intermediate := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 5 {
			t.Fatalf("bad cross-version row: %q", row)
		}
		if row[2] == "63.0" || row[2] == "64.0" || row[2] == "65.0" || row[2] == "66.0" {
			var got map[string]any
			if err := json.Unmarshal([]byte(row[4]), &got); err != nil {
				t.Fatal(err)
			}
			if row[0] != "anonymous" || row[1] != row[2] || row[3] != "" || got["count"] != float64(279) || got["en_BR"] != false || got["en_MX"] != false || got["en_SA"] != false || intermediate[row[2]] {
				t.Fatalf("invalid intermediate control: %q", row)
			}
			intermediate[row[2]] = true
		}
	}
	if len(intermediate) != 4 {
		t.Fatalf("missing intermediate controls: %v", intermediate)
	}
	index := loadTestIndex(t, filepath.Join(fixture, "project"))
	tests := Discover(index, Options{SelectedClasses: []string{"LocaleSingleTest", "LocaleWarmTest"}})
	methods, methodErrs := compileTestMethodsForIndex(&index, tests)
	programs, programErrs := compileTestInvokePrograms(tests)
	byName := map[string]TestCase{}
	for _, c := range tests {
		byName[c.ClassName+"."+c.MethodName] = c
	}
	for _, row := range rows[1:] {
		route, transport, first, second := row[0], row[1], row[2], row[3]
		if first != "62.0" && first != "67.0" && first != "68.0" {
			continue
		}
		name := strings.TrimSuffix(first, ".0")
		if second != "" {
			name += "-" + strings.TrimSuffix(second, ".0")
		}
		t.Run(route+"/rest="+transport+"/"+name, func(t *testing.T) {
			org := orgFromIndex(index)
			org.APIVersion = transport
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			var runErr error
			marker := "LOCALE_SINGLE|"
			if second != "" {
				marker = "LOCALE_WARM|"
			}
			if route == "anonymous" {
				fileName := "source" + strings.TrimSuffix(first, ".0") + ".apex"
				if second != "" {
					fileName = "warm-" + strings.TrimSuffix(first, ".0") + "-" + strings.TrimSuffix(second, ".0") + ".apex"
				}
				source, err := os.ReadFile(filepath.Join(fixture, "project", fileName))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: transport})
				if err != nil {
					t.Fatal(err)
				}
				_, runErr = machine.Execute(program)
			} else {
				keyName := "LocaleSingleTest.source" + strings.TrimSuffix(first, ".0")
				if second != "" {
					keyName = "LocaleWarmTest.first" + strings.TrimSuffix(first, ".0")
				}
				c, ok := byName[keyName]
				if !ok {
					t.Fatalf("missing method %s", keyName)
				}
				key := testCaseKey(c)
				if methodErrs[key] != nil || programErrs[key] != nil {
					t.Fatalf("compile: %v %v", methodErrs[key], programErrs[key])
				}
				machine.EnableTestContext()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				_, runErr = machine.ExecuteTestInClass(programs[key], c.ClassName)
			}
			var runtimeErr *vm.RuntimeError
			if !errors.As(runErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
				t.Fatalf("native observation missing: %v", runErr)
			}
			_, payload, ok := strings.Cut(runtimeErr.ExceptionMessage(), marker)
			if !ok {
				t.Fatalf("missing %s: %s", marker, runtimeErr.ExceptionMessage())
			}
			var got, want any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native %s, Glade %s", row[4], fmt.Sprint(got))
			}
		})
	}
}

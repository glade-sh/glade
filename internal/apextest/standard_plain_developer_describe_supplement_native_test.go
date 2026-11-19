package apextest

import (
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/standard_plain_developer_describe_supplement/org.tsv retains source row numbers.
func TestStandardPlainDeveloperDescribeSupplementNative(t *testing.T) {
	dir := filepath.Join("testdata", "standard_plain_developer_describe_supplement")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 31 {
		t.Fatalf("native rows: %d", len(rows)-1)
	}
	for _, api := range []string{"62.0", "67.0"} {
		suffix := strings.TrimSuffix(api, ".0")
		classes := map[string]string{}
		for _, prefix := range []string{"StandardProbe", "StandardTest"} {
			name := prefix + suffix
			b, err := os.ReadFile(filepath.Join(dir, "classes", name+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			classes[name] = string(b)
		}
		classes["StandardResultException"] = "public class StandardResultException extends Exception {}"
		index := standardDescribeDeclaredBoundaryIndex(t, api, classes, nil)
		if index.HasErrors() {
			t.Fatalf("index: %v", index.Diagnostics)
		}
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		for _, row := range rows[1:] {
			if row[1] != api {
				continue
			}
			row := row
			t.Run(api+"/"+row[2]+"/"+row[0], func(t *testing.T) {
				machine := runner.newMachine()
				var got string
				if row[2] == "anonymous" {
					source, err := os.ReadFile(filepath.Join(dir, "anonymous", row[0]+suffix+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					p, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(p)
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) {
						t.Fatalf("expected exception: %v", err)
					}
					got = fault.ExceptionMessage()
				} else {
					cases := Discover(index, Options{SelectedClasses: []string{"StandardTest" + suffix}, SelectedMethod: row[0]})
					if len(cases) != 1 {
						t.Fatalf("discovery %d", len(cases))
					}
					methods, me := compileTestMethods(cases)
					programs, pe := compileTestInvokePrograms(cases)
					key := testCaseKey(cases[0])
					if me[key] != nil || pe[key] != nil {
						t.Fatalf("compile %v %v", me[key], pe[key])
					}
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("expected assertion: %v", err)
					}
					var found bool
					_, got, found = strings.Cut(fault.ExceptionMessage(), "Actual: ")
					if !found {
						t.Fatalf("missing actual: %v", err)
					}
				}
				if got != row[4] {
					t.Errorf("native row %s\ngot %s\nwant %s", row[0], got, row[4])
				}
			})
		}
	}
}

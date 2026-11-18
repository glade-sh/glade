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

// NATIVE testdata/standard_plain_developer_describe/org.tsv retains source row numbers.
func TestStandardPlainDeveloperDescribeNative(t *testing.T) {
	dir := filepath.Join("testdata", "standard_plain_developer_describe")
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
	if len(rows) != 21 {
		t.Fatalf("native rows: %d", len(rows)-1)
	}
	for _, api := range []string{"62.0", "67.0"} {
		suffix := strings.TrimSuffix(api, ".0")
		classes := map[string]string{}
		for _, prefix := range []string{"SchemaProbe", "SchemaTest"} {
			name := prefix + suffix
			b, err := os.ReadFile(filepath.Join(dir, "classes", name+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			classes[name] = string(b)
		}
		index := standardDescribeDeclaredBoundaryIndex(t, api, classes, nil)
		if index.HasErrors() {
			t.Fatalf("index: %v", index.Diagnostics)
		}
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		for _, row := range rows[1:] {
			if row[2] != api {
				continue
			}
			row := row
			t.Run(api+"/"+row[3]+"/"+row[1], func(t *testing.T) {
				machine := runner.newMachine()
				var got string
				if row[3] == "anonymous" {
					parts := strings.Split(row[1], "_")
					object := map[string]string{"account": "Account", "case": "Case", "contact": "Contact", "lead": "Lead"}[parts[0]]
					call := "SchemaProbe" + suffix + ".selected('" + object + "','" + parts[1] + "')"
					if row[1] == "account_p1" {
						call = "SchemaProbe" + suffix + ".names('Account',1)"
					}
					p, err := vm.CompileAnonymousWithOptions("String observed="+call+";", vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err = machine.Execute(p); err != nil {
						t.Fatal(err)
					}
					got = machine.Globals["observed"].Text
				} else {
					cases := Discover(index, Options{SelectedClasses: []string{"SchemaTest" + suffix}, SelectedMethod: row[1]})
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

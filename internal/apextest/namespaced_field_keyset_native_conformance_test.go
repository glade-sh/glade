package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE field key-set text testdata/conformance/namespaced_field_keyset/org.tsv:2 anonymous 62.0
// NATIVE field key-set text testdata/conformance/namespaced_field_keyset/org.tsv:3 isTest 62.0
// NATIVE field key-set text testdata/conformance/namespaced_field_keyset/org.tsv:4 anonymous 67.0
// NATIVE field key-set text testdata/conformance/namespaced_field_keyset/org.tsv:5 isTest 67.0
// NATIVE repeat testdata/conformance/namespaced_field_keyset/org.tsv:6 anonymous 62.0
// NATIVE repeat testdata/conformance/namespaced_field_keyset/org.tsv:7 isTest 62.0
// NATIVE repeat testdata/conformance/namespaced_field_keyset/org.tsv:8 anonymous 67.0
// NATIVE repeat testdata/conformance/namespaced_field_keyset/org.tsv:9 isTest 67.0
func TestNamespacedFieldKeySetNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "namespaced_field_keyset")
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
	if len(rows) != 9 || strings.Join(rows[0], ",") != "sample,source_api,route,observation" {
		t.Fatal("incomplete native matrix")
	}
	caseFile, err := os.Open(filepath.Join(dir, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer caseFile.Close()
	caseReader := csv.NewReader(caseFile)
	caseReader.Comma = '\t'
	cases, err := caseReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 5 || strings.Join(cases[0], ",") != "source_api,route,source,class" {
		t.Fatal("incomplete case matrix")
	}
	caseByRoute := make(map[string][]string)
	for _, c := range cases[1:] {
		if len(c) != 4 {
			t.Fatalf("invalid case: %v", c)
		}
		key := c[0] + "/" + c[1]
		if _, duplicate := caseByRoute[key]; duplicate {
			t.Fatalf("duplicate case: %s", key)
		}
		caseByRoute[key] = c
	}
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatalf("invalid row: %v", row)
		}
		sample, api, route, want := row[0], row[1], row[2], row[3]
		key := sample + "/" + api + "/" + route
		if seen[key] || (sample != "first" && sample != "repeat") || (api != "62.0" && api != "67.0") || (route != "anonymous" && route != "isTest") {
			t.Fatalf("invalid cell %s", key)
		}
		seen[key] = true
		c, ok := caseByRoute[api+"/"+route]
		if !ok {
			t.Fatalf("missing case for %s", key)
		}
		t.Run(key, func(t *testing.T) {
			root := filepath.Join(dir, "project")
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			var got string
			if route == "anonymous" {
				source, err := os.ReadFile(filepath.Join(dir, c[2]))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				assignCapturedPermissionSets(t, &org, configuredTestPermissionSets(t, root)...)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, value, ok := strings.Cut(line, "OBS|"); ok {
						if got != "" {
							t.Fatal("duplicate observation")
						}
						got = value
					}
				}
			} else {
				run := Run(index, Options{SelectedClasses: []string{c[3]}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("unexpected test results: %+v", run.Summary())
				}
				problem := run.Suites[0].Cases[0].Problem
				if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
					t.Fatalf("missing assertion transport: %+v", problem)
				}
				_, got, _ = strings.Cut(problem.Message, "Actual: ")
			}
			if got != want {
				t.Fatalf("native=%q; Glade=%q", want, got)
			}
		})
	}
}

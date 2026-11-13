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

// Replays the native dynamic-bind scope rows (anonymous and @IsTest routes at
// API 62.0 and 67.0) against the shared runner.
func TestDynamicBindScopePathsNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/dynamic_bind_scope_paths"
	readTSV := func(name string) [][]string {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		r.LazyQuotes = true
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows[1:]
	}
	files := map[string]string{}
	for _, name := range []string{"cases.tsv", "cases_supplementary.tsv"} {
		for _, row := range readTSV(name) {
			files[row[0]] = row[2]
		}
	}
	classDir := filepath.Join(dir, "project", "force-app", "main", "default", "classes")
	rows := readTSV("org.tsv")
	if len(rows) != 200 {
		t.Fatalf("native rows: %d", len(rows))
	}
	for _, row := range rows {
		id, api, route, outcome := row[0], row[1], row[2], row[5]
		t.Run(id+"/"+api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			classes := filepath.Join(root, "force-app", "main", "default", "classes")
			entries, err := os.ReadDir(classDir)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if !strings.HasSuffix(e.Name(), ".cls") {
					continue
				}
				src, err := os.ReadFile(filepath.Join(classDir, e.Name()))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(classes, e.Name()), string(src))
				writeFile(t, filepath.Join(classes, e.Name()+"-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			var got string
			if route == "anonymous" {
				src, err := os.ReadFile(filepath.Join(dir, "cases", files[id]))
				if err != nil {
					t.Fatal(err)
				}
				body := string(src)
				if result := sema.AnalyzeAnonymous(index, body, api); result.HasErrors() {
					t.Fatal(result.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, body, api)})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, v, ok := strings.Cut(line, "R|"); ok {
						got = "R|" + v
					}
				}
			} else {
				run := Run(index, Options{SelectedClasses: []string{files[id]}, SelectedMethod: "run", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("test runner: %+v", run.Summary())
				}
				c := run.Suites[0].Cases[0]
				if c.Problem == nil {
					got = "run=Pass"
				} else {
					got = "run=Fail " + c.Problem.Type + ": " + c.Problem.Message
				}
			}
			if got != outcome {
				t.Fatalf("native=%q\nGlade =%q", outcome, got)
			}
		})
	}
}

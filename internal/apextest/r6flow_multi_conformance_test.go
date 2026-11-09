package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestR6FlowMultiNativeConformance(t *testing.T) {
	fixture := filepath.Join("testdata", "conformance", "r6flow_multi")
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 13 {
		t.Fatalf("rows=%d error=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			root := t.TempDir()
			copyDiskRuntimeScopeProject(t, filepath.Join(fixture, "api"+strings.TrimSuffix(row[0], ".0")), root)
			index := loadTestIndex(t, root)
			if row[1] == "isTest" {
				method := row[2]
				run := Run(index, Options{SelectedClasses: []string{"R6MultiTest"}, SelectedMethod: method, Parallelism: 1, NoDiskCache: true})
				if run.Summary().Total != 1 || !run.Summary().OK() {
					t.Fatalf("%s", firstRunProblem(run))
				}
				return
			}
			name, setup := "R6 None", ""
			if row[2] == "one" {
				name = "R6 One"
				setup = "insert new Account(Name='R6 One');"
			}
			if row[2] == "two" {
				name = "R6 Two"
				setup = "insert new List<Account>{new Account(Name='R6 Two'),new Account(Name='R6 Two')};"
			}
			program, err := vm.CompileAnonymousWithOptions(setup+"System.debug('R6='+R6MultiProbe.observe('"+name+"'));", vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range result.Debug {
				if line == "R6="+row[3] {
					return
				}
			}
			t.Fatalf("native=%s local debug=%v", row[3], result.Debug)
		})
	}
}

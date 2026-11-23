package apextest

import (
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowRelationshipFilterFaultNativeConformance(t *testing.T) {
	fixture := filepath.Join("testdata", "conformance", "flow_relationship_filter_fault")
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows=%d: %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[1]+"/"+row[2], func(t *testing.T) {
			root := filepath.Join(fixture, "api"+strings.TrimSuffix(row[1], ".0"))
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("fixture diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			var message string
			if row[2] == "anonymous" {
				org := capturedFixtureOrg(t, index, row[1])
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions("System.assert(false, 'OBS=' + RelationshipFilterCase.run());", vm.CompileOptions{APIVersion: row[1]})
				if err != nil {
					t.Fatal(err)
				}
				_, err = runner.newMachine().Execute(program)
				var runtimeErr *vm.RuntimeError
				if !errors.As(err, &runtimeErr) || flowRefsExceptionType(runtimeErr.Type) != "System.AssertException" {
					t.Fatalf("expected observation, got %v", err)
				}
				message = runtimeErr.ExceptionMessage()
			} else if row[2] == "istest" {
				result := Run(index, Options{SelectedClasses: []string{"RelationshipFilterTest"}, SelectedMethod: "probe", NoDiskCache: true, Parallelism: 1})
				if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
					t.Fatalf("unexpected selection: %#v", result.Summary())
				}
				problem := result.Suites[0].Cases[0].Problem
				if problem == nil || flowRefsExceptionType(problem.Type) != "System.AssertException" {
					t.Fatalf("expected observation, got %#v", problem)
				}
				message = problem.Message
			} else {
				t.Fatalf("unknown route %q", row[2])
			}
			_, want, ok := strings.Cut(row[5], "OBS=")
			if !ok {
				t.Fatalf("missing native observation: %q", row[5])
			}
			_, got, ok := strings.Cut(message, "OBS=")
			if !ok || got != want {
				t.Fatalf("native %q, Glade %q", want, message)
			}
		})
	}
}

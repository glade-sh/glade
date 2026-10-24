package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

func TestThrowSuccessorProjectAnonymousAndIsTestNativeConformance(t *testing.T) {
	const dir = "../sema/testdata/throw_successor"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Body, Name string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{}
	names := map[string]string{}
	for _, c := range cases {
		if c.ID == "" || c.Body == "" || bodies[c.ID] != "" {
			t.Fatal("invalid source case")
		}
		bodies[c.ID] = c.Body
		names[c.ID] = c.Name
	}
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(cases) != 8 || len(rows) != 1+6*len(cases) || strings.Join(rows[0], ",") != "api,route,case,outcome,diagnostic,runtime_error" {
		t.Fatalf("incomplete native matrix: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		key := strings.Join(row[:3], "/")
		body := bodies[row[2]]
		if seen[key] || body == "" || (row[0] != "62.0" && row[0] != "67.0" && row[0] != "68.0") || (row[1] != "anonymous" && row[1] != "istest") || (row[3] != "accepted" && row[3] != "rejected") {
			t.Fatalf("invalid native row: %v", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			classes := map[string]string{}
			class := names[row[2]]
			if row[1] == "istest" {
				classes[class] = "@IsTest private class " + class + " { @IsTest static void observe() { " + body + " } }"
			}
			index := nativeBoundaryIndex(t, row[0], classes, nil)
			analysis := sema.Analyze(index)
			if row[1] == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, body, row[0])
			}
			if index.HasErrors() {
				t.Fatalf("source index: %v", index.Diagnostics)
			}
			if analysis.HasErrors() != (row[3] == "rejected") {
				t.Fatalf("native %s (%s); Glade diagnostics=%v", row[3], row[4], analysis.Diagnostics)
			}
			if row[3] == "rejected" {
				for _, d := range analysis.Diagnostics {
					if d.NativeMessage == row[4] || d.Message == row[4] {
						return
					}
				}
				t.Fatalf("native diagnostic %q; Glade diagnostics=%v", row[4], analysis.Diagnostics)
			}
			if row[1] == "istest" {
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("missing test route: %+v", run.Summary())
				}
				result := run.Suites[0].Cases[0]
				if row[5] == "-" {
					if result.Status != testreport.StatusPass {
						t.Fatalf("native passes; Glade problem=%+v", result.Problem)
					}
				} else if result.Problem == nil || strings.TrimPrefix(result.Problem.Type, "System.")+": "+result.Problem.Message != strings.TrimPrefix(row[5], "System.") {
					t.Fatalf("native runtime=%s; Glade problem=%+v", row[5], result.Problem)
				}
				return
			}
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = row[0]
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			_, execErr := runner.execute(program)
			if row[5] == "-" {
				if execErr != nil {
					t.Fatal(execErr)
				}
				return
			}
			var runtimeErr *vm.RuntimeError
			if !errors.As(execErr, &runtimeErr) || strings.TrimPrefix(runtimeErr.Type, "System.")+": "+runtimeErr.ExceptionMessage() != strings.TrimPrefix(row[5], "System.") {
				t.Fatalf("native runtime=%s; Glade=%v", row[5], execErr)
			}
		})
	}
}

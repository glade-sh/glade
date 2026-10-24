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

// Native Lead email-activity rows: email_lead_activity/org.tsv, captured
// from an owned scratch org (see ORG-RUN.txt). Each row runs anonymously and
// as an @IsTest method through the project runner, at API 62.0 and 67.0.
func TestEmailLeadActivityNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata/conformance/email_lead_activity")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 21 {
		t.Fatalf("incomplete native matrix: %v rows=%d", err, len(rows))
	}
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		project := filepath.Join(dir, "project")
		err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(project, path)
			writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "67.0", api))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		index := loadTestIndex(t, root)
		org := orgFromIndex(index)
		org.APIVersion = api
		runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows[1:] {
			if row[0] != api {
				continue
			}
			route, method, want := row[1], row[2], row[3]
			t.Run(api+"/"+route+"/"+method, func(t *testing.T) {
				var got string
				if route == "anonymous" {
					orgCopy := orgFromIndex(index)
					orgCopy.APIVersion = api
					request := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &orgCopy})
					source := "System.debug('OBS|'+EmailCap.run('" + method + "'));"
					if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
						t.Fatal(analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := request.execute(program)
					if err != nil {
						t.Fatal(err)
					}
					for _, line := range result.Debug {
						if strings.HasPrefix(line, "OBS|") {
							got = strings.TrimPrefix(line, "OBS|")
						}
					}
				} else {
					run := Run(index, Options{SelectedClasses: []string{"EmailCapTest"}, SelectedMethod: strings.ToLower(method), NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("missing observation: %+v", run.Summary())
					}
					got = run.Suites[0].Cases[0].Problem.Message
				}
				got = strings.TrimSpace(strings.TrimPrefix(got, "Assertion Failed: "))
				if got != want {
					t.Fatalf("got %q want %q", got, want)
				}
			})
		}
	}
}

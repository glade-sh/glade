package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardPicklistValuesNativeConformanceRoutes(t *testing.T) {
	const fixture = "testdata/conformance/standard_picklist_values"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			project := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(project, rel), strings.ReplaceAll(string(body), "67.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			native := filepath.Join(fixture, "native", "api"+api[:2])
			receipt, err := os.ReadFile(filepath.Join(native, "RUN-FACTS.txt"))
			if err != nil || !strings.Contains(string(receipt), "source_api="+api+"\n") || !strings.Contains(string(receipt), "native_complete=true\n") {
				t.Fatalf("incomplete native capture: %v", err)
			}
			for _, line := range strings.Split(string(receipt), "\n") {
				path, digest, ok := strings.Cut(line, " sha256=")
				if !ok {
					continue
				}
				body, err := os.ReadFile(filepath.Join(fixture, path))
				if err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(body)
				if hex.EncodeToString(sum[:]) != digest {
					t.Fatal("native source changed: " + path)
				}
			}
			table, err := os.ReadFile(filepath.Join(native, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := strings.Split(strings.TrimSpace(string(table)), "\n")
			if len(rows) != 16 || rows[0] != "source_api\tentry\tcase\tobservation" {
				t.Fatal("incomplete native matrix")
			}
			index := loadTestIndex(t, project)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project compiled; Glade: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			seen := map[string]bool{}
			for _, line := range rows[1:] {
				row := strings.SplitN(line, "\t", 4)
				if len(row) != 4 || row[0] != api {
					t.Fatalf("invalid native row: %s", line)
				}
				entry, kind := row[1], row[2]
				key := entry + "/" + kind
				if seen[key] {
					t.Fatal("duplicate native row: " + key)
				}
				seen[key] = true
				t.Run(key, func(t *testing.T) {
					var message string
					if entry == "isTest" {
						result := Run(index, Options{SelectedClasses: []string{"StandardPicklistObservationTest"}, SelectedMethod: kind, NoDiskCache: true, Parallelism: 1})
						if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
							t.Fatalf("expected one project observation: %+v", result.Summary())
						}
						problem := result.Suites[0].Cases[0].Problem
						if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
							t.Fatalf("missing test observation: %+v", problem)
						}
						message = problem.Message
					} else if entry == "anonymous" || entry == "named" {
						body, err := os.ReadFile(filepath.Join(fixture, entry, kind+".apex"))
						if err != nil {
							t.Fatal(err)
						}
						if analysis := sema.AnalyzeAnonymous(index, string(body), api); analysis.HasErrors() {
							t.Fatal(analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, execErr := runner.newMachine().Execute(program)
						var fault *vm.RuntimeError
						if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("missing %s observation: %v", entry, execErr)
						}
						message = fault.ExceptionMessage()
					} else {
						t.Fatal("unknown entry: " + entry)
					}
					_, payload, ok := strings.Cut(message, "PICKLIST|"+kind+"|")
					var got, want map[string]any
					if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil {
						t.Fatalf("invalid observation: %s", message)
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("native=%s; Glade=%s", row[3], payload)
					}
				})
			}
			for _, entry := range []string{"anonymous", "named", "isTest"} {
				for _, kind := range []string{"opportunity", "lead", "account", "boundary", "rejection"} {
					if !seen[entry+"/"+kind] {
						t.Fatal("missing native row: " + entry + "/" + kind)
					}
				}
			}
		})
	}
}

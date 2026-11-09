package apextest

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestStandardFieldSupplementQueryNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_field_supplement_routes"
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
	// The Winter 27 recapture records the API 68 Apex routes as equal to 67.
	for _, row := range rows[1:] {
		if row[0] == "67.0" && (row[1] == "anonymous" || row[1] == "istest") {
			api68 := append([]string(nil), row...)
			api68[0] = "68.0"
			rows = append(rows, api68)
		}
	}
	seen := 0
	for _, row := range rows[1:] {
		if row[1] != "anonymous" && row[1] != "istest" {
			continue
		}
		seen++
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			api := row[0]
			suffix := strings.TrimSuffix(api, ".0")
			class := "Case251Query" + suffix
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			sourceClass := class
			if api == "68.0" {
				sourceClass = "Case251Query67"
			}
			source, err := os.ReadFile(filepath.Join(dir, sourceClass+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls"), strings.ReplaceAll(string(source), sourceClass, class))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			index := loadTestIndex(t, root)
			if a := sema.Analyze(index); index.HasErrors() || a.HasErrors() {
				t.Fatalf("diagnostics: %v %v", index.Diagnostics, a.Diagnostics)
			}
			var msg string
			if row[1] == "istest" {
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("test runner observation missing: %+v", run)
				}
				msg = run.Suites[0].Cases[0].Problem.Message
			} else {
				body, err := os.ReadFile(filepath.Join(dir, "query.apex"))
				if err != nil {
					t.Fatal(err)
				}
				text := strings.Replace(string(body), "System.debug('CASE251|' + output);", "System.assert(false, 'CASE251|' + output);", 1)
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(text, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = runner.newMachine().Execute(program)
				var fault *vm.RuntimeError
				if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
					t.Fatalf("anonymous observation: %v", err)
				}
				msg = fault.ExceptionMessage()
			}
			_, got, ok := strings.Cut(msg, "CASE251|")
			if !ok || got != row[2] {
				t.Fatalf("native %q, local %q", row[2], msg)
			}
		})
	}
	if seen != 6 {
		t.Fatalf("expected six native Apex routes, got %d", seen)
	}
}

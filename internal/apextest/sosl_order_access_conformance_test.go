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

// NATIVE fixed candidates with readable and unreadable SOSL sort fields:
// testdata/conformance/sosl_order_access/org.tsv:9 (istest 62.0) and :22 (istest 67.0).
// Bare anonymous context and empty search controls are in the same table.
func TestSOSLOrderAccessConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "sosl_order_access")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 37 {
		t.Fatalf("native capture: %d rows, error %v", len(rows), err)
	}
	schema := filepath.Join("testdata", "conformance", "api67_sosl_locator_matching", "project", "force-app", "main", "default")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			metadata := filepath.Join(root, "force-app", "main", "default")
			for _, sub := range []string{"objects", "permissionsets", "profiles"} {
				if err := os.CopyFS(filepath.Join(metadata, sub), os.DirFS(filepath.Join(schema, sub))); err != nil {
					t.Fatal(err)
				}
			}
			var classes []string
			for _, name := range []string{"SearchAccess", "SearchInlineAccess", "SearchModeAccess"} {
				version := strings.TrimSuffix(api, ".0")
				class := name + version
				classes = append(classes, class)
				body, err := os.ReadFile(filepath.Join(dir, name+".cls.tmpl"))
				if err != nil {
					t.Fatal(err)
				}
				classPath := filepath.Join(metadata, "classes", class+".cls")
				writeFile(t, classPath, strings.ReplaceAll(string(body), "{{API}}", version))
				writeFile(t, classPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: classes, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != len(classes) {
				t.Fatalf("incomplete execution: %+v", run.Summary())
			}
			observed := map[string]string{}
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					if c.Problem == nil || !strings.HasPrefix(c.Problem.Message, "Assertion Failed: OBS|") {
						t.Fatalf("%s failed before observation: %+v", c.MethodName, c.Problem)
					}
					observed[c.MethodName] = strings.TrimPrefix(c.Problem.Message, "Assertion Failed: OBS|")
				}
			}
			if len(observed) != 15 {
				t.Fatalf("observed %d restricted-user methods", len(observed))
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				t.Run(row[2], func(t *testing.T) {
					actual, ok := observed[row[2]]
					if row[1] == "anonymous" {
						body, err := os.ReadFile(filepath.Join(dir, row[2]+".apex"))
						if err != nil {
							t.Fatal(err)
						}
						_, err = runAnonymousObservation(t, runner, index, string(body), api)
						var failure *vm.RuntimeError
						if !errors.As(err, &failure) {
							t.Fatalf("expected native exception, got %v", err)
						}
						typ := failure.Type
						if !strings.Contains(typ, ".") {
							typ = "System." + typ
						}
						actual, ok = typ+": "+failure.ExceptionMessage(), true
					}
					if !ok || actual != row[3] {
						t.Errorf("native %q; Glade %q", row[3], actual)
					}
				})
			}
		})
	}
}

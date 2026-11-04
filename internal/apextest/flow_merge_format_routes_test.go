package apextest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowMergeFormatNativeRoutes(t *testing.T) {
	const fixture = "../automation/testdata/flow_merge_format"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			for _, name := range []string{"Format_constant.flow-meta.xml", "Format_datetime.flow-meta.xml", "FormatProbe.cls", "FormatProbeTest.cls"} {
				b, err := os.ReadFile(filepath.Join(fixture, name))
				if err != nil {
					t.Fatal(err)
				}
				dir := "classes"
				if strings.Contains(name, ".flow-") {
					dir = "flows"
				}
				writeFile(t, filepath.Join(root, "force-app/main/default", dir, name), strings.ReplaceAll(string(b), "62.0", api))
				if dir == "classes" {
					writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+"-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				}
			}
			index := loadTestIndex(t, root)
			org, err := orgFromIndexWithError(index)
			if err != nil {
				t.Fatal(err)
			}
			org.APIVersion = api
			for _, route := range []string{"anonymous", "istest"} {
				for _, kind := range []string{"constant", "datetime"} {
					t.Run(kind, func(t *testing.T) {
						t.Run(route, func(t *testing.T) {
							want := ""
							for _, line := range strings.Split(string(rows), "\n") {
								fields := strings.Split(line, "\t")
								if len(fields) == 4 && fields[0] == api && fields[1] == route && fields[2] == kind {
									want = fields[3]
								}
							}
							if want == "" {
								t.Fatal("missing native row")
							}
							var message string
							if route == "anonymous" {
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
								program, err := vm.CompileAnonymousWithOptions(fmt.Sprintf(`System.assert(false, 'OBS|%s|'+FormatProbe.observe('%s'));`, kind, kind), vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								_, err = runner.execute(program)
								var runtimeErr *vm.RuntimeError
								if !errors.As(err, &runtimeErr) {
									t.Fatalf("expected assertion, got %v", err)
								}
								message = runtimeErr.ExceptionMessage()
							} else {
								result := Run(index, Options{SelectedClasses: []string{"FormatProbeTest"}, SelectedMethod: kind + "Test", RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
								if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem == nil {
									t.Fatalf("missing assertion: %+v", result.Summary())
								}
								message = result.Suites[0].Cases[0].Problem.Message
							}
							if kind == "datetime" {
								if !strings.Contains(message, "OBS|datetime|FIELD_CUSTOM_VALIDATION_EXCEPTION|datetime=5/2/2026, 5:00 AM") {
									t.Fatalf("incorrect datetime for the VM's fixed clock: %q", message)
								}
							} else {
								expected := "OBS|constant|FIELD_CUSTOM_VALIDATION_EXCEPTION|" + want
								if !strings.Contains(message, expected) {
									t.Fatalf("got %q; want %q", message, expected)
								}
							}
						})
					})
				}
			}
		})
	}
}

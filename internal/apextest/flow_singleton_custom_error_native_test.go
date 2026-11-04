package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowSingletonCustomErrorNativeRoutes(t *testing.T) {
	const fixture = "testdata/conformance/flow_singleton_custom_error"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, file, want string }{
			{"unconditional", "Unconditional.flow-meta.xml", "FIELD_CUSTOM_VALIDATION_EXCEPTION|plain failure"},
			{"conditional_no_match", "Conditional.flow-meta.xml", "SUCCESS"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				for _, route := range []string{"anonymous", "istest"} {
					if !strings.Contains(string(rows), api+"\t"+route+"\t"+tc.name+"\t"+tc.want) {
						t.Fatalf("missing native %s row", route)
					}
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				flow, err := os.ReadFile(filepath.Join(fixture, tc.file))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/flows/Singleton_Probe.flow-meta.xml"), strings.ReplaceAll(string(flow), "62.0", api))
				name := "control"
				if tc.name == "unconditional" {
					name = "capture"
				}
				body := fmt.Sprintf(`Database.SaveResult r=Database.insert(new Account(Name='%s'),false); System.assertEquals(%t,r.isSuccess());`, name, tc.want == "SUCCESS")
				if tc.want != "SUCCESS" {
					body += `System.assertEquals('FIELD_CUSTOM_VALIDATION_EXCEPTION',String.valueOf(r.getErrors()[0].getStatusCode())); System.assertEquals('plain failure',r.getErrors()[0].getMessage());`
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/SingletonRouteTest.cls"), `@IsTest private class SingletonRouteTest { @IsTest static void run() { `+body+` } }`)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/SingletonRouteTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				index := loadTestIndex(t, root)
				org, err := orgFromIndexWithError(index)
				if err != nil {
					t.Fatal(err)
				}
				org.APIVersion = api
				t.Run("anonymous", func(t *testing.T) {
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err = runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
				t.Run("istest", func(t *testing.T) {
					result := Run(index, Options{SelectedClasses: []string{"SingletonRouteTest"}, SelectedMethod: "run", RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
					if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem != nil {
						t.Fatalf("result: %+v", result.Summary())
					}
				})
			})
		}
	}
}

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

func TestLocatorSystemModePrecedenceNativeRoutes(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata/conformance/locator_system_precedence", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 61 {
		t.Fatalf("native observations=%d, want 60", len(rows)-1)
	}
	observed := make(map[string]string, len(rows)-1)
	for _, row := range rows[1:] {
		observed[row[0]+"/"+row[1]+"/"+row[2]] = row[3]
	}
	classify := func(observation string) string {
		switch {
		case observation == "ok":
			return "ok"
		case strings.HasPrefix(observation, "System.QueryException:"):
			return "query"
		case strings.HasPrefix(observation, "System.SecurityException: Cannot use SYSTEM_MODE"):
			return "security"
		default:
			t.Fatalf("unexpected native observation %q", observation)
			return ""
		}
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		for _, route := range []string{"account", "contact", "invalid", "missingBind", "count"} {
			{
				key := route
				namedWant := classify(observed[api+"/istest/"+key])
				anonymousWant := classify(observed[api+"/anonymous/"+key])
				t.Run(api+"/"+key, func(t *testing.T) {
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
					query := map[string]string{
						"account":     "SELECT Id FROM Account LIMIT 1",
						"contact":     "SELECT Id FROM Contact LIMIT 1",
						"invalid":     "SELECT FROM",
						"missingBind": "SELECT Id FROM Account WHERE Id = :absent",
						"count":       "SELECT COUNT() FROM Account",
					}[route]
					method := "getQueryLocatorWithBinds"
					body := func(want string) string {
						return `String outcome='ok'; try { Database.` + method + `('` + query + `',new Map<String,Object>(),AccessLevel.SYSTEM_MODE); } catch(QueryException e) { outcome='query'; } catch(SecurityException e) { outcome='security'; } System.assertEquals('` + want + `',outcome);`
					}
					namedPath := filepath.Join(root, "force-app/main/default/classes/SyntheticPrecedenceNamed.cls")
					writeFile(t, namedPath, "public class SyntheticPrecedenceNamed { public static void sample(){"+body(namedWant)+"} }")
					writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					path := filepath.Join(root, "force-app/main/default/classes/SyntheticPrecedenceTest.cls")
					writeFile(t, path, "@IsTest private class SyntheticPrecedenceTest { @IsTest static void sample(){ SyntheticPrecedenceNamed.sample(); } }")
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					index := loadTestIndex(t, root)
					analysis := sema.Analyze(index)
					anonymous := sema.AnalyzeAnonymous(index, body(anonymousWant), api)
					if index.HasErrors() || analysis.HasErrors() || anonymous.HasErrors() {
						t.Fatalf("compilation: %v %v %v", index.Diagnostics, analysis.Diagnostics, anonymous.Diagnostics)
					}
					org := orgFromIndex(index)
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					program, err := vm.CompileAnonymousWithOptions(body(anonymousWant), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					named, err := vm.CompileAnonymousWithOptions("SyntheticPrecedenceNamed.sample();", vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					for _, state := range []string{"cold", "warm"} {
						t.Run(state, func(t *testing.T) {
							if _, err := runner.execute(program); err != nil {
								t.Fatal(err)
							}
							if _, err := runner.execute(named); err != nil {
								t.Fatal(err)
							}
							run := Run(index, Options{SelectedClasses: []string{"SyntheticPrecedenceTest"}, SelectedMethod: "sample", NoDiskCache: true, Parallelism: 1})
							if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
								t.Fatalf("project runner: %+v", run.Summary())
							}
						})
					}
				})
			}
		}
	}
}

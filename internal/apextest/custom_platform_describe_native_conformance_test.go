package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCustomPlatformDescribeNativeConformanceRoutes(t *testing.T) {
	const fixture = "testdata/conformance/custom_platform_describe"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	if len(rows) != 337 {
		t.Fatal("incomplete native matrix")
	}
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/objects/LinkProbe__c/LinkProbe__c.object-meta.xml"), read("LinkProbe__c.object-meta.xml"))
			for _, class := range []string{"CustomPlatformDescribeProbe", "CustomPlatformDescribeProbeTest"} {
				path := filepath.Join(root, "force-app/main/default/classes", class+".cls")
				writeFile(t, path, read(class+".cls"))
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, typ := range index.Types {
				if typ.EffectiveAPIVersion != api {
					t.Fatalf("native source API %s; %s uses %s", api, typ.Name, typ.EffectiveAPIVersion)
				}
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			counts := newConformanceCountLog(t, "CustomPlatformDescribe", api)
			for _, route := range []string{"anonymous", "named", "istest"} {
				for _, kind := range []string{"describe", "query"} {
					t.Run(route+"/"+kind, func(t *testing.T) {
						var want []string
						for _, row := range rows[1:] {
							if row[0] != api || row[1] != route || row[2] != kind {
								continue
							}
							if kind == "describe" {
								want = append(want, strings.Join(row[3:8], "|"))
							} else if row[8] == "reject" {
								want = append(want, row[3]+"|reject|"+row[9])
							} else {
								want = append(want, row[3]+"|"+row[9])
							}
						}
						if len(want) != 28 {
							t.Fatal("incomplete native route cell")
						}
						sort.Strings(want)
						matchKind := "exact"
						if kind == "query" {
							matchKind = "category"
						}
						counts.TrackKind(t, route, matchKind, len(want))
						method, marker := "describeRows", "DESCRIBE_ROWS|"
						if kind == "query" {
							method, marker = "queryRows", "QUERY_ROWS|"
						}
						var message string
						if route == "istest" {
							run := Run(index, Options{SelectedClasses: []string{"CustomPlatformDescribeProbeTest"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
							if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
								t.Fatalf("native test selection: %+v", run.Summary())
							}
							problem := run.Suites[0].Cases[0].Problem
							if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
								t.Fatalf("missing assertion transport: %+v", problem)
							}
							message = problem.Message
						} else {
							source := read("setup.apex")
							if route == "anonymous" {
								source += read(method + ".apex")
							} else {
								source += "System.debug(LoggingLevel.ERROR,'" + marker + "'+JSON.serialize(CustomPlatformDescribeProbe." + method + "()));"
							}
							if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
								t.Fatalf("native anonymous accepted; Glade: %v", analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							result, err := base.execute(program)
							if err != nil {
								t.Fatal(err)
							}
							for _, line := range result.Debug {
								if strings.HasPrefix(line, marker) {
									message = line
								}
							}
						}
						_, payload, found := strings.Cut(message, marker)
						var got []string
						if !found || json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatalf("invalid observation: %s", message)
						}
						sort.Strings(got)
						if !reflect.DeepEqual(want, got) {
							t.Fatalf("native %s (%d entries)=%v; Glade (%d entries)=%v", kind, len(want), want, len(got), got)
						}
					})
				}
			}
		})
	}
}

package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const jsonObservationMarker = "QUARTZ|"

func runJSONRuntimeNativeRows(t *testing.T, family string, sources ...string) {
	t.Helper()
	dir := filepath.Join("testdata", "conformance", family)
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	reader := csv.NewReader(strings.NewReader(read("org.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || strings.Join(rows[0], ",") != "api,route,case,observed" {
		t.Fatal("invalid native observation header")
	}
	caseReader := csv.NewReader(strings.NewReader(read("cases.tsv")))
	caseReader.Comma = '\t'
	cases, err := caseReader.ReadAll()
	if err != nil || len(cases) < 2 || strings.Join(cases[0], ",") != "case,source" {
		t.Fatalf("invalid native case table: %v", err)
	}
	body := read("probe.apex")
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(
				`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			path := filepath.Join(root, "force-app", "main", "default")
			classMeta := "<ApexClass><apiVersion>" + api + "</apiVersion></ApexClass>"
			for _, source := range sources {
				writeFile(t, filepath.Join(path, source), read(source))
				if strings.HasSuffix(source, ".cls") {
					writeFile(t, filepath.Join(path, source+"-meta.xml"), classMeta)
				}
			}
			const testClass = "JSONRuntimeObservationTest"
			named := "@IsTest private class " + testClass + " {\n" +
				"@IsTest static void observe() {\n" + body +
				"\nSystem.assert(false, 'QUARTZ|' + JSON.serialize(rows));\n}\n}\n"
			classPath := filepath.Join(path, "classes", testClass+".cls")
			writeFile(t, classPath, named)
			writeFile(t, classPath+"-meta.xml", classMeta)
			index := loadTestIndex(t, root)
			for _, route := range []string{"anonymous", "isTest"} {
				t.Run(route, func(t *testing.T) {
					expected := map[string]json.RawMessage{}
					for _, row := range rows[1:] {
						if row[0] == api && row[1] == route {
							if _, duplicate := expected[row[2]]; duplicate {
								t.Fatalf("duplicate native observation %q", row[2])
							}
							expected[row[2]] = json.RawMessage(row[3])
						}
					}
					var observed map[string]json.RawMessage
					if route == "anonymous" {
						source := body + "\nSystem.debug('QUARTZ|' + JSON.serialize(rows));"
						if result := sema.AnalyzeAnonymous(index, source, api); result.HasErrors() {
							t.Fatal(result.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						result, err := runner.execute(program)
						if err != nil {
							t.Fatal(err)
						}
						observed = decodeJSONRuntimeRows(t, strings.Join(result.Debug, "\n"))
					} else {
						result := Run(index, Options{SelectedClasses: []string{testClass}, NoDiskCache: true, Parallelism: 1})
						if summary := result.Summary(); summary.Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
							t.Fatalf("unexpected named run: %#v", summary)
						}
						problem := result.Suites[0].Cases[0].Problem
						if problem == nil || problem.Type != "System.AssertException" {
							t.Fatalf("expected named observation assertion, got %+v", problem)
						}
						observed = decodeJSONRuntimeRows(t, problem.Message)
					}
					for _, spec := range cases[1:] {
						name := spec[0]
						t.Run(name, func(t *testing.T) {
							var want, got any
							if err := json.Unmarshal(expected[name], &want); err != nil {
								t.Fatalf("invalid or missing native observation: %v", err)
							}
							if err := json.Unmarshal(observed[name], &got); err != nil {
								t.Fatalf("invalid or missing local observation: %v", err)
							}
							if !reflect.DeepEqual(want, got) {
								t.Errorf("native %s, Glade %s", expected[name], observed[name])
							}
						})
					}
				})
			}
		})
	}
}

func decodeJSONRuntimeRows(t *testing.T, message string) map[string]json.RawMessage {
	t.Helper()
	_, payload, ok := strings.Cut(message, jsonObservationMarker)
	if !ok {
		t.Fatalf("missing observation marker: %s", message)
	}
	var rows map[string]json.RawMessage
	if err := json.NewDecoder(strings.NewReader(payload)).Decode(&rows); err != nil {
		t.Fatalf("invalid observation: %v", err)
	}
	return rows
}

func TestJSONComputedGetterNativeConformance(t *testing.T) {
	runJSONRuntimeNativeRows(t, "json_computed_getter", "classes/QuartzPayload.cls")
}

func TestJSONCustomPlatformChildrenNativeConformance(t *testing.T) {
	runJSONRuntimeNativeRows(t, "json_custom_platform_children", "objects/QuartzNote__c/QuartzNote__c.object-meta.xml")
}

func TestJSONHistoricalDatetimeNativeConformance(t *testing.T) {
	runJSONRuntimeNativeRows(t, "json_historical_datetime")
}

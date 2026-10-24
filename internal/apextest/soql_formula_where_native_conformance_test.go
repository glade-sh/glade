package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestSOQLFormulaWhereNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/soql_formula_where"
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	reader := csv.NewReader(strings.NewReader(read("org.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 241 {
		t.Fatalf("incomplete native formula matrix: %d rows, %v", len(rows), err)
	}
	want := map[string]map[string]string{}
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[1]
		if want[key] == nil {
			want[key] = map[string]string{}
		}
		if _, duplicate := want[key][row[2]]; duplicate {
			t.Fatal("duplicate native formula row")
		}
		want[key][row[2]] = row[3]
	}
	if len(want) != 6 {
		t.Fatal("expected six native API/route pairs")
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		root := t.TempDir()
		base := "force-app/main/default/"
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
		for _, name := range []string{"Start__c", "Finish__c", "StartTime__c", "FinishTime__c"} {
			path := base + "objects/Account/fields/" + name + ".field-meta.xml"
			writeFile(t, filepath.Join(root, path), read("project/"+path))
		}
		classPath := base + "classes/FormulaWhereProbe62"
		writeFile(t, filepath.Join(root, classPath+".cls"), read("project/"+classPath+".cls"))
		writeFile(t, filepath.Join(root, classPath+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
		named := strings.Replace(read("project/"+classPath+".cls"), "@IsTest private class FormulaWhereProbe62 { @IsTest static void observe()", "public class FormulaWhereNamed { public static String observe()", 1)
		named = strings.Replace(named, "System.assert(false,'OBS|'+JSON.serialize(observations));", "return JSON.serialize(observations);", 1)
		writeFile(t, filepath.Join(root, base+"classes/FormulaWhereNamed.cls"), named)
		writeFile(t, filepath.Join(root, base+"classes/FormulaWhereNamed.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
		index := loadTestIndex(t, root)
		if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
			t.Fatalf("native dynamic probe compiled; Glade: %v %v", index.Diagnostics, analysis.Diagnostics)
		}
		for _, route := range []string{"anonymous", "named", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				var observed string
				if route == "anonymous" || route == "named" {
					source := read("anonymous.apex")
					if route == "named" {
						source = "System.debug('OBS|'+FormulaWhereNamed.observe());"
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					options := conformanceRunnerOptions{LinkProject: true, Org: &org}
					if route == "named" {
						runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
						if err != nil {
							t.Fatal(err)
						}
						options.LinkProject, options.RequestRuntime = false, &runtime
					}
					runner := newConformanceRunner(t, index, options)
					result, err := runner.execute(program)
					if err != nil || len(result.Debug) != 1 {
						t.Fatalf("missing anonymous observation: %v %v", err, result.Debug)
					}
					_, observed, _ = strings.Cut(result.Debug[0], "OBS|")
				} else {
					run := Run(index, Options{SelectedClasses: []string{"FormulaWhereProbe62"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("unexpected named selection: %+v", run.Summary())
					}
					problem := run.Suites[0].Cases[0].Problem
					if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
						t.Fatalf("missing named assertion transport: %+v", problem)
					}
					_, observed, _ = strings.Cut(problem.Message, "OBS|")
				}
				// The ordinary named call reuses the captured named test payload.
				nativeRoute := route
				if route == "named" {
					nativeRoute = "istest"
				}
				var got map[string]string
				if err := json.Unmarshal([]byte(observed), &got); err != nil || len(got) != 40 || len(want[api+"/"+nativeRoute]) != 40 {
					t.Fatalf("incomplete observations: %v %s", err, observed)
				}
				for id, expected := range want[api+"/"+nativeRoute] {
					t.Run(id, func(t *testing.T) {
						if normalizeFormulaObservation(got[id]) != normalizeFormulaObservation(expected) {
							t.Fatalf("native=%q; Glade=%q", expected, got[id])
						}
					})
				}
			})
		}
	}
}

// Compare rejection kinds and the owned seed records. Anonymous orgs may also
// contain setup records; raw native messages and rows remain in org.tsv.
func normalizeFormulaObservation(value string) string {
	if !strings.HasPrefix(value, "OK|") {
		kind, _, _ := strings.Cut(value, "|")
		return kind
	}
	var owned []string
	for _, name := range strings.Split(strings.TrimPrefix(value, "OK|"), ",") {
		if strings.HasPrefix(name, "Arithmetic ") {
			owned = append(owned, name)
		}
	}
	return "OK|" + strings.Join(owned, ",")
}

func TestSOQLFormulaWhereInlineNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/soql_formula_where"
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	reader := csv.NewReader(strings.NewReader(read("compile.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 7 {
		t.Fatalf("incomplete native inline matrix: %v", err)
	}
	for _, row := range append([][]string(nil), rows[1:]...) {
		if row[1] == "istest" {
			rows = append(rows, []string{row[0], "named", row[2]})
		}
	}
	for _, row := range rows[1:] {
		api, route, expected := row[0], row[1], row[2]
		t.Run(api+"/"+route, func(t *testing.T) {
			root := t.TempDir()
			base := "force-app/main/default/"
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for _, name := range []string{"Start__c", "Finish__c", "StartTime__c", "FinishTime__c"} {
				path := base + "objects/Account/fields/" + name + ".field-meta.xml"
				writeFile(t, filepath.Join(root, path), read("project/"+path))
			}
			classPath := base + "classes/FormulaWhereStatic68"
			source := read("project/" + classPath + ".cls")
			if route == "named" {
				source = strings.Replace(source, "@IsTest private class FormulaWhereStatic68 { @IsTest static void observe()", "public class FormulaWhereStatic68 { public static Integer observe()", 1)
				source = strings.Replace(source, "System.assert(false,'OBS|'+found.size());", "return found.size();", 1)
			}
			writeFile(t, filepath.Join(root, classPath+".cls"), source)
			writeFile(t, filepath.Join(root, classPath+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if route == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, read("inline.apex"), api)
			}
			if strings.HasPrefix(expected, "REJECT|") {
				for _, item := range analysis.Diagnostics {
					if strings.Contains(item.Message+item.NativeMessage, "FORMULA() in a SOQL WHERE clause is a Beta feature") {
						return
					}
				}
				t.Fatalf("missing native version rejection: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native inline query compiled; Glade: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			var message string
			if route == "anonymous" || route == "named" {
				source := read("inline.apex")
				if route == "named" {
					source = "System.debug('OBS|'+FormulaWhereStatic68.observe());"
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				options := conformanceRunnerOptions{LinkProject: true, Org: &org}
				if route == "named" {
					runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
					if err != nil {
						t.Fatal(err)
					}
					options.LinkProject, options.RequestRuntime = false, &runtime
				}
				runner := newConformanceRunner(t, index, options)
				result, err := runner.execute(program)
				if err != nil || len(result.Debug) != 1 {
					t.Fatalf("inline execution: %v %v", err, result.Debug)
				}
				message = result.Debug[0]
			} else {
				run := Run(index, Options{SelectedClasses: []string{"FormulaWhereStatic68"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing inline named observation: %+v", run.Summary())
				}
				message = run.Suites[0].Cases[0].Problem.Message
			}
			_, observed, found := strings.Cut(message, "OBS|")
			if !found || "OK|"+observed != expected {
				t.Fatalf("native=%s; Glade=%s", expected, message)
			}
		})
	}
}

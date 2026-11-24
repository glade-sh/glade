package apextest

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE: testdata/conformance/absent_org_wide_email/org.tsv.
func TestAbsentOrgWideEmailNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/absent_org_wide_email"
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
	header := []string{"id", "api", "route", "compiled", "success", "outcome", "class_api", "trigger_api", "residue"}
	if len(rows) != 79 || !reflect.DeepEqual(rows[0], header) {
		t.Fatalf("invalid table: %q", rows)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		id, api, route := row[0], row[1], row[2]
		if strings.HasPrefix(route, "trigger_") || strings.HasPrefix(id, "shape") {
			continue
		}
		key := id + "/" + api + "/" + route
		if seen[key] {
			t.Fatalf("duplicate row %s", key)
		}
		seen[key] = true
		if (id != "direct" && id != "lookup") || (api != "62.0" && api != "63.0" && api != "64.0" && api != "65.0" && api != "66.0" && api != "67.0") || (route != "anonymous" && route != "istest") {
			t.Fatalf("unexpected row: %q", row)
		}
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			actual := []string{id, api, route, "true", "", "", "n/a", "n/a", "n/a"}
			if route == "anonymous" {
				source, err := os.ReadFile(filepath.Join(fixture, "missing.apex"))
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				seedOrgWideEmailCatalog(&org)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				actual[4] = "true"
				if len(result.Debug) != 2 {
					t.Fatalf("debug = %q", result.Debug)
				}
				pos := 0
				if id == "lookup" {
					pos = 1
				}
				prefix := fmt.Sprintf("ROAD548_CASE_%d=", pos)
				var ok bool
				actual[5], ok = strings.CutPrefix(result.Debug[pos], prefix)
				if !ok {
					t.Fatalf("debug = %q", result.Debug)
				}
			} else {
				source, err := os.ReadFile(filepath.Join(fixture, "force-app/main/default/classes/Road548Probe.cls"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/Road548Probe.cls"), string(source))
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/Road548Probe.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				index := loadTestIndex(t, root)
				if result := sema.Analyze(index); result.HasErrors() {
					t.Fatalf("class semantics: %v", result.Diagnostics)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				seedOrgWideEmailCatalog(&org)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				cases := Discover(index, Options{SelectedClasses: []string{"Road548Probe"}})
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				runtimeMethods := indexTestRuntimeMethods(methods)
				count := 0
				for _, testCase := range cases {
					if !strings.EqualFold(testCase.MethodName, id) {
						continue
					}
					count++
					key := testCaseKey(testCase)
					result := runCase(context.Background(), testCase, methods[key], runtimeMethods[testMethodSourceKey(testCase.ClassName, testCase.File)], methodErrors[key], programs[key], programErrors[key], runner.base, nil, nil, nil, org, 0, Options{NoDiskCache: true, Parallelism: 1}, true, nil, newRunPerfCounters(false))
					actual[4] = string(result.Status)
					if actual[4] == "fail" {
						actual[4] = "Failed"
					}
					if result.Problem == nil {
						t.Fatalf("missing assertion: %+v", result)
					}
					if result.Problem.Type != "System.AssertException" {
						t.Fatalf("unexpected assertion type: %q", result.Problem.Type)
					}
					const prefix = "Assertion Failed: ROAD548="
					var ok bool
					actual[5], ok = strings.CutPrefix(result.Problem.Message, prefix)
					if !ok {
						t.Fatalf("unexpected assertion message: %q", result.Problem.Message)
					}
				}
				if count != 1 {
					t.Fatalf("found %d cases for %s", count, id)
				}
				actual[6] = api
			}
			if !reflect.DeepEqual(actual, row) {
				t.Fatalf("got  %q\nwant %q", actual, row)
			}
		})
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			for _, id := range []string{"direct", "lookup"} {
				key := id + "/" + api + "/" + route
				if !seen[key] {
					t.Fatalf("missing row %s", key)
				}
			}
		}
	}
}

func TestAbsentOrgWideEmailShapesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/absent_org_wide_email"
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
	header := []string{"id", "api", "route", "compiled", "success", "outcome", "class_api", "trigger_api", "residue"}
	if len(rows) != 79 || !reflect.DeepEqual(rows[0], header) {
		t.Fatalf("invalid table: %q", rows)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		id, api, route := row[0], row[1], row[2]
		if !strings.HasPrefix(id, "shape") {
			continue
		}
		key := id + "/" + api + "/" + route
		if seen[key] {
			t.Fatalf("duplicate row %s", key)
		}
		seen[key] = true
		if (id != "shape0" && id != "shape1" && id != "shape2" && id != "shape3") || (api != "62.0" && api != "63.0" && api != "64.0" && api != "65.0" && api != "66.0" && api != "67.0") || (route != "anonymous" && route != "istest") {
			t.Fatalf("unexpected row: %q", row)
		}
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			actual := []string{id, api, route, "true", "", "", "n/a", "n/a", "n/a"}
			if route == "anonymous" {
				source, err := os.ReadFile(filepath.Join(fixture, "shapes.apex"))
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				seedOrgWideEmailCatalog(&org)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				actual[4] = "true"
				if len(result.Debug) != 4 {
					t.Fatalf("debug = %q", result.Debug)
				}
				pos := int(id[5] - '0')
				prefix := fmt.Sprintf("ROAD548_SHAPE_%d=", pos)
				var ok bool
				actual[5], ok = strings.CutPrefix(result.Debug[pos], prefix)
				if !ok {
					t.Fatalf("debug = %q", result.Debug)
				}
			} else {
				source, err := os.ReadFile(filepath.Join(fixture, "force-app/main/default/classes/Road548Shapes.cls"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/Road548Shapes.cls"), string(source))
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/Road548Shapes.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				index := loadTestIndex(t, root)
				if result := sema.Analyze(index); result.HasErrors() {
					t.Fatalf("class semantics: %v", result.Diagnostics)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				seedOrgWideEmailCatalog(&org)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				cases := Discover(index, Options{SelectedClasses: []string{"Road548Shapes"}})
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				runtimeMethods := indexTestRuntimeMethods(methods)
				count := 0
				for _, testCase := range cases {
					if !strings.EqualFold(testCase.MethodName, id) {
						continue
					}
					count++
					key := testCaseKey(testCase)
					result := runCase(context.Background(), testCase, methods[key], runtimeMethods[testMethodSourceKey(testCase.ClassName, testCase.File)], methodErrors[key], programs[key], programErrors[key], runner.base, nil, nil, nil, org, 0, Options{NoDiskCache: true, Parallelism: 1}, true, nil, newRunPerfCounters(false))
					actual[4] = string(result.Status)
					if actual[4] == "fail" {
						actual[4] = "Failed"
					}
					if result.Problem == nil {
						t.Fatalf("missing assertion: %+v", result)
					}
					if result.Problem.Type != "System.AssertException" {
						t.Fatalf("unexpected assertion type: %q", result.Problem.Type)
					}
					const prefix = "Assertion Failed: ROAD548="
					var ok bool
					actual[5], ok = strings.CutPrefix(result.Problem.Message, prefix)
					if !ok {
						t.Fatalf("unexpected assertion message: %q", result.Problem.Message)
					}
				}
				if count != 1 {
					t.Fatalf("found %d cases for %s", count, id)
				}
				actual[6] = api
			}
			if !reflect.DeepEqual(actual, row) {
				t.Fatalf("got  %q\nwant %q", actual, row)
			}
		})
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			for _, id := range []string{"shape0", "shape1", "shape2", "shape3"} {
				key := id + "/" + api + "/" + route
				if !seen[key] {
					t.Fatalf("missing row %s", key)
				}
			}
		}
	}
}

// A populated local catalog lets the VM validate the captured missing ID.
// Its address deliberately differs from the fixture's missing lookup.
func seedOrgWideEmailCatalog(org *storage.OrgState) {
	const id = storage.ID("0D2000000000002")
	object := org.Objects["OrgWideEmailAddress"]
	object.Records = map[storage.ID]storage.Record{
		id: {ID: id, Object: "OrgWideEmailAddress", Fields: map[string]storage.Value{"Address": storage.StringValue("seed@example.invalid")}},
	}
	org.Objects["OrgWideEmailAddress"] = object
}

// Trigger bodies use the same sender validation as anonymous and test methods.
func TestAbsentOrgWideEmailTriggerNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/absent_org_wide_email"
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
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		api, route := row[1], row[2]
		if !strings.HasPrefix(route, "trigger_") {
			continue
		}
		key := api + "/" + route
		if seen[key] || row[0] != "direct" || (api != "62.0" && api != "63.0" && api != "67.0") {
			t.Fatalf("unexpected row: %q", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			for _, spec := range []struct{ dir, name, ext, meta string }{{"triggers", "Road548Trigger", "trigger", "ApexTrigger"}, {"classes", "Road548TriggerTest", "cls", "ApexClass"}} {
				rel := filepath.Join("force-app/main/default", spec.dir, spec.name+"."+spec.ext)
				source, err := os.ReadFile(filepath.Join(fixture, rel))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, rel), string(source))
				writeFile(t, filepath.Join(root, rel+"-meta.xml"), fmt.Sprintf(`<%s xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></%s>`, spec.meta, api, spec.meta))
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			seedOrgWideEmailCatalog(&org)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			triggers, failures := compileProjectTriggers(index)
			if len(failures) != 0 || len(triggers) != 1 {
				t.Fatalf("trigger compilation: %v, triggers: %d", failures, len(triggers))
			}
			for _, trigger := range triggers {
				if err := runner.base.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
			}
			actual := []string{"direct", api, route, "true", "", "", "n/a", api, "n/a"}
			if route == "trigger_anonymous" {
				source, err := os.ReadFile(filepath.Join(fixture, "trigger.apex"))
				if err != nil {
					t.Fatal(err)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Debug) != 1 {
					t.Fatalf("debug = %q", result.Debug)
				}
				var ok bool
				actual[5], ok = strings.CutPrefix(result.Debug[0], "ROAD548TRIGGER=")
				if !ok {
					t.Fatalf("debug = %q", result.Debug)
				}
				actual[4] = "true"
			} else if route == "trigger_istest" {
				cases := Discover(index, Options{SelectedClasses: []string{"Road548TriggerTest"}})
				if len(cases) != 1 {
					t.Fatalf("found %d cases", len(cases))
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				runtimeMethods := indexTestRuntimeMethods(methods)
				tc := cases[0]
				k := testCaseKey(tc)
				result := runCase(context.Background(), tc, methods[k], runtimeMethods[testMethodSourceKey(tc.ClassName, tc.File)], methodErrors[k], programs[k], programErrors[k], runner.base, nil, nil, nil, org, 0, Options{NoDiskCache: true, Parallelism: 1}, true, nil, newRunPerfCounters(false))
				if result.Problem == nil || result.Problem.Type != "System.AssertException" {
					t.Fatalf("unexpected result: %+v", result)
				}
				var ok bool
				actual[5], ok = strings.CutPrefix(result.Problem.Message, "Assertion Failed: ROAD548TRIGGER=")
				if !ok {
					t.Fatalf("unexpected assertion: %q", result.Problem.Message)
				}
				if string(result.Status) == "fail" {
					actual[4] = "Failed"
				} else {
					actual[4] = string(result.Status)
				}
				actual[6] = api
			} else {
				t.Fatalf("unexpected route: %q", route)
			}
			if !reflect.DeepEqual(actual, row) {
				t.Fatalf("got %q\nwant %q", actual, row)
			}
		})
	}
	for _, api := range []string{"62.0", "63.0", "67.0"} {
		for _, route := range []string{"trigger_anonymous", "trigger_istest"} {
			if !seen[api+"/"+route] {
				t.Fatalf("missing %s/%s", api, route)
			}
		}
	}
}

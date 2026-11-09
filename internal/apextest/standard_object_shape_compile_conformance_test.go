package apextest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native rows: testdata/conformance/standard_object_shape_compile/org.tsv. A
// plain Developer scratch org (no features) rejects every static reference to
// a standard object that needs a feature the org lacks, at compile time, in
// anonymous Apex and named classes, API 62.0 and 67.0. Glade rejects the same
// references when the project declares a shape without the feature; the
// declared shape here is a scratch definition that lists one unrelated
// feature. Dynamic SOQL, global describe and Type.forName stay run-time
// answers. Column numbers are not reproduced; the test compares messages.
const standardObjectShapeDir = "testdata/conformance/standard_object_shape_compile"

func standardObjectShapeProject(t *testing.T, api string, features []string, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	if features != nil {
		quoted := make([]string, len(features))
		for i, f := range features {
			quoted[i] = `"` + f + `"`
		}
		writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"orgName":"x","edition":"Developer","features":[`+strings.Join(quoted, ",")+`]}`)
	}
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
	for name, code := range classes {
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), code)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), meta)
	}
	return loadTestIndex(t, root)
}

func readStandardObjectShapeRows(t *testing.T) []nestedVisibilityRow {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(standardObjectShapeDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if lines[0] != "case_id\tapi\troute\tcompile_outcome\tobserved_value_or_diagnostic\tsource" {
		t.Fatalf("org.tsv header: %q", lines[0])
	}
	var rows []nestedVisibilityRow
	for _, line := range lines[1:] {
		c := strings.Split(line, "\t")
		if len(c) != 6 {
			t.Fatalf("bad row %q", line)
		}
		rows = append(rows, nestedVisibilityRow{Case: c[0], API: c[1], Route: c[2], Compile: c[3], Observed: c[4], Source: c[5]})
	}
	return rows
}

// standardObjectShapeCarried rows are not reproduced yet. b30 chains a
// describe call on the absent object's token; native names "X.SObjectType"
// while the rejection Glade makes for `X.SObjectType` alone names "X"
// (b07, b35). One row each shape; no other chain is captured.
var standardObjectShapeCarried = map[string]string{
	"b30": "describe call chained on an absent object's SObjectType token",
}

// standardObjectShapeBaseClasses are the shared owned classes of the rows
// (classes/*.cls): NsHolder declares nested types named like gated standard
// objects, NsOuter declares private and default nested types.
func standardObjectShapeBaseClasses(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(standardObjectShapeDir, "classes", "*.cls"))
	if err != nil || len(files) == 0 {
		t.Fatalf("classes: %v %v", files, err)
	}
	classes := map[string]string{}
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		classes[strings.TrimSuffix(filepath.Base(f), ".cls")] = string(body)
	}
	return classes
}

func TestStandardObjectShapeCompileNativeConformance(t *testing.T) {
	rows := readStandardObjectShapeRows(t)
	if len(rows) != 320 {
		t.Fatalf("org.tsv rows=%d, want 320", len(rows))
	}
	declared := []string{"EnableSetPasswordInApi"}
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		t.Run(api, func(t *testing.T) {
			base := standardObjectShapeBaseClasses(t)
			index := standardObjectShapeProject(t, api, declared, base)
			org := orgFromIndex(index)
			org.APIVersion = api
			for _, row := range rows {
				if row.API != api {
					continue
				}
				if reason, carried := standardObjectShapeCarried[row.Case]; carried {
					t.Logf("carried %s %s: %s; native %q", row.Case, api, reason, row.Observed)
					continue
				}
				source, err := os.ReadFile(filepath.Join(standardObjectShapeDir, row.Source))
				if err != nil {
					t.Fatal(err)
				}
				switch row.Route {
				case "anonymous":
					errs := subscriberErrors(sema.AnalyzeAnonymous(index, string(source), api).Diagnostics)
					switch row.Compile {
					case "COMPILE_ERROR":
						m := nestedVisibilityPattern.FindStringSubmatch(row.Observed)
						if !slices.Contains(errs, m[3]) {
							t.Errorf("%s anonymous %s: Glade %q; native %q", row.Case, api, errs, m[3])
						}
					default:
						if len(errs) > 0 {
							t.Errorf("%s anonymous %s: Glade rejects %q; native compiles (%s)", row.Case, api, errs[0], row.Compile)
							continue
						}
						program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Errorf("%s anonymous %s: %v", row.Case, api, err)
							continue
						}
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						result, runErr := runner.execute(program)
						if row.Compile == "RUNTIME_ERROR" {
							if runErr == nil || !strings.Contains(runErr.Error(), "sObject type 'ProductCatalog' is not supported") {
								t.Errorf("%s anonymous %s: Glade run %v; native %q", row.Case, api, runErr, row.Observed)
							}
							continue
						}
						if runErr != nil {
							t.Errorf("%s anonymous %s: execution error %v", row.Case, api, runErr)
							continue
						}
						var want string
						if strings.HasPrefix(row.Case, "fam_") {
							want = "OBS|" + strings.TrimPrefix(row.Case, "fam_") + "|" + row.Observed
						} else {
							want = "OBS|" + row.Case + "|" + row.Observed
						}
						found := false
						for _, line := range result.Debug {
							found = found || strings.HasSuffix(line, want)
						}
						if !found {
							t.Errorf("%s anonymous %s: debug %q lacks %q", row.Case, api, result.Debug, want)
						}
					}
				case "named":
					name := strings.TrimSuffix(filepath.Base(row.Source), ".cls")
					classes := standardObjectShapeBaseClasses(t)
					classes[name] = string(source)
					got := subscriberErrors(sema.Analyze(standardObjectShapeProject(t, api, declared, classes)).Diagnostics)
					if row.Compile == "COMPILE_OK" {
						if len(got) > 0 {
							t.Errorf("%s named %s: Glade rejects %q; native compiles", name, api, got)
						}
						continue
					}
					m := nestedVisibilityPattern.FindStringSubmatch(row.Observed)
					if !slices.Contains(got, m[3]) {
						t.Errorf("%s named %s: Glade %q; native %q", name, api, got, m[3])
					}
				}
			}
		})
	}
}

// An unspecified shape cannot establish absence, and a
// declared shape that has the feature keeps the object.
func TestStandardObjectShapeCompileGateScope(t *testing.T) {
	const catalog = "insert new ProductCatalog(Name = 'x');"
	const menu = "List<SObject> items = [SELECT Id FROM NavigationMenuItem];"
	for _, tc := range []struct {
		name     string
		features []string
		source   string
		rejected bool
	}{
		{"unspecified shape is ungated", nil, catalog, false},
		{"unspecified shape query is ungated", nil, menu, false},
		{"declared shape without the feature", []string{"EnableSetPasswordInApi"}, catalog, true},
		{"declared order management keeps ProductCatalog", []string{"OrderManagement"}, catalog, false},
		{"declared shape without networks", []string{"EnableSetPasswordInApi"}, menu, true},
		{"declared communities keeps NavigationMenuItem", []string{"Communities", "NetworksEnabled"}, menu, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := standardObjectShapeProject(t, "67.0", tc.features, nil)
			errs := subscriberErrors(sema.AnalyzeAnonymous(index, tc.source, "67.0").Diagnostics)
			if tc.rejected != (len(errs) > 0) {
				t.Fatalf("errors %q, rejected want %v", errs, tc.rejected)
			}
		})
	}
}

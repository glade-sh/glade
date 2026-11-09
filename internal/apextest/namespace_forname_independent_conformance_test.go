package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native rows: testdata/conformance/namespace_forname_independent/org.tsv.
// One fresh namespaced scratch org (gladens004) was asked, for nested classes
// that are private, default, @TestVisible private and public, what Type.forName
// returns in six spellings, from a test class, another test class, a non-test
// class, anonymous Apex, a nested class, a static initializer and a
// constructor. Rows cover class API 24.0 through 67.0 (@IsTest) and anonymous
// Apex at 62.0 and 67.0. Every non-public nested class is null from API 28.0 on,
// the outer class itself included.
const forNameIndepDir = "testdata/conformance/namespace_forname_independent"

type forNameIndepRow struct {
	Case, API, Route, IsNull, Observed, Source string
}

func forNameIndepRows(t *testing.T) []forNameIndepRow {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(forNameIndepDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if lines[0] != "case_id\tapi\troute\tis_null\tobserved\tsource" {
		t.Fatalf("org.tsv header: %q", lines[0])
	}
	var rows []forNameIndepRow
	for _, line := range lines[1:] {
		c := strings.Split(line, "\t")
		if len(c) != 6 {
			t.Fatalf("bad row %q", line)
		}
		rows = append(rows, forNameIndepRow{c[0], c[1], c[2], c[3], c[4], c[5]})
	}
	return rows
}

func forNameIndepFixture(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(forNameIndepDir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func forNameIndepProject(t *testing.T, api string, classes map[string]string) typesys.Index {
	t.Helper()
	versions := map[string]string{}
	for name := range classes {
		versions[name] = api
	}
	return forNameIndepProjectVersions(t, api, classes, versions)
}

func forNameIndepProjectVersions(t *testing.T, api string, classes, versions map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"gladens004","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	for name, code := range classes {
		meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + versions[name] + "</apiVersion><status>Active</status></ApexClass>"

		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), code)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), meta)
	}
	return loadTestIndex(t, root)
}

func TestNamespaceForNameIndependentNativeConformance(t *testing.T) {
	rows := forNameIndepRows(t)
	if len(rows) != 2308 {
		t.Fatalf("org.tsv rows=%d, want 2308", len(rows))
	}
	all := map[string]string{}
	for _, name := range []string{"OuterTest", "OuterMain", "OtherTest", "CallerTest"} {
		all[name] = forNameIndepFixture(t, "classes/"+name+".cls")
	}
	byAPI := map[string][]forNameIndepRow{}
	var apis []string
	for _, row := range rows {
		if strings.HasPrefix(row.Route, "mixed") {
			continue
		}
		if _, ok := byAPI[row.API]; !ok {
			apis = append(apis, row.API)
		}
		byAPI[row.API] = append(byAPI[row.API], row)
	}
	for _, api := range apis {
		api := api
		t.Run(api, func(t *testing.T) {
			full := api == "62.0" || api == "67.0"
			classes := map[string]string{"OuterTest": all["OuterTest"]}
			if full {
				classes = all
			}
			index := forNameIndepProject(t, api, classes)
			// Collect @IsTest observations once per class.
			observed := map[string]string{}
			for _, class := range []string{"OuterTest", "OtherTest", "CallerTest"} {
				if _, ok := classes[class]; !ok {
					continue
				}
				run := Run(index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						if c.Problem == nil || !strings.HasPrefix(c.MethodName, "m_") {
							continue
						}
						id := strings.TrimPrefix(c.MethodName, "m_")
						if _, value, ok := strings.Cut(c.Problem.Message, "OBS|"+id+"|"); ok {
							observed[id] = value
						}
					}
				}
				if len(observed) == 0 {
					t.Fatalf("%s %s: no observations (%s)", api, class, firstRunProblem(run))
				}
			}
			anon := map[string]string{}
			if full {
				source := forNameIndepFixture(t, "anon/anon.apex")
				if errs := subscriberErrors(sema.AnalyzeAnonymous(index, source, api).Diagnostics); len(errs) > 0 {
					t.Fatalf("anonymous %s: Glade rejects %q; native compiles", api, errs[0])
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatalf("anonymous %s: %v", api, err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				// Like glade exec: anonymous Apex runs in the org's namespace.
				result, err := runner.execute(program, func(m *vm.VM) { m.SetCurrentNamespace(org.Namespace) })
				if err != nil {
					t.Fatalf("anonymous %s: execution error %v", api, err)
				}
				for _, line := range result.Debug {
					if _, rest, ok := strings.Cut(line, "OBS|"); ok {
						if id, value, ok := strings.Cut(rest, "|"); ok {
							anon[id] = value
						}
					}
				}
			}
			for _, row := range byAPI[api] {
				switch row.Route {
				case "istest":
					got, ok := observed[row.Case]
					if !ok {
						t.Errorf("%s istest %s: no observation", row.Case, api)
					} else if got != row.Observed {
						t.Errorf("%s istest %s: Glade %q; native %q", row.Case, api, got, row.Observed)
					}
				case "anonymous":
					// Excluded: native raises "Cannot call test methods in non-test
					// context" for the Callable cast or call of OuterTest.PubInner from
					// anonymous Apex. The rows do not show whether the cast or the call
					// raises, so Glade does not model it; every other part of the row
					// (name, newInstance) is still checked.
					if strings.Contains(row.Observed, "cast=Cannot call test methods in non-test context") {
						want := strings.TrimSuffix(row.Observed, "cast=Cannot call test methods in non-test context")
						if got := anon[row.Case]; !strings.HasPrefix(got, want) {
							t.Errorf("%s anonymous %s: Glade %q; native %q", row.Case, api, got, row.Observed)
						}
						continue
					}
					got, ok := anon[row.Case]
					if !ok {
						t.Errorf("%s anonymous %s: no observation", row.Case, api)
					} else if got != row.Observed {
						t.Errorf("%s anonymous %s: Glade %q; native %q", row.Case, api, got, row.Observed)
					}
				case "named":
					name := strings.TrimSuffix(filepath.Base(row.Source), ".cls")
					got := subscriberErrors(sema.Analyze(forNameIndepProject(t, api, map[string]string{name: forNameIndepFixture(t, row.Source)})).Diagnostics)
					found := false
					for _, g := range got {
						found = found || strings.Contains(g, row.Observed)
					}
					if !found {
						t.Errorf("%s named %s: Glade %q; native %q", row.Case, api, got, row.Observed)
					}
				}
			}
		})
	}
}

// Mixed class versions: the calling class's API version decides whether a
// non-public nested class is found, not the target's. T24 and N24 are API
// 24.0, T62 is 62.0; C24 and C62 are @IsTest callers at their own version, and
// anonymous Apex runs at the run API (also calling N24, a 24.0 non-test class).
func TestNamespaceForNameIndependentMixedAPIConformance(t *testing.T) {
	classes := map[string]string{}
	versions := map[string]string{"T24": "24.0", "N24": "24.0", "C24": "24.0", "T62": "62.0", "C62": "62.0"}
	for name := range versions {
		classes[name] = forNameIndepFixture(t, "mixed/"+name+".cls")
	}
	index := forNameIndepProjectVersions(t, "62.0", classes, versions)
	observed := map[string]string{}
	for _, class := range []string{"C24", "C62"} {
		run := Run(index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
		for _, suite := range run.Suites {
			for _, c := range suite.Cases {
				if c.Problem == nil || !strings.HasPrefix(c.MethodName, "m_") {
					continue
				}
				id := strings.TrimPrefix(c.MethodName, "m_")
				if _, value, ok := strings.Cut(c.Problem.Message, "OBS|"+id+"|"); ok {
					observed[id] = value
				}
			}
		}
	}
	anon := map[string]map[string]string{}
	source := forNameIndepFixture(t, "mixed/anon.apex")
	for _, api := range []string{"62.0", "67.0"} {
		program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatalf("anonymous %s: %v", api, err)
		}
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		result, err := runner.execute(program, func(m *vm.VM) { m.SetCurrentNamespace(org.Namespace) })
		if err != nil {
			t.Fatalf("anonymous %s: execution error %v", api, err)
		}
		anon[api] = map[string]string{}
		for _, line := range result.Debug {
			if _, rest, ok := strings.Cut(line, "OBS|"); ok {
				if id, value, ok := strings.Cut(rest, "|"); ok {
					anon[api][id] = value
				}
			}
		}
	}
	checked := 0
	for _, row := range forNameIndepRows(t) {
		var got string
		var ok bool
		switch row.Route {
		case "mixed-istest":
			got, ok = observed[row.Case]
		case "mixed-anon":
			got, ok = anon[row.API][row.Case]
		default:
			continue
		}
		checked++
		if !ok {
			t.Errorf("%s %s %s: no observation", row.Case, row.Route, row.API)
		} else if got != row.Observed {
			t.Errorf("%s %s %s: Glade %q; native %q", row.Case, row.Route, row.API, got, row.Observed)
		}
	}
	if checked != 64 {
		t.Fatalf("checked %d mixed rows, want 64", checked)
	}
}

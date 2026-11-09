package apextest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native rows: testdata/conformance/namespace_anonymous_owner/org.tsv.
// A namespaced owner org (namespace gladens004) is asked about its own public
// and global classes, nested types, enums, interfaces, reflection and missing
// names from anonymous Apex and from an @IsTest class, API 62.0 and 67.0.
// Anonymous Apex runs in the org's namespace: its public classes are visible,
// unqualified and qualified, in any case.
func TestNamespaceAnonymousOwnerNativeConformance(t *testing.T) {
	previous := nestedVisibilityDir
	nestedVisibilityDir = "testdata/conformance/namespace_anonymous_owner"
	defer func() { nestedVisibilityDir = previous }()
	rows := readNestedVisibilityRows(t)
	entries, err := os.ReadDir(filepath.Join(nestedVisibilityDir, "classes"))
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]string{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".cls") {
			classes[strings.TrimSuffix(e.Name(), ".cls")] = nestedVisibilityFixture(t, "classes/"+e.Name())
		}
	}
	rtClasses := map[string]string{"V035OwnProbeT": nestedVisibilityFixture(t, "rt/V035OwnProbeT.cls")}
	for k, v := range classes {
		rtClasses[k] = v
	}
	if len(rows) != 140 {
		t.Fatalf("org.tsv rows=%d, want 140", len(rows))
	}
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		t.Run(api, func(t *testing.T) {
			index := nestedVisibilityProject(t, "ns", api, classes)
			rtIndex := nestedVisibilityProject(t, "ns", api, rtClasses)
			org := orgFromIndex(index)
			org.APIVersion = api
			for _, row := range rows {
				if row.API != api {
					continue
				}
				row := row
				switch row.Route {
				case "anonymous":
					source := nestedVisibilityFixture(t, row.Source)
					errs := subscriberErrors(sema.AnalyzeAnonymous(index, source, api).Diagnostics)
					if row.Compile == "COMPILE_ERROR" {
						m := nestedVisibilityPattern.FindStringSubmatch(row.Observed)
						if len(errs) == 0 || errs[0] != m[3] {
							t.Errorf("%s anonymous %s: Glade %q; native %q", row.Case, api, errs, m[3])
						}
						continue
					}
					if len(errs) > 0 {
						t.Errorf("%s anonymous %s: Glade rejects %q; native compiles", row.Case, api, errs[0])
						continue
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Errorf("%s anonymous %s: %v", row.Case, api, err)
						continue
					}
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					// Like glade exec: anonymous Apex runs in the org's namespace.
					result, err := runner.execute(program, func(m *vm.VM) { m.SetCurrentNamespace(org.Namespace) })
					if err != nil {
						t.Errorf("%s anonymous %s: execution error %v", row.Case, api, err)
						continue
					}
					want := "OBS|" + row.Case + "|" + row.Observed
					found := false
					for _, line := range result.Debug {
						found = found || strings.HasSuffix(line, want)
					}
					if !found {
						t.Errorf("%s anonymous %s: debug %q lacks %q", row.Case, api, result.Debug, want)
					}
				case "istest":
					run := Run(rtIndex, Options{SelectedClasses: []string{"V035OwnProbeT"}, NoDiskCache: true, Parallelism: 1})
					found := false
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem == nil || c.MethodName != "m_"+row.Case {
								continue
							}
							if _, value, ok := strings.Cut(c.Problem.Message, "OBS|"+row.Case+"|"); ok {
								found = true
								if value != row.Observed {
									t.Errorf("%s istest %s: Glade %q; native %q", row.Case, api, value, row.Observed)
								}
							}
						}
					}
					if !found {
						t.Errorf("%s istest %s: no observation (%s)", row.Case, api, firstRunProblem(run))
					}
				case "named":
					classNames := map[string]string{}
					for k, v := range classes {
						classNames[k] = v
					}
					name := strings.TrimSuffix(filepath.Base(row.Source), ".cls")
					classNames[name] = nestedVisibilityFixture(t, row.Source)
					want := nestedVisibilityPattern.FindStringSubmatch(row.Observed)
					got := subscriberErrors(sema.Analyze(nestedVisibilityProject(t, "ns", api, classNames)).Diagnostics)
					if !slices.Contains(got, want[3]) {
						t.Errorf("%s named %s: Glade %q; native %q", row.Case, api, got, want[3])
					}
				}
			}
		})
	}
}

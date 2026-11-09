package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native rows: testdata/conformance/nested_type_visibility/org.tsv. A nested
// class, enum or interface that is neither public nor global is visible only
// inside its top-level type; @TestVisible lifts that for @IsTest classes, and
// Type.forName never finds such a type. Rows come from a namespaced owner org
// (ns) and a plain org (plain), API 62.0 and 67.0, anonymous, named
// (check-only deploy) and @IsTest routes. Native names the type in its
// declared case with the project namespace prefix; the named rows carry
// column numbers that Glade does not reproduce, so the test compares the
// line and the message.
const nestedTypeVisibilityDir = "testdata/conformance/nested_type_visibility"

// nestedVisibilityDir is the fixture directory the shared helpers read.
var nestedVisibilityDir = nestedTypeVisibilityDir

type nestedVisibilityRow struct {
	Case, API, Kind, Route, Compile, Observed, Source string
}

func readNestedVisibilityRows(t *testing.T) []nestedVisibilityRow {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(nestedVisibilityDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if lines[0] != "case_id\tapi\tkind\troute\tcompile_outcome\tobserved_value_or_diagnostic\tsource" {
		t.Fatalf("org.tsv header: %q", lines[0])
	}
	var rows []nestedVisibilityRow
	for _, line := range lines[1:] {
		c := strings.Split(line, "\t")
		if len(c) != 7 {
			t.Fatalf("bad row %q", line)
		}
		rows = append(rows, nestedVisibilityRow{c[0], c[1], c[2], c[3], c[4], c[5], c[6]})
	}
	return rows
}

func nestedVisibilityFixture(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(nestedVisibilityDir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func nestedVisibilityProject(t *testing.T, kind, api string, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	namespace := ""
	if kind == "ns" {
		namespace = "gladens004"
	}
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"`+namespace+`","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
	for name, code := range classes {
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), code)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), meta)
	}
	return loadTestIndex(t, root)
}

var nestedVisibilityPattern = regexp.MustCompile(`^line (\d+) col (\d+): (.*)$`)

func TestNestedTypeVisibilityNativeConformance(t *testing.T) {
	rows := readNestedVisibilityRows(t)
	if len(rows) != 338 {
		t.Fatalf("org.tsv rows=%d, want 338", len(rows))
	}
	type group struct{ kind, api string }
	groups := map[group][]nestedVisibilityRow{}
	var order []group
	for _, row := range rows {
		g := group{row.Kind, row.API}
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], row)
	}
	for _, g := range order {
		g := g
		t.Run(fmt.Sprintf("%s_%s", g.kind, g.api), func(t *testing.T) {
			base := map[string]string{"V035Vis": nestedVisibilityFixture(t, "classes/V035Vis.cls")}
			withRt := map[string]string{"V035Vis": base["V035Vis"], "V035VisRt": nestedVisibilityFixture(t, "rt/V035VisRt.cls")}
			rtIndex := nestedVisibilityProject(t, g.kind, g.api, withRt)
			index := nestedVisibilityProject(t, g.kind, g.api, base)
			org := orgFromIndex(index)
			org.APIVersion = g.api
			namedWant := map[string][]nestedVisibilityRow{}
			var namedOrder []string
			for _, row := range groups[g] {
				row := row
				switch row.Route {
				case "anonymous":
					source := nestedVisibilityFixture(t, row.Source)
					analysis := sema.AnalyzeAnonymous(index, source, g.api)
					errs := subscriberErrors(analysis.Diagnostics)
					if row.Compile == "COMPILE_ERROR" {
						m := nestedVisibilityPattern.FindStringSubmatch(row.Observed)
						if len(errs) == 0 {
							t.Errorf("%s anonymous %s: Glade accepted; native rejects with %q", row.Case, g.api, m[3])
						} else if errs[0] != m[3] {
							t.Errorf("%s anonymous %s: Glade says %q; native says %q", row.Case, g.api, errs[0], m[3])
						}
						continue
					}
					if len(errs) > 0 {
						t.Errorf("%s anonymous %s: Glade rejects %q; native compiles", row.Case, g.api, errs[0])
						continue
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: g.api})
					if err != nil {
						t.Errorf("%s anonymous %s: %v", row.Case, g.api, err)
						continue
					}
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					// Like glade exec: anonymous Apex runs in the org's namespace.
					result, err := runner.execute(program, func(m *vm.VM) { m.SetCurrentNamespace(org.Namespace) })
					if err != nil {
						t.Errorf("%s anonymous %s: execution error %v", row.Case, g.api, err)
						continue
					}
					want := "OBS|" + row.Case + "|" + row.Observed
					found := false
					for _, line := range result.Debug {
						found = found || strings.HasSuffix(line, want)
					}
					if !found {
						t.Errorf("%s anonymous %s: debug %q lacks %q", row.Case, g.api, result.Debug, want)
					}
				case "named":
					if _, seen := namedWant[row.Case]; !seen {
						namedOrder = append(namedOrder, row.Case)
					}
					namedWant[row.Case] = append(namedWant[row.Case], row)
				case "istest":
					run := Run(rtIndex, Options{SelectedClasses: []string{"V035VisRt"}, NoDiskCache: true, Parallelism: 1})
					found := false
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem == nil || c.MethodName != "m_"+row.Case {
								continue
							}
							if _, value, ok := strings.Cut(c.Problem.Message, "OBS|"+row.Case+"|"); ok {
								found = true
								if value != row.Observed {
									t.Errorf("%s istest %s: Glade %q; native %q", row.Case, g.api, value, row.Observed)
								}
							}
						}
					}
					if !found {
						t.Errorf("%s istest %s: no observation (%s) %+v", row.Case, g.api, firstRunProblem(run), run.Summary())
					}
				default:
					t.Fatalf("route %q", row.Route)
				}
			}
			for _, class := range namedOrder {
				want := namedWant[class]
				classes := map[string]string{"V035Vis": base["V035Vis"], class: nestedVisibilityFixture(t, want[0].Source)}
				single := nestedVisibilityProject(t, g.kind, g.api, classes)
				nativeLines := map[string]map[string]bool{}
				for _, row := range want {
					if row.Compile != "COMPILE_ERROR" {
						continue
					}
					m := nestedVisibilityPattern.FindStringSubmatch(row.Observed)
					if nativeLines[m[1]] == nil {
						nativeLines[m[1]] = map[string]bool{}
					}
					nativeLines[m[1]][m[3]] = true
				}
				glade := map[string]map[string]bool{}
				for _, d := range sema.Analyze(single).Diagnostics {
					if d.Severity != diagnostic.Error || d.Range == nil || !strings.HasSuffix(d.File, class+".cls") {
						continue
					}
					line := fmt.Sprint(d.Range.Start.Line)
					if glade[line] == nil {
						glade[line] = map[string]bool{}
					}
					glade[line][nativeDiagnosticText(d)] = true
				}
				for line, messages := range nativeLines {
					for message := range messages {
						if !glade[line][message] {
							t.Errorf("%s named %s: line %s lacks %q (Glade: %v)", class, g.api, line, message, glade[line])
						}
					}
				}
				for line, messages := range glade {
					for message := range messages {
						if !nativeLines[line][message] {
							t.Errorf("%s named %s: line %s: Glade reports %q; native does not", class, g.api, line, message)
						}
					}
				}
			}
		})
	}
}

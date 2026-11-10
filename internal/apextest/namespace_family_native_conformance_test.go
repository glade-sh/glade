package apextest

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native rows for the v0.3.5 namespace family (V035-NAMESPACE-DESIGN E14, E15):
//   - testdata/conformance/namespace_external_positions/org.tsv: an org with no
//     package and no namespace resolves the undeclared namespace MissingNsV035
//     in every position, anonymous and named, at API 62.0 and 67.0.
//   - testdata/conformance/namespace_owner_nested_inventory/org.tsv: a
//     namespaced owner org resolves its own nested types, missing nested types
//     under a present outer, and private nested reflection, same routes and APIs.
//
// The owner capture writes the scrubbed namespace as {{NAMESPACE}} and, where
// the source spelled it in lowercase, {{namespace}}. Glade replays it with a
// synthetic mixed-case namespace so the lowercase rows also exercise
// case-insensitive resolution.
const (
	namespaceExternalPositionsDir     = "testdata/conformance/namespace_external_positions"
	namespaceOwnerNestedInventoryDir  = "testdata/conformance/namespace_owner_nested_inventory"
	namespaceOwnerNestedInventoryName = "NsOwnV035"
)

type namespaceFamilyRow struct {
	Case, API, Context, Route, Compile, Runtime, Observed, Source string
}

var namespaceFamilyDiagnostic = regexp.MustCompile(`^line (\d+) col (\d+): (.*)$`)

func readNamespaceFamilyRows(t *testing.T, dir string) []namespaceFamilyRow {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if lines[0] != "case_id\tapi_version\tcontext\troute\tcompile_outcome\truntime_outcome\tobserved_value_or_diagnostic\tsource" {
		t.Fatalf("%s org.tsv header: %q", dir, lines[0])
	}
	var rows []namespaceFamilyRow
	for _, line := range lines[1:] {
		c := strings.Split(line, "\t")
		if len(c) != 8 {
			t.Fatalf("bad row %q", line)
		}
		rows = append(rows, namespaceFamilyRow{c[0], c[1], c[2], c[3], c[4], c[5], c[6], c[7]})
	}
	return rows
}

// namespaceFamilyFixture reads a capture file and substitutes the scrubbed
// owner namespace.
func namespaceFamilyFixture(t *testing.T, dir, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return namespaceFamilySubstitute(string(body))
}

func namespaceFamilySubstitute(text string) string {
	text = strings.ReplaceAll(text, "{{NAMESPACE}}", namespaceOwnerNestedInventoryName)
	return strings.ReplaceAll(text, "{{namespace}}", strings.ToLower(namespaceOwnerNestedInventoryName))
}

func namespaceFamilyProject(t *testing.T, namespace, api string, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"`+namespace+`","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
	for name, code := range classes {
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), code)
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), meta)
	}
	return loadTestIndex(t, root)
}

func namespaceFamilyClasses(t *testing.T, dir, sub string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, sub))
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]string{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".cls") {
			classes[strings.TrimSuffix(e.Name(), ".cls")] = namespaceFamilyFixture(t, dir, sub+"/"+e.Name())
		}
	}
	return classes
}

// namespaceFamilyFileErrors returns the error texts Glade reports for one class file.
func namespaceFamilyFileErrors(ds []diagnostic.Diagnostic, class string) []string {
	var out []string
	for _, d := range ds {
		if d.Severity == diagnostic.Error && strings.TrimSuffix(filepath.Base(d.File), ".cls") == class {
			out = append(out, nativeDiagnosticText(d))
		}
	}
	return out
}

type namespaceFamilyReplay struct {
	dir       string
	namespace string
	// owner holds the classes deployed before the rows ran (owner org only).
	owner map[string]string
	// istest names the deployed @IsTest observer, its fixture directory and
	// the OBSERVED key its assertion message transports.
	istestDir, istestClass, istestKey string
}

func (r namespaceFamilyReplay) run(t *testing.T, rows []namespaceFamilyRow) {
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		t.Run(api, func(t *testing.T) {
			index := namespaceFamilyProject(t, r.namespace, api, r.owner)
			org := orgFromIndex(index)
			org.APIVersion = api
			seen := 0
			for _, row := range rows {
				if row.API != api {
					continue
				}
				seen++
				observed := namespaceFamilySubstitute(row.Observed)
				switch row.Route {
				case "anonymous":
					source := namespaceFamilyFixture(t, r.dir, row.Source)
					errs := subscriberErrors(sema.AnalyzeAnonymous(index, source, api).Diagnostics)
					if row.Compile == "COMPILE_ERROR" {
						m := namespaceFamilyDiagnostic.FindStringSubmatch(observed)
						if m == nil {
							t.Fatalf("%s: unparsable native diagnostic %q", row.Case, observed)
						}
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
						t.Errorf("%s anonymous %s: execution error %v; native %q", row.Case, api, err, observed)
						continue
					}
					want := "OBSERVED|" + observed
					found := false
					for _, line := range result.Debug {
						found = found || strings.HasSuffix(line, want)
					}
					if !found {
						t.Errorf("%s anonymous %s: debug %q lacks %q", row.Case, api, result.Debug, want)
					}
				case "istest-checkonly":
					name := strings.TrimSuffix(filepath.Base(row.Source), ".cls")
					classes := map[string]string{name: namespaceFamilyFixture(t, r.dir, row.Source)}
					for k, v := range r.owner {
						classes[k] = v
					}
					m := namespaceFamilyDiagnostic.FindStringSubmatch(observed)
					if m == nil {
						t.Fatalf("%s: unparsable native diagnostic %q", row.Case, observed)
					}
					got := namespaceFamilyFileErrors(sema.Analyze(namespaceFamilyProject(t, r.namespace, api, classes)).Diagnostics, name)
					if !slices.Contains(got, m[3]) {
						t.Errorf("%s named %s: Glade %q; native %q", row.Case, api, got, m[3])
					}
				case "istest":
					classes := namespaceFamilyClasses(t, r.dir, r.istestDir)
					for k, v := range r.owner {
						classes[k] = v
					}
					rtIndex := namespaceFamilyProject(t, r.namespace, api, classes)
					if errs := subscriberErrors(sema.Analyze(rtIndex).Diagnostics); len(errs) > 0 {
						t.Errorf("%s istest %s: deployed observer reports errors %q; native deploys", row.Case, api, errs)
					}
					run := Run(rtIndex, Options{SelectedClasses: []string{r.istestClass}, NoDiskCache: true, Parallelism: 1})
					found := false
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem == nil {
								continue
							}
							if _, value, ok := strings.Cut(c.Problem.Message, "OBSERVED|"+r.istestKey+"|"); ok {
								found = true
								if value != observed {
									t.Errorf("%s istest %s: Glade %q; native %q", row.Case, api, value, observed)
								}
							}
						}
					}
					if !found {
						t.Errorf("%s istest %s: no observation (summary %+v problem %s)", row.Case, api, run.Summary(), firstRunProblem(run))
					}
				default:
					t.Fatalf("%s: route %q", row.Case, row.Route)
				}
			}
			if seen == 0 {
				t.Fatalf("no rows at %s", api)
			}
		})
	}
}

func TestNamespaceExternalPositionsNativeConformance(t *testing.T) {
	rows := readNamespaceFamilyRows(t, namespaceExternalPositionsDir)
	if len(rows) != 48 {
		t.Fatalf("org.tsv rows=%d, want 48", len(rows))
	}
	namespaceFamilyReplay{
		dir:         namespaceExternalPositionsDir,
		owner:       map[string]string{},
		istestDir:   "controls",
		istestClass: "ExtForNameOnlyV035Test",
		istestKey:   "forName",
	}.run(t, rows)
}

func TestNamespaceOwnerNestedInventoryNativeConformance(t *testing.T) {
	rows := readNamespaceFamilyRows(t, namespaceOwnerNestedInventoryDir)
	if len(rows) != 50 {
		t.Fatalf("org.tsv rows=%d, want 50", len(rows))
	}
	namespaceFamilyReplay{
		dir:         namespaceOwnerNestedInventoryDir,
		namespace:   namespaceOwnerNestedInventoryName,
		owner:       namespaceFamilyClasses(t, namespaceOwnerNestedInventoryDir, "owner"),
		istestDir:   "transport",
		istestClass: "V035ReflectTest",
		istestKey:   "reflect",
	}.run(t, rows)
}

// testdata/conformance/namespace_unqualified_root/org.tsv: an unqualified
// root that names no class and no namespace (RootV035) rejects like an
// absent namespace in every type position, and a static read or call
// through it is `Variable does not exist: <receiver>`. Anonymous and @IsTest
// check-only routes at API 62.0 and 67.0.
func TestNamespaceUnqualifiedRootNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/namespace_unqualified_root"
	rows := readNamespaceFamilyRows(t, dir)
	if len(rows) != 36 {
		t.Fatalf("org.tsv rows=%d, want 36", len(rows))
	}
	namespaceFamilyReplay{dir: dir, owner: map[string]string{}}.run(t, rows)
}

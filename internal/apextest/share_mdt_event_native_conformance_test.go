package apextest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native rows for four shapes found in the FormulaShare corpus run, captured
// from scratch orgs at API 62.0 and 67.0 (66.0 for the event handler):
//
//   - SOQL on a custom object's __Share table, as the administrator and as a
//     runAs user whose profile has no object access of its own;
//   - MasterLabel, DeveloperName and custom fields of a custom metadata type in
//     a USER_MODE query by a runAs user holding a Custom Metadata Types grant;
//   - BatchApexErrorEvent publication with field values from a named test;
//   - ORDER BY MasterLabel and Label on a custom metadata type.
//
// Provenance: docs/native-evidence/share_mdt_native_fixtures/native-run.md.
// Each fixture holds the owned project, cases.tsv and the capture's org.tsv.
// Rows whose residue column starts with tainted: are not answers and are
// skipped. A row asserts every test method (istest) or the debug line
// (anonymous) of the case at its API.
type shareMdtFixture struct {
	dir string
	// classes lists the test classes of the project that the istest rows run.
	classes map[string]string
}

var shareMdtFixtures = []shareMdtFixture{
	{dir: "share_mdt_basic_queries", classes: map[string]string{"t_share": "ShareProbeTest"}},
	{dir: "share_mdt_runas_access", classes: map[string]string{"t_runas": "RunAsProbeTest"}},
	{dir: "share_table_visibility", classes: map[string]string{"t_runas": "RunAsProbeTest"}},
	{dir: "share_mdt_event_runas", classes: map[string]string{"t_evt": "EvtProbeTest"}},
}

var (
	shareMdtUUID      = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	shareMdtMethodRow = regexp.MustCompile(`; (\w+)=(?:Pass|Fail)`)
)

func shareMdtNormalize(text string) string {
	text = strings.ReplaceAll(text, "&#124;", "|")
	text = strings.ReplaceAll(text, `\n`, "\n")
	text = shareMdtUUID.ReplaceAllString(text, "UUID")
	text = strings.TrimPrefix(text, "System.")
	return strings.TrimSpace(text)
}

// shareMdtNativeMethods splits "m1=Fail x; m2=Fail y" into method -> outcome.
func shareMdtNativeMethods(outcome string) map[string]string {
	out := map[string]string{}
	locs := shareMdtMethodRow.FindAllStringSubmatchIndex("; "+outcome, -1)
	text := "; " + outcome
	for i, loc := range locs {
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		method := text[loc[2]:loc[3]]
		value := text[loc[3]+1 : end]
		value = strings.TrimPrefix(strings.TrimPrefix(value, "Fail "), "Pass ")
		out[method] = value
	}
	return out
}

func TestShareMdtEventNativeConformance(t *testing.T) {
	for _, fixture := range shareMdtFixtures {
		fixture := fixture
		t.Run(fixture.dir, func(t *testing.T) {
			dir := filepath.Join("testdata", "conformance", fixture.dir)
			raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			cases := map[string][2]string{}
			caseRaw, err := os.ReadFile(filepath.Join(dir, "cases.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(strings.TrimSpace(string(caseRaw)), "\n") {
				fields := strings.Split(line, "\t")
				if len(fields) < 3 {
					t.Fatalf("invalid case row %q", line)
				}
				cases[fields[0]] = [2]string{fields[1], fields[2]}
			}
			lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
			header := strings.Split(lines[0], "\t")
			column := map[string]int{}
			for i, name := range header {
				column[name] = i
			}
			for _, name := range []string{"id", "api", "route", "outcome", "residue"} {
				if _, ok := column[name]; !ok {
					t.Fatalf("org.tsv lacks column %s", name)
				}
			}
			indexes := map[string]*shareMdtProject{}
			asserted := 0
			for n, line := range lines[1:] {
				fields := strings.Split(line, "\t")
				if len(fields) < len(header)-1 {
					t.Fatalf("org.tsv:%d short row", n+2)
				}
				id, api, route := fields[column["id"]], fields[column["api"]], fields[column["route"]]
				outcome, residue := fields[column["outcome"]], fields[column["residue"]]
				if strings.HasPrefix(residue, "tainted") {
					continue
				}
				c, ok := cases[id]
				if !ok {
					t.Fatalf("org.tsv:%d case %s is not in cases.tsv", n+2, id)
				}
				label := id + "/" + api + "/" + route
				project := indexes[api]
				if project == nil {
					project = loadShareMdtProject(t, dir, api)
					indexes[api] = project
				}
				if route == "istest" {
					want := shareMdtNativeMethods(outcome)
					if len(want) == 0 {
						t.Fatalf("org.tsv:%d no method outcomes in %q", n+2, outcome)
					}
					got := project.runTestClass(t, fixture.classes[id])
					for method, expected := range want {
						asserted++
						t.Run(label+"/"+method, func(t *testing.T) {
							actual, ok := got[method]
							if !ok {
								t.Fatalf("Glade did not run %s", method)
							}
							if shareMdtNormalize(actual) != shareMdtNormalize(expected) {
								t.Fatalf("NATIVE %s org.tsv:%d %s %s\nnative <%s>\nGlade  <%s>", fixture.dir, n+2, route, api, shareMdtNormalize(expected), shareMdtNormalize(actual))
							}
						})
					}
					continue
				}
				asserted++
				t.Run(label, func(t *testing.T) {
					actual := project.runAnonymous(t, filepath.Join(dir, c[1]))
					if shareMdtNormalize("OUT:"+actual) != shareMdtNormalize(outcome) {
						t.Fatalf("NATIVE %s org.tsv:%d %s %s\nnative <%s>\nGlade  <OUT:%s>", fixture.dir, n+2, route, api, shareMdtNormalize(outcome), shareMdtNormalize(actual))
					}
				})
			}
			if asserted == 0 {
				t.Fatal("no native row asserted")
			}
		})
	}
}

type shareMdtProject struct {
	index typesys.Index
	api   string
}

func loadShareMdtProject(t *testing.T, dir, api string) *shareMdtProject {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(dir, "project")
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(raw), "62.0", api))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native accepted project at %s; Glade: %v %v", api, index.Diagnostics, analysis.Diagnostics)
	}
	return &shareMdtProject{index: index, api: api}
}

// runTestClass returns "Type: message" (or "pass") by method name.
func (p *shareMdtProject) runTestClass(t *testing.T, class string) map[string]string {
	t.Helper()
	run := Run(p.index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
	out := map[string]string{}
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Problem == nil {
				out[c.MethodName] = "pass"
				continue
			}
			out[c.MethodName] = c.Problem.Type + ": " + c.Problem.Message
		}
	}
	return out
}

// runAnonymous runs one anonymous script and returns the text after "OUT:".
func (p *shareMdtProject) runAnonymous(t *testing.T, file string) string {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	analysis := sema.AnalyzeAnonymous(p.index, string(raw), p.api)
	program, err := vm.CompileAnonymousWithOptions(string(raw), vm.CompileOptions{APIVersion: p.api})
	if analysis.HasErrors() || err != nil {
		t.Fatalf("native accepted anonymous; Glade: %v %v", analysis.Diagnostics, err)
	}
	org := orgFromIndex(p.index)
	org.APIVersion = p.api
	result, err := newConformanceRunner(t, p.index, conformanceRunnerOptions{LinkProject: true, Org: &org}).execute(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range result.Debug {
		if rest, ok := strings.CutPrefix(line, "OUT:"); ok {
			return rest
		}
	}
	t.Fatalf("no OUT: line in %v", result.Debug)
	return ""
}

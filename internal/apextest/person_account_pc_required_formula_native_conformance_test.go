package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The native probe derives its record suffix from Crypto.getRandomLong. The
// test fixes that suffix so the captured {u} placeholder is reproducible.
const personAccountPCRandomSuffix = "4242"

var (
	personAccountPCC1 = regexp.MustCompile(`^C1 biz=(true|false)(.*?) pa-no-pc=(true|false)(.*?) pa-pc=(true|false)(.*?) nillable=(true|false) createable=(true|false)$`)
	personAccountPCC2 = regexp.MustCompile(`^C2 acct=(.*?)\|(.*?) cont=(.*?)\|(.*)$`)
)

type personAccountPCCases struct {
	Files     map[string]string
	Anonymous string
}

func loadPersonAccountPCFixture(t *testing.T, dir string) (personAccountPCCases, map[string]map[string]any) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases personAccountPCCases
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	wants := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		row := strings.SplitN(line, "\t", 4)
		if len(row) != 4 {
			t.Fatal(line)
		}
		key := row[0] + "/" + row[1]
		if wants[key] == nil {
			wants[key] = map[string]any{}
		}
		var value any
		if err := json.Unmarshal([]byte(strings.ReplaceAll(row[3], "{u}", personAccountPCRandomSuffix)), &value); err != nil {
			t.Fatal(err)
		}
		if _, ok := wants[key][row[2]]; ok {
			t.Fatal("duplicate native row")
		}
		wants[key][row[2]] = value
	}
	if len(wants) != 4 {
		t.Fatalf("native routes=%d", len(wants))
	}
	return cases, wants
}

func personAccountPCSource(body string) string {
	return strings.ReplaceAll(body, "Crypto.getRandomLong()", personAccountPCRandomSuffix+"L")
}

// parsePersonAccountPCObservation splits the probe's C1/C2 lines into the
// org.tsv row ids. Lines that do not match keep their raw text for diagnosis.
func parsePersonAccountPCObservation(got map[string]any, line string) {
	line = strings.TrimSpace(line)
	if match := personAccountPCC1.FindStringSubmatch(line); match != nil {
		insert := func(success, errs string) any {
			return []any{success == "true", strings.TrimSpace(errs)}
		}
		got["biz_insert"] = insert(match[1], match[2])
		got["pa_no_pc_insert"] = insert(match[3], match[4])
		got["pa_pc_insert"] = insert(match[5], match[6])
		got["must_describe"] = map[string]any{"nillable": match[7] == "true", "createable": match[8] == "true"}
		return
	}
	if match := personAccountPCC2.FindStringSubmatch(line); match != nil {
		got["mail_read"] = []any{match[1], match[3]}
		got["key_read"] = []any{match[2], match[4]}
		return
	}
	got["unparsed:"+line] = line
}

func TestPersonAccountPCRequiredFormulaNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/person_account_pc_required_formula"
	cases, wants := loadPersonAccountPCFixture(t, fixture)
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
		writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [PersonAccounts]\n")
		for name, body := range cases.Files {
			writeFile(t, filepath.Join(root, name), strings.ReplaceAll(personAccountPCSource(body), "62.0", api))
		}
		index := loadTestIndex(t, root)
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				want := wants[api+"/"+route]
				if len(want) != 6 {
					t.Fatalf("native rows=%d", len(want))
				}
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native declarations accepted; Glade: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				got := map[string]any{}
				if route == "anonymous" {
					if analysis := sema.AnalyzeAnonymous(index, cases.Anonymous, api); analysis.HasErrors() {
						t.Fatal(analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(cases.Anonymous, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					result, err := runner.newMachine().Execute(program)
					if err != nil {
						t.Fatalf("native anonymous run succeeded; Glade: %v", err)
					}
					for _, line := range result.Debug {
						if _, observation, ok := strings.Cut(line, "OUT "); ok {
							parsePersonAccountPCObservation(got, observation)
						}
					}
				} else {
					run := Run(index, Options{SelectedClasses: []string{"PaCapTest"}, NoDiskCache: true, Parallelism: 1})
					if run.Summary().Total != 2 || len(run.Suites) != 1 {
						t.Fatalf("unexpected selection: %+v", run.Summary())
					}
					for _, result := range run.Suites[0].Cases {
						if result.Problem == nil || flowRefsExceptionType(result.Problem.Type) != "System.AssertException" {
							t.Fatalf("missing observation: %+v", result)
						}
						observation := result.Problem.Message
						if _, after, found := strings.Cut(observation, "Assertion Failed: "); found {
							observation = after
						}
						parsePersonAccountPCObservation(got, observation)
					}
				}
				if len(got) != len(want) {
					t.Fatalf("observations=%v, want %d rows", got, len(want))
				}
				for id, value := range want {
					t.Run(id, func(t *testing.T) {
						if !reflect.DeepEqual(got[id], value) {
							t.Fatalf("native=%v; Glade=%v", value, got[id])
						}
					})
				}
			})
		}
	}
}

package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native rows for four surfaces that ordinary code leans on: JSON, Datetime
// and locale, collections and strings, and async options. Each family holds
// org.tsv (case, api, route, outcome), the probe project and the anonymous
// sources. The two repeats and both APIs agreed, so one row per API is kept.
// Outcomes are the full native text with record ids masked to <id>.
//
// parkedSurfRows lists rows whose native answer Glade does not match yet. A
// parked row fails once Glade matches, so the entry is removed with the fix.
var parkedSurfRows = map[string]string{
	"collections_strings/cs03/62.0": "SObject.hashCode() returns a platform-specific integer; the native algorithm is not derivable from two captured values",
	"collections_strings/cs03/67.0": "same as cs03 at API 62",
}

var surfIDMask = regexp.MustCompile(`\b(?:001|003|005|006|00D|012|01p|a0[A-Za-z0-9])[0-9A-Za-z]{11,15}\b`)

func TestSurfNativeConformance(t *testing.T) {
	for _, family := range []string{"json", "datetime_locale", "collections_strings", "async_options"} {
		t.Run(family, func(t *testing.T) { runSurfFamily(t, family) })
	}
}

type surfCase struct {
	id, route, file string
}

func readSurfTSV(t *testing.T, path string) [][]string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for i, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
		if i == 0 {
			continue
		}
		rows = append(rows, strings.SplitN(line, "\t", 4))
	}
	return rows
}

func runSurfFamily(t *testing.T, family string) {
	dir := filepath.Join("testdata", "conformance", "surf_"+family)
	var cases []surfCase
	for _, row := range readSurfTSV(t, filepath.Join(dir, "cases.tsv")) {
		cases = append(cases, surfCase{row[0], row[1], row[2]})
	}
	native := map[string]string{}
	for i, row := range readSurfTSV(t, filepath.Join(dir, "org.tsv")) {
		if len(row) != 4 {
			t.Fatalf("org.tsv:%d invalid row", i+2)
		}
		var text string
		if err := json.Unmarshal([]byte(row[3]), &text); err != nil {
			t.Fatalf("org.tsv:%d outcome must be a JSON string: %v", i+2, err)
		}
		native[row[0]+"/"+row[1]] = text
	}
	if len(native) != 2*len(cases) {
		t.Fatalf("native rows %d, want %d", len(native), 2*len(cases))
	}
	// Queried records carry the org instance's REST version in their url, which
	// differs from the class API version under test.
	runtimeAPI := ""
	urlVersion := regexp.MustCompile(`/services/data/v([0-9]+\.0)/`)
	for _, text := range native {
		if match := urlVersion.FindStringSubmatch(text); match != nil {
			runtimeAPI = match[1]
		}
	}
	releaseRuntimeCachesAfterTest(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			classes := filepath.Join(root, "force-app/main/default/classes")
			entries, err := os.ReadDir(filepath.Join(dir, "project"))
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			for _, entry := range entries {
				body, err := os.ReadFile(filepath.Join(dir, "project", entry.Name()))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(classes, entry.Name()), string(body))
				writeFile(t, filepath.Join(classes, entry.Name()+"-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project accepted; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			check := func(t *testing.T, c surfCase, got string) {
				key := family + "/" + c.id + "/" + api
				want := native[c.id+"/"+api]
				got = surfIDMask.ReplaceAllString(got, "<id>")
				if reason, parked := parkedSurfRows[key]; parked {
					if got == want {
						t.Errorf("stale parked row (%s); Glade now matches", reason)
					}
					return
				}
				if got != want {
					t.Fatalf("native:\n%s\nGlade:\n%s", want, got)
				}
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: func() *storage.OrgState {
				o := orgFromIndex(index)
				o.APIVersion = api
				if runtimeAPI != "" {
					o.APIVersion = runtimeAPI
				}
				return &o
			}()})
			var tests []string
			byClass := map[string]surfCase{}
			for _, c := range cases {
				if c.route == "istest" {
					tests = append(tests, c.file)
					byClass[c.file] = c
				}
			}
			for _, c := range cases {
				if c.route != "anonymous" {
					continue
				}
				t.Run(c.id, func(t *testing.T) {
					source, err := os.ReadFile(filepath.Join(dir, c.file))
					if err != nil {
						t.Fatal(err)
					}
					check(t, c, runSurfAnonymous(t, runner, index, string(source), api))
				})
			}
			if len(tests) == 0 {
				return
			}
			sort.Strings(tests)
			result := Run(index, Options{SelectedClasses: tests, RuntimeRESTAPIVersion: runtimeAPI, NoDiskCache: true, Parallelism: 1})
			observed := map[string]string{}
			for _, suite := range result.Suites {
				for _, c := range suite.Cases {
					if c.Problem == nil {
						observed[c.ClassName] = "Pass|"
						continue
					}
					observed[c.ClassName] = "Fail|" + c.Problem.Type + ": " + c.Problem.Message
				}
			}
			for _, class := range tests {
				c := byClass[class]
				t.Run(c.id, func(t *testing.T) {
					got, ok := observed[class]
					if !ok {
						t.Fatalf("%s did not run", class)
					}
					check(t, c, got)
				})
			}
		})
	}
}

func runSurfAnonymous(t *testing.T, runner *conformanceRunner, index typesys.Index, source, api string) string {
	t.Helper()
	if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
		for _, d := range analysis.Diagnostics {
			message := d.Message
			if d.NativeMessage != "" {
				message = d.NativeMessage
			}
			return "COMPILE|" + message
		}
	}
	program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
	if err != nil {
		return "COMPILE|" + err.Error()
	}
	result, err := runner.execute(program)
	lines := append([]string(nil), result.Debug...)
	if err != nil {
		lines = append(lines, "EXC|"+err.Error())
	}
	// The native debug log carries a carriage return in a message as a line
	// feed (cs06 csvCr), so compare on that footing.
	return strings.ReplaceAll(strings.Join(lines, "\n"), "\r", "\n")
}

package apextest

import (
	"encoding/json"
	"fmt"
	"github.com/glade-sh/glade/internal/config"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
var parkedSurfRows = map[string]string{}

var surfIDMask = regexp.MustCompile(`\b(?:001|003|005|006|00D|012|01p|a0[A-Za-z0-9])[0-9A-Za-z]{11,15}\b`)

// maskSurfGeneratedIDs masks ids Glade generates. The synthetic ids that the
// probe sources write literally (001000000000... and 003000000000...) stay
// visible, so a 15- versus 18-character text difference is not hidden.
func maskSurfGeneratedIDs(text string) string {
	return surfIDMask.ReplaceAllStringFunc(text, func(id string) string {
		if strings.HasPrefix(id, "001000000000") || strings.HasPrefix(id, "003000000000") {
			return id
		}
		return "<id>"
	})
}

func TestSurfNativeConformance(t *testing.T) {
	for _, family := range []string{"json", "datetime_locale", "collections_strings", "async_options"} {
		t.Run(family, func(t *testing.T) { runSurfFamily(t, family) })
	}
}

type surfCase struct {
	id, route, file string
	// user holds "locale|timeZone" for an anonymous case that ran as a user
	// with those settings (an optional fourth cases.tsv column).
	user string
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
		user := ""
		if len(row) > 3 {
			user = row[3]
		}
		cases = append(cases, surfCase{row[0], row[1], row[2], user})
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
			root := func() string {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				classes := filepath.Join(root, "force-app/main/default/classes")
				entries, err := os.ReadDir(filepath.Join(dir, "project"))
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if entry.IsDir() {
						// Custom object metadata (objects/<Name>/...) deploys as is.
						err := filepath.WalkDir(filepath.Join(dir, "project", entry.Name()), func(path string, d os.DirEntry, err error) error {
							if err != nil || d.IsDir() {
								return err
							}
							rel, err := filepath.Rel(filepath.Join(dir, "project"), path)
							if err != nil {
								return err
							}
							body, err := os.ReadFile(path)
							if err != nil {
								return err
							}
							writeFile(t, filepath.Join(root, "force-app/main/default", rel), string(body))
							return nil
						})
						if err != nil {
							t.Fatal(err)
						}
						continue
					}
					body, err := os.ReadFile(filepath.Join(dir, "project", entry.Name()))
					if err != nil {
						t.Fatal(err)
					}
					if !strings.HasSuffix(entry.Name(), ".cls") {
						// glade.yml replays the permission sets the capture assigned.
						writeFile(t, filepath.Join(root, entry.Name()), string(body))
						continue
					}
					writeFile(t, filepath.Join(classes, entry.Name()), string(body))
					writeFile(t, filepath.Join(classes, entry.Name()+"-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				}
				return root
			}()
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project accepted; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			check := func(t *testing.T, c surfCase, got string) {
				key := family + "/" + c.id + "/" + api
				want := native[c.id+"/"+api]
				got = maskSurfGeneratedIDs(got)
				if strings.HasPrefix(c.id, "dth_") || strings.HasPrefix(c.id, "dec_") {
					// The test-result message drops trailing whitespace.
					want, got = strings.TrimRight(want, " \t\r\n"), strings.TrimRight(got, " \t\r\n")
				}
				if strings.HasPrefix(c.id, "dth_") {
					want, got = maskSurfNowTail(want), maskSurfNowTail(got)
				}
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
				o := capturedFixtureOrg(t, index, api)
				if runtimeAPI != "" {
					o.APIVersion = runtimeAPI
				}
				return &o
			}()})
			// An istest case file is a class, or Class.method when a class holds
			// several captured methods (the hash families).
			var tests []string
			seenClass := map[string]bool{}
			byClass := map[string]surfCase{}
			for _, c := range cases {
				if c.route == "istest" {
					class, _, _ := strings.Cut(c.file, ".")
					if !seenClass[class] {
						seenClass[class] = true
						tests = append(tests, class)
					}
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
					if c.user != "" {
						locale, zone, _ := strings.Cut(c.user, "|")
						configure := func(machine *vm.VM) {
							if err := machine.ConfigureTestEnvironment(config.TestConfig{User: config.TestUserConfig{Locale: locale, TimeZone: zone, Language: "en_US"}}); err != nil {
								t.Fatal(err)
							}
						}
						check(t, c, runSurfAnonymous(t, runner, index, string(source), api, configure))
						return
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
						observed[c.ClassName+"."+c.MethodName] = "Pass|"
						continue
					}
					observed[c.ClassName] = "Fail|" + c.Problem.Type + ": " + c.Problem.Message
					observed[c.ClassName+"."+c.MethodName] = observed[c.ClassName]
				}
			}
			var files []string
			for file := range byClass {
				files = append(files, file)
			}
			sort.Strings(files)
			for _, file := range files {
				c := byClass[file]
				t.Run(c.id, func(t *testing.T) {
					got, ok := observed[file]
					if !ok {
						t.Fatalf("%s did not run", file)
					}
					check(t, c, got)
				})
			}
		})
	}
}

func runSurfAnonymous(t *testing.T, runner *conformanceRunner, index typesys.Index, source, api string, configure ...func(*vm.VM)) string {
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
	result, err := runner.execute(program, configure...)
	lines := append([]string(nil), result.Debug...)
	if err != nil {
		lines = append(lines, "EXC|"+err.Error())
	}
	// The native debug log carries a carriage return in a message as a line
	// feed (cs06 csvCr), so compare on that footing.
	return strings.ReplaceAll(strings.Join(lines, "\n"), "\r", "\n")
}

// surfNowTailMarkers start the part of a Datetime hash payload that holds a
// value built from the clock (an inserted record's CreatedDate and its fields).
// The instant differs on every run, so the tail is compared by its identity:
// each hash that precedes a millisecond value must be Long.hashCode(ms) XOR
// 577755. A tail that holds the identity compares as masked text.
var surfNowTailMarkers = []string{"|S=", "|SOQL="}

var surfNumberToken = regexp.MustCompile(`-?\d+`)

func maskSurfNowTail(text string) string {
	for _, marker := range surfNowTailMarkers {
		at := strings.LastIndex(text, marker)
		if at < 0 {
			continue
		}
		head, tail := text[:at], text[at:]
		prev := int64(0)
		havePrev := false
		ok := true
		masked := surfNumberToken.ReplaceAllStringFunc(tail, func(token string) string {
			n, err := strconv.ParseInt(token, 10, 64)
			if err != nil {
				ok = false
				return token
			}
			digits := strings.TrimPrefix(token, "-")
			if len(digits) >= 12 {
				// A millisecond value: the number before it is its hash.
				hash := int32(n^int64(uint64(n)>>32)) ^ 577755
				if !havePrev || int64(int32(prev)) != int64(hash) {
					ok = false
				}
			}
			prev, havePrev = n, true
			return "#"
		})
		if !ok {
			return text
		}
		return head + masked
	}
	return text
}

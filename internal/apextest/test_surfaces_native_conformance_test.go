package apextest

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native captures of test-facing surfaces: platform events, StubProvider,
// callout mocks, loadData and custom settings, roll-up summaries and async
// ordering. Each family holds one project per API (62.0 and 67.0), the case
// list, the org.tsv summary and observations.txt, the full untruncated
// native messages. Most cases end in System.assert(false, 'OBS|...'), so a
// failed test is the transport. Glade must give the same type and message.
//
// Stack line numbers and elapsed time are not compared. Ids are masked on
// both sides.

var testSurfaceFamilies = []string{
	"platform-events",
	"stub-provider",
	"callout-mocks",
	"loaddata-settings",
	"rollup-summary",
	"async-ordering",
}

// testSurfaceResiduals lists rows Glade does not match yet, by family/case. A
// listed row must still differ from native; when it starts to match, the test
// fails until the entry is removed, so the ledger only shrinks.
var testSurfaceResiduals = map[string]string{
	"rollup-summary/rollanon": "the probe puts the static events list into a Map and the final children delete appends the fifth AFTER_DELETE entry afterwards; Glade copies a static-origin list on Map.put (review_round2 x_alias shows native aliasing it); trigger firing matches the rollup_trigger_timing rows",
}

type testSurfaceObservation struct {
	method  string
	message string
	debug   string
}

var testSurfaceIDPattern = regexp.MustCompile(`\b(?:001|003|005|006|00D|00G|00Q|00e|01p|0Hp|0HM|707|a0[0-9A-Za-z])[0-9A-Za-z]{12}(?:[0-9A-Za-z]{3})?\b`)

// Row timestamps are wall-clock in the org and fixed in Glade.
var testSurfaceTimestampPattern = regexp.MustCompile(`@\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)

func maskTestSurfaceIDs(s string) string {
	return testSurfaceTimestampPattern.ReplaceAllString(testSurfaceIDPattern.ReplaceAllString(s, "<id>"), "@<time>")
}

// parseTestSurfaceObservations reads observations.txt into api -> case id.
func parseTestSurfaceObservations(t *testing.T, path string) map[string]map[string]testSurfaceObservation {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]testSurfaceObservation{}
	var api, id string
	var current testSurfaceObservation
	flush := func() {
		if id != "" {
			out[api][id] = current
		}
		id, current = "", testSurfaceObservation{}
	}
	for _, line := range strings.Split(string(body), "\n") {
		switch {
		case strings.HasPrefix(line, "## API "):
			flush()
			api = strings.TrimSpace(strings.TrimPrefix(line, "## API "))
			out[api] = map[string]testSurfaceObservation{}
		case strings.HasPrefix(line, "### "):
			flush()
			id, _, _ = strings.Cut(strings.TrimPrefix(line, "### "), " ")
		case strings.HasPrefix(line, "method="):
			current.method, _, _ = strings.Cut(strings.TrimPrefix(line, "method="), " ")
		case strings.HasPrefix(line, "message="):
			current.message = strings.TrimPrefix(line, "message=")
		case strings.HasPrefix(line, "debug="):
			current.debug = strings.TrimPrefix(line, "debug=")
		}
	}
	flush()
	return out
}

func readTestSurfaceCases(t *testing.T, path string) [][3]string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows [][3]string
	for i, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		if i == 0 {
			continue
		}
		cells := strings.Split(line, "\t")
		if len(cells) != 3 {
			t.Fatalf("%s:%d: bad case row %q", path, i+1, line)
		}
		rows = append(rows, [3]string{cells[0], cells[1], cells[2]})
	}
	return rows
}

func TestTestSurfacesNativeConformance(t *testing.T) {
	for _, family := range testSurfaceFamilies {
		family := family
		t.Run(family, func(t *testing.T) {
			dir := filepath.Join("testdata/conformance", family)
			observations := parseTestSurfaceObservations(t, filepath.Join(dir, "observations.txt"))
			for _, api := range []string{"62.0", "67.0"} {
				api := api
				t.Run(api, func(t *testing.T) {
					apiDir := filepath.Join(dir, "api"+strings.TrimSuffix(api, ".0"))
					cases := readTestSurfaceCases(t, filepath.Join(apiDir, "cases.tsv"))
					if len(cases) == 0 || len(observations[api]) != len(cases) {
						t.Fatalf("incomplete native rows: %d cases, %d observations", len(cases), len(observations[api]))
					}
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
					source := filepath.Join(apiDir, "force-app")
					if err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
						if err != nil || entry.IsDir() {
							return err
						}
						body, err := os.ReadFile(path)
						if err != nil {
							return err
						}
						rel, err := filepath.Rel(source, path)
						if err != nil {
							return err
						}
						writeFile(t, filepath.Join(root, "force-app", rel), string(body))
						return nil
					}); err != nil {
						t.Fatal(err)
					}
					index := loadTestIndex(t, root)
					ids := make([]string, 0, len(cases))
					byID := map[string][3]string{}
					for _, row := range cases {
						ids = append(ids, row[0])
						byID[row[0]] = row
					}
					sort.Strings(ids)
					for _, id := range ids {
						row := byID[id]
						want := observations[api][id]
						t.Run(id, func(t *testing.T) {
							residual, isResidual := testSurfaceResiduals[family+"/"+id]
							compare := func(wantText, gotText string) {
								matched := maskTestSurfaceIDs(gotText) == maskTestSurfaceIDs(wantText)
								switch {
								case isResidual && matched:
									t.Fatalf("NATIVE %s %s %s now matches; remove it from testSurfaceResiduals", family, api, id)
								case isResidual:
									// A known residual passes without comparing (Apex shards
									// reject skips); the case above fails once Glade matches.
									t.Logf("known residual: %s\nnative=%q\nglade =%q", residual, wantText, gotText)
								case !matched:
									t.Fatalf("NATIVE %s %s %s\nnative=%q\nglade =%q", family, api, id, wantText, gotText)
								}
							}
							if testSurfaceIDPattern.MatchString(want.message + want.debug) {
								t.Fatalf("native observation carries an unmasked id")
							}
							switch row[1] {
							case "istest":
								run := Run(index, Options{SelectedClasses: []string{row[2]}, SelectedMethod: want.method, NoDiskCache: true, Parallelism: 1})
								if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
									t.Fatalf("missing test runner case: %+v", run.Summary())
								}
								problem := run.Suites[0].Cases[0].Problem
								if problem == nil {
									compare(want.message, "passed")
									return
								}
								compare(want.message, problem.Type+": "+problem.Message)
							case "anonymous":
								body, err := os.ReadFile(filepath.Join(apiDir, row[2]))
								if err != nil {
									t.Fatal(err)
								}
								src := string(body)
								if analysis := sema.AnalyzeAnonymous(index, src, api); analysis.HasErrors() {
									t.Fatalf("native accepted anonymous: %v", analysis.Diagnostics)
								}
								program, err := vm.CompileAnonymousWithOptions(src, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								org := orgFromIndex(index)
								org.APIVersion = api
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
								triggers, triggerErrors := compileProjectTriggers(index)
								if len(triggerErrors) != 0 {
									t.Fatalf("trigger compile: %v", triggerErrors)
								}
								for _, trigger := range triggers {
									if err := runner.base.RegisterTrigger(trigger); err != nil {
										t.Fatal(err)
									}
								}
								result, err := runner.execute(program)
								if err != nil {
									t.Fatal(err)
								}
								var got string
								for _, line := range result.Debug {
									if _, rest, ok := strings.Cut(line, "OBS|"); ok {
										got = "OBS|" + rest
									}
								}
								_, wantObs, _ := strings.Cut(want.debug, "OBS|")
								compare("OBS|"+wantObs, got)
							default:
								t.Fatalf("unknown route %q", row[1])
							}
						})
					}
				})
			}
		})
	}
}

package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const aggregateGroupingRoutesFixture = "testdata/conformance/aggregate_grouping_routes"

// Native rows: aggregate_grouping_routes org.tsv (one fresh Developer scratch
// org, API 62.0 and 67.0, named @IsTest methods and anonymous scripts). A
// selected field that is neither grouped nor aggregated raises the uncatchable
// UnexpectedException when the dynamic query is evaluated as a value, and a
// catchable QueryException when Database.query is the iterable of a for loop,
// for every field order, field type, alias form and grouping shape captured.
func TestAggregateGroupingRoutesNativeConformance(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(aggregateGroupingRoutesFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 335 || lines[0] != "api\troute\tclass_or_script\tprobe\tobservation" {
		t.Fatalf("incomplete aggregate grouping rows: %d", len(lines))
	}
	root := t.TempDir()
	project := filepath.Join(aggregateGroupingRoutesFixture, "project")
	if err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(project, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, name), string(content))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// ORG-RUN.txt: NpAgg_Access was assigned to the running user.
	writeCapturedAssignments(t, root, "NpAgg_Access")
	index := loadTestIndex(t, root)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("project compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	named := map[string]string{}
	run := Run(index, Options{SelectedClasses: []string{"NpAgg62", "NpAgg67", "NpAggLoop62", "NpAggLoop67"}, NoDiskCache: true, Parallelism: 1})
	for _, suite := range run.Suites {
		for _, tc := range suite.Cases {
			observed := "PASS"
			if tc.Problem != nil {
				observed = "UNCAUGHT|" + tc.Problem.Type + ": " + tc.Problem.Message
				if _, capture, ok := strings.Cut(tc.Problem.Message, "CAPTURE|"); ok {
					if _, value, ok := strings.Cut(capture, "|"); ok {
						observed = strings.TrimSpace(value)
					}
				}
			}
			named[tc.ClassName+"."+tc.MethodName] = observed
		}
	}
	runners := map[string]*conformanceRunner{}
	for _, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		api, route, source, probe, want := cells[0], cells[1], cells[2], cells[3], cells[4]
		t.Run(api+"/"+route+"/"+probe, func(t *testing.T) {
			if route == "isTest" {
				got, ok := named[source]
				if !ok {
					t.Fatalf("missing named result %s", source)
				}
				if got != want {
					t.Fatalf("native %q; Glade %q", want, got)
				}
				return
			}
			script, err := os.ReadFile(filepath.Join(aggregateGroupingRoutesFixture, source))
			if err != nil {
				t.Fatal(err)
			}
			runner := runners[api]
			if runner == nil {
				org := capturedFixtureOrg(t, index, api)
				runner = newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
				runners[api] = runner
			}
			program, err := vm.CompileAnonymousWithOptions(string(script), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := runner.execute(program)
			got := ""
			if err != nil {
				got = "UNCAUGHT|" + err.Error()
			} else {
				for _, debug := range result.Debug {
					if _, capture, ok := strings.Cut(debug, "CAPTURE|"); ok {
						if _, value, ok := strings.Cut(capture, "|"); ok {
							got = strings.TrimSpace(value)
						}
					}
				}
			}
			if got != want {
				t.Fatalf("native %q; Glade %q", want, got)
			}
		})
	}
}

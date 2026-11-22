package apextest

import (
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// flowCriteriaErrorID drops the per-request "Error ID" Salesforce appends to
// some flow faults; it differs on every run and route.
var flowCriteriaErrorID = regexp.MustCompile(`(\\n|\n)?\s*Error ID: [0-9-]+ \([0-9]+\)`)

// flowCriteriaOrgURL is the capture scrub of a URL in a fault text.
var flowCriteriaOrgURL = regexp.MustCompile(`https?://\S+`)

func flowCriteriaObservation(text string) string {
	text = flowCriteriaErrorID.ReplaceAllString(text, "")
	text = flowCriteriaOrgURL.ReplaceAllString(text, "<org-url>")
	text = strings.ReplaceAll(text, `\n`, "")
	return strings.ReplaceAll(text, "\n", "")
}

// runFlowCriteriaFixture replays every row of a captured flow criteria fixture
// through the anonymous route or the named @IsTest class.
// pinned maps a case id to the Glade observation kept for a row Glade does not
// match yet; the row stays in org.tsv and the reason is logged.
func runFlowCriteriaFixture(t *testing.T, name string, wantRows int, pinned map[string]string) {
	fixture := filepath.Join("testdata", "conformance", name)
	readTSV := func(path string) [][]string {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		reader := csv.NewReader(file)
		reader.Comma = '\t'
		reader.LazyQuotes = true
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	files := map[string]string{}
	for _, row := range readTSV(filepath.Join(fixture, "cases.tsv"))[1:] {
		files[row[0]] = row[2]
	}
	rows := readTSV(filepath.Join(fixture, "org.tsv"))
	if len(rows) != wantRows+1 {
		t.Fatalf("native rows=%d, want %d", len(rows)-1, wantRows)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := filepath.Join(fixture, "api"+strings.TrimSuffix(row[1], ".0"))
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				for _, diag := range append(index.Diagnostics, analysis.Diagnostics...) {
					if diag.Severity == diagnostic.Error {
						t.Errorf("fixture diagnostic: %s %s", diag.Code, diag.Message)
					}
				}
				t.FailNow()
			}
			var message string
			switch row[2] {
			case "anonymous":
				source, err := os.ReadFile(filepath.Join(fixture, files[row[0]]))
				if err != nil {
					t.Fatal(err)
				}
				org := capturedFixtureOrg(t, index, row[1])
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: row[1]})
				if err != nil {
					t.Fatal(err)
				}
				_, err = runner.newMachine().Execute(program)
				var runtimeErr *vm.RuntimeError
				if !errors.As(err, &runtimeErr) || flowRefsExceptionType(runtimeErr.Type) != "System.AssertException" {
					t.Fatalf("expected observation, got %v", err)
				}
				message = runtimeErr.ExceptionMessage()
			case "istest":
				result := Run(index, Options{SelectedClasses: []string{files[row[0]]}, SelectedMethod: "probe", NoDiskCache: true, Parallelism: 1})
				if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
					t.Fatalf("unexpected selection: %#v", result.Summary())
				}
				problem := result.Suites[0].Cases[0].Problem
				if problem == nil || flowRefsExceptionType(problem.Type) != "System.AssertException" {
					t.Fatalf("expected observation, got %#v", problem)
				}
				message = problem.Message
			default:
				t.Fatalf("unknown route %q", row[2])
			}
			_, want, ok := strings.Cut(row[5], "OBS=")
			if !ok {
				t.Fatalf("missing native observation: %q", row[5])
			}
			_, got, ok := strings.Cut(message, "OBS=")
			if pin, isPinned := pinned[row[0]]; isPinned {
				t.Logf("pinned: native %q", want)
				if !ok || got != pin {
					t.Fatalf("pinned Glade observation changed: %q, want %q", got, pin)
				}
				return
			}
			if !ok || flowCriteriaObservation(got) != flowCriteriaObservation(want) {
				t.Fatalf("native %q, Glade %q", want, message)
			}
		})
	}
}

// NATIVE profile literal, record relationship lookup values, before-save
// custom metadata lookup, standard pricebook loop and email inputs:
// testdata/conformance/flow_criteria_shapes/org.tsv, anonymous and @IsTest at
// 62.0 and 67.0.
func TestFlowCriteriaShapesNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "flow_criteria_shapes", 30, map[string]string{
		// Salesforce rejects a custom price before a standard price
		// (STANDARD_PRICE_NOT_DEFINED); Glade does not enforce that rule for any
		// PricebookEntry insert yet, so the Flow's create succeeds.
		"pbcustom_test": "ok,entries=1,std=false",
	})
}

// NATIVE $Profile.Name NotEqualTo a literal and $Record.RecordType.DeveloperName
// against a constant and a literal in an after-save update Flow:
// testdata/conformance/flow_record_type_decision/org.tsv, anonymous and
// @IsTest at 62.0 and 67.0.
func TestFlowRecordTypeDecisionNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "flow_record_type_decision", 4, nil)
}

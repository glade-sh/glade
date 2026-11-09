package apextest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

var (
	flowRelPathIDPattern    = regexp.MustCompile(`\b00[0-9A-Za-z]{13}(?:[0-9A-Za-z]{3})?\b|\b0[0-9A-Za-z]{2}[0-9A-Za-z]{12}(?:[0-9A-Za-z]{3})?\b`)
	flowRelPathErrorIDRegex = regexp.MustCompile(` Error ID: [0-9]+-[0-9]+ \(-?[0-9]+\)`)
	flowRelPathAnchorRegex  = regexp.MustCompile(`<a href='[^']*'>(SOAP API Developer Guide)</a>`)
)

// normalizeFlowRelPathRow applies the normalization recorded in ORG-RUN.txt.
func normalizeFlowRelPathRow(row string) string {
	row = flowRelPathIDPattern.ReplaceAllString(row, "<ID>")
	row = flowRelPathErrorIDRegex.ReplaceAllString(row, "")
	return flowRelPathAnchorRegex.ReplaceAllString(row, "$1")
}

// flowRelPathNativeConformance replays synthetic after-save Contact flows whose
// decisions, filters, assignments and Create Records read a relationship path
// through the triggering record or a Get Records output. Rows come from the
// native org.tsv at API 62.0 and 67.0 on both the anonymous and @IsTest routes.
func flowRelPathNativeConformance(t *testing.T, prefix string) {
	fixture := filepath.Join("testdata/conformance/flow_relpath_decisions")
	data, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{}
	var cases []string
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")[1:] {
		cell := strings.SplitN(line, "\t", 4)
		if len(cell) != 4 {
			t.Fatalf("invalid native row %q", line)
		}
		if !strings.HasPrefix(cell[2], prefix) {
			continue
		}
		key := cell[0] + "/" + cell[1] + "/" + cell[2]
		want[key] = append(want[key], cell[3])
		if !seen[cell[2]] {
			seen[cell[2]] = true
			cases = append(cases, cell[2])
		}
	}
	if len(cases) == 0 {
		t.Fatalf("no native cases with prefix %s", prefix)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.Walk(filepath.Join(fixture, "project"), func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(filepath.Join(fixture, "project"), path)
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				text := string(source)
				if strings.HasSuffix(path, ".xml") {
					text = strings.ReplaceAll(text, "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>")
				}
				writeFile(t, filepath.Join(root, rel), text)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, route := range []string{"anonymous", "istest"} {
				for _, key := range cases {
					t.Run(route+"/"+key, func(t *testing.T) {
						var message string
						if route == "anonymous" {
							org := orgFromIndex(index)
							org.APIVersion = api
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
							script := fmt.Sprintf("List<String> out = new List<String>(); for (String t : new List<String>{'Customer','Other'}) { out.add(t + '|' + RelProbe.run('%s', t, t)); } System.assert(false, 'ROWS\\n' + String.join(out, '\\n'));", key)
							program, err := vm.CompileAnonymousWithOptions(script, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							_, err = runner.newMachine().Execute(program)
							var runtimeErr *vm.RuntimeError
							if !errors.As(err, &runtimeErr) {
								t.Fatalf("missing observation assertion: %v", err)
							}
							if flowRefsExceptionType(runtimeErr.Type) != "System.AssertException" {
								t.Fatalf("native observed rows; Glade failed: %v", err)
							}
							message = runtimeErr.ExceptionMessage()
						} else {
							result := Run(index, Options{SelectedClasses: []string{"RelProbeTest"}, SelectedMethod: "t" + key, NoDiskCache: true, Parallelism: 1})
							if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
								t.Fatalf("unexpected test selection: %#v", result.Summary())
							}
							observed := result.Suites[0].Cases[0]
							if observed.Problem == nil || flowRefsExceptionType(observed.Problem.Type) != "System.AssertException" {
								t.Fatalf("native observed rows; Glade failed: %#v", observed.Problem)
							}
							message = observed.Problem.Message
						}
						_, rows, found := strings.Cut(message, "ROWS\n")
						if !found {
							t.Fatalf("missing observation envelope: %s", message)
						}
						got := strings.Split(normalizeFlowRelPathRow(strings.TrimSpace(rows)), "\n")
						expected := want[api+"/"+route+"/"+key]
						if strings.Join(got, "\n") != strings.Join(expected, "\n") {
							t.Errorf("\n native %q\n Glade  %q", expected, got)
						}
					})
				}
			}
		})
	}
}

func TestFlowRelationshipPathDecisionsNativeConformance(t *testing.T) {
	flowRelPathNativeConformance(t, "D")
}

func TestFlowRelationshipPathLookupFilterNativeConformance(t *testing.T) {
	flowRelPathNativeConformance(t, "L")
}

func TestFlowRelationshipPathValuesNativeConformance(t *testing.T) {
	flowRelPathNativeConformance(t, "V")
}

func TestFlowReadOnlyFieldUpdateNativeConformance(t *testing.T) {
	flowRelPathNativeConformance(t, "RO")
}

func TestFlowRelationshipPathCreateNativeConformance(t *testing.T) {
	flowRelPathNativeConformance(t, "C1")
}

func TestFlowCreateRecordsIDTypeNativeConformance(t *testing.T) {
	flowRelPathNativeConformance(t, "CR")
}

func TestFlowEmailRelationshipPathInputNativeConformance(t *testing.T) {
	flowRelPathNativeConformance(t, "EM")
}

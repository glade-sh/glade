package apextest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// TestFlowDecisionShapesAnonymousAndIsTestNativeConformance replays a synthetic project of
// record-triggered Flows through the anonymous and @IsTest routes and compares
// the observed rows with the native org.tsv at API 62.0 and 67.0.
func TestFlowDecisionShapesAnonymousAndIsTestNativeConformance(t *testing.T) {
	fixture := filepath.Join("testdata/conformance/flow_decision_shapes")
	data, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")[1:] {
		cell := strings.SplitN(line, "\t", 4)
		if len(cell) != 4 {
			t.Fatalf("invalid native row %q", line)
		}
		kind, _, _ := strings.Cut(cell[2], ":")
		key := cell[0] + "/" + cell[1]
		want[key] = append(want[key], kind+"|"+cell[3])
	}
	var anonymous [3]string
	for i := range anonymous {
		source, err := os.ReadFile(filepath.Join(fixture, fmt.Sprintf("anonymous%d.apex", i+1)))
		if err != nil {
			t.Fatal(err)
		}
		anonymous[i] = string(source)
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
				if strings.HasSuffix(path, ".xml") && !strings.HasSuffix(path, "sfdx-project.json") {
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
				t.Run(route, func(t *testing.T) {
					for part := 1; part <= len(anonymous); part++ {
						t.Run(fmt.Sprintf("part%d", part), func(t *testing.T) {
							var got []string
							var message string
							if route == "anonymous" {
								org := orgFromIndex(index)
								org.APIVersion = api
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
								program, err := vm.CompileAnonymousWithOptions(anonymous[part-1], vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								_, err = runner.newMachine().Execute(program)
								var runtimeErr *vm.RuntimeError
								if !errors.As(err, &runtimeErr) || flowRefsExceptionType(runtimeErr.Type) != "System.AssertException" {
									t.Fatalf("part %d: missing observation assertion: %v", part, err)
								}
								message = runtimeErr.ExceptionMessage()
							} else {
								result := Run(index, Options{SelectedClasses: []string{"LindenProbeTest"}, SelectedMethod: fmt.Sprintf("t%d", part), NoDiskCache: true, Parallelism: 1})
								if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
									t.Fatalf("unexpected test selection: %#v", result.Summary())
								}
								observed := result.Suites[0].Cases[0]
								if observed.Problem == nil || flowRefsExceptionType(observed.Problem.Type) != "System.AssertException" {
									t.Fatalf("part %d: missing observation assertion: %#v", part, observed.Problem)
								}
								message = observed.Problem.Message
							}
							_, rows, found := strings.Cut(message, "LindenROWS\n")
							if !found {
								t.Fatalf("part %d: missing observation envelope: %s", part, message)
							}
							if rows = strings.TrimSpace(rows); rows != "" {
								got = append(got, strings.Split(rows, "\n")...)
							}
							var expected []string
							groups := []string{"", "CONTACT|,CU|,BS|", "VARS|,OUT|", "INC|,PARENT|,CONST|,OWNER|,TASKS|,ACCT|"}
							for _, row := range want[api+"/"+route] {
								for _, prefix := range strings.Split(groups[part], ",") {
									if strings.HasPrefix(row, prefix) {
										expected = append(expected, row)
										break
									}
								}
							}
							for i := 0; i < len(expected) || i < len(got); i++ {
								var g, w string
								if i < len(got) {
									g = got[i]
								}
								if i < len(expected) {
									w = expected[i]
								}
								if g != w {
									t.Errorf("row %d\n native %s\n Glade  %s", i, w, g)
								}
							}
						})
					}
				})
			}
		})
	}
}

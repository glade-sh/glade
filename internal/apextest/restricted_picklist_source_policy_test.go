package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Replays the captured source/transport matrix through anonymous execution,
// the named-test runner, and the local UI adapters that share the VM query path.
// The adapter checks are local route consistency, not native browser captures.
func TestRestrictedPicklistSourceVersionNativeConformance(t *testing.T) {
	const fixture = "../soql/testdata/restricted_picklist_null_order/source_policy"
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	observations := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 5 {
			t.Fatal(line)
		}
		key := strings.Join(cells[:3], "/")
		observations[key] = append(observations[key], cells[3]+":"+cells[4])
	}
	for _, source := range []string{"63.0", "64.0", "65.0", "66.0"} {
		t.Run(source, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
			err := filepath.WalkDir(filepath.Join(fixture, "force-app"), func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), string(raw))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			class := "OrderingSource" + source[:2]
			for _, transport := range []string{source, "67.0"} {
				want := strings.Join(observations[source+"/"+transport+"/anonymous"], "|")
				if len(strings.Split(want, "|")) != 6 {
					t.Fatal("missing six-clause native oracle")
				}
				t.Run("anonymous/"+transport, func(t *testing.T) {
					org := capturedFixtureOrg(t, index, transport)
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
					program, err := vm.CompileAnonymousWithOptions("System.debug("+class+".run());", vm.CompileOptions{APIVersion: transport})
					if err != nil {
						t.Fatal(err)
					}
					result, err := runner.execute(program)
					if err != nil {
						t.Fatal(err)
					}
					if got := strings.Join(result.Debug, "\n"); !strings.Contains(got, want) {
						t.Fatalf("got %s, want %s", got, want)
					}
				})
			}
			want := strings.Join(observations[source+"/"+source+"/istest"], "|")
			if want == "" {
				t.Fatal("missing named native oracle")
			}
			t.Run("runner", func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"OrderingTest" + source[:2]}, RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); summary.Total != 1 || summary.Failed != 1 {
					t.Fatalf("expected observation assertion, got %+v", summary)
				}
				if problem := firstRunProblem(run); !strings.Contains(problem, want) {
					t.Fatalf("native %s, local %s", want, problem)
				}
			})
			for _, route := range []string{"lwc", "vf"} {
				t.Run(route, func(t *testing.T) {
					org := capturedFixtureOrg(t, index, "67.0")
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
					machine := runner.newMachine()
					program, err := vm.CompileAnonymousWithOptions("return "+class+".run();", vm.CompileOptions{APIVersion: "67.0"})
					if err != nil {
						t.Fatal(err)
					}
					host := vm.Class{Name: "OrderingHost", Methods: map[string]vm.Method{
						"lwc": {Name: "OrderingHost.lwc", ClassName: "OrderingHost", APIVersion: "67.0", ReturnType: "String", IsStatic: true, Modifiers: []string{"public", "AuraEnabled"}, Program: program},
						"vf":  {Name: "OrderingHost.vf", ClassName: "OrderingHost", APIVersion: "67.0", ReturnType: "String", Modifiers: []string{"public"}, Program: program},
					}}
					if err := machine.RegisterClass(host); err != nil {
						t.Fatal(err)
					}
					var result vm.UIInvocationResult
					if route == "lwc" {
						result, err = machine.InvokeLWCMethod("OrderingHost", "lwc", nil)
					} else {
						result, err = machine.InvokeVisualforceAction("OrderingHost", "vf", "/apex/OrderingHost", nil)
					}
					if err != nil || !result.Success || fmt.Sprint(result.ReturnValue) != want {
						t.Fatalf("native %s, local %+v, error %v", want, result, err)
					}
				})
			}
		})
	}
}

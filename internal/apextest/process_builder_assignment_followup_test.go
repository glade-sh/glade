package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestProcessBuilderAssignmentHeaderNative(t *testing.T) {
	const fixture = "testdata/conformance/process_builder_assignment"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("bad row %q", line)
		}
		api, route, want := fields[0], fields[1], fields[3]
		t.Run(api+"/"+route, func(t *testing.T) {
			class := "FernLead" + strings.TrimSuffix(api, ".0")
			if route == "istest" {
				run := Run(index, Options{SelectedClasses: []string{class + "Test"}, SelectedMethod: "probe", NoDiskCache: true, Parallelism: 1})
				const marker = "Expected: SENTINEL, Actual: "
				count := 0
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						count++
						if c.Problem == nil || !strings.Contains(c.Problem.Message, marker) {
							t.Errorf("missing sentinel: %+v", c.Problem)
							continue
						}
						got := strings.TrimSpace(strings.SplitN(c.Problem.Message, marker, 2)[1])
						if got != want {
							t.Errorf("got %s; want %s", got, want)
						}
					}
				}
				if count != 1 {
					t.Fatalf("ran %d cases", count)
				}
				return
			}
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			program, err := vm.CompileAnonymousWithOptions("System.debug('ROW' + "+class+".run());", vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			got := ""
			for _, line := range result.Debug {
				if s, ok := strings.CutPrefix(line, "ROW"); ok {
					got = s
				}
			}
			if got != want {
				t.Errorf("got %s; want %s", got, want)
			}
		})
	}
}

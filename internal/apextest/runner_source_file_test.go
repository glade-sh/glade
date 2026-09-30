package apextest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRunReportsBoundTestSource(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		name := "unique"
		if duplicate {
			name = "duplicate"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			first := filepath.Join(root, "first", "SourceTest.cls")
			second := filepath.Join(root, "second", "SourceTest.cls")
			writeFile(t, first, `@isTest class SourceTest { @isTest static void check() { System.assert(false, 'first body'); } }`)
			secondClass := "OtherTest"
			if duplicate {
				secondClass = "SourceTest"
			}
			writeFile(t, second, `@isTest class `+secondClass+` { @isTest static void check() { System.assert(true); } }`)
			index := typesys.Build(project.Project{Root: root, ApexFiles: []string{first, second}}, gladeschema.Schema{})
			cases := Discover(index, Options{})
			if len(cases) != 2 {
				t.Fatalf("cases = %#v", cases)
			}
			// Keep the overwrite order explicit: the second body's compiled method wins.
			if cases[0].File != first {
				cases[0], cases[1] = cases[1], cases[0]
			}
			run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true}, cases)
			data, err := json.Marshal(run)
			if err != nil {
				t.Fatal(err)
			}
			var report struct {
				Suites []struct {
					Cases []struct {
						SourceFile         string            `json:"sourceFile"`
						SelectedSourceFile string            `json:"selectedSourceFile"`
						Status             testreport.Status `json:"status"`
					} `json:"cases"`
				} `json:"suites"`
			}
			if err := json.Unmarshal(data, &report); err != nil {
				t.Fatal(err)
			}
			t.Logf("fixtureRoot: %s", root)
			t.Logf("report: %s", data)
			sources := map[string]int{}
			selected := map[string]int{}
			statuses := map[testreport.Status]int{}
			for _, suite := range report.Suites {
				for _, row := range suite.Cases {
					sources[row.SourceFile]++
					selected[row.SelectedSourceFile]++
					statuses[row.Status]++
				}
			}
			if selected[first] != 1 || selected[second] != 1 {
				t.Fatalf("lost selected occurrences: %s", data)
			}
			if duplicate {
				if sources[first] != 1 || sources[second] != 1 || statuses[testreport.StatusFail] != 1 || statuses[testreport.StatusPass] != 1 {
					t.Fatalf("duplicate class bodies must remain source-bound: %s", data)
				}
			} else if sources[first] != 1 || sources[second] != 1 || statuses[testreport.StatusFail] != 1 || statuses[testreport.StatusPass] != 1 {
				t.Fatalf("unique pass/fail sources: %s", data)
			}
		})
	}
}

func TestRunOmitsUnestablishedTestSource(t *testing.T) {
	for _, scenario := range []string{"compile-error", "cancelled", "unsupported"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "SourceTest.cls")
			source := `@isTest class SourceTest { @isTest static void check() { System.assert(true); } }`
			if scenario == "unsupported" {
				source = `public class Gap { public static void run() { Integer flags = 1 >>> 2; } } @isTest class SourceTest { @isTest static void check() { Gap.run(); } }`
			}
			writeFile(t, path, source)
			index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
			opts := Options{NoDiskCache: true}
			ctx := context.Background()
			want := testreport.StatusUnsupported
			if scenario == "compile-error" {
				opts.RuntimeRESTAPIVersion = "invalid"
				want = testreport.StatusCompileError
			}
			if scenario == "cancelled" {
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = cancelled
			}
			run := RunCasesContext(ctx, index, opts, Discover(index, Options{}))
			if run.Summary().Total != 1 {
				t.Fatalf("run = %#v", run)
			}
			row := run.Suites[0].Cases[0]
			if row.Status != want || row.SourceFile != "" || row.SelectedSourceFile != path {
				t.Fatalf("non-completed entry claimed source: %#v", row)
			}
		})
	}
}

func TestRunSetupFailurePreservesSelectedSourceAndActualError(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "SetupFailure.cls")
	writeFile(t, path, `@isTest class SetupFailure {
 @TestSetup static void setup() { List<Integer> values = new List<Integer>(); Integer value = values[0]; }
 @isTest static void selected() { System.assert(false, 'entry must not run'); }
 }`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
	run := RunCasesContext(context.Background(), index, Options{NoDiskCache: true}, Discover(index, Options{}))
	row := run.Suites[0].Cases[0]
	if row.Status != testreport.StatusRuntimeError || row.Reason != testreport.ReasonRuntimeError || row.Problem.Type == "UnsupportedFeature" || row.SourceFile != "" || row.SelectedSourceFile != path {
		t.Fatalf("pre-invocation attribution/error contradiction: %#v", row)
	}
}

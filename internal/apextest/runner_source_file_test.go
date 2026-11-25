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
	"github.com/glade-sh/glade/internal/vm"
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
			// Exercise explicit source-bound cases, including both copies.
			cases := discover(index, Options{}, nil)
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
				source = `public class Gap { public static void run() { System.assert(true); } } @isTest class SourceTest { @isTest static void check() { Gap.run(); } }`
			}
			writeFile(t, path, source)
			index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, gladeschema.Schema{})
			opts := Options{NoDiskCache: true}
			if scenario == "unsupported" {
				seedUnlowerableMethod(t, index, "Gap.run")
			}
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
	// A setup exception is a Fail row on the selected method (test_runner_coverage/org.tsv ic04-sync).
	if row.Status != testreport.StatusFail || row.Reason != testreport.ReasonRuntimeError || row.Problem.Type == "UnsupportedFeature" || row.SourceFile != "" || row.SelectedSourceFile != path {
		t.Fatalf("pre-invocation attribution/error contradiction: %#v", row)
	}
}

// Seed a lowering failure explicitly so classification coverage does not depend
// on an Apex operator remaining unimplemented.
func seedUnlowerableMethod(t *testing.T, index typesys.Index, name string) {
	t.Helper()
	sources := newSourceCache()
	key, entry, err := runtimeFromIndexWithSourceDigests(index, nil, sources, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for methodKey, method := range entry.Methods {
		if method.Name == name {
			method.Unsupported = "injected runtime lowering failure"
			method.RuntimeLowering = true
			entry.Methods[methodKey] = method
			found = true
		}
	}
	if !found {
		t.Fatalf("missing lowering fixture method %s", name)
	}
	entry.Classes = compileProjectClasses(index, entry.Methods, sources)
	machine := vm.New(nil)
	machine.SetTraceEnabled(false)
	if err := registerBaseRuntime(machine, entry.Methods, entry.Classes, entry.Triggers); err != nil {
		t.Fatal(err)
	}
	entry.restored = vm.NewRestoredRuntimeTemplate(orgFromIndex(index, sources), machine)
	entry.patchAuthority = nil
	validated, ok := validateRestoredRuntimeEntry(entry)
	if !ok {
		t.Fatal("invalid lowering fixture runtime")
	}
	runtimeCacheMu.Lock()
	runtimeCache[key] = validated
	runtimeCacheMu.Unlock()
	t.Cleanup(func() { evictSnapshotCaches(key) })
}

// Discovery follows the project's lexical source selection before filtering
// methods. The deployment count and source winner are captured in
// internal/project/testdata/source_collisions/org.tsv at 62.0 and 67.0.
func TestRunDiscoversOnlySelectedDuplicateTestClass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	older := filepath.Join(root, "force-app", "amber", "GorseTrailTest.cls")
	selected := filepath.Join(root, "force-app", "willow", "GorseTrailTest.cls")
	writeFile(t, older, `@IsTest private class GorseTrailTest {
        @TestSetup static void seed() { System.assert(false, 'shadowed setup'); }
        @IsTest static void amberOnly() { System.assert(false, 'shadowed test'); }
    }`)
	writeFile(t, selected, `@IsTest private class GorseTrailTest {
        @TestSetup static void seed() { insert new Account(Name = 'willow trail'); }
        @IsTest static void willowOnly() {
            System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'willow trail']);
        }
    }`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{selected, older}}, gladeschema.Schema{})
	for _, opts := range []Options{
		{NoDiskCache: true},
		{NoDiskCache: true, SelectedClasses: []string{"GorseTrailTest"}, SelectedMethod: "willowOnly"},
	} {
		cases := Discover(index, opts)
		if len(cases) != 1 || cases[0].File != selected || cases[0].MethodName != "willowOnly" {
			t.Fatalf("discovered %#v, want only willowOnly from the selected source", cases)
		}
		run := RunCasesContext(context.Background(), index, opts, cases)
		if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
			t.Fatalf("summary = %#v, problem = %q", got, firstRunProblem(run))
		}
	}
	if cases := Discover(index, Options{Filter: "GorseTrailTest.amberOnly"}); len(cases) != 0 {
		t.Fatalf("filter selected a method from the shadowed source: %#v", cases)
	}
}

package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

func TestProcessBuilderReviewForceIgnoreNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/process_builder_review_forceignore"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("bad native row: %q", line)
		}
		api, route, name, want := fields[0], fields[1], fields[2], fields[3]
		seen++
		t.Run(api+"/"+route+"/"+name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
				t.Fatal(err)
			}
			triggerMeta := filepath.Join(root, "force-app/main/default/triggers/FernMetadataTrigger.trigger-meta.xml")
			content, err := os.ReadFile(triggerMeta)
			if err != nil {
				t.Fatal(err)
			}
			content = []byte(strings.Replace(string(content), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>", 1))
			if err := os.WriteFile(triggerMeta, content, 0644); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			class := "FernMetadata" + strings.TrimSuffix(api, ".0")
			if route == "istest" {
				run := Run(index, Options{SelectedClasses: []string{class + "Test"}, SelectedMethod: name, NoDiskCache: true, Parallelism: 1})
				n := 0
				for _, suite := range run.Suites {
					for _, c := range suite.Cases {
						n++
						if c.Problem == nil || !strings.Contains(c.Problem.Message, "Expected: SENTINEL, Actual: "+want) {
							t.Errorf("named result differs from native: %+v", c.Problem)
						}
					}
				}
				if n != 1 {
					t.Fatalf("ran %d cases", n)
				}
				return
			}
			if route != "anonymous" {
				t.Fatalf("unknown route %q", route)
			}
			loaded, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			if err := applyConfiguredTestPermissions(&org, loaded); err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			triggers, triggerErrors := compileProjectTriggers(index)
			if len(triggerErrors) > 0 {
				t.Fatal(triggerErrors)
			}
			for _, trigger := range triggers {
				if err := runner.base.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
			}
			program, err := vm.CompileAnonymousWithOptions("System.debug('FERNROW' + "+class+".observe());", vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			got := ""
			for _, debug := range result.Debug {
				if value, ok := strings.CutPrefix(debug, "FERNROW"); ok {
					got = value
				}
			}
			if got != want {
				t.Errorf("got %s; want %s", got, want)
			}
		})
	}
	if seen != 4 {
		t.Fatalf("expected 4 native rows; got %d", seen)
	}
}

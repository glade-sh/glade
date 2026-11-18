package apextest

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

// Replays captured queue-owner and transition-formula Case after-save routes.
func TestProcessBuilderReviewAfterCaseFormulaAnonymousNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/process_builder_review_aftercaseformula"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/conformance/process_builder_review_aftercase/project")); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(fixture, "project")
	if err := filepath.WalkDir(overlay, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(overlay, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	}); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	loaded, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	errorID := regexp.MustCompile(`Error ID: [0-9]+`)
	seen := 0
	for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("malformed row %q", line)
		}
		api, route, name, want := fields[0], fields[1], fields[2], fields[3]
		if route != "anonymous" {
			continue
		}
		seen++
		t.Run(api+"/"+name, func(t *testing.T) {
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
			class := "FernCaseCases" + strings.TrimSuffix(api, ".0")
			program, err := vm.CompileAnonymousWithOptions("System.debug('FERNROW' + "+class+"."+name+"());", vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := runner.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			got := ""
			for _, debug := range result.Debug {
				if observed, found := strings.CutPrefix(debug, "FERNROW"); found {
					got = observed
				}
			}
			if got = errorID.ReplaceAllString(got, "Error ID: X"); got != want {
				t.Errorf("\n got  %s\n want %s", got, want)
			}
		})
	}
	if seen == 0 {
		t.Fatal("no anonymous rows")
	}
}

// Named tests replay the same source and rows through the project test runner.
func TestProcessBuilderReviewAfterCaseFormulaNamedNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/process_builder_review_aftercaseformula"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/conformance/process_builder_review_aftercase/project")); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(fixture, "project")
	if err := filepath.WalkDir(overlay, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(overlay, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	}); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("bad row %q", line)
		}
		api, route, name, want := fields[0], fields[1], fields[2], fields[3]
		if route != "istest" {
			continue
		}
		count++
		t.Run(api+"/"+name, func(t *testing.T) {
			class := "FernCaseCases" + strings.TrimSuffix(api, ".0") + "Test"
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: name, NoDiskCache: true, Parallelism: 1})
			n := 0
			const marker = "Expected: SENTINEL, Actual: "
			for _, suite := range run.Suites {
				for _, c := range suite.Cases {
					n++
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
			if n != 1 {
				t.Fatalf("ran %d cases", n)
			}
			if name == "owner" || name == "changed" {
				replayCapturedAfterSaveWarm(t, index, class, name, want)
			}

		})
	}
	if count != 10 {
		t.Fatalf("got %d rows", count)
	}
}

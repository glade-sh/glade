package apextest

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

// TestProcessBuilderShapesNativeConformance replays the process_builder_shapes
// capture through the named-test route at Apex API 62.0 and 67.0. Each case
// logs trigger and action events and reads back the rows; see docs/native-evidence/process_builder_shapes/native-run.md for
// the shapes. The sentinel assertion carries the observed text org.tsv records.
// The test route cuts a long assertion message; such a row pins its prefix and
// the anonymous replay below pins the full text.
func TestProcessBuilderShapesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/process_builder_shapes"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	native := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("malformed row %q", line)
		}
		native[fields[0]+"/"+fields[1]+"/"+fields[2]] = fields[3]
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	run := Run(index, Options{SelectedClasses: []string{"FernCases62Test", "FernCases67Test"}, NoDiskCache: true, Parallelism: 1})
	const marker = "Expected: SENTINEL, Actual: "
	seen := 0
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			api := "67.0"
			if strings.Contains(c.ClassName, "62") {
				api = "62.0"
			}
			key := api + "/istest/" + c.MethodName
			want, ok := native[key]
			if !ok {
				t.Fatalf("no native row for %s", key)
			}
			seen++
			if c.Problem == nil || !strings.Contains(c.Problem.Message, marker) {
				t.Errorf("%s: want the sentinel assertion, got %+v", key, c.Problem)
				continue
			}
			got := strings.TrimRight(c.Problem.Message[strings.Index(c.Problem.Message, marker)+len(marker):], " \n")
			full := native[api+"/anonymous/"+c.MethodName]
			if cut := strings.HasPrefix(full, want) && full != want; cut && strings.HasPrefix(got, want) {
				continue
			}
			if got != want {
				t.Errorf("%s:\n got  %s\n want %s", key, got, want)
			}
		}
	}
	if want := len(native) / 2; seen != want {
		t.Fatalf("ran %d probe cases, want %d: %+v", seen, want, run.Summary())
	}
}

// TestProcessBuilderShapesAnonymousNativeConformance replays the anonymous rows.
func TestProcessBuilderShapesAnonymousNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/process_builder_shapes"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
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
			class := "FernCases" + strings.TrimSuffix(api, ".0")
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

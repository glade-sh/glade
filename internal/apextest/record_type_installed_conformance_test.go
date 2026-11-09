package apextest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

var recordTypeInstalledPhases = map[string][]string{
	"inst":    nil,
	"extra":   {"extra"},
	"profile": {"extra", "profile"},
	// depprofile adds an Admin profile to the dependency project (phases/depprofile).
	"depprofile": {"extra", "profile"},
}

// recordTypeInstalledGap: with no Admin profile file listing the object,
// Glade keeps every record type available (see
// applyProjectProfileRecordTypeAvailability); native reports them
// unavailable until a profile grants them. Each must still differ.
func recordTypeInstalledGap(phase, key string) bool {
	return (phase == "inst" && (key == "Cash" || key == "Check")) || (phase == "extra" && (key == "Cash" || key == "Check" || key == "Extra"))
}

// Native observations: testdata/conformance/record_type_installed (ORG-RUN.txt).
// Record types an installed package declares stay available to the
// administrator whatever the source Admin profile lists; record types the
// project itself declares follow the profile.
func TestRecordTypeInstalledNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "record_type_installed")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 17 || lines[0] != "source_api\tphase\troute\tobservation" {
		t.Fatal("incomplete native installed record type observations")
	}
	copyTree := func(t *testing.T, source, target string) {
		t.Helper()
		err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			name, err := filepath.Rel(source, path)
			if err != nil {
				return err
			}
			writeFile(t, filepath.Join(target, name), string(body))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, line := range lines[1:] {
		row := strings.SplitN(line, "\t", 4)
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid row %q", line)
		}
		api, phase, route := row[0], row[1], row[2]
		overlays, ok := recordTypeInstalledPhases[phase]
		if !ok {
			t.Fatalf("unknown phase %s", phase)
		}
		t.Run(api+"/"+phase+"/"+route, func(t *testing.T) {
			var want map[string]any
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			base := t.TempDir()
			root := filepath.Join(base, "consumer")
			copyTree(t, filepath.Join(dir, "dep"), filepath.Join(base, "dep"))
			copyTree(t, filepath.Join(dir, "project"), root)
			for _, overlay := range overlays {
				copyTree(t, filepath.Join(dir, "phases", overlay), root)
			}
			if phase == "depprofile" {
				copyTree(t, filepath.Join(dir, "phases", "depprofile"), filepath.Join(base, "dep"))
			}
			writeFile(t, filepath.Join(root, "glade.yml"), "project:\n  managedPackageDependencies: [\"gladens004:../dep\"]\n")
			index := loadTestIndex(t, root)
			var got map[string]any
			observe := func(message string) {
				_, observed, ok := strings.Cut(message, "OBS|")
				if !ok {
					return
				}
				got = map[string]any{}
				if err := json.Unmarshal([]byte(observed), &got); err != nil {
					t.Fatalf("invalid observation %q", observed)
				}
			}
			if route == "isTest" {
				class := "RtInst" + strings.TrimSuffix(api, ".0") + "Test"
				run := Run(index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("expected one observation: %+v", run.Summary())
				}
				observe(run.Suites[0].Cases[0].Problem.Message)
			} else {
				source := readTestFile(t, filepath.Join(dir, "anon", "inst.apex"))
				if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, message := range result.Debug {
					observe(message)
				}
			}
			if got == nil {
				t.Fatal("no observation")
			}
			for key, value := range want {
				gap := recordTypeInstalledGap(phase, key)
				same := reflect.DeepEqual(got[key], value)
				switch {
				case gap && same:
					t.Errorf("%s: stale known gap; Glade now matches native=%v", key, value)
				case !gap && !same:
					t.Errorf("%s: native=%v; Glade=%v", key, value, got[key])
				}
			}
			if len(got) != len(want) {
				t.Errorf("observation keys: native %d; Glade %d", len(want), len(got))
			}
		})
	}
}

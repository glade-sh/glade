package apextest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

var recordTypeInfoMapsPhases = map[string][]string{
	"plain":   nil,
	"profile": {"profile"},
	"std":     {"profile", "std"},
}

// recordTypeInfoMapsGap lists observations Glade does not yet match. Each must
// still differ, so a fix removes its entry.
func recordTypeInfoMapsGap(api, phase, route, user, key string) bool {
	if strings.Contains(key, ".get.alpha") {
		// Native map keys are case sensitive; Glade's describe maps are not.
		return true
	}
	name := key[strings.LastIndex(key, ".")+1:]
	switch {
	case user == "admin" && phase == "plain":
		// No profile file: native reports every deployed record type
		// unavailable; Glade keeps them available (see
		// applyProjectProfileRecordTypeAvailability).
		return name == "Alpha" || name == "Beta" || name == "Gamma"
	case user == "standard":
		// Glade models the administrator's visibility only; the Standard
		// User reads the same describe.
		if phase == "plain" {
			return name == "Alpha" || name == "Beta" || name == "Gamma"
		}
		if phase == "profile" {
			return name == "Alpha" || name == "Master"
		}
		return name == "Beta"
	}
	return false
}

// Native observations: testdata/conformance/record_type_info_maps (ORG-RUN.txt).
// The describe record type maps list every record type, with isAvailable
// reporting profile visibility, for the administrator and a Standard User.
func TestRecordTypeInfoMapsNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "record_type_info_maps")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 19 || lines[0] != "source_api\tphase\troute\tuser\tobservation" {
		t.Fatal("incomplete native record type info map observations")
	}
	for _, line := range lines[1:] {
		row := strings.SplitN(line, "\t", 5)
		if len(row) != 5 || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid row %q", line)
		}
		api, phase, route, user := row[0], row[1], row[2], row[3]
		overlays, ok := recordTypeInfoMapsPhases[phase]
		if !ok {
			t.Fatalf("unknown phase %s", phase)
		}
		t.Run(api+"/"+phase+"/"+route+"/"+user, func(t *testing.T) {
			var want map[string]any
			if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			sources := []string{filepath.Join(dir, "project")}
			for _, overlay := range overlays {
				sources = append(sources, filepath.Join(dir, "phases", overlay))
			}
			for _, source := range sources {
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
					writeFile(t, filepath.Join(root, name), string(body))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			index := loadTestIndex(t, root)
			var got map[string]any
			observe := func(message string) {
				t.Helper()
				_, observed, ok := strings.Cut(message, "OBS|"+user+"|")
				if !ok {
					return
				}
				got = map[string]any{}
				if err := json.Unmarshal([]byte(observed), &got); err != nil {
					t.Fatalf("invalid observation %q", observed)
				}
			}
			if route == "isTest" {
				class := "RtMapProbe" + strings.TrimSuffix(api, ".0") + "Test"
				run := Run(index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 2 {
					t.Fatalf("expected two observations: %+v", run.Summary())
				}
				for _, testCase := range run.Suites[0].Cases {
					if testCase.Problem == nil {
						t.Fatalf("%s passed; expected an observation", testCase.Name)
					}
					observe(testCase.Problem.Message)
				}
			} else {
				source := readTestFile(t, filepath.Join(dir, "anon", "maps.apex"))
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
				t.Fatalf("no %s observation", user)
			}
			keys := make([]string, 0, len(want))
			for key := range want {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				gap := recordTypeInfoMapsGap(api, phase, route, user, key)
				same := reflect.DeepEqual(got[key], want[key])
				switch {
				case gap && same:
					t.Errorf("%s: stale known gap; Glade now matches native=%v", key, want[key])
				case !gap && !same:
					t.Errorf("%s: native=%v; Glade=%v", key, want[key], got[key])
				}
			}
			if len(got) != len(want) {
				t.Errorf("observation keys: native %d; Glade %d", len(want), len(got))
			}
		})
	}
}

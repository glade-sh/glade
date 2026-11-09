package apextest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

var recordTypeVisibilityPhases = map[string][]string{
	"plain":      nil,
	"account_rt": {"account_rt"},
	"profile":    {"account_rt", "profile"},
	"mode":       {"account_rt", "profile", "mode"},
}

// recordTypeVisibilityNoProfileGaps: without the System Administrator
// profile in the project, Glade keeps every deployed record type available
// (applyProjectProfileRecordTypeAvailability). A fresh scratch org reports
// them unavailable until a profile grants them, so describe availability,
// the omitted Account default, and USER_MODE DML with those record types
// differ in the plain and account_rt phases. Each must still differ.
func recordTypeVisibilityNoProfileGap(api, phase, route, key string) bool {
	if phase != "plain" && phase != "account_rt" {
		return false
	}
	if key == "V35Rt__c.infos" || (phase == "account_rt" && (key == "Account.infos" || key == "Account.dml.omit")) {
		return true
	}
	userMode := route == "anonymous" || api == "67.0"
	if !userMode {
		return false
	}
	for _, name := range []string{"Alpha", "Beta", "Gamma"} {
		if key == "V35Rt__c.dml.rt:"+name || key == "V35Rt__c.update."+name {
			return true
		}
	}
	return phase == "account_rt" && (key == "Account.dml.rt:LgAcct" || key == "Account.update.LgAcct")
}

var recordTypeVisibilityRecordID = regexp.MustCompile(`with id [a-zA-Z0-9]{15,18};`)

// Native observations: testdata/conformance/record_type_visibility (ORG-RUN.txt).
func TestRecordTypeVisibilityNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "record_type_visibility")
	data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 17 || lines[0] != "source_api\tphase\troute\tobservation" {
		t.Fatal("incomplete native record type visibility observations")
	}
	for _, line := range lines[1:] {
		row := strings.SplitN(line, "\t", 4)
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid row %q", line)
		}
		api, phase, route := row[0], row[1], row[2]
		overlays, ok := recordTypeVisibilityPhases[phase]
		if !ok {
			t.Fatalf("unknown phase %s", phase)
		}
		t.Run(api+"/"+phase+"/"+route, func(t *testing.T) {
			var want map[string]any
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
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
			got := map[string]any{}
			observe := func(message string) {
				t.Helper()
				_, observed, ok := strings.Cut(message, "OBS|")
				if !ok {
					t.Fatalf("no observation in %q", message)
				}
				part := map[string]any{}
				if err := json.Unmarshal([]byte(recordTypeVisibilityRecordID.ReplaceAllString(observed, "with id <id>;")), &part); err != nil {
					t.Fatalf("invalid observation %q", observed)
				}
				for key, value := range part {
					got[key] = value
				}
			}
			if route == "isTest" {
				class := "RtVisProbe" + strings.TrimSuffix(api, ".0") + "Test"
				run := Run(index, Options{SelectedClasses: []string{class}, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 5 {
					t.Fatalf("expected five observations: %+v", run.Summary())
				}
				for _, testCase := range run.Suites[0].Cases {
					if testCase.Problem == nil {
						t.Fatalf("%s passed; expected an observation", testCase.Name)
					}
					observe(testCase.Problem.Message)
				}
			} else {
				probe := map[string]string{"anonymous": "vis", "class": "modeClass"}[route]
				if phase == "mode" && route == "anonymous" {
					probe = "mode"
				}
				source := readTestFile(t, filepath.Join(dir, "anon", probe+".apex"))
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
					if strings.Contains(message, "OBS|") {
						observe(message)
					}
				}
			}
			keys := make([]string, 0, len(want))
			for key := range want {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				gap := recordTypeVisibilityNoProfileGap(api, phase, route, key)
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

package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The observations supersede neither original capture. This validates the
// repeated, independent native evidence used to distinguish org shape from
// org identity and to rule out a global SOQL windowing change.
func TestCarriedConflictIsolationCapture(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "conformance", "carried_conflict_isolation", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 397 || strings.Join(rows[0], ",") != "org_shape,phase,source_api,route,sharing,user_profile,repetition,observation" {
		t.Fatalf("incomplete capture: %d rows", len(rows)-1)
	}
	type group struct {
		repetitions map[string]bool
		value       any
	}
	groups := map[string]*group{}
	observations := map[string]map[string]any{}
	for _, row := range rows[1:] {
		key := strings.Join(row[:6], "/")
		var value map[string]any
		if err := json.Unmarshal([]byte(row[7]), &value); err != nil {
			t.Fatalf("%s: invalid native JSON: %v", key, err)
		}
		g := groups[key]
		if g == nil {
			g = &group{repetitions: map[string]bool{}, value: value}
			groups[key] = g
			observations[key] = value
		} else if !reflect.DeepEqual(g.value, value) {
			t.Errorf("%s: repeated native observations differ", key)
		}
		if g.repetitions[row[6]] {
			t.Errorf("%s: duplicate repetition %s", key, row[6])
		}
		g.repetitions[row[6]] = true
	}
	if len(groups) != 132 {
		t.Fatalf("got %d native route groups, want 132", len(groups))
	}
	for key, g := range groups {
		if len(g.repetitions) != 3 || !g.repetitions["1"] || !g.repetitions["2"] || !g.repetitions["3"] {
			t.Errorf("%s: missing repeat: %v", key, g.repetitions)
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		first := observations["plain/baseline/"+api+"/anonymous/with/administrator"]
		second := observations["plain2/baseline/"+api+"/anonymous/with/administrator"]
		if first == nil || second == nil {
			t.Fatalf("%s: missing independent plain-org observations", api)
		}
		for _, kind := range []string{"active", "inactive"} {
			left := first[kind].([]any)[0].(map[string]any)["domainLabelLengths"]
			right := second[kind].([]any)[0].(map[string]any)["domainLabelLengths"]
			if reflect.DeepEqual(left, right) {
				t.Errorf("%s %s: expected captured cross-org difference", api, kind)
			}
		}
		for _, item := range []struct {
			shape string
			want  bool
		}{{"plain", true}, {"plain2", true}, {"feature", false}} {
			key := item.shape + "/profile_id_controls/" + api + "/anonymous/inherited/administrator"
			control, ok := observations[key]["profile"].(map[string]any)
			if !ok || control["limitMatchesMinimumId"] != item.want {
				t.Errorf("%s: unexpected captured Profile ordering control", key)
			}
		}
	}
}

// runas_user/org.tsv compares the same org before and after changing the
// configured email-service user. Domain fingerprints are reduced to equality
// booleans before export; the complete domains and fingerprints are absent.
func TestCarriedEmailRunAsIsolationCapture(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "conformance", "carried_conflict_isolation", "runas_user", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 145 || strings.Join(rows[0], ",") != "org_shape,phase,source_api,route,sharing,user_profile,repetition,observation" {
		t.Fatalf("incomplete run-as capture: %d rows", len(rows)-1)
	}
	groups := map[string]map[string]bool{}
	for _, row := range rows[1:] {
		key := strings.Join(row[:6], "/")
		if groups[key] == nil {
			groups[key] = map[string]bool{}
		}
		if groups[key][row[6]] {
			t.Errorf("%s: duplicate repetition %s", key, row[6])
		}
		groups[key][row[6]] = true
		if strings.HasPrefix(row[1], "identity_") {
			var equal map[string]bool
			if err := json.Unmarshal([]byte(row[7]), &equal); err != nil {
				t.Fatalf("%s: %v", key, err)
			}
			if len(equal) != 2 {
				t.Errorf("%s: got %d address comparisons, want two", key, len(equal))
			}
			for _, same := range equal {
				if !same {
					t.Errorf("%s: domain changed with configured user", key)
				}
			}
		}
	}
	if len(groups) != 48 {
		t.Fatalf("got %d native route groups, want 48", len(groups))
	}
	for key, repetitions := range groups {
		if len(repetitions) != 3 || !repetitions["1"] || !repetitions["2"] || !repetitions["3"] {
			t.Errorf("%s: missing repeat: %v", key, repetitions)
		}
	}
}

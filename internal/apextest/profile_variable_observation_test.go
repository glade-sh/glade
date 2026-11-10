package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Profile LIMIT observations can vary after redeployment. The original rows
// remain exact unless a repeated observation of the same cell differs.
func assertVariableProfileObservation(t *testing.T, row []string, rows [][]string, got any) bool {
	t.Helper()
	if row[3] != "limited" {
		return false
	}
	file, err := os.Open(filepath.Join("testdata", "conformance", "standard_profile_inventory", "limit-ordering-recheck.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rechecks, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rechecks) == 0 || !reflect.DeepEqual(rechecks[0], []string{"source_api", "configuration", "route", "case_id", "observation"}) {
		t.Fatal("invalid Profile recheck header")
	}
	var original any
	if err := json.Unmarshal([]byte(row[4]), &original); err != nil {
		t.Fatal(err)
	}
	observations := []any{}
	for _, recheck := range rechecks[1:] {
		if len(recheck) != 5 || !reflect.DeepEqual(recheck[:4], row[:4]) {
			continue
		}
		var repeated any
		if err := json.Unmarshal([]byte(recheck[4]), &repeated); err != nil {
			t.Fatal(err)
		}
		observations = append(observations, repeated)
	}
	recapture, err := os.Open(filepath.Join("testdata", "conformance", "profile_limit_recheck", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer recapture.Close()
	recaptureReader := csv.NewReader(recapture)
	recaptureReader.Comma, recaptureReader.LazyQuotes = '\t', true
	recaptured, err := recaptureReader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(recaptured) == 0 || !reflect.DeepEqual(recaptured[0], []string{"source_api", "configuration", "route", "case_id", "repetition", "observation"}) {
		t.Fatal("invalid Profile recapture header")
	}
	for _, repeatedRow := range recaptured[1:] {
		if len(repeatedRow) != 6 || !reflect.DeepEqual(repeatedRow[:4], row[:4]) {
			continue
		}
		var repeated any
		if err := json.Unmarshal([]byte(repeatedRow[5]), &repeated); err != nil {
			t.Fatal(err)
		}
		observations = append(observations, repeated)
	}
	for _, inventory := range rows {
		if inventory[0] != row[0] || inventory[1] != row[1] || inventory[2] != row[2] || inventory[3] != "inventory" {
			continue
		}
		var eligible any
		if err := json.Unmarshal([]byte(inventory[4]), &eligible); err != nil {
			t.Fatal(err)
		}
		variable, err := checkProfileNativeEvidence(original, observations, eligible)
		if err != nil {
			t.Fatal(err)
		}
		if !variable {
			return false
		}
		if err := checkProfileLimitedMembership(got, original, eligible); err != nil {
			t.Fatal(err)
		}
		return true
	}
	t.Fatal("variable Profile observation lacks corresponding inventory")
	return true
}

// Every observation must support the invariant before differences can prove
// that a cell is variable.
func checkProfileNativeEvidence(original any, observations []any, eligible any) (bool, error) {
	if err := checkProfileLimitedMembership(original, original, eligible); err != nil {
		return false, fmt.Errorf("original Profile observation: %w", err)
	}
	variable := false
	for _, observation := range observations {
		if err := checkProfileLimitedMembership(observation, original, eligible); err != nil {
			return false, fmt.Errorf("repeated Profile observation: %w", err)
		}
		if !reflect.DeepEqual(original, observation) {
			variable = true
		}
	}
	return variable, nil
}

func checkProfileLimitedMembership(got, native, eligible any) error {
	actual, ok := got.([]any)
	if !ok {
		return fmt.Errorf("Profile LIMIT observation is not a list: %v", got)
	}
	captured, ok := native.([]any)
	if !ok || len(actual) != len(captured) {
		return fmt.Errorf("Profile LIMIT returned %d rows; native count is %d", len(actual), len(captured))
	}
	inventory, ok := eligible.([]any)
	if !ok {
		return fmt.Errorf("Profile inventory is not a list: %v", eligible)
	}
	for _, value := range actual {
		found := false
		for _, candidate := range inventory {
			if reflect.DeepEqual(value, candidate) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Profile LIMIT row is absent from captured inventory: %v", value)
		}
	}
	return nil
}

func TestVariableProfileObservationMembership(t *testing.T) {
	first := map[string]any{"name": "First", "license": "One"}
	second := map[string]any{"name": "Second", "license": "Two"}
	cases := []struct {
		name      string
		got       any
		wantError bool
	}{
		{"first eligible row", []any{first}, false},
		{"second eligible row", []any{second}, false},
		{"wrong cardinality", []any{first, second}, true},
		{"ineligible row", []any{map[string]any{"name": "First", "license": "Wrong"}}, true},
		{"not a list", first, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkProfileLimitedMembership(tc.got, []any{first}, []any{first, second})
			if (err != nil) != tc.wantError {
				t.Fatalf("membership error = %v, want error %t", err, tc.wantError)
			}
		})
	}
}

func TestProfileVariableCellsRequireContradictoryCapture(t *testing.T) {
	capture := setupRenderingCaptures[0]
	rows := readSetupRenderingCaptures(t, capture)
	variables := 0
	for _, row := range rows {
		if row[3] != "limited" {
			continue
		}
		var got any
		if err := json.Unmarshal([]byte(row[4]), &got); err != nil {
			t.Fatal(err)
		}
		if assertVariableProfileObservation(t, row, rows, got) {
			variables++
		}
	}
	if variables != 12 {
		t.Fatalf("variable native Profile cells = %d, want 12", variables)
	}
}

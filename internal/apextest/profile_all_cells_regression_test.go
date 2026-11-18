package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// All LIMIT cells have two differing observations at matching API, route,
// and scratch configuration; the repetitions are retained in org.tsv.
func TestProfileEveryLimitCellHasVariableNativeObservation(t *testing.T) {
	capture := setupRenderingCaptures[0]
	rows := readSetupRenderingCaptures(t, capture)
	f, err := os.Open(filepath.Join("testdata", "conformance", "profile_limit_recheck", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	captures, err := reader.ReadAll()
	if err != nil || len(captures) == 0 || strings.Join(captures[0], ",") != "source_api,configuration,route,case_id,repetition,observation" {
		t.Fatalf("invalid Profile limit recapture: %v", err)
	}
	counts := map[string]int{}
	for _, row := range captures[1:] {
		if len(row) != 6 || !json.Valid([]byte(row[5])) {
			t.Fatalf("invalid Profile recapture row: %v", row)
		}
		counts[strings.Join(row[:4], "/")]++
	}
	variables := 0
	for _, row := range rows {
		if row[3] != "limited" {
			continue
		}
		if got := counts[strings.Join(row[:4], "/")]; got != 3 {
			t.Fatalf("%v: got %d repeated observations, want 3", row[:4], got)
		}
		var original any
		if err := json.Unmarshal([]byte(row[4]), &original); err != nil {
			t.Fatal(err)
		}
		if assertVariableProfileObservation(t, row, rows, original) {
			variables++
		}
	}
	if variables != 12 {
		t.Fatalf("Profile cells proved variable = %d, want 12", variables)
	}
}

package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// These are evidence integrity checks, not Glade behavior comparisons. The
// capture depends on wall-clock context; a separate test controls that clock.
func TestSOQLClockHeldNativeEvidence(t *testing.T) {
	root := filepath.Join("testdata", "conformance", "soql_clock_held_c14")
	file, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 13 {
		t.Fatalf("invalid clock observations: %v", err)
	}
	for _, row := range rows[1:] {
		if len(row) != 5 || !json.Valid([]byte(row[4])) {
			t.Fatal("malformed clock observation")
		}
		t.Logf("clock-dependent native observation: %s %s %s", row[0], row[1], row[2])
	}
}

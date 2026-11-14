package dml

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Name Variant table holds exactly the first-name pairs the native rows
// score 100 where Jaro-Winkler under the match key does not explain the score:
// no entry without a captured pair, no captured pair without an entry.
func TestFuzzyNameTablesMatchNativeRows(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(fuzzyFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	proven := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 || cells[0] != "67" || strings.HasPrefix(cells[2], "Holdout4.") || strings.HasPrefix(cells[2], "Holdout6.") {
			// Held-out rows are scored, never used as a source for the tables.
			continue
		}
		var row fuzzyNativeRow
		if err := json.Unmarshal([]byte(cells[3]), &row); err != nil {
			t.Fatalf("%s: %v", cells[2], err)
		}
		if row.Method != "FirstName" {
			continue
		}
		for _, m := range row.Matches {
			candidate, existing := row.Values[int(m[0])], row.Values[int(m[1])]
			if m[2] != 100 || candidate == nil || existing == nil || m[0] == m[1] {
				continue
			}
			x, y := foldText(*candidate), foldText(*existing)
			if x == y {
				continue
			}
			score := jaroWinkler(x, y)
			if doubleMetaphone(x, firstNameKeyLength) == doubleMetaphone(y, firstNameKeyLength) && score >= firstNameJaroWinkler-1e-9 && math.Floor(averageWith100(100*score)+0.5) == 100 {
				continue
			}
			proven[x+"\x00"+y] = true
		}
	}
	for pair := range proven {
		if !nameVariants[pair] {
			t.Errorf("native Name Variant pair %q is missing from nameVariantTable", strings.ReplaceAll(pair, "\x00", " -> "))
		}
	}
	for pair := range nameVariants {
		if !proven[pair] {
			t.Errorf("nameVariantTable entry %q has no native row", strings.ReplaceAll(pair, "\x00", " -> "))
		}
	}
}

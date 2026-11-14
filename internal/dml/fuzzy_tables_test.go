package dml

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
	for pair := range fuzzyPanelNameVariants(t) {
		proven[pair] = true
	}
	for pair := range proven {
		if !nameVariants[pair] && !customNameVariants[pair] {
			t.Errorf("native Name Variant pair %q is missing from nameVariantTable", strings.ReplaceAll(pair, "\x00", " -> "))
		}
	}
	for pair := range nameVariants {
		if !proven[pair] {
			t.Errorf("nameVariantTable entry %q has no native row", strings.ReplaceAll(pair, "\x00", " -> "))
		}
	}
	for pair := range customNameVariants {
		if !proven[pair] {
			t.Errorf("custom Name Variant entry %q has no native row", strings.ReplaceAll(pair, "\x00", " -> "))
		}
	}
}

const fuzzyPanelsFixture = "../apextest/testdata/conformance/duplicate_fuzzy_panels"

// fuzzyPanelNameVariants reads the First Name panels of the second fixture
// (API 67.0, @IsTest; every API, route and repeat is identical and compared
// in TestDuplicateFuzzyPanelsNativeRoutes) and returns the pairs native scores
// 100 that Jaro-Winkler under the match key does not explain.
func fuzzyPanelNameVariants(t *testing.T) map[string]bool {
	t.Helper()
	var groups map[string]map[string]string
	raw, err := os.ReadFile(filepath.Join(fuzzyPanelsFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &groups); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(fuzzyPanelsFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	quoted := regexp.MustCompile(`'([^']*)'`)
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	proven := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 6 || f[0] != "67.0" || f[1] != "istest" || f[2] != "1" || f[3] != "first" {
			continue
		}
		_, list, _ := strings.Cut(groups[f[3]][f[4]], "{")
		var values []string
		for _, m := range quoted.FindAllStringSubmatch(list, -1) {
			values = append(values, m[1])
		}
		_, body, _ := strings.Cut(f[5], ":")
		for _, token := range strings.Split(body, ",") {
			i, j := strings.IndexByte(digits, token[0]), strings.IndexByte(digits, token[1])
			if len(token) > 2 {
				if c, err := strconv.ParseFloat(token[2:], 64); err != nil || c != 100 {
					continue
				}
			}
			x, y := foldText(values[i]), foldText(values[j])
			if i == j || x == y || firstNameInitials(x) || firstNameInitials(y) {
				continue
			}
			score := jaroWinkler(x, y)
			if doubleMetaphone(x, firstNameKeyLength) == doubleMetaphone(y, firstNameKeyLength) && score >= firstNameJaroWinkler-1e-9 && math.Floor(averageWith100(100*score)+0.5) == 100 {
				continue
			}
			proven[x+"\x00"+y] = true
		}
	}
	if len(proven) == 0 {
		t.Fatal("no First Name panel rows")
	}
	return proven
}

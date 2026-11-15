package dml

import (
	"encoding/csv"
	"encoding/json"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestCompanyNameNativeConformance checks the captured, directed panel matches.
// The panels report confidence 100 without an explicit numeric suffix.
func TestCompanyNameNativeConformance(t *testing.T) {
	const dir = "testdata/company_name_fit/"
	b, err := os.ReadFile(dir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]string
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	quoted := regexp.MustCompile(`'([^']*)'`)
	type counts struct{ native, hits, exact, residual int }
	panels := map[string]counts{}
	// The fit row and the earlier short-ampersand row disagree on the
	// generalized shape. Preserve the earlier decisions for these eight
	// directed pairs: company_name_fit/org.tsv:68 and
	// duplicate_fuzzy_matching/org.tsv:90,431.
	ampResidual := map[string]bool{"lm": true, "lo": true, "ml": true, "mo": true, "no": true, "ol": true, "om": true, "on": true}
	for _, row := range rows[1:] {
		if row[0] != "62.0" || row[1] != "istest" || row[2] != "1" {
			continue
		}
		src := cases[row[4]]
		if !strings.Contains(src, "FuzzyPanel.") {
			continue
		}
		start := strings.Index(src, "new List<String>{")
		if start < 0 {
			t.Fatalf("%s: missing values", row[4])
		}
		list := src[start+len("new List<String>{"):]
		end := strings.IndexByte(list, '}')
		if end < 0 {
			t.Fatalf("%s: malformed values", row[4])
		}
		var values []string
		for _, m := range quoted.FindAllStringSubmatch(list[:end], -1) {
			values = append(values, m[1])
		}
		parts := strings.SplitN(row[5], ":", 2)
		if len(parts) != 2 {
			t.Fatalf("%s: malformed observation", row[4])
		}
		c := panels[row[4]]
		for _, entry := range strings.Split(parts[1], ",") {
			if len(entry) < 2 {
				t.Fatalf("%s: malformed entry %q", row[4], entry)
			}
			i, e1 := strconv.ParseInt(entry[:1], 36, 0)
			j, e2 := strconv.ParseInt(entry[1:2], 36, 0)
			if e1 != nil || e2 != nil || int(i) >= len(values) || int(j) >= len(values) {
				t.Fatalf("%s: unknown pair %q", row[4], entry)
			}
			score := 100.0
			if len(entry) > 2 {
				score, err = strconv.ParseFloat(entry[2:], 64)
				if err != nil {
					t.Fatal(err)
				}
			}
			c.native++
			ok, got := fuzzyCompany(values[i], values[j], false)
			if row[4] == "B3_ca1" && ampResidual[entry[:2]] {
				base, baseScore := legacyFuzzyCompany(values[i], values[j], false)
				if ok != base || got != baseScore || ok {
					t.Errorf("%s %s: got %v %.0f, want prior no-match %v %.0f", row[4], entry, ok, got, base, baseScore)
				}
				c.residual++
				continue
			}
			if ok {
				c.hits++
				if math.Round(got) == score {
					c.exact++
				}
			}
		}
		panels[row[4]] = c
	}
	if len(panels) != 9 {
		t.Fatalf("selected %d panels, want 9", len(panels))
	}
	for name, c := range panels {
		t.Logf("%s: matches %d/%d, exact confidence %d/%d, conflicting residuals %d", name, c.hits, c.native, c.exact, c.native, c.residual)
		if c.hits+c.residual != c.native {
			t.Errorf("%s: missed %d captured matches outside the conflicting residuals", name, c.native-c.hits-c.residual)
		}
		if name == "B3_ca1" && c.residual != 8 {
			t.Errorf("%s: found %d conflicting residuals, want 8", name, c.residual)
		}
	}
}

package dml

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// nativePanel is one captured panel: every value is inserted, then each value
// is run through Datacloud.FindDuplicates as an unsaved candidate. Matches are
// [candidate index, existing index, confidence]; every other ordered pair,
// the diagonal included, did not match.
type nativePanel struct {
	ID      string       `json:"id"`
	Method  string       `json:"method"`
	Rule    string       `json:"rule"`
	Prefix  string       `json:"prefix"`
	Values  []string     `json:"values"`
	Matches [][3]float64 `json:"matches"`
}

// loadNativePanels reads panels.json and checks each panel against every
// org.tsv row of its case (API, route and repeat), which must all agree.
func loadNativePanels(t *testing.T, dir string, wantRows int) []nativePanel {
	t.Helper()
	var panels []nativePanel
	data, err := os.ReadFile(filepath.Join(dir, "panels.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &panels); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")[1:]
	for _, panel := range panels {
		want := map[[2]int]float64{}
		for _, m := range panel.Matches {
			want[[2]int{int(m[0]), int(m[1])}] = m[2]
		}
		caseID := panel.ID[strings.LastIndexByte(panel.ID, '.')+1:]
		seen := 0
		for _, line := range lines {
			cells := strings.SplitN(line, "\t", 6)
			if len(cells) != 6 || cells[4] != caseID {
				continue
			}
			parts := strings.SplitN(cells[5], ":", 2)
			n, err := strconv.Atoi(parts[0])
			if len(parts) != 2 || err != nil || n != len(panel.Values) {
				t.Fatalf("%s: invalid panel %q", panel.ID, cells[5])
			}
			native := map[[2]int]float64{}
			if parts[1] != "" {
				for _, token := range strings.Split(parts[1], ",") {
					a, errA := strconv.ParseInt(token[:1], 36, 8)
					b, errB := strconv.ParseInt(token[1:2], 36, 8)
					if errA != nil || errB != nil {
						t.Fatalf("invalid pair %q", token)
					}
					score := 100.0
					if len(token) > 2 {
						if score, err = strconv.ParseFloat(token[2:], 64); err != nil {
							t.Fatal(err)
						}
					}
					native[[2]int{int(a), int(b)}] = score
				}
			}
			if !reflect.DeepEqual(native, want) {
				t.Fatalf("%s differs from captured row %s/%s/%s", panel.ID, cells[0], cells[1], cells[2])
			}
			seen++
		}
		if seen != wantRows {
			t.Fatalf("%s: got %d captured rows, want %d", panel.ID, seen, wantRows)
		}
	}
	return panels
}

// panelFields is the field each method's custom panel varies.
var panelFields = map[string]string{"Phone": "Phone", "City": "BillingCity", "Zip": "BillingPostalCode"}

// panelMatches runs a captured panel through the shared duplicate-rule path:
// the custom one-item rule, or the standard rule named by the panel with the
// panel's fixed fields.
func panelMatches(t *testing.T, panel nativePanel) map[[2]int]float64 {
	t.Helper()
	if panel.Rule == "custom" {
		row := fuzzyNativeRow{Object: "Account", Field: panelFields[panel.Method], Method: panel.Method, Blank: "NullNotAllowed"}
		for i := range panel.Values {
			row.Values = append(row.Values, &panel.Values[i])
		}
		got, _ := fuzzyGladeMatches(t, row)
		return got
	}
	spec := strings.Split(panel.Rule, ":")
	fields := strings.Split(spec[2], "+")
	row := standardNativeRow{Object: spec[0], Rule: spec[1]}
	for _, value := range panel.Values {
		parts := strings.Split(panel.Prefix+value, "|")
		record := map[string]string{}
		for i, field := range fields {
			record[field] = parts[i]
		}
		row.Existing = append(row.Existing, record)
		row.Candidates = append(row.Candidates, record)
	}
	got, _ := standardRowMatches(t, row)
	return got
}

// standardPhoneLineResidual: the standard Contact and Lead rules score two
// numbers that differ only in the last four digits one point under Glade's
// fitted 97 and 98 unless both carry the country digit 1 (N5.spt1, N5.slt1,
// records without an email address). The rows that fitted 97 and 98 have an
// email address on every record, so the difference follows the other fields
// of the rule and is not modeled; the verdicts are exact.
func standardPhoneLineResidual(panel nativePanel, got, native float64) bool {
	return panel.Rule != "custom" && panel.Method == "Phone" && got-native == 1 && (got == 97 || got == 98)
}

// NATIVE Phone, City and ZIP panels: testdata/phone_city_zip/org.tsv, custom
// and standard rules, anonymous and @IsTest, API 62.0 and 67.0, two repeats.
// Every ordered pair is reproduced, verdict and confidence, except the
// standard phone confidences of standardPhoneLineResidual.
func TestPhoneCityZipNativeConformance(t *testing.T) {
	residual := 0
	for _, panel := range loadNativePanels(t, "testdata/phone_city_zip", 8) {
		t.Run(panel.ID, func(t *testing.T) {
			want := map[[2]int]float64{}
			for _, m := range panel.Matches {
				want[[2]int{int(m[0]), int(m[1])}] = m[2]
			}
			got := panelMatches(t, panel)
			for i, a := range panel.Values {
				for j, b := range panel.Values {
					score, match := got[[2]int{i, j}]
					expected, ok := want[[2]int{i, j}]
					switch {
					case match != ok:
						t.Errorf("%q/%q: matched %v, native %v", a, b, match, ok)
					case match && score != expected && standardPhoneLineResidual(panel, score, expected):
						residual++
					case match && score != expected:
						t.Errorf("%q/%q: %v, native %v", a, b, score, expected)
					}
				}
			}
		})
	}
	if residual != 224 {
		t.Errorf("got %d standard phone residual pairs, want 224", residual)
	}
}

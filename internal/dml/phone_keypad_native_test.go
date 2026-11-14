package dml

import (
	"encoding/json"
	"os"
	"testing"
)

// The fit panels include all positive and negative ordered pairs, including
// invalid phone values. The captured match sets and scores are exhaustive.
func TestPhoneKeypadNativeConformance(t *testing.T) {
	var panels []struct {
		ID      string       `json:"id"`
		Values  []string     `json:"values"`
		Matches [][3]float64 `json:"matches"`
	}
	data, err := os.ReadFile("testdata/phone_keypad/panels.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &panels); err != nil {
		t.Fatal(err)
	}
	if len(panels) != 3 {
		t.Fatalf("got %d panels", len(panels))
	}
	for _, panel := range panels {
		t.Run(panel.ID, func(t *testing.T) {
			want := map[[2]int]float64{}
			for _, m := range panel.Matches {
				want[[2]int{int(m[0]), int(m[1])}] = m[2]
			}
			for i, a := range panel.Values {
				for j, b := range panel.Values {
					match, score := fuzzyPhone(a, b, false)
					expected, ok := want[[2]int{i, j}]
					if match != ok || (match && score != expected) {
						t.Errorf("%d,%d: (%v,%v), want (%v,%v)", i, j, match, score, ok, expected)
					}
				}
			}
		})
	}
}

func TestPhoneKeypadRuleNativeConformance(t *testing.T) {
	row := fuzzyNativeRow{Object: "Account", Field: "Phone", Method: "Phone", Blank: "NullNotAllowed"}
	a, b := "1-800-FLOWERS", "800-356-9377"
	row.Values = []*string{&a, &b}
	got, _ := fuzzyGladeMatches(t, row)
	for _, pair := range [][2]int{{0, 1}, {1, 0}} {
		if got[pair] != 90 {
			t.Errorf("%v: got %v, want 90", pair, got[pair])
		}
	}
}

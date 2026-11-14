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

// This uncaptured neighbor had a score of 60 and did not match on origin/main.
// Keeping the raw score matters to compound custom matching rules.
func TestPhoneKeypadUncapturedNeighborKeepsBase(t *testing.T) {
	if ok, score := fuzzyPhone("1-800-FLOWERS", "800-356-9378", false); ok || score != 60 {
		t.Fatalf("got %v, %v; base false, 60", ok, score)
	}
	if score := FuzzyRawConfidence("phone", "1-800-FLOWERS", "800-356-9378", false); score != 60 {
		t.Fatalf("raw got %v; base 60", score)
	}
}

func BenchmarkPhoneKeypadPanel(b *testing.B) {
	values := []string{"1-800-FLOWERS", "800-356-9377", "1 (800) 356-9377", "+44 20 7946 0958", "(628) 555-0142 x12", "628-555-0142", "62855501420"}
	for i := 0; i < b.N; i++ {
		for _, a := range values {
			for _, c := range values {
				fuzzyPhone(a, c, false)
			}
		}
	}
}

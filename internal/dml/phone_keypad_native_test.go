package dml

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
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
	raw, err := os.ReadFile("testdata/phone_keypad/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, panel := range panels {
		t.Run(panel.ID, func(t *testing.T) {
			want := map[[2]int]float64{}
			for _, m := range panel.Matches {
				want[[2]int{int(m[0]), int(m[1])}] = m[2]
			}
			seen := 0
			caseID := strings.Split(panel.ID, ".")[1]
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				cells := strings.SplitN(line, "\t", 6)
				if len(cells) != 6 || cells[4] != caseID {
					continue
				}
				parts := strings.SplitN(cells[5], ":", 2)
				if len(parts) != 2 {
					t.Fatalf("invalid panel: %q", cells[5])
				}
				n, err := strconv.Atoi(parts[0])
				if err != nil || n != len(panel.Values) {
					t.Fatalf("invalid panel size %q", parts[0])
				}
				native := map[[2]int]float64{}
				for _, token := range strings.Split(parts[1], ",") {
					if len(token) < 2 {
						t.Fatalf("invalid pair %q", token)
					}
					a, errA := strconv.ParseInt(token[:1], 36, 8)
					b, errB := strconv.ParseInt(token[1:2], 36, 8)
					if errA != nil || errB != nil {
						t.Fatalf("invalid pair %q", token)
					}
					score := 100.0
					if len(token) > 2 {
						score, err = strconv.ParseFloat(token[2:], 64)
						if err != nil {
							t.Fatal(err)
						}
					}
					native[[2]int{int(a), int(b)}] = score
				}
				if !reflect.DeepEqual(native, want) {
					t.Fatalf("panel differs from captured row %s/%s/%s", cells[0], cells[1], cells[2])
				}
				seen++
			}
			if seen != 8 {
				t.Fatalf("got %d captured rows, want 8", seen)
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

// This uncaptured neighbor had a score of 50 and did not match on origin/main.
// Keeping the raw score matters to compound custom matching rules.
func TestPhoneKeypadUncapturedNeighborKeepsBase(t *testing.T) {
	if ok, score := fuzzyPhone("1-800-FLOWERS", "800-356-9378", false); ok || score != 50 {
		t.Fatalf("got %v, %v; base false, 50", ok, score)
	}
	if score := FuzzyRawConfidence("phone", "1-800-FLOWERS", "800-356-9378", false); score != 50 {
		t.Fatalf("raw got %v; base 50", score)
	}
}

func TestPhoneKeypadE161Alphabet(t *testing.T) {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	const digits = "22233344455566677778889999"
	for i := range letters {
		vanity := "1-800-" + strings.Repeat(letters[i:i+1], 7)
		number := "800-" + strings.Repeat(digits[i:i+1], 7)
		if ok, score := FuzzyScore("phone", vanity, number, false); !ok || score != 90 {
			t.Errorf("letter %c: got %v, %v; want true, 90", letters[i], ok, score)
		}
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

// Native pair 6: the stored vanity number blocks insert and update of the
// digit form. A single bulk insert into an empty org does not self-match.
func TestPhoneKeypadDMLNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			newOrg := func() *storage.OrgState {
				org := &storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
				storage.EnsureStandardObject(org, "Account")
				st := org.Objects["Account"]
				rule := fuzzyRule(fuzzyNativeRow{Object: "Account", Field: "Phone", Method: "Phone", Blank: "NullNotAllowed"})
				rule.ActionOnInsert, rule.ActionOnUpdate = "Block", "Block"
				st.Definition.DuplicateRules = append(st.Definition.DuplicateRules, rule)
				org.Objects["Account"] = st
				return org
			}
			record := func(name, phone string) storage.Record {
				return storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(name), "Phone": storage.StringValue(phone)}}
			}
			org := newOrg()
			eng := NewEngine(org)
			eng.Options.EnforceDuplicateRules = true
			if r := eng.Insert([]storage.Record{record("Seed", "1-800-FLOWERS")})[0]; !r.Success {
				t.Fatalf("seed: %+v", r)
			}
			if r := eng.Insert([]storage.Record{record("Match", "800-356-9377")})[0]; r.Success || r.StatusCode != "DUPLICATES_DETECTED" {
				t.Fatalf("insert: %+v", r)
			}
			if r := eng.Insert([]storage.Record{record("Other", "999-123-4567")})[0]; !r.Success {
				t.Fatalf("other: %+v", r)
			}
			var other storage.Record
			for _, r := range org.Objects["Account"].Records {
				if v, ok := r.GetField("Name"); ok && v.String == "Other" {
					other = r.Clone()
				}
			}
			other.Fields["Phone"] = storage.StringValue("800-356-9377")
			if r := eng.Update([]storage.Record{other})[0]; r.Success || r.StatusCode != "DUPLICATES_DETECTED" {
				t.Fatalf("update: %+v", r)
			}
			bulkOrg := newOrg()
			bulk := NewEngine(bulkOrg)
			bulk.Options.EnforceDuplicateRules = true
			for _, r := range bulk.Insert([]storage.Record{record("One", "1-800-FLOWERS"), record("Two", "800-356-9377")}) {
				if !r.Success {
					t.Fatalf("bulk: %+v", r)
				}
			}
		})
	}
}

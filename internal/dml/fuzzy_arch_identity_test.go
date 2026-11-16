package dml

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Go fuses x*y+z into one FMA instruction on arm64 and not on amd64, so an
// unguarded product feeding a sum changes the last bit of a confidence by
// architecture. The scorers convert such products with float64(...), which the
// Go spec says prevents fusion. These bit patterns come from amd64, which never
// fuses, and must hold on every architecture.
func TestFuzzyConfidenceBitsAreArchitectureIndependent(t *testing.T) {
	cases := []struct {
		method, left, right string
		bits                uint64
	}{
		{"FirstName", "Angelica", "Angelina", 0x4058955555555556},
		{"FirstName", "Angelica", "Ahngelica", 0x4058955555555555},
		{"FirstName", "Angelica", "Angelika", 0x4058955555555556},
		{"FirstName", "Angelica", "Angel", 0x4058380000000000},
		{"FirstName", "Michael", "Michaeley", 0x4058b8e38e38e38e},
		{"LastName", "Hartwell", "Hartwel", 0x4059000000000000},
		{"LastName", "Smith", "Smyth", 0x4059000000000000},
		{"City", "Los Angeles", "Los Angles", 0x4057dd1745d1745d},
		{"Street", "100 Main Street", "100 Main Stret", 0x4052cf3cf3cf3cf4},
	}
	for _, c := range cases {
		_, got := FuzzyScore(c.method, c.left, c.right, false)
		if math.Float64bits(got) != c.bits {
			t.Errorf("%s %q %q = %v (%#x); want bits %#x", c.method, c.left, c.right, got, math.Float64bits(got), c.bits)
		}
	}
}

// The confidence dump covers every native fuzzy fixture pair through the scorers
// and the shared duplicate-rule path, with raw float bits and match order. Set
// GLADE_FUZZY_DUMP to a path and run the test built for each architecture; the
// two files must be byte identical.
func TestFuzzyConfidenceDump(t *testing.T) {
	path := os.Getenv("GLADE_FUZZY_DUMP")
	if path == "" {
		t.Skip("GLADE_FUZZY_DUMP is not set")
	}
	raw, err := os.ReadFile(filepath.Join(fuzzyFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 4)
		var row fuzzyNativeRow
		if err := json.Unmarshal([]byte(cells[3]), &row); err != nil {
			t.Fatal(err)
		}
		id := cells[0] + "/" + cells[2]
		matchBlanks := strings.EqualFold(row.Blank, "MatchBlanks")
		if !strings.Contains(row.Field, "+") {
			for i, a := range row.Values {
				for j, b := range row.Values {
					if a == nil || b == nil {
						continue
					}
					ok, score := FuzzyScore(row.Method, *a, *b, matchBlanks)
					rawConf := FuzzyRawConfidence(row.Method, *a, *b, matchBlanks)
					out = append(out, fmt.Sprintf("%s score %d %d %t %016x %016x", id, i, j, ok, math.Float64bits(score), math.Float64bits(rawConf)))
				}
			}
		}
		org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
		storage.EnsureStandardObject(&org, row.Object)
		state := org.Objects[row.Object]
		if state.Records == nil {
			state.Records = map[storage.ID]storage.Record{}
		}
		index := map[storage.ID]int{}
		for i, value := range row.Values {
			record := fuzzyRecord(row, value)
			record.ID = storage.ID(fmt.Sprintf("%s%012d", map[string]string{"Account": "001", "Contact": "003"}[row.Object], i+1))
			state.Records[record.ID] = record
			index[record.ID] = i
		}
		org.Objects[row.Object] = state
		rule := fuzzyRule(row)
		for b, value := range row.Values {
			result, supported := FindDuplicateMatches(&org, rule, fuzzyRecord(row, value), nil)
			if !supported {
				out = append(out, fmt.Sprintf("%s match %d unsupported", id, b))
				continue
			}
			for _, match := range result.Matches {
				var parts []string
				for i, record := range match.Records {
					parts = append(parts, fmt.Sprintf("%d:%016x", index[record.ID], math.Float64bits(match.Confidences[i])))
				}
				out = append(out, fmt.Sprintf("%s match %d %s", id, b, strings.Join(parts, " ")))
			}
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatal("empty dump")
	}
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

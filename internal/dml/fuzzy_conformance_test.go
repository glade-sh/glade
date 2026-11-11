package dml

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const fuzzyFixture = "../apextest/testdata/conformance/duplicate_fuzzy_matching"

type fuzzyNativeRow struct {
	Object  string            `json:"object"`
	Field   string            `json:"field"`
	Method  string            `json:"method"`
	Blank   string            `json:"blank"`
	Const   map[string]string `json:"const"`
	Values  []*string         `json:"values"`
	Matches [][3]float64      `json:"matches"`
}

type fuzzyBound struct {
	// MinRecall is the least share of native matching pairs Glade must match.
	MinRecall float64 `json:"min_recall"`
	// MaxExtra is the most pairs Glade may match that native did not.
	MaxExtra int `json:"max_extra"`
	// MaxConfidenceError is the largest mean absolute difference allowed
	// between Glade's rounded match confidence and the native confidence over
	// the pairs both match.
	MaxConfidenceError float64 `json:"max_conf_mae"`
}

type fuzzyCases struct {
	Bounds map[string]fuzzyBound `json:"bounds"`
}

func fuzzyRequiredField(object string) string {
	if object == "Account" {
		return "Name"
	}
	return "LastName"
}

// fuzzyRecord builds one probe record. Values with no text stay absent, and the
// object's required field takes a placeholder only when the probe leaves it
// out, as the native capture did.
func fuzzyRecord(row fuzzyNativeRow, value *string) storage.Record {
	record := storage.Record{Object: row.Object, Fields: map[string]storage.Value{}}
	for field, constant := range row.Const {
		record.Fields[field] = storage.StringValue(constant)
	}
	if value != nil {
		record.Fields[row.Field] = storage.StringValue(*value)
	}
	if _, ok := record.Fields[fuzzyRequiredField(row.Object)]; !ok {
		record.Fields[fuzzyRequiredField(row.Object)] = storage.StringValue("Blankrow")
	}
	return record
}

func fuzzyRule(row fuzzyNativeRow) storage.DuplicateRule {
	matching := storage.MatchingRule{
		Object: row.Object, Name: "Probe", Status: "Active",
		Items: []storage.MatchingRuleItem{{Field: row.Field, Method: row.Method, BlankBehavior: row.Blank}},
	}
	if row.Field == "Title" {
		matching.BooleanFilter = "1 AND 2"
		matching.Items = append(matching.Items, storage.MatchingRuleItem{Field: "LastName", Method: "Exact", BlankBehavior: "NullNotAllowed"})
	}
	return storage.DuplicateRule{Name: "ProbeRule", Object: row.Object, Active: true, MatchRules: []storage.MatchingRule{matching}}
}

// fuzzyGladeMatches returns the (candidate, existing) index pairs Glade
// reports through the shared duplicate-rule path with their match
// confidences.
func fuzzyGladeMatches(t *testing.T, row fuzzyNativeRow) (map[[2]int]float64, string) {
	t.Helper()
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
	got := map[[2]int]float64{}
	engine := ""
	for b, value := range row.Values {
		result, supported := FindDuplicateMatches(&org, rule, fuzzyRecord(row, value), nil)
		if !supported {
			return nil, "unsupported"
		}
		for _, match := range result.Matches {
			engine = match.Engine
			for i, record := range match.Records {
				got[[2]int{b, index[record.ID]}] = math.Floor(match.Confidences[i] + 0.5)
			}
		}
	}
	return got, engine
}

// The native Datacloud.FindDuplicates verdicts and match confidences for
// custom fuzzy matching rules (every pair of probe values per method, API 62.0
// and 67.0) are reproduced by the shared duplicate-rule matcher. The bounds in
// cases.json are the measured agreement of the fitted scorers; unmeasured
// behavior is listed in docs/DUPLICATE_FUZZY_MATCHING.md.
func TestFuzzyMatchingNativeRows(t *testing.T) {
	var cases fuzzyCases
	raw, err := os.ReadFile(filepath.Join(fuzzyFixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(fuzzyFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if lines[0] != "api\troute\tid\tvalue" {
		t.Fatal("invalid native row header")
	}
	seen := 0
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 {
			t.Fatalf("invalid native row: %q", line)
		}
		var row fuzzyNativeRow
		if err := json.Unmarshal([]byte(cells[3]), &row); err != nil {
			t.Fatalf("%s: %v", cells[2], err)
		}
		name := cells[0] + "/" + cells[2]
		bound, ok := cases.Bounds[name]
		if !ok {
			t.Errorf("%s: no bound in cases.json", name)
			continue
		}
		seen++
		t.Run(name, func(t *testing.T) {
			got, engine := fuzzyGladeMatches(t, row)
			if got == nil {
				t.Fatalf("custom fuzzy rule is not enforced (%s)", engine)
			}
			native := map[[2]int]float64{}
			for _, m := range row.Matches {
				native[[2]int{int(m[0]), int(m[1])}] = m[2]
			}
			hit, extra, confidenceError := 0, 0, 0.0
			for pair, confidence := range got {
				if nativeConfidence, ok := native[pair]; ok {
					hit++
					confidenceError += math.Abs(confidence - nativeConfidence)
				} else {
					extra++
				}
			}
			recall, mae := 1.0, 0.0
			if len(native) > 0 {
				recall = float64(hit) / float64(len(native))
			}
			if hit > 0 {
				mae = confidenceError / float64(hit)
			}
			t.Logf("recall=%.4f extra=%d conf_mae=%.3f native=%d glade=%d", recall, extra, mae, len(native), len(got))
			if recall+1e-9 < bound.MinRecall || extra > bound.MaxExtra || mae > bound.MaxConfidenceError+1e-9 {
				t.Errorf("recall=%.4f (min %.4f) extra=%d (max %d) conf_mae=%.3f (max %.3f); native=%d glade=%d", recall, bound.MinRecall, extra, bound.MaxExtra, mae, bound.MaxConfidenceError, len(native), len(got))
			}
			if engine != storage.StandardMatchEngine {
				t.Errorf("match engine %q; native reports %q for fuzzy custom rules", engine, storage.StandardMatchEngine)
			}
		})
	}
	if seen != len(cases.Bounds) {
		t.Errorf("native rows %d; bounds %d", seen, len(cases.Bounds))
	}
}

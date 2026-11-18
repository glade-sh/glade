package dml

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE fuzzy ties keep creation order anonymous 67.0 ../apextest/testdata/conformance/duplicate_fuzzy_tie_order/org.tsv:3
// NATIVE fuzzy ties keep creation order istest 67.0 ../apextest/testdata/conformance/duplicate_fuzzy_tie_order/org.tsv:7
// Native match records whose shown (whole number) confidences are equal come
// back in creation order, even when their unrounded scores differ. Glade sorts
// on the shown confidence with a stable sort, so the order cannot depend on a
// last-bit difference between architectures. A variant is checked when native
// shows one confidence for every record; Glade must return the same names in
// the same order.
func TestFuzzyTieOrderIsCreationOrder(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/duplicate_fuzzy_tie_order", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 7)
		if cells[2] != "anonymous" {
			continue
		}
		for _, variant := range strings.Split(cells[5], ";") {
			fields := map[string]string{}
			for _, part := range strings.Fields(variant) {
				if k, v, ok := strings.Cut(part, "="); ok {
					fields[k] = v
				}
			}
			created, returned := strings.Split(fields["created"], ","), strings.Split(fields["returned"], ",")
			shown := map[string]bool{}
			var wantNames []string
			for _, item := range returned {
				name, confidence, _ := strings.Cut(item, "=")
				shown[confidence] = true
				wantNames = append(wantNames, name)
			}
			if len(shown) != 1 {
				continue
			}
			org := storage.OrgState{APIVersion: cells[1], Objects: map[string]storage.ObjectState{}}
			storage.EnsureStandardObject(&org, "Contact")
			state := org.Objects["Contact"]
			state.Records = map[storage.ID]storage.Record{}
			start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			for i, name := range created {
				id := storage.ID(fmt.Sprintf("003%012d", i+1))
				state.Records[id] = storage.Record{ID: id, Object: "Contact", Fields: map[string]storage.Value{"FirstName": storage.StringValue(name), "LastName": storage.StringValue("Zz")}, System: storage.SystemFields{CreatedDate: start.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)}}
			}
			org.Objects["Contact"] = state
			rule := storage.DuplicateRule{Name: "Tie", Object: "Contact", Active: true, MatchRules: []storage.MatchingRule{{Object: "Contact", Name: "First", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "FirstName", Method: "FirstName", BlankBehavior: "NullNotAllowed"}}}}}
			candidate := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"FirstName": storage.StringValue(fields["cand"]), "LastName": storage.StringValue("Qq")}}
			found, supported := FindDuplicateMatches(&org, rule, candidate, nil)
			if !supported || len(found.Matches) != 1 {
				t.Fatalf("%s: supported=%t matches=%d", variant, supported, len(found.Matches))
			}
			var gotNames []string
			for _, record := range found.Matches[0].Records {
				name, _ := duplicateFieldText(record, "FirstName")
				gotNames = append(gotNames, name)
			}
			if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
				t.Errorf("%s: Glade order %v; native %v", variant, gotNames, wantNames)
			}
			checked++
		}
	}
	if checked < 7 {
		t.Fatalf("checked %d tie variants; want at least 7", checked)
	}
}

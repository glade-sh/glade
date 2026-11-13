package dml

import (
	"fmt"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE earliest-created selection and stable ties ../apextest/testdata/conformance/duplicate_rule_evaluation/account_overflow/org.tsv:146 anonymous 62.0
// Give the captured 120 exact seeds reversed IDs so ID order cannot stand in
// for creation order. Updating LastModifiedDate must not affect selection.
func TestDuplicateMembersUseCreationOrder(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	state := org.Objects["Account"]
	state.Records = map[storage.ID]storage.Record{}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 120; i++ {
		id := storage.ID(fmt.Sprintf("001%012d", 120-i))
		state.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Exact synthetic company")}, System: storage.SystemFields{CreatedDate: start.Add(time.Duration(i) * time.Millisecond).Format(time.RFC3339Nano), LastModifiedDate: start.Add(time.Duration(120-i) * time.Hour).Format(time.RFC3339Nano)}}
	}
	org.Objects["Account"] = state
	rule := storage.DuplicateRule{Object: "Account", Name: "CreationOrder", MatchRules: []storage.MatchingRule{{Object: "Account", Name: "CompanyExact", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Name", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}}}
	found, supported := FindDuplicateMatches(&org, rule, storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Exact synthetic company")}}, nil)
	if !supported || len(found.Matches) != 1 || len(found.Matches[0].Records) != 100 {
		t.Fatalf("unexpected result: %+v supported=%t", found, supported)
	}
	for i, record := range found.Matches[0].Records {
		want := storage.ID(fmt.Sprintf("001%012d", 120-i))
		if record.ID != want || found.Matches[0].Confidences[i] != 100 {
			t.Fatalf("member %d: got %s/%v want %s/100", i, record.ID, found.Matches[0].Confidences[i], want)
		}
	}
}

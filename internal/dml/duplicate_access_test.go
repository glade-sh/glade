package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local performance contract: record visibility is evaluated only for matches.
// Detection and disclosure keep their independent meanings.
func TestDuplicateAccessOnlyForMatches(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Contact")
	state := org.Objects["Contact"]
	state.Records = map[storage.ID]storage.Record{}
	for i := 0; i < 1000; i++ {
		id := storage.ID(fmt.Sprintf("003%012d", i+1))
		state.Records[id] = storage.Record{ID: id, Object: "Contact", Fields: map[string]storage.Value{"Email": storage.StringValue(fmt.Sprintf("contact%d@example.invalid", i))}}
	}
	org.Objects["Contact"] = state
	rule := storage.DuplicateRule{Object: "Contact", Name: "Email", MatchRules: []storage.MatchingRule{{Object: "Contact", Name: "Email", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}}}
	candidate := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"Email": storage.StringValue("contact42@example.invalid")}}
	for _, policy := range []struct {
		name             string
		detect, disclose bool
	}{{"visible", true, true}, {"hiddenEnforce", false, false}, {"hiddenBypass", true, false}} {
		t.Run(policy.name, func(t *testing.T) {
			calls := 0
			got, supported := FindDuplicateMatchesWithAccess(&org, rule, candidate, nil, func(existing storage.Record) (bool, bool) { calls++; return policy.detect, policy.disclose })
			if !supported || calls != 1 {
				t.Fatalf("supported=%t access calls=%d, want 1", supported, calls)
			}
			if got.HasRecords() != policy.detect || (len(got.Matches[0].Records) == 1) != policy.disclose {
				t.Fatalf("unexpected access result: %+v", got)
			}
		})
	}
}

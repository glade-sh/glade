package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func partnerLeadDuplicateVisibility(t testing.TB) (*VM, storage.DuplicateRule, storage.Record) {
	t.Helper()
	machine, _ := partnerLeadChildVisibilityMachine(t)
	machine.currentMethod = Method{APIVersion: "67.0"}
	rule := storage.DuplicateRule{Name: "SyntheticExact", Object: "Lead", Active: true, SecurityOption: "EnforceSharingRules", MatchRules: []storage.MatchingRule{{Object: "Lead", Name: "SyntheticExact", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "LastName", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}}}
	candidate := storage.Record{Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue("Neighbor")}}
	return machine, rule, candidate
}

func TestPartnerLeadDuplicateVisibilityPublicNeighbor(t *testing.T) {
	machine, rule, candidate := partnerLeadDuplicateVisibility(t)
	result, ok := machine.findDuplicateMatches(rule, candidate, nil, nil, &dml.DuplicateMatchScan{}, func(string, string) bool { return true })
	if !ok || len(result.Matches) != 1 || len(result.Matches[0].Records) != 100 {
		t.Fatalf("duplicate matches: ok=%v result=%+v", ok, result)
	}
}

func BenchmarkPartnerLeadDuplicateVisibilityOwnerShares(b *testing.B) {
	machine, rule, candidate := partnerLeadDuplicateVisibility(b)
	scan := &dml.DuplicateMatchScan{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, ok := machine.findDuplicateMatches(rule, candidate, nil, nil, scan, func(string, string) bool { return true })
		if !ok || len(result.Matches) != 1 || len(result.Matches[0].Records) != 100 {
			b.Fatalf("duplicate matches: ok=%v result=%+v", ok, result)
		}
	}
}

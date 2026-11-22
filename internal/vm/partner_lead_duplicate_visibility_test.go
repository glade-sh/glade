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

func TestPartnerLeadDuplicateVisibilitySkipsInapplicableShareScan(t *testing.T) {
	machine, rule, candidate := partnerLeadDuplicateVisibility(t)
	leads := machine.Org.Objects["Lead"]
	first := leads.Records["00Q000000000001"]
	second := leads.Records["00Q000000000002"]
	among := map[storage.ID]storage.Record{first.ID: first, second.ID: second}
	find := func() []storage.Record {
		t.Helper()
		result, ok := machine.findDuplicateMatches(rule, candidate, nil, among, &dml.DuplicateMatchScan{}, func(string, string) bool { return true })
		if !ok || len(result.Matches) != 1 {
			t.Fatalf("expected one matching rule, got %+v", result)
		}
		return result.Matches[0].Records
	}
	end := machine.beginPartnerLeadShareScan()
	both := find()
	scanned := machine.partnerLeadShares.ready
	end()
	if len(both) != 2 || scanned {
		t.Fatalf("two public neighbors: returned %d records, scanned shares = %v", len(both), scanned)
	}
	second.System.CreatedByID = second.System.OwnerID
	leads.PutRecord(second.ID, second)
	among[second.ID] = second
	if rows := find(); len(rows) != 1 || rows[0].ID != first.ID {
		t.Fatalf("mixed public and restricted Leads: visible rows = %v", rows)
	}
}

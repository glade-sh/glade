package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPicklistSourcePolicyPreservesCachedQueries(t *testing.T) {
	for _, tc := range []struct {
		source, transport string
		legacy            bool
	}{
		{"62.0", "67.0", true},
		{"64.0", "67.0", true},
		{"65.0", "67.0", true},
		{"66.0", "67.0", false},
		{"67.0", "62.0", false},
		// Uncaptured missing/older source retains the local zero-value policy.
		{"", "62.0", false},
		{"61.0", "67.0", false},
	} {
		t.Run(tc.source+"/"+tc.transport, func(t *testing.T) {
			machine := New(nil)
			org := storage.NewOrgState()
			org.APIVersion = tc.transport
			machine.SetOrg(&org)
			machine.currentMethod.APIVersion = tc.source
			query := soql.Query{Order: []soql.OrderSpec{{Nulls: "LAST"}}, ChildQueries: []soql.ChildQuery{{
				Relationship: "Entries__r", Query: soql.Query{ChildQueries: []soql.ChildQuery{{Relationship: "Nested__r"}}},
			}}}
			got := machine.applyPicklistSourcePolicy(query)
			if got.LegacyPicklistNullOrder != tc.legacy || got.ChildQueries[0].Query.LegacyPicklistNullOrder != tc.legacy || got.ChildQueries[0].Query.ChildQueries[0].Query.LegacyPicklistNullOrder != tc.legacy {
				t.Fatal("query policy did not follow the declaring source")
			}
			if query.LegacyPicklistNullOrder || query.ChildQueries[0].Query.LegacyPicklistNullOrder || query.ChildQueries[0].Query.ChildQueries[0].Query.LegacyPicklistNullOrder || query.Order[0].Nulls != "LAST" {
				t.Fatal("source policy mutated cached query syntax")
			}
			if tc.legacy {
				got.ChildQueries[0].Relationship = "Changed__r"
				got.ChildQueries[0].Query.ChildQueries[0].Relationship = "ChangedNested__r"
				if query.ChildQueries[0].Relationship != "Entries__r" || query.ChildQueries[0].Query.ChildQueries[0].Relationship != "Nested__r" {
					t.Fatal("child query execution state shares cached slices")
				}
			}
		})
	}
}

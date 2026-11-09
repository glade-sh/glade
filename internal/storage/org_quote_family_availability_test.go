package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

// Native plain and quote-setting rows: org_quote_family/org.tsv at 62 and 67.
func TestOrgQuoteFamilyAvailabilityShapes(t *testing.T) {
	objects := []string{"QuoteDocument", "QuoteFeed", "QuoteHistory", "QuoteLineItemHistory", "QuoteShare"}
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false}, {"unrelated", []string{"Communities"}, false},
		{"enabled", []string{"Quotes"}, true}, {"unspecified", []string{project.UnspecifiedStandardFieldShape}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			for _, name := range objects {
				EnsureStandardObject(&org, name)
			}
			ApplyProjectOrgShape(&org, tc.features)
			for _, name := range objects {
				if got := org.AllowsStandardObject(name); got != tc.want {
					t.Errorf("%s allowed=%t want=%t", name, got, tc.want)
				}
				if _, got := org.Objects[name]; got != tc.want {
					t.Errorf("%s present=%t want=%t", name, got, tc.want)
				}
			}
		})
	}
}

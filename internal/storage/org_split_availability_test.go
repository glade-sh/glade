package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

func TestOrgSplitAvailabilityShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false}, {"team-only", []string{"OpportunityTeam"}, false}, {"split-only", []string{"OpportunitySplits"}, false}, {"enabled", []string{"OpportunityTeam", "OpportunitySplits"}, true}, {"unspecified", []string{project.UnspecifiedStandardFieldShape}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, "OpportunitySplit")
			ApplyProjectOrgShape(&org, tc.features)
			if got := org.AllowsStandardObject("OpportunitySplit"); got != tc.want {
				t.Errorf("allowed=%t want=%t", got, tc.want)
			}
			if _, got := org.Objects["OpportunitySplit"]; got != tc.want {
				t.Errorf("present=%t want=%t", got, tc.want)
			}
		})
	}
}

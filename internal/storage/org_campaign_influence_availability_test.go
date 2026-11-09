package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

// Both plain and feature-on scratch orgs without a CRM User license reject
// query and global describe: org_object_availability/org.tsv:2-5 and
// org_influence2_availability/org.tsv:2-5.
func TestOrgCampaignInfluenceAvailabilityShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false}, {"communities", []string{"Communities"}, false},
		{"feature-no-license", []string{"CampaignInfluence2"}, false},
		{"unspecified", []string{project.UnspecifiedStandardFieldShape}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, "CampaignInfluence")
			ApplyProjectOrgShape(&org, tc.features)
			if got := org.AllowsStandardObject("CampaignInfluence"); got != tc.want {
				t.Errorf("allowed=%t want=%t", got, tc.want)
			}
			if _, got := org.Objects["CampaignInfluence"]; got != tc.want {
				t.Errorf("present=%t want=%t", got, tc.want)
			}
		})
	}
}

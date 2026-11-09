package storage

import "testing"

func TestOrgMergeFeatureGates(t *testing.T) {
	for _, tc := range []struct {
		name         string
		features     []string
		split, model bool
	}{
		{"plain", nil, false, false},
		{"split-only", []string{"OpportunitySplits"}, false, false},
		{"team-only", []string{"OpportunityTeam"}, false, false},
		{"both", []string{"OpportunityTeam", "OpportunitySplits"}, true, false},
		{"campaign", []string{"CampaignInfluence2Enabled"}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			ApplyProjectOrgShape(&org, tc.features)
			for _, object := range []string{"OpportunitySplit", "OpportunitySplitType"} {
				if got := org.AllowsStandardObject(object); got != tc.split {
					t.Errorf("%s allowed=%t want=%t", object, got, tc.split)
				}
			}
			if got := org.AllowsStandardObject("CampaignInfluenceModel"); got != tc.model {
				t.Errorf("CampaignInfluenceModel allowed=%t want=%t", got, tc.model)
			}
		})
	}
}

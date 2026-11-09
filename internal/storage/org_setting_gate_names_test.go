package storage

import "testing"

// Feature names match the documented ones: Forecasts and OpportunityTeam.
// The setting-triggered gates (org_settings_source/org.tsv) key on them, and
// a differently spelled name must not enable the objects.
func TestSettingGateFeatureNames(t *testing.T) {
	for _, tc := range []struct {
		features []string
		object   string
		want     bool
	}{
		{[]string{"Forecasts"}, "ForecastingItem", true},
		{[]string{"Forecasting"}, "ForecastingItem", false},
		{[]string{"OpportunityTeam"}, "OpportunityTeamMember", true},
		{[]string{"OpportunityTeams"}, "OpportunityTeamMember", false},
		{[]string{"AccountTeams"}, "AccountTeamMember", true},
		{[]string{"Territory2"}, "Territory2", true},
		{[]string{"Communities"}, "ForecastingItem", false},
	} {
		org := NewOrgState()
		ApplyProjectOrgShape(&org, tc.features)
		if got := org.AllowsStandardObject(tc.object); got != tc.want {
			t.Errorf("%v: %s allowed=%t want=%t", tc.features, tc.object, got, tc.want)
		}
	}
}

// Network and NetworkMember follow communitiesSettings.enableNetworksEnabled
// (org_settings_source/org.tsv), not the Communities feature.
func TestNetworkObjectsNeedNetworksEnabledShape(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false},
		{"communities-only", []string{"Communities"}, false},
		{"networks-enabled", []string{"NetworksEnabled"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			ApplyProjectOrgShape(&org, tc.features)
			for _, object := range []string{"Network", "NetworkMember"} {
				if got := org.AllowsStandardObject(object); got != tc.want {
					t.Errorf("%s allowed=%t want=%t", object, got, tc.want)
				}
				if _, got := org.Objects[object]; got != tc.want {
					t.Errorf("%s present=%t want=%t", object, got, tc.want)
				}
			}
		})
	}
}

package project

import (
	"slices"
	"testing"
)

func TestSetupShapeSettingsCaptured(t *testing.T) {
	for _, tc := range []struct{ block, key, feature string }{
		{"accountSettings", "enableAccountTeams", "AccountTeams"},
		{"forecastingSettings", "enableForecasts", "Forecasts"},
		{"opportunitySettings", "enableOpportunityTeam", "OpportunityTeam"},
		{"omniChannelSettings", "enableOmniChannel", "OmniChannel"},
		{"surveySettings", "enableSurvey", "Surveys"},
	} {
		t.Run(tc.feature, func(t *testing.T) {
			for _, v := range []bool{false, true} {
				got := scratchSettingsFeatures(map[string]any{tc.block: map[string]any{tc.key: v}})
				if slices.Contains(got, tc.feature) != v {
					t.Errorf("value %t: features %v", v, got)
				}
			}
		})
	}
}

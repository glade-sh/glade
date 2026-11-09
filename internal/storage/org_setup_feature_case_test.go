package storage

import (
	"strings"
	"testing"
)

func TestOrgSetupAvailabilityFeatureCaseFold(t *testing.T) {
	for _, row := range []struct{ feature, object string }{
		{"Territory2", "Territory2"}, {"AccountTeams", "AccountTeamMember"}, {"Forecasts", "ForecastingType"}, {"OpportunityTeam", "OpportunityTeamMember"}, {"OmniChannel", "ServiceChannel"}, {"Surveys", "Survey"},
	} {
		t.Run(row.feature, func(t *testing.T) {
			org := NewOrgState()
			ApplyProjectOrgShape(&org, []string{strings.ToLower(row.feature)})
			if !org.AllowsStandardObject(row.object) {
				t.Errorf("%s should be enabled by %s case-insensitively", row.object, row.feature)
			}
		})
	}
}

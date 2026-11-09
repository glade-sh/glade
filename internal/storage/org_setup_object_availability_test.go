package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

func TestOrgSetupObjectAvailabilityShapes(t *testing.T) {
	groups := []struct {
		feature string
		names   []string
	}{
		{"AccountTeams", []string{"AccountTeamMember"}},
		{"Forecasts", []string{"ForecastingItem", "ForecastingQuota", "ForecastingType"}},
		{"OpportunityTeam", []string{"OpportunityTeamMember"}},
		{"OmniChannel", []string{"ServiceChannel"}},
		{"Surveys", []string{"Survey"}},
	}
	for _, g := range groups {
		t.Run(g.feature, func(t *testing.T) {
			for _, tc := range []struct {
				name     string
				features []string
				want     bool
			}{
				{"plain", nil, false}, {"unrelated", []string{"Communities"}, false}, {"enabled", []string{g.feature}, true}, {"unspecified", []string{project.UnspecifiedStandardFieldShape}, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					org := NewOrgState()
					for _, n := range g.names {
						EnsureStandardObject(&org, n)
					}
					ApplyProjectOrgShape(&org, tc.features)
					for _, n := range g.names {
						if got := org.AllowsStandardObject(n); got != tc.want {
							t.Errorf("%s allowed=%t want=%t", n, got, tc.want)
						}
						if _, got := org.Objects[n]; got != tc.want {
							t.Errorf("%s present=%t want=%t", n, got, tc.want)
						}
					}
				})
			}
		})
	}
}

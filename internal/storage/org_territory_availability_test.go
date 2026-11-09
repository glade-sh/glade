package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

func TestOrgTerritoryAvailabilityShapes(t *testing.T) {
	names := []string{"Territory2", "Territory2Model", "Territory2Type", "UserTerritory2Association", "ObjectTerritory2Association"}
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false}, {"communities", []string{"Communities"}, false}, {"enabled", []string{"Territory2"}, true}, {"unspecified", []string{project.UnspecifiedStandardFieldShape}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			for _, n := range names {
				EnsureStandardObject(&org, n)
			}
			ApplyProjectOrgShape(&org, tc.features)
			for _, n := range names {
				if got := org.AllowsStandardObject(n); got != tc.want {
					t.Errorf("%s allowed=%t want=%t", n, got, tc.want)
				}
				if _, got := org.Objects[n]; got != tc.want {
					t.Errorf("%s present=%t want=%t", n, got, tc.want)
				}
			}
		})
	}
}

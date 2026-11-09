package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

// Native rows: ../apextest/testdata/conformance/org_knowledge_availability/org.tsv.
func TestOrgKnowledgeAvailabilityShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false},
		{"feature-only", []string{"Knowledge"}, false},
		{"setting-only", []string{"KnowledgeEnabled"}, true},
		{"enabled", []string{"Knowledge", "KnowledgeEnabled"}, true},
		{"unspecified", []string{project.UnspecifiedStandardFieldShape}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			for _, n := range []string{"Knowledge__ka", "Knowledge__kav"} {
				EnsureStandardObject(&org, n)
			}
			ApplyProjectOrgShape(&org, tc.features)
			for _, n := range []string{"Knowledge__ka", "Knowledge__kav"} {
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

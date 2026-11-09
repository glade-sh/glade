package storage

import "testing"

// Native rows: org_features_additive/org.tsv and org_network_setting/org.tsv.
func TestOrgNetworkAvailabilityShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false},
		{"communities-only", []string{"Communities"}, false},
		{"enabled", []string{"Communities", "NetworksEnabled"}, true},
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

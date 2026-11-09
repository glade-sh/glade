package storage

import "testing"

// Native org_community_history/org.tsv: describe/type and filtered query, APIs 62/67.
func TestOrgCommunityHistoryAvailabilityCaptured(t *testing.T) {
	for _, tc := range []struct {
		features []string
		want     bool
	}{
		{nil, false}, {[]string{"Communities"}, false}, {[]string{"NetworksEnabled"}, true}, {[]string{"Communities", "NetworksEnabled"}, true},
	} {
		org := NewOrgState()
		ApplyProjectOrgShape(&org, tc.features)
		if got := org.AllowsStandardObject("NetworkUserHistoryRecent"); got != tc.want {
			t.Errorf("%v: got %t, want %t", tc.features, got, tc.want)
		}
	}
}

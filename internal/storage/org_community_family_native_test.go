package storage

import (
	"encoding/csv"
	"os"
	"testing"
)

// org_community_family/org.tsv: setting-only and combined shapes at both APIs and routes.
func TestOrgCommunityFamilyAvailabilityCaptured(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/org_community_family/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 20 {
		t.Fatalf("insufficient rows: %d", len(rows)-1)
	}
	for _, tc := range []struct {
		features []string
		want     bool
	}{{nil, false}, {[]string{"Communities"}, false}, {[]string{"NetworksEnabled"}, true}, {[]string{"Communities", "NetworksEnabled"}, true}} {
		org := NewOrgState()
		ApplyProjectOrgShape(&org, tc.features)
		for _, row := range rows[1:] {
			if got := org.AllowsStandardObject(row[0]); got != tc.want {
				t.Errorf("%v %s: got %t, want %t", tc.features, row[0], got, tc.want)
			}
		}
	}
}

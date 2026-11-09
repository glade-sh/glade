package storage

import (
	"reflect"
	"slices"
	"testing"
)

// The 68 inventory observations are captured under standard_describe_api68_drift.
func TestStandardDescribeProfileCatalogAPI68(t *testing.T) {
	for _, profile := range []string{"P0", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9", "P10", "P11", "PU"} {
		t.Run(profile, func(t *testing.T) {
			delta, ok := StandardDescribeCatalogProfileDelta(profile, "68.0")
			if !ok {
				t.Fatal("captured API 68 profile missing")
			}
			baseline := "P0"
			if profile == "P10" {
				baseline = "P4"
			}
			if delta.BaselineProfile != baseline {
				t.Fatalf("baseline=%s, want %s", delta.BaselineProfile, baseline)
			}
			if profile == "P0" && (len(delta.Fields.Added)+len(delta.Fields.Removed)+len(delta.Objects.Added)+len(delta.Objects.Removed) != 0) {
				t.Fatalf("P0 differs from itself: %+v", delta)
			}
			if profile == "P10" && !slices.Contains(delta.Objects.Added, "WishlistItemRelationship") {
				t.Fatal("captured Commerce object addition missing")
			}
			if profile == "P11" && !slices.Contains(delta.Objects.Removed, "ShiftAllocationRequest") {
				t.Fatal("captured Enterprise object removal missing")
			}
			if len(delta.Objects.Added) > 0 {
				before := delta.Objects.Added[0]
				delta.Objects.Added[0] = "mutated"
				again, _ := StandardDescribeCatalogProfileDelta(profile, "68.0")
				if reflect.DeepEqual(delta, again) || again.Objects.Added[0] != before {
					t.Fatal("lookup did not return an owned copy")
				}
			}
		})
	}
}

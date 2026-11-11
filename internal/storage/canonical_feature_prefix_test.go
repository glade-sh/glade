package storage

import (
	"strings"
	"testing"
)

// canonicalFeatureName runs for every token of a feature signature on every
// overlay check. It must answer the HealthCloud prefix as the lower-cased
// prefix test did, without lower-casing the token.
func TestCanonicalFeatureNameHealthCloudPrefixWithoutLowerCasing(t *testing.T) {
	reference := func(feature string) bool {
		if idx := strings.IndexByte(feature, ':'); idx >= 0 {
			feature = feature[:idx]
		}
		return strings.HasPrefix(strings.ToLower(feature), "healthcloud")
	}
	for _, feature := range []string{
		"HealthCloud", "healthcloud", "HEALTHCLOUD", "HealthCloudAddOn", "HealthCloud:1", "healthCloudUser",
		"Health", "HealthCloudX", "xHealthCloud", "", "MultiCurrency", "PersonAccounts", "UnspecifiedShape", "Industries",
		"HealthCloüd", "HEALTHCLOUDé", "ĤealthCloud", "KealthCloud", "healthclou", "healthclouD",
	} {
		got := canonicalFeatureName(feature) == "HealthCloud"
		if want := reference(feature); got != want {
			t.Fatalf("canonicalFeatureName(%q) HealthCloud = %v, lower-cased prefix test = %v", feature, got, want)
		}
	}
	for _, feature := range []string{"Industries", "UnspecifiedShape", "HealthCloudAddOn", "Chatter", "Quotes"} {
		allocations := testing.AllocsPerRun(50, func() {
			_ = canonicalFeatureName(feature)
		})
		if allocations != 0 {
			t.Fatalf("canonicalFeatureName(%q) allocated %.0f times", feature, allocations)
		}
	}
}

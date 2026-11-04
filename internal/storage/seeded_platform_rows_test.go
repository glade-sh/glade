package storage

import "testing"

// Native rows: ../apextest/testdata/conformance/seeded_platform_rows/org.tsv.
// Enabling a capability supplies schema, not configured setup records.
func TestOrgShapeDoesNotInventConfiguredPlatformRows(t *testing.T) {
	for _, feature := range []string{"MultiCurrency", "PlatformCache"} {
		for _, route := range []string{"shape", "fixture"} {
			t.Run(feature+"/"+route, func(t *testing.T) {
				org := NewOrgState()
				EnsureDeterministicPlatformData(&org)
				if route == "fixture" {
					if err := ApplyFixture(&org, Fixture{Version: FixtureVersion, Org: FixtureOrg{Features: []string{feature}}}); err != nil {
						t.Fatal(err)
					}
				} else {
					ApplyOrgShape(&org, []string{feature})
				}
				for pass := 0; pass < 2; pass++ {
					for _, name := range []string{"DatedConversionRate", "PlatformCachePartition"} {
						if got := len(org.Objects[name].Records); got != 0 {
							t.Errorf("%s has %d implicit rows after pass %d; native has zero", name, got, pass)
						}
					}
					if feature == "MultiCurrency" && len(org.Objects["CurrencyType"].Records) != 1 {
						t.Fatal("corporate CurrencyType seed must remain available")
					}
					org = org.CloneRuntimeFrozenDefinition()
					EnsureDeterministicPlatformData(&org)
					ApplyOrgShape(&org, []string{feature})
				}
			})
		}
	}
}

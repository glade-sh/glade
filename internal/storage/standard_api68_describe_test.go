package storage

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
)

// The inputs are the independently captured 68 object and field observations.
func TestStandardAPI68DescribeCapturedShapes(t *testing.T) {
	if _, ok := StandardAPI68DescribeObject("P0", "AddressChangeEvent"); !ok {
		t.Fatal("P0 captured object missing")
	}
	if _, ok := StandardAPI68DescribeObject("P11", "AddressChangeEvent"); ok {
		t.Fatal("P11 received an unobserved P0 object")
	}
	if _, ok := StandardAPI68DescribeObject("P10", "WishlistItemRelationship"); !ok {
		t.Fatal("P10 captured object missing")
	}
	if _, ok := StandardAPI68DescribeObject("P0", "WishlistItemRelationship"); ok {
		t.Fatal("P0 received an unobserved Commerce object")
	}
	fields := StandardAPI68DescribeFields("P0", "AssetStatePeriod", false)
	if _, ok := fields["RampUpliftType"]; !ok {
		t.Fatal("captured P0 field missing")
	}
	fields["RampUpliftType"] = Field{}
	if StandardAPI68DescribeFields("P0", "AssetStatePeriod", false)["RampUpliftType"].APIName == "" {
		t.Fatal("caller mutated retained fields")
	}
	if _, ok := StandardAPI68DescribeFields("P0", "WorkloadUnit", false)["IntradayActualVolume"]; ok {
		t.Fatal("named-only field leaked to anonymous route")
	}
	if _, ok := StandardAPI68DescribeFields("P0", "WorkloadUnit", true)["IntradayActualVolume"]; !ok {
		t.Fatal("named-only captured field missing")
	}
	if len(StandardAPI68DescribeFields("unknown", "AssetStatePeriod", false)) != 0 {
		t.Fatal("unknown profile acquired captured fields")
	}
}

// The generated name set must equal the bundle's captured 200 objects, so the
// cheap membership test can stand in for decoding the whole sidecar.
func TestStandardAPI68NewObjectNamesMatchBundle(t *testing.T) {
	want := standardAPI68BundleNewObjects()
	if len(want) == 0 {
		t.Fatal("bundle has no captured new objects")
	}
	for name := range want {
		if _, ok := standardAPI68NewObjectNames[name]; !ok {
			t.Errorf("generated set is missing %q; rerun scripts/generate-standard-api68-new-objects.py", name)
		}
		if !standardAPI68CapturedNewObject(strings.ToUpper(name)) {
			t.Errorf("standardAPI68CapturedNewObject(%q) = false", name)
		}
	}
	for name := range standardAPI68NewObjectNames {
		if _, ok := want[name]; !ok {
			t.Errorf("generated set has %q, which the bundle does not capture with status 200", name)
		}
		if strings.ContainsFunc(name, unicode.IsUpper) {
			t.Errorf("generated key %q is not lower-cased", name)
		}
	}
	if standardAPI68CapturedNewObject("Account") {
		t.Error("Account is not an API 68 new object")
	}
}

// The generated feature lists must equal the bundle's, so the profile lookup
// resolves the same profile as the decoded sidecar would.
func TestStandardAPI68ProfileFeaturesMatchBundle(t *testing.T) {
	want := standardAPI68BundleProfileFeatures()
	if len(want) == 0 {
		t.Fatal("bundle has no profiles")
	}
	if !reflect.DeepEqual(want, standardAPI68ProfileFeatures) {
		t.Fatalf("generated profile features differ from the bundle; rerun scripts/generate-standard-api68-new-objects.py\nbundle: %v\ngenerated: %v", want, standardAPI68ProfileFeatures)
	}
	if !reflect.DeepEqual(standardAPI68ProfileSignatures, standardAPI68GeneratedSignatures()) {
		t.Fatalf("generated signatures %v differ from the bundle's %v", standardAPI68GeneratedSignatures(), standardAPI68ProfileSignatures)
	}
	for name, features := range want {
		got, ok := StandardAPI68DescribeProfile(features)
		if !ok || got != name {
			t.Errorf("StandardAPI68DescribeProfile(%v) = %q, %v; want %q", features, got, ok, name)
		}
	}
	if _, ok := StandardAPI68DescribeProfile(nil); ok {
		t.Error("no features must not select a profile")
	}
}

// A plain Developer scratch definition also carries the derived plain
// object-shape marker; it must still select the captured P0 profile.
func TestStandardAPI68DescribeProfileIgnoresPlainObjectMarker(t *testing.T) {
	for _, features := range [][]string{
		{"__glade_edition_developer"},
		{"__glade_edition_developer", "__glade_plain_developer_objects"},
		{"__glade_plain_developer_objects", "__glade_edition_developer"},
	} {
		if profile, ok := StandardAPI68DescribeProfile(features); !ok || profile != "P0" {
			t.Fatalf("StandardAPI68DescribeProfile(%v) = %q, %v; want P0", features, profile, ok)
		}
	}
}

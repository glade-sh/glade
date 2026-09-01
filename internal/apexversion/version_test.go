package apexversion

import (
	"strings"
	"testing"
)

func TestMajor(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want int
		ok   bool
	}{
		{raw: "67.0", want: 67, ok: true},
		{raw: " 29 ", want: 29, ok: true},
		{raw: "", ok: false},
		{raw: "67.x", ok: false},
		{raw: "67.-1", ok: false},
		{raw: "+67.0", ok: false},
		{raw: "67.0.1", ok: false},
		{raw: "0.0", ok: false},
	} {
		got, ok := Major(test.raw)
		if got != test.want || ok != test.ok {
			t.Errorf("Major(%q) = (%d, %t), want (%d, %t)", test.raw, got, ok, test.want, test.ok)
		}
	}
}

func TestEnabledBoundaries(t *testing.T) {
	for _, test := range []struct {
		feature    Feature
		before     string
		after      string
		wantBefore bool
		wantAfter  bool
	}{
		{LegacySiteURLHelpers, "29.0", "30.0", true, false},
		{LegacyCacheValueSize, "49.0", "50.0", true, false},
		{LegacyCacheValidateKeys, "54.0", "55.0", true, false},
		{SecureDefaults, "66.0", "67.0", false, true},
	} {
		if Enabled(test.before, test.feature) != test.wantBefore || Enabled(test.after, test.feature) != test.wantAfter {
			t.Errorf("feature %d boundary %q/%q behaved incorrectly", test.feature, test.before, test.after)
		}
	}
	for _, raw := range []string{"", "not-a-version", "67.x"} {
		if Enabled(raw, SecureDefaults) || Enabled(raw, LegacySiteURLHelpers) {
			t.Errorf("Enabled(%q, ...) accepted malformed version", raw)
		}
	}
}

func TestResolveSourceNormalizesSupportedVersions(t *testing.T) {
	for raw, want := range map[string]string{
		"":      "65.0",
		"v65.0": "65.0",
		"65.00": "65.0",
		"66.0":  "66.0",
		"67.0":  "67.0",
	} {
		got, err := ResolveSource(raw)
		if err != nil || got != want {
			t.Errorf("ResolveSource(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
}

func TestResolveSourceRejectsUnsupportedWithoutFallback(t *testing.T) {
	for _, raw := range []string{"64.0", "68.0", "67.1", "v", "+65.0", "junk"} {
		got, err := ResolveSource(raw)
		if err == nil || got != "" || !strings.Contains(err.Error(), raw) || !strings.Contains(err.Error(), "65.0, 66.0, 67.0") {
			t.Errorf("ResolveSource(%q) = %q, %v", raw, got, err)
		}
	}
}

func TestPreserveSourceKeepsHistoricalVersionsAndRejectsMalformedOrFuture(t *testing.T) {
	for _, raw := range []string{"1.0", "43.0", "61.0"} {
		got, err := PreserveSource(raw)
		if err != nil || got != raw {
			t.Errorf("PreserveSource(%q) = %q, %v", raw, got, err)
		}
	}
	for _, raw := range []string{"68.0", "67.1", "v", "+65.0", "junk"} {
		got, err := PreserveSource(raw)
		if err == nil || got != "" {
			t.Errorf("PreserveSource(%q) = %q, %v", raw, got, err)
		}
	}
}

func TestPreserveSourceRejectsWithoutCheckedVersions(t *testing.T) {
	versions := SupportedSourceAPIVersions
	SupportedSourceAPIVersions = nil
	t.Cleanup(func() { SupportedSourceAPIVersions = versions })

	if got, err := PreserveSource("43.0"); err == nil || got != "" {
		t.Errorf("PreserveSource(\"43.0\") = %q, %v", got, err)
	}
}

func TestRangeAllowsSinceInclusiveUntilExclusive(t *testing.T) {
	rng := Range{Since: 66, Until: 68}
	for raw, want := range map[string]bool{"65.0": false, "66.0": true, "67.0": true, "68.0": false, "junk": false} {
		if got := rng.Allows(raw); got != want {
			t.Errorf("Range.Allows(%q) = %t, want %t", raw, got, want)
		}
	}
}

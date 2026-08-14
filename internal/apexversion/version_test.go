package apexversion

import "testing"

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

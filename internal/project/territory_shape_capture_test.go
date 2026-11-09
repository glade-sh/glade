package project

import (
	"slices"
	"testing"
)

func TestTerritoryShapeSettingCaptured(t *testing.T) {
	for _, tc := range []struct {
		value bool
		want  bool
	}{{false, false}, {true, true}} {
		got := scratchSettingsFeatures(map[string]any{"territory2Settings": map[string]any{"enableTerritoryManagement2": tc.value}})
		if slices.Contains(got, "Territory2") != tc.want {
			t.Errorf("setting %t: features %v", tc.value, got)
		}
	}
}

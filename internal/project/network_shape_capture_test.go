package project

import (
	"slices"
	"testing"
)

func TestNetworkSettingShape(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		got := scratchSettingsFeatures(map[string]any{"communitiesSettings": map[string]any{"enableNetworksEnabled": enabled}})
		if slices.Contains(got, "NetworksEnabled") != enabled {
			t.Errorf("setting=%t features=%v", enabled, got)
		}
	}
}

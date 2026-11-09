package project

import (
	"slices"
	"testing"
)

func TestOrderManagementSettingShape(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		features := scratchSettingsFeatures(map[string]any{"orderManagementSettings": map[string]any{"enableOrderManagement": enabled}})
		if slices.Contains(features, "OrderManagementEnabled") != enabled {
			t.Errorf("setting=%t features=%v", enabled, features)
		}
	}
}

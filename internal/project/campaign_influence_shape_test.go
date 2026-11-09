package project

import (
	"slices"
	"testing"
)

func TestCampaignInfluenceSettingShape(t *testing.T) {
	for _, v := range []bool{false, true} {
		got := scratchSettingsFeatures(map[string]any{"campaignSettings": map[string]any{"enableCampaignInfluence2": v}})
		if slices.Contains(got, "CampaignInfluence2Enabled") != v {
			t.Errorf("setting %t: features %v", v, got)
		}
	}
}

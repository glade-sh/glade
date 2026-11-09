package project

import (
	"slices"
	"testing"
)

func TestKnowledgeSettingShape(t *testing.T) {
	for _, v := range []bool{false, true} {
		got := scratchSettingsFeatures(map[string]any{"knowledgeSettings": map[string]any{"enableKnowledge": v}})
		if slices.Contains(got, "KnowledgeEnabled") != v {
			t.Errorf("setting %t: features %v", v, got)
		}
	}
}

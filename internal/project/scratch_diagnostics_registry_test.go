package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Each trigger read by the shape loader must also be recognized by diagnostics.
func TestScratchDefinitionWarningsTrackShapeTriggers(t *testing.T) {
	for _, trigger := range scratchSettingTriggers {
		t.Run(trigger.block+"."+trigger.key, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "config", "project-scratch-def.json")
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			definition := map[string]any{"settings": map[string]any{
				trigger.block: map[string]any{trigger.key: true, "unreadOne": "private-value", "unreadTwo": false},
			}}
			data, err := json.Marshal(definition)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if got := OrgShapeFeatures(root); !slices.Contains(got, trigger.feature) {
				t.Fatalf("shape features %v do not include %s", got, trigger.feature)
			}
			warnings := ScratchDefinitionWarnings(root)
			if len(warnings) != 2 || !strings.Contains(warnings[0], trigger.block+".unreadOne") || !strings.Contains(warnings[1], trigger.block+".unreadTwo") {
				t.Fatalf("unexpected warnings: %v", warnings)
			}
			if strings.Contains(strings.Join(warnings, " "), "private-value") {
				t.Fatal("warning exposed a setting value")
			}
		})
	}
}

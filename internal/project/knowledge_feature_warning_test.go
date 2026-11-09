package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Knowledge is consumed as a project org feature by semantic visibility.
func TestKnowledgeFeatureIsNotReportedUnread(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "config")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project-scratch-def.json"), []byte(`{"edition":"Developer","features":["Knowledge"],"settings":{"knowledgeSettings":{"enableKnowledge":true}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	features := OrgShapeFeatures(root)
	// The setting also contributes KnowledgeEnabled (b8features).
	if !slices.Contains(features, "Knowledge") {
		t.Fatalf("feature not read: %v", features)
	}
	for _, warning := range ScratchDefinitionWarnings(root) {
		if strings.Contains(warning, "features.Knowledge") {
			t.Fatalf("read feature reported unread: %s", warning)
		}
	}
}

// Scratch definitions distinguish the Knowledge license feature and setting.
func TestKnowledgeScratchOrgSettingControlsFeature(t *testing.T) {
	cases := []struct {
		name, definition string
		want             bool
	}{
		{"feature_only", `{"features":["Knowledge"]}`, false},
		{"disabled", `{"features":["Knowledge"],"settings":{"knowledgeSettings":{"enableKnowledge":false}}}`, false},
		{"setting_only", `{"features":[],"settings":{"knowledgeSettings":{"enableKnowledge":true}}}`, true},
		{"both", `{"features":["Knowledge"],"settings":{"knowledgeSettings":{"enableKnowledge":true}}}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "config")
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "project-scratch-def.json"), []byte(tc.definition), 0600); err != nil {
				t.Fatal(err)
			}
			got := false
			for _, feature := range OrgShapeFeatures(root) {
				if feature == "Knowledge" {
					got = true
				}
			}
			if got != tc.want {
				t.Fatalf("knowledge enabled=%v, want %v", got, tc.want)
			}
		})
	}
}

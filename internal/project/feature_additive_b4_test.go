package project

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A local feature list declares the target field shape, including for Knowledge.
func TestConfigFeaturesDeclareStandardFieldShape(t *testing.T) {
	for _, feature := range []string{"Knowledge", "MultiCurrency", "PersonAccounts"} {
		t.Run(feature, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "glade.yml"), []byte("org:\n  features: ["+feature+"]\n"), 0600); err != nil {
				t.Fatal(err)
			}
			got := StandardFieldPolicyFeatures(Project{Root: root})
			if !slices.Contains(got, feature) || slices.Contains(got, UnspecifiedStandardFieldShape) {
				t.Fatalf("features=%v, want configured feature and declared shape", got)
			}
		})
	}
	t.Run("scratch_definition_remains_explicit", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "config")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "project-scratch-def.json"), []byte(`{"edition":"Developer","features":[]}`), 0600); err != nil {
			t.Fatal(err)
		}
		got := StandardFieldPolicyFeatures(Project{Root: root})
		if slices.Contains(got, UnspecifiedStandardFieldShape) {
			t.Fatalf("explicit scratch shape marked unspecified: %v", got)
		}
	})
}

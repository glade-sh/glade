package apextest

import (
	"os"
	"path/filepath"
	"testing"
)

// The active-language setting is org state, not a schema field. A warm
// runtime must not reuse an active template after that setting changes.
func TestSObjectJSON3ActiveLanguageRuntimeCache(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	root := t.TempDir()
	fixture := filepath.Join("testdata", "conformance", "sobject_json3", "project")
	if err := os.CopyFS(root, os.DirFS(fixture)); err != nil {
		t.Fatal(err)
	}
	active := loadTestIndex(t, root)
	activeKey, activeEntry := runtimeFromIndex(active, newSourceCache(), false)
	activeOrg := activeEntry.restored.CloneOrg()
	if !activeOrg.HasFeature("TranslationLanguage-fr") {
		t.Fatal("active language missing from initial runtime")
	}
	writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: []\n")
	inactive := loadTestIndex(t, root)
	inactiveKey, inactiveEntry := runtimeFromIndex(inactive, newSourceCache(), false)
	if activeKey == inactiveKey {
		t.Fatal("active and inactive language settings reused the same runtime key")
	}
	inactiveOrg := inactiveEntry.restored.CloneOrg()
	if inactiveOrg.HasFeature("TranslationLanguage-fr") {
		t.Fatal("warm runtime retained the inactive language")
	}
}

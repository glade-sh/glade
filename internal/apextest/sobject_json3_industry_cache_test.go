package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Changing the project-owned Industry value set must invalidate a warm runtime
// so the native binding in sobject_json3_collision/org.tsv:2 is not stale.
func TestSObjectJSON3IndustryValueSetRuntimeCache(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	root := t.TempDir()
	fixture := filepath.Join("testdata", "conformance", "sobject_json3_collision", "project")
	if err := os.CopyFS(root, os.DirFS(fixture)); err != nil {
		t.Fatal(err)
	}
	first := loadTestIndex(t, root)
	firstKey, firstEntry := runtimeFromIndex(first, newSourceCache(), false)
	firstOrg := firstEntry.restored.CloneOrg()
	path := filepath.Join(root, "standardValueSets", "Industry.standardValueSet-meta.xml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(body), "<label>Banking</label>", "<label>Changed Label</label>", 1)
	if changed == string(body) {
		t.Fatal("missing captured Banking label")
	}
	writeFile(t, path, changed)
	second := loadTestIndex(t, root)
	secondKey, secondEntry := runtimeFromIndex(second, newSourceCache(), false)
	secondOrg := secondEntry.restored.CloneOrg()
	if firstKey == secondKey {
		t.Fatal("changed Industry values reused the old runtime key")
	}
	firstValues := firstOrg.Objects["Account"].Definition.Fields["Industry"].PicklistValues
	secondValues := secondOrg.Objects["Account"].Definition.Fields["Industry"].PicklistValues
	if len(firstValues) == 0 || len(secondValues) == 0 || firstValues[0].Label != "Banking" || secondValues[0].Label != "Changed Label" {
		t.Fatalf("runtime snapshots: before=%#v after=%#v", firstValues, secondValues)
	}
}

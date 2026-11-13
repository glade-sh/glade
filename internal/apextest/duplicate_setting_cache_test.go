package apextest

import (
	"path/filepath"
	"testing"
)

func TestRuntimeCacheStandardDuplicateSettingTransitions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	var previous runtimeCacheKey
	for _, tc := range []struct {
		value string
		want  bool
	}{{"on", true}, {"off", false}, {"on", true}} {
		writeFile(t, filepath.Join(root, "glade.yml"), "duplicateRules:\n  standard: "+tc.value+"\n")
		index := loadTestIndex(t, root)
		key, entry := runtimeFromIndex(index, newSourceCache(), false)
		if entry.BaseErr != nil {
			t.Fatal(entry.BaseErr)
		}
		if previous != "" && previous == key {
			t.Errorf("%s: runtime key did not change", tc.value)
		}
		if got := entry.restored.CloneOrg().StandardDuplicateRules; got != tc.want {
			t.Errorf("%s: org setting = %v", tc.value, got)
		}
		previous = key
	}
}

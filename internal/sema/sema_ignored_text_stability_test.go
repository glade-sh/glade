package sema

import "testing"

func TestSemaIgnoredTextPartitionStableWhenFilesAreAdded(t *testing.T) {
	files := []ignoredTextFile{{"a.cls", 9}, {"b.cls", 8}, {"c.cls", 3}, {"d.cls", 1}, {"e.trigger", 2}}
	before := partitionIgnoredTextFiles(files, 3)
	after := partitionIgnoredTextFiles(append(files, ignoredTextFile{"new.cls", 100}), 3)
	owner := func(groups [][]ignoredTextFile) map[string]int {
		result := map[string]int{}
		for i, group := range groups {
			for _, file := range group {
				result[file.path] = i
			}
		}
		return result
	}
	old, next := owner(before), owner(after)
	for path, shard := range old {
		if next[path] != shard {
			t.Errorf("adding a file moved %s from shard %d to %d", path, shard, next[path])
		}
	}
}

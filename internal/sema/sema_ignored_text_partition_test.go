package sema

import (
	"reflect"
	"testing"
)

func TestSemaIgnoredTextPartitionKeepsEveryFile(t *testing.T) {
	files := []ignoredTextFile{{"d.cls", 1}, {"c.cls", 3}, {"b.cls", 8}, {"a.cls", 9}, {"e.trigger", 2}}
	groups := partitionIgnoredTextFiles(files, 3)
	seen := map[string]bool{}
	for _, group := range groups {
		if len(group) == 0 {
			t.Fatal("empty group")
		}
		for _, file := range group {
			if seen[file.path] {
				t.Fatalf("duplicate %s", file.path)
			}
			seen[file.path] = true
		}
	}
	if len(seen) != len(files) {
		t.Fatalf("lost files: %v", seen)
	}
	reversed := append([]ignoredTextFile(nil), files...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	if !reflect.DeepEqual(groups, partitionIgnoredTextFiles(reversed, 3)) {
		t.Fatal("partition depends on traversal order")
	}
}

package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFoldedSchemaRelationshipColdAndDiskProjectRoutes(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	root := t.TempDir()
	copyDiskRuntimeScopeProject(t, filepath.Join("testdata", "conformance", "folded_schema_names", "project"), root)
	index := loadTestIndex(t, root)
	for _, route := range []string{"cold", "disk_restore"} {
		t.Run(route, func(t *testing.T) {
			InvalidateRuntimeCaches()
			counters := newRunPerfCounters(true)
			_, entry, err := runtimeFromIndexWithSourceDigestsAndPerf(index, nil, newSourceCache(), true, counters)
			if err != nil {
				t.Fatal(err)
			}
			if got := counters.phases.diskCacheHits.Load(); (got == 1) != (route == "disk_restore") {
				t.Fatalf("route %s: disk hits=%d", route, got)
			}
			org := entry.restored.CloneOrg()
			found := false
			for _, relation := range org.Objects["Contact"].Definition.Relations {
				if !strings.EqualFold(relation.Field, "AccountId") {
					continue
				}
				found = true
				matcher, ok := any(relation).(interface {
					PreparedParentObjectMatch(string) (bool, bool)
				})
				if !ok {
					t.Fatal("restored relationship has no prepared lookup")
				}
				if match, prepared := matcher.PreparedParentObjectMatch("Account"); !match || !prepared {
					t.Errorf("Contact.AccountId: match=%v prepared=%v, want true/true", match, prepared)
				}
			}
			if !found {
				t.Fatal("missing Contact.AccountId relationship")
			}
		})
	}
}

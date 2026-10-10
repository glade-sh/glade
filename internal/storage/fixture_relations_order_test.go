package storage

import (
	"sort"
	"strings"
	"testing"
)

// Platform data and org shape objects are created from field maps. Their
// Relations order must not follow map iteration, or identical orgs differ.
func TestDeterministicPlatformDataRelationsOrderIsStable(t *testing.T) {
	build := func() map[string]string {
		org := NewOrgState()
		EnsureDeterministicPlatformData(&org)
		ApplyOrgShape(&org, []string{"Sites", "Communities", "ContactsToMultipleAccounts"})
		out := make(map[string]string, len(org.Objects))
		for name, object := range org.Objects {
			order := make([]string, 0, len(object.Definition.Relations))
			for _, relation := range object.Definition.Relations {
				order = append(order, relation.Field+"/"+relation.ParentRelationship)
			}
			out[name] = strings.Join(order, ",")
		}
		return out
	}
	want := build()
	names := make([]string, 0, len(want))
	for name := range want {
		names = append(names, name)
	}
	sort.Strings(names)
	for i := 1; i < 20; i++ {
		got := build()
		for _, name := range names {
			if got[name] != want[name] {
				t.Fatalf("build %d %s relations order:\n got %s\nwant %s", i, name, got[name], want[name])
			}
		}
	}
}

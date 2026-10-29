package storage

import "testing"

func TestOrgClonePreservesFeatureProfilePresence(t *testing.T) {
	clones := map[string]func(OrgState) OrgState{
		"clone":             OrgState.Clone,
		"runtime":           OrgState.CloneRuntime,
		"frozen":            OrgState.CloneRuntimeFrozenDefinition,
		"shared":            OrgState.CloneRuntimeFrozenShared,
		"rollback":          OrgState.CloneRollbackSnapshot,
		"template":          func(o OrgState) OrgState { return NewRuntimeTemplate(o).CloneRuntimeOrg() },
		"template snapshot": func(o OrgState) OrgState { return NewRuntimeTemplate(o).CloneSnapshotOrg() },
		"snapshot":          func(o OrgState) OrgState { return SnapshotRuntimeOrg(&o) },
	}
	for name, clone := range clones {
		for _, tc := range []struct {
			name     string
			features []string
		}{
			{"legacy", nil}, {"empty", []string{}}, {"populated", []string{"Chatter"}},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				org := NewOrgState()
				org.Features = tc.features
				got := clone(org)
				if (got.Features == nil) != (tc.features == nil) || len(got.Features) != len(tc.features) {
					t.Fatalf("features = %#v, want %#v", got.Features, tc.features)
				}
				if len(got.Features) > 0 {
					got.Features[0] = "changed"
					if org.Features[0] != "Chatter" {
						t.Fatal("clone shares features")
					}
				}
			})
		}
	}
}

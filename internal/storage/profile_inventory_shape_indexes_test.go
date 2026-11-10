package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

// Existing indexes must remain compatible when a captured shape refresh
// repairs seeded license links and removes an unreferenced bootstrap profile.
func TestProfileInventoryShapeRefreshIndexes(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	state := org.Objects["Profile"]
	state.Definition.Indexes = append(state.Definition.Indexes, IndexDefinition{Name: "Profile.UserLicenseId", Object: "Profile", Fields: []string{"UserLicenseId"}}, IndexDefinition{Name: "Profile.Name", Object: "Profile", Fields: []string{"Name"}})
	org.Objects["Profile"] = state
	RebuildObjectIndexes(&org, "Profile")
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
	state = org.Objects["Profile"]
	id := ID("00e000000000004")
	profile := state.Records[id]
	target := profile.Fields["UserLicenseId"].ID
	ids, ok := LookupIndex(state, "UserLicenseId", IDValue(target))
	if !ok {
		t.Fatal("license index unavailable")
	}
	found := false
	for _, candidate := range ids {
		if IDsEqual(candidate, id) {
			found = true
		}
	}
	if !found {
		t.Error("reconciled profile missing from license index")
	}
	ids, ok = LookupIndex(state, "Name", StringValue("Customer Community Guest User"))
	if !ok {
		t.Fatal("name index unavailable")
	}
	for _, candidate := range ids {
		if IDsEqual(candidate, ID("00e000000000007")) {
			t.Error("pruned profile remains in name index")
		}
	}
}

// A persisted org can already carry the captured feature profile before its
// deterministic seed is refreshed; existing indexes must be migrated too.
func TestProfileInventorySeedRefreshIndexes(t *testing.T) {
	org := NewOrgState()
	EnsureDeterministicPlatformData(&org)
	state := org.Objects["Profile"]
	state.Definition.Indexes = append(state.Definition.Indexes, IndexDefinition{Name: "Profile.UserLicenseId", Object: "Profile", Fields: []string{"UserLicenseId"}}, IndexDefinition{Name: "Profile.Name", Object: "Profile", Fields: []string{"Name"}})
	org.Objects["Profile"] = state
	RebuildObjectIndexes(&org, "Profile")
	org.setFeatureProfile([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
	EnsureDeterministicPlatformData(&org)
	state = org.Objects["Profile"]
	id := ID("00e000000000004")
	profile := state.Records[id]
	target := profile.Fields["UserLicenseId"].ID
	ids, ok := LookupIndex(state, "UserLicenseId", IDValue(target))
	if !ok {
		t.Fatal("license index unavailable")
	}
	found := false
	for _, candidate := range ids {
		if IDsEqual(candidate, id) {
			found = true
		}
	}
	if !found {
		t.Error("reconciled profile missing from license index")
	}
	ids, ok = LookupIndex(state, "Name", StringValue("Customer Community Guest User"))
	if !ok {
		t.Fatal("name index unavailable")
	}
	for _, candidate := range ids {
		if IDsEqual(candidate, ID("00e000000000007")) {
			t.Error("pruned profile remains in name index")
		}
	}
}

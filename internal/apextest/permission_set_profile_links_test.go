package apextest

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// The supplemental native queries link profile-owned sets to their profiles.
// Their org-specific names are translated to the local profile identity.
func TestPermissionSetInventoryProfileLinks(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			data, err := os.ReadFile("testdata/conformance/permission_set_inventory/profile-links/permissions-" + api + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var native struct {
				Result struct {
					Records []struct {
						Name, Label, ProfileId, LicenseId string
						IsOwnedByProfile                  bool
						Profile                           *struct{ Name string }
					}
				}
			}
			if err := json.Unmarshal(data, &native); err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api + ".0"
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "PermissionSet")
			result, err := soql.Execute(org, soql.Query{Object: "PermissionSet", Fields: []string{"Name", "Label", "ProfileId", "IsOwnedByProfile", "LicenseId"}})
			if err != nil {
				t.Fatal(err)
			}
			byName := map[string]storage.Record{}
			for _, set := range result.Records {
				byName[set.Fields["Name"].String] = set
			}
			used := map[storage.ID]bool{}
			count := 0
			for _, want := range native.Result.Records {
				if !want.IsOwnedByProfile {
					continue
				}
				count++
				if want.Profile == nil {
					t.Fatal("native profile relationship missing")
				}
				set, ok := byName[want.Name]
				if !ok && want.Name == "X"+want.ProfileId && want.Label == want.ProfileId {
					for _, candidate := range result.Records {
						profile := org.Objects["Profile"].Records[candidate.Fields["ProfileId"].ID]
						if profile.Fields["Name"].String == want.Profile.Name && candidate.Fields["Name"].String == "X"+storage.ID18(string(profile.ID)) {
							set, ok = candidate, true
							break
						}
					}
				}
				if !ok {
					t.Errorf("missing native profile set for %s", want.Profile.Name)
					continue
				}
				profile := org.Objects["Profile"].Records[set.Fields["ProfileId"].ID]
				if profile.Fields["Name"].String != want.Profile.Name {
					t.Errorf("%s linked to profile %q", want.Name, profile.Fields["Name"].String)
				}
				if set.Fields["LicenseId"].ID == "" {
					t.Errorf("%s has no license", want.Name)
				}
				if used[set.ID] {
					t.Errorf("profile set reused: %s", set.ID)
				}
				used[set.ID] = true
			}
			if count != 38 || len(used) != 38 {
				t.Errorf("profile-owned sets: native %d, matched %d", count, len(used))
			}
			assignments, err := soql.Execute(org, soql.Query{Object: "PermissionSetAssignment", Fields: []string{"AssigneeId", "PermissionSetId"}})
			if err != nil {
				t.Fatal(err)
			}
			if len(assignments.Records) != 1 {
				t.Fatalf("admin assignments=%d; native 1", len(assignments.Records))
			}
			assignment := assignments.Records[0]
			set := org.Objects["PermissionSet"].Records[assignment.Fields["PermissionSetId"].ID]
			if assignment.Fields["AssigneeId"].ID != storage.ID(storage.DefaultUserID) || set.Fields["ProfileId"].ID != storage.ID(storage.DefaultProfileID) {
				t.Fatal("native admin assignment not linked to administrator profile set")
			}
		})
	}
}

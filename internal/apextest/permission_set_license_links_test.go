package apextest

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPermissionSetInventoryNativeLicenseLinks(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance/permission_set_inventory/profile-links/permissions-67.json")
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
	storage.EnsureDeterministicPlatformData(&org)
	sets := map[string]storage.Record{}
	for _, r := range org.Objects["PermissionSet"].Records {
		sets[r.Fields["Name"].String] = r
	}
	forward := map[string]storage.ID{}
	reverse := map[storage.ID]string{}
	matched := 0
	for _, want := range native.Result.Records {
		set, ok := sets[want.Name]
		if !ok && want.IsOwnedByProfile && want.Profile != nil && want.Name == "X"+want.ProfileId && want.Label == want.ProfileId {
			for _, candidate := range org.Objects["PermissionSet"].Records {
				profile := org.Objects["Profile"].Records[candidate.Fields["ProfileId"].ID]
				if profile.Fields["Name"].String == want.Profile.Name && candidate.Fields["Name"].String == "X"+storage.ID18(string(profile.ID)) {
					set, ok = candidate, true
					break
				}
			}
		}
		if !ok {
			t.Errorf("missing native set %s", want.Name)
			continue
		}
		matched++
		id := set.Fields["LicenseId"].ID
		if len(want.LicenseId) < 3 || len(id) < 3 || string(id[:3]) != want.LicenseId[:3] {
			t.Errorf("%s license type differs", want.Name)
			continue
		}
		object := "UserLicense"
		if want.LicenseId[:3] == "0PL" {
			object = "PermissionSetLicense"
		}
		if _, _, ok := storage.LookupRecordByID(org.Objects[object].Records, id); !ok {
			t.Errorf("%s license target is missing", want.Name)
		}
		if previous, ok := forward[want.LicenseId]; ok && previous != id {
			t.Errorf("%s native shared license differs locally", want.Name)
		}
		if previous, ok := reverse[id]; ok && previous != want.LicenseId {
			t.Errorf("%s distinct native licenses were merged", want.Name)
		}
		forward[want.LicenseId], reverse[id] = id, want.LicenseId
	}
	if matched != 88 {
		t.Errorf("matched license links=%d, want 88", matched)
	}
}

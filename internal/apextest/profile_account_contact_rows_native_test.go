package apextest

import (
	"encoding/json"
	"os"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// The native capture lists the Account and Contact ObjectPermissions row of
// every profile-owned permission set (56 rows, identical at API 62.0 and 67.0).
func TestProfileAccountContactRowsNative(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			data, err := os.ReadFile("testdata/conformance/profile_account_contact_rows/objperm-" + api + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var native struct {
				Result struct {
					Records []struct {
						Object    string `json:"SobjectType"`
						Read      bool   `json:"PermissionsRead"`
						Create    bool   `json:"PermissionsCreate"`
						Edit      bool   `json:"PermissionsEdit"`
						Delete    bool   `json:"PermissionsDelete"`
						ViewAll   bool   `json:"PermissionsViewAllRecords"`
						ModifyAll bool   `json:"PermissionsModifyAllRecords"`
						Parent    struct {
							Profile struct{ Name string }
						}
					}
				}
			}
			if err := json.Unmarshal(data, &native); err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api + ".0"
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "ObjectPermissions")
			result, err := soql.Execute(org, soql.Query{Object: "ObjectPermissions", Fields: []string{"ParentId", "SobjectType", "PermissionsRead", "PermissionsCreate", "PermissionsEdit", "PermissionsDelete", "PermissionsViewAllRecords", "PermissionsModifyAllRecords"}})
			if err != nil {
				t.Fatal(err)
			}
			profileOfSet := map[storage.ID]string{}
			for id, set := range org.Objects["PermissionSet"].Records {
				if set.Fields["IsOwnedByProfile"].Boolean {
					profileOfSet[id] = org.Objects["Profile"].Records[set.Fields["ProfileId"].ID].Fields["Name"].String
				}
			}
			type flags [6]bool
			got := map[string]flags{}
			for _, row := range result.Records {
				name, ok := profileOfSet[row.Fields["ParentId"].ID]
				if !ok {
					continue
				}
				object := row.Fields["SobjectType"].String
				if object != "Account" && object != "Contact" {
					continue
				}
				got[name+"/"+object] = flags{row.Fields["PermissionsRead"].Boolean, row.Fields["PermissionsCreate"].Boolean, row.Fields["PermissionsEdit"].Boolean, row.Fields["PermissionsDelete"].Boolean, row.Fields["PermissionsViewAllRecords"].Boolean, row.Fields["PermissionsModifyAllRecords"].Boolean}
			}
			want := map[string]flags{}
			for _, r := range native.Result.Records {
				want[r.Parent.Profile.Name+"/"+r.Object] = flags{r.Read, r.Create, r.Edit, r.Delete, r.ViewAll, r.ModifyAll}
			}
			var keys []string
			for key := range want {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if got[key] != want[key] {
					t.Errorf("%s = %v, want %v", key, got[key], want[key])
				}
			}
			for key := range got {
				if _, ok := want[key]; !ok {
					t.Errorf("unexpected row %s", key)
				}
			}
			if len(want) != 56 {
				t.Errorf("native rows = %d, want 56", len(want))
			}
		})
	}
}

package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func TestProfilePermissionRowsNativeObjects(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		for _, route := range []string{"anonymous", "test"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				file := fmt.Sprintf("s0-%s-%s-objects", api, route)
				if api == "67" {
					file += "-nullguard"
				}
				data, err := os.ReadFile(filepath.Join("testdata/conformance/profile_permission_rows/native-payload", file+".json"))
				if err != nil {
					t.Fatal(err)
				}
				var rows []struct {
					Kind string `json:"kind"`
					Row  struct {
						Object    string `json:"SobjectType"`
						Read      bool   `json:"PermissionsRead"`
						Create    bool   `json:"PermissionsCreate"`
						Edit      bool   `json:"PermissionsEdit"`
						Delete    bool   `json:"PermissionsDelete"`
						ViewAll   bool   `json:"PermissionsViewAllRecords"`
						ModifyAll bool   `json:"PermissionsModifyAllRecords"`
						Parent    struct {
							Profile struct {
								Name string `json:"Name"`
							} `json:"Profile"`
						} `json:"Parent"`
					} `json:"row"`
				}
				if err := json.Unmarshal(data, &rows); err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				org.APIVersion = api + ".0"
				storage.EnsureDeterministicPlatformData(&org)
				profiles := map[storage.ID]string{}
				for _, r := range org.Objects["Profile"].Records {
					profiles[r.ID] = r.Fields["Name"].String
				}
				parents := map[storage.ID]string{}
				for _, r := range org.Objects["PermissionSet"].Records {
					if r.Fields["IsOwnedByProfile"].Boolean {
						parents[r.ID] = profiles[r.Fields["ProfileId"].ID]
					}
				}
				fields := []string{"ParentId", "SobjectType", "PermissionsRead", "PermissionsCreate", "PermissionsEdit", "PermissionsDelete", "PermissionsViewAllRecords", "PermissionsModifyAllRecords"}
				result, err := soql.Execute(org, soql.Query{Object: "ObjectPermissions", Fields: fields})
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]storage.Record{}
				for _, r := range result.Records {
					profile := parents[r.Fields["ParentId"].ID]
					object := r.Fields["SobjectType"].String
					if (profile == "System Administrator" || profile == "Standard User") && (object == "Account" || object == "Contact") {
						got[profile+"/"+object] = r
					}
				}
				count := 0
				for _, item := range rows {
					if item.Kind != "permissionRow" {
						continue
					}
					row := item.Row
					profile := row.Parent.Profile.Name
					if (profile != "System Administrator" && profile != "Standard User") || (row.Object != "Account" && row.Object != "Contact") {
						continue
					}
					count++
					key := profile + "/" + row.Object
					record, ok := got[key]
					if !ok {
						t.Errorf("missing %s", key)
						continue
					}
					for field, want := range map[string]bool{"PermissionsRead": row.Read, "PermissionsCreate": row.Create, "PermissionsEdit": row.Edit, "PermissionsDelete": row.Delete, "PermissionsViewAllRecords": row.ViewAll, "PermissionsModifyAllRecords": row.ModifyAll} {
						if record.Fields[field].Boolean != want {
							t.Errorf("%s %s = %t, native %t", key, field, record.Fields[field].Boolean, want)
						}
					}
				}
				if count != 4 || len(got) != count {
					t.Errorf("selected objects: native %d, Glade %d", count, len(got))
				}
			})
		}
	}
}

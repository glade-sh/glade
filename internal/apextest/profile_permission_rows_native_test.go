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

type capturedFieldPermissionRow struct {
	Kind string `json:"kind"`
	Row  struct {
		Object string `json:"SobjectType"`
		Field  string `json:"Field"`
		Read   bool   `json:"PermissionsRead"`
		Edit   bool   `json:"PermissionsEdit"`
		Parent struct {
			Profile struct {
				Name string `json:"Name"`
			} `json:"Profile"`
		} `json:"Parent"`
	} `json:"row"`
}

// Captured selected rows cover both built-in profiles, both routes and APIs.
func TestProfilePermissionRowsNativeFields(t *testing.T) {
	const dir = "testdata/conformance/profile_permission_rows/native-payload"
	for _, api := range []string{"62", "67"} {
		for _, route := range []string{"anonymous", "test"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("s0-%s-%s-fields.json", api, route)))
				if err != nil {
					t.Fatal(err)
				}
				var captured []capturedFieldPermissionRow
				if err := json.Unmarshal(data, &captured); err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				org.APIVersion = api + ".0"
				storage.EnsureDeterministicPlatformData(&org)
				profiles := map[storage.ID]string{}
				for _, record := range org.Objects["Profile"].Records {
					profiles[record.ID] = record.Fields["Name"].String
				}
				parents := map[storage.ID]string{}
				for _, record := range org.Objects["PermissionSet"].Records {
					if record.Fields["IsOwnedByProfile"].Boolean {
						parents[record.ID] = profiles[record.Fields["ProfileId"].ID]
					}
				}
				result, err := soql.Execute(org, soql.Query{Object: "FieldPermissions", Fields: []string{"ParentId", "SobjectType", "Field", "PermissionsRead", "PermissionsEdit"}})
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]storage.Record{}
				for _, record := range result.Records {
					profile := parents[record.Fields["ParentId"].ID]
					if profile == "System Administrator" || profile == "Standard User" {
						got[profile+"/"+record.Fields["Field"].String] = record
					}
				}
				want := 0
				for _, item := range captured {
					if item.Kind != "permissionRow" {
						continue
					}
					row := item.Row
					profile := row.Parent.Profile.Name
					if profile != "System Administrator" && profile != "Standard User" {
						continue
					}
					want++
					key := profile + "/" + row.Field
					record, ok := got[key]
					if !ok {
						t.Errorf("missing %s", key)
						continue
					}
					if record.Fields["SobjectType"].String != row.Object || record.Fields["PermissionsRead"].Boolean != row.Read || record.Fields["PermissionsEdit"].Boolean != row.Edit {
						t.Errorf("%s has mismatched flags or object", key)
					}
				}
				if want != 94 || len(got) != want {
					t.Errorf("selected fields: native=%d Glade=%d", want, len(got))
				}
			})
		}
	}
}

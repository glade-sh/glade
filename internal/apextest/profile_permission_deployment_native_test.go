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

// Replay the three captured metadata states through project org construction.
func TestProfilePermissionRowsNativeDeployment(t *testing.T) {
	const fixture = "testdata/conformance/profile_permission_rows"
	read := func(t *testing.T, p string) string {
		t.Helper()
		b, e := os.ReadFile(filepath.Join(fixture, "source", p))
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	for _, api := range []string{"62", "67"} {
		for _, stage := range []string{"s0", "s1", "s2"} {
			for _, route := range []string{"anonymous", "test"} {
				t.Run(api+"/"+stage+"/"+route, func(t *testing.T) {
					root := t.TempDir()
					base := filepath.Join(root, "force-app/main/default")
					writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api+".0"))
					for _, name := range []string{"SynthFlsParent__c", "SynthFlsChild__c"} {
						writeFile(t, filepath.Join(base, "objects", name, name+".object-meta.xml"), read(t, "stages/S0/objects/"+name+".object"))
					}
					if stage != "s0" {
						writeFile(t, filepath.Join(base, "profiles/Admin.profile-meta.xml"), read(t, "stages/S1/profiles/Admin.profile"))
					}
					if stage == "s2" {
						writeFile(t, filepath.Join(base, "permissionsets/SynthFlsUnassigned.permissionset-meta.xml"), read(t, "stages/S2/permissionsets/SynthFlsUnassigned.permissionset"))
					}
					index := loadTestIndex(t, root)
					org := OrgFromIndex(index)
					fieldData, err := os.ReadFile(filepath.Join(fixture, "native-payload", fmt.Sprintf("%s-%s-%s-fields.json", stage, api, route)))
					if err != nil {
						t.Fatal(err)
					}
					var native []struct {
						Kind string `json:"kind"`
						Row  struct {
							Object string `json:"SobjectType"`
							Field  string `json:"Field"`
							Read   bool   `json:"PermissionsRead"`
							Edit   bool   `json:"PermissionsEdit"`
							Parent struct {
								Name    string `json:"Name"`
								Profile *struct {
									Name string `json:"Name"`
								} `json:"Profile"`
							} `json:"Parent"`
						} `json:"row"`
					}
					if err := json.Unmarshal(fieldData, &native); err != nil {
						t.Fatal(err)
					}
					wanted := map[string][2]bool{}
					for _, v := range native {
						if v.Kind != "permissionRow" || v.Row.Object != "SynthFlsParent__c" {
							continue
						}
						name := v.Row.Parent.Name
						if v.Row.Parent.Profile != nil {
							name = v.Row.Parent.Profile.Name
						}
						wanted[name+"/"+v.Row.Field] = [2]bool{v.Row.Read, v.Row.Edit}
					}
					parents := map[storage.ID]string{}
					for _, set := range org.Objects["PermissionSet"].Records {
						name := set.Fields["Name"].String
						if set.Fields["IsOwnedByProfile"].Boolean {
							name = org.Objects["Profile"].Records[set.Fields["ProfileId"].ID].Fields["Name"].String
						}
						parents[set.ID] = name
					}
					result, err := soql.Execute(org, soql.Query{Object: "FieldPermissions", Fields: []string{"ParentId", "SobjectType", "Field", "PermissionsRead", "PermissionsEdit"}})
					if err != nil {
						t.Fatal(err)
					}
					got := map[string][2]bool{}
					for _, r := range result.Records {
						if r.Fields["SobjectType"].String != "SynthFlsParent__c" {
							continue
						}
						name := parents[r.Fields["ParentId"].ID]
						if name != "System Administrator" && name != "Standard User" && name != "SynthFlsUnassigned" {
							continue
						}
						got[name+"/"+r.Fields["Field"].String] = [2]bool{r.Fields["PermissionsRead"].Boolean, r.Fields["PermissionsEdit"].Boolean}
					}
					if len(got) != len(wanted) {
						t.Errorf("custom field rows native=%v Glade=%v", wanted, got)
					}
					for k, v := range wanted {
						if x, ok := got[k]; !ok || x != v {
							t.Errorf("%s native=%v Glade=%v present=%t", k, v, x, ok)
						}
					}
					for _, a := range org.Objects["PermissionSetAssignment"].Records {
						if parents[a.Fields["PermissionSetId"].ID] == "SynthFlsUnassigned" {
							t.Error("native unassigned set acquired an assignment")
						}
					}
					objectFile := fmt.Sprintf("%s-%s-%s-objects", stage, api, route)
					if stage == "s0" && api == "67" {
						objectFile += "-nullguard"
					}
					objectData, err := os.ReadFile(filepath.Join(fixture, "native-payload", objectFile+".json"))
					if err != nil {
						t.Fatal(err)
					}
					var objects []struct {
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
								Name    string `json:"Name"`
								Profile *struct {
									Name string `json:"Name"`
								} `json:"Profile"`
							} `json:"Parent"`
						} `json:"row"`
					}
					if err := json.Unmarshal(objectData, &objects); err != nil {
						t.Fatal(err)
					}
					wantObjects := map[string][6]bool{}
					for _, v := range objects {
						if v.Kind != "permissionRow" || (v.Row.Object != "SynthFlsParent__c" && v.Row.Object != "SynthFlsChild__c") {
							continue
						}
						name := v.Row.Parent.Name
						if v.Row.Parent.Profile != nil {
							name = v.Row.Parent.Profile.Name
						}
						if name != "System Administrator" && name != "Standard User" && name != "SynthFlsUnassigned" {
							continue
						}
						wantObjects[name+"/"+v.Row.Object] = [6]bool{v.Row.Read, v.Row.Create, v.Row.Edit, v.Row.Delete, v.Row.ViewAll, v.Row.ModifyAll}
					}
					objectResult, err := soql.Execute(org, soql.Query{Object: "ObjectPermissions", Fields: []string{"ParentId", "SobjectType", "PermissionsRead", "PermissionsCreate", "PermissionsEdit", "PermissionsDelete", "PermissionsViewAllRecords", "PermissionsModifyAllRecords"}})
					if err != nil {
						t.Fatal(err)
					}
					gotObjects := map[string][6]bool{}
					for _, v := range objectResult.Records {
						object := v.Fields["SobjectType"].String
						if object != "SynthFlsParent__c" && object != "SynthFlsChild__c" {
							continue
						}
						name := parents[v.Fields["ParentId"].ID]
						if name != "System Administrator" && name != "Standard User" && name != "SynthFlsUnassigned" {
							continue
						}
						gotObjects[name+"/"+object] = [6]bool{v.Fields["PermissionsRead"].Boolean, v.Fields["PermissionsCreate"].Boolean, v.Fields["PermissionsEdit"].Boolean, v.Fields["PermissionsDelete"].Boolean, v.Fields["PermissionsViewAllRecords"].Boolean, v.Fields["PermissionsModifyAllRecords"].Boolean}
					}
					if len(gotObjects) != len(wantObjects) {
						t.Errorf("custom object rows native=%v Glade=%v", wantObjects, gotObjects)
					}
					for k, v := range wantObjects {
						if got, ok := gotObjects[k]; !ok || got != v {
							t.Errorf("%s native=%v Glade=%v present=%t", k, v, got, ok)
						}
					}
				})
			}
		}
	}
}

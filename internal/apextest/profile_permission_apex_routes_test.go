package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Replay the captured profile rows through Apex anonymous and project test
// execution, including parent relationship projection.
func TestProfilePermissionSOQLNativeRoutes(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			data, err := os.ReadFile("testdata/conformance/profile_permission_rows/native-payload/s0-" + api + "-anonymous-fields.json")
			if err != nil {
				t.Fatal(err)
			}
			var rows []capturedFieldPermissionRow
			if err := json.Unmarshal(data, &rows); err != nil {
				t.Fatal(err)
			}
			var body strings.Builder
			body.WriteString(`Map<String,String> found=new Map<String,String>();
for(FieldPermissions p:[SELECT Field,PermissionsRead,PermissionsEdit,Parent.Profile.Name FROM FieldPermissions WHERE Parent.IsOwnedByProfile=true AND SobjectType IN ('Account','Contact')]) {
 String profile=p.Parent.Profile.Name;
 if(profile=='System Administrator'||profile=='Standard User') found.put(profile+'/'+p.Field,String.valueOf(p.PermissionsRead)+'/'+String.valueOf(p.PermissionsEdit));
}
System.assertEquals(94,found.size());`)
			count := 0
			for _, r := range rows {
				if r.Kind != "permissionRow" {
					continue
				}
				profile := r.Row.Parent.Profile.Name
				if profile != "System Administrator" && profile != "Standard User" {
					continue
				}
				count++
				fmt.Fprintf(&body, "System.assertEquals('%t/%t',found.get('%s/%s'));", r.Row.Read, r.Row.Edit, profile, r.Row.Field)
			}
			if count != 94 {
				t.Fatalf("captured fields=%d", count)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api+".0"))
			classDir := filepath.Join(root, "force-app/main/default/classes")
			writeFile(t, filepath.Join(classDir, "ProfilePermissionRowsTest.cls"), "@IsTest private class ProfilePermissionRowsTest { @IsTest static void queryRows(){"+body.String()+"} }")
			writeFile(t, filepath.Join(classDir, "ProfilePermissionRowsTest.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+".0</apiVersion><status>Active</status></ApexClass>")
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api + ".0"
			program, err := vm.CompileAnonymousWithOptions(body.String(), vm.CompileOptions{APIVersion: api + ".0"})
			if err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.execute(program); err != nil {
				t.Errorf("anonymous: %v", err)
			}
			for _, phase := range []string{"cold", "warm"} {
				run := Run(index, Options{SelectedClasses: []string{"ProfilePermissionRowsTest"}, Parallelism: 1})
				if sum := run.Summary(); sum.Total != 1 || sum.Passed != 1 {
					t.Errorf("%s test route: %+v suites=%+v", phase, sum, run.Suites)
				}
			}
		})
	}
}

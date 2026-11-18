package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The two controls are captured for both objects and APIs in
// native-capture/user_manage_users/permission_values/org.tsv.
func TestLightningManagePermissionsHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, host := range []string{"59.0", "67.0"} {
			t.Run(api+"/host="+host, func(t *testing.T) {
				root := t.TempDir()
				classes := filepath.Join(root, "force-app/main/default/classes")
				writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				writeLightningFixtureFile(t, filepath.Join(classes, "PermissionHost.cls"), `public class PermissionHost {
      @AuraEnabled public static String observe() {
       Profile admin = [SELECT Id, PermissionsManageUsers, PermissionsManageInternalUsers FROM Profile WHERE Name = 'System Administrator' LIMIT 1];
       Profile standard = [SELECT Id, PermissionsManageUsers, PermissionsManageInternalUsers FROM Profile WHERE Name = 'Standard User' LIMIT 1];
       PermissionSet a = [SELECT PermissionsManageUsers, PermissionsManageInternalUsers FROM PermissionSet WHERE ProfileId = :admin.Id LIMIT 1];
       PermissionSet s = [SELECT PermissionsManageUsers, PermissionsManageInternalUsers FROM PermissionSet WHERE ProfileId = :standard.Id LIMIT 1];
       return '' + admin.PermissionsManageUsers + ',' + admin.PermissionsManageInternalUsers + ',' + a.PermissionsManageUsers + ',' + a.PermissionsManageInternalUsers + ',' + standard.PermissionsManageUsers + ',' + standard.PermissionsManageInternalUsers + ',' + s.PermissionsManageUsers + ',' + s.PermissionsManageInternalUsers;
      }
    }`)
				writeLightningFixtureFile(t, filepath.Join(classes, "PermissionHost.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				bundle := filepath.Join(root, "force-app/main/default/lwc/permissionHost")
				writeLightningFixtureFile(t, filepath.Join(bundle, "permissionHost.js"), `import observe from '@salesforce/apex/PermissionHost.observe'; export default observe;`)
				writeLightningFixtureFile(t, filepath.Join(bundle, "permissionHost.js-meta.xml"), "<LightningComponentBundle><apiVersion>"+host+"</apiVersion><isExposed>true</isExposed></LightningComponentBundle>")
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				schema, err := gladeschema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				metadata, err := NewSourceMetadataFromProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, schema)
				org := apextest.OrgFromIndex(index)
				handler := NewWithStoreAndSource(&org, &memoryStore{}, metadata)
				handler.SetProjectIndex(index)
				for pass := 0; pass < 2; pass++ {
					request := httptest.NewRequest(http.MethodPost, "/lightning/apex/PermissionHost/observe", strings.NewReader(`{}`))
					request.Header.Set("Content-Type", "application/json")
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, request)
					var result lwcbrowser.WireResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
						t.Fatalf("HTTP %d: %s: %v", rec.Code, rec.Body.String(), err)
					}
					if rec.Code != http.StatusOK || result.Error != nil || result.Data != "true,true,true,true,false,false,false,false" {
						t.Fatalf("pass %d HTTP %d: %s", pass, rec.Code, rec.Body.String())
					}
				}
			})
		}
	}
}

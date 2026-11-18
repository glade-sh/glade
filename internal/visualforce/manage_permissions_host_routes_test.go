package visualforce_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

// Expectations are captured in native-capture/user_manage_users/permission_values/org.tsv.
func TestVisualforceManagePermissionsHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, host := range []string{"59.0", "67.0"} {
			t.Run(api+"/host="+host, func(t *testing.T) {
				root := t.TempDir()
				classes := filepath.Join(root, "force-app/main/default/classes")
				pages := filepath.Join(root, "force-app/main/default/pages")
				writeDescribeHostFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				writeDescribeHostFile(t, filepath.Join(classes, "PermissionHost.cls"), `public class PermissionHost {
      public String getValues() {
       Profile admin = [SELECT Id, PermissionsManageUsers, PermissionsManageInternalUsers FROM Profile WHERE Name = 'System Administrator' LIMIT 1];
       Profile standard = [SELECT Id, PermissionsManageUsers, PermissionsManageInternalUsers FROM Profile WHERE Name = 'Standard User' LIMIT 1];
       PermissionSet a = [SELECT PermissionsManageUsers, PermissionsManageInternalUsers FROM PermissionSet WHERE ProfileId = :admin.Id LIMIT 1];
       PermissionSet s = [SELECT PermissionsManageUsers, PermissionsManageInternalUsers FROM PermissionSet WHERE ProfileId = :standard.Id LIMIT 1];
       return '' + admin.PermissionsManageUsers + ',' + admin.PermissionsManageInternalUsers + ',' + a.PermissionsManageUsers + ',' + a.PermissionsManageInternalUsers + ',' + standard.PermissionsManageUsers + ',' + standard.PermissionsManageInternalUsers + ',' + s.PermissionsManageUsers + ',' + s.PermissionsManageInternalUsers;
      }
    }`)
				writeDescribeHostFile(t, filepath.Join(classes, "PermissionHost.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				writeDescribeHostFile(t, filepath.Join(pages, "PermissionHost.page"), `<apex:page controller="PermissionHost">{!values}</apex:page>`)
				writeDescribeHostFile(t, filepath.Join(pages, "PermissionHost.page-meta.xml"), "<ApexPage><apiVersion>"+host+"</apiVersion></ApexPage>")
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				schema, err := gladeschema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				vf, err := visualforce.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, schema)
				org := apextest.OrgFromIndex(index)
				machine := vm.New(nil)
				machine.SetOrg(&org)
				if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
					t.Fatal(err)
				}
				result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, Machine: machine, PageName: "PermissionHost"})
				if err != nil || result.Error != nil || !strings.Contains(result.HTML, "true,true,true,true,false,false,false,false") {
					t.Fatalf("render: %v / %+v", err, result)
				}
			})
		}
	}
}

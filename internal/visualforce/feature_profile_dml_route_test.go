package visualforce_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

func TestVisualforceControllerDMLFeatureProfileRollback(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, host := range []string{"59.0", "67.0"} {
			for _, features := range [][]string{{}, {"Chatter"}} {
				t.Run(fmt.Sprintf("apex=%s/host=%s/features=%v", api, host, features), func(t *testing.T) {
					root := t.TempDir()
					writeDescribeHostFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
					classes := filepath.Join(root, "force-app/main/default/classes")
					pages := filepath.Join(root, "force-app/main/default/pages")
					writeDescribeHostFile(t, filepath.Join(classes, "ProfileController.cls"), `public class ProfileController { public String getStatus() { Database.NestedSaveResult result = Database.treeSave(new Account()); return result.isSuccess() ? 'unexpected-success' : 'rolled-back'; } }`)
					writeDescribeHostFile(t, filepath.Join(classes, "ProfileController.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
					writeDescribeHostFile(t, filepath.Join(pages, "Profile.page"), `<apex:page controller="ProfileController">{!status}</apex:page>`)
					writeDescribeHostFile(t, filepath.Join(pages, "Profile.page-meta.xml"), "<ApexPage><apiVersion>"+host+"</apiVersion></ApexPage>")
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
					org := storage.NewOrgState()
					storage.EnsureStandardObject(&org, "Account")
					org.Features = append([]string{}, features...)
					machine := vm.New(nil)
					machine.SetOrg(&org)
					if err := apextest.RegisterProjectRuntimeForRequest(machine, typesys.Build(p, schema)); err != nil {
						t.Fatal(err)
					}
					result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, Machine: machine, PageName: "Profile"})
					if err != nil || result.Error != nil || !strings.Contains(result.HTML, "rolled-back") {
						t.Fatalf("render: %v / %+v", err, result)
					}
					if org.Features == nil || len(org.Features) != len(features) {
						t.Fatalf("profile after failed DML: %#v, want %#v", org.Features, features)
					}
					if len(features) > 0 && org.Features[0] != features[0] {
						t.Fatalf("profile changed: %#v", org.Features)
					}
				})
			}
		}
	}
}

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

// The controller's named Apex observation is in P0-expanded/org.tsv.
func TestVisualforceAPI68DescribeControllerRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, host := range []string{"59.0", "67.0"} {
			t.Run(api+"/"+host, func(t *testing.T) {
				root := t.TempDir()
				classes := filepath.Join(root, "force-app/main/default/classes")
				pages := filepath.Join(root, "force-app/main/default/pages")
				writeDescribeHostFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				writeDescribeHostFile(t, filepath.Join(classes, "V035DescribeController.cls"), `public class V035DescribeController { public String getStatus() { return String.valueOf(Schema.getGlobalDescribe().get('AssetStatePeriod').getDescribe().fields.getMap().containsKey('RampUpliftType')); } }`)
				writeDescribeHostFile(t, filepath.Join(classes, "V035DescribeController.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				writeDescribeHostFile(t, filepath.Join(pages, "V035Describe.page"), `<apex:page controller="V035DescribeController">{!status}</apex:page>`)
				writeDescribeHostFile(t, filepath.Join(pages, "V035Describe.page-meta.xml"), "<ApexPage><apiVersion>"+host+"</apiVersion></ApexPage>")
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
				org := storage.NewOrgState()
				storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper})
				storage.EnsureStandardObject(&org, "AssetStatePeriod")
				machine := vm.New(nil)
				machine.SetOrg(&org)
				if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
					t.Fatal(err)
				}
				result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, Machine: machine, PageName: "V035Describe"})
				want := "false"
				if api == "68.0" {
					want = "true"
				}
				if err != nil || result.Error != nil || !strings.Contains(result.HTML, want) {
					t.Fatalf("render want %s: %v / %+v", want, err, result)
				}
			})
		}
	}
}

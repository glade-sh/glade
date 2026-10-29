package visualforce_test

import (
	"fmt"
	"os"
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

// Render an actual custom-controller getter through each page API, with
// separately versioned Apex at the supported endpoints. Fresh request VMs
// share preparation, while writable map holders and tokens remain private.
func TestVisualforceDescribeHolderRequestIsolation(t *testing.T) {
	source, err := os.ReadFile("../vm/testdata/describe_ownership/DescribeOwner.cls")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, host := range []string{"59.0", "62.0", "67.0"} {
			for _, namespace := range []string{"", "zzns"} {
				t.Run(api+"/host="+host+"/"+namespace, func(t *testing.T) {
					root := t.TempDir()
					classes := filepath.Join(root, "force-app/main/default/classes")
					pages := filepath.Join(root, "force-app/main/default/pages")
					writeDescribeHostFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q,"namespace":%q}`, api, namespace))
					writeDescribeHostFile(t, filepath.Join(classes, "DescribeOwner.cls"), string(source))
					writeDescribeHostFile(t, filepath.Join(classes, "DescribeOwner.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
					writeDescribeHostFile(t, filepath.Join(pages, "DescribeOwner.page"), `<apex:page controller="DescribeOwner">{!status}</apex:page>`)
					writeDescribeHostFile(t, filepath.Join(pages, "DescribeOwner.page-meta.xml"), "<ApexPage><apiVersion>"+host+"</apiVersion></ApexPage>")
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
					for range 3 {
						org := storage.NewOrgState()
						org.Namespace = namespace
						for _, name := range []string{"Account", "User", "RecordType"} {
							storage.EnsureStandardObject(&org, name)
						}
						machine := vm.New(nil)
						machine.SetOrg(&org)
						if err := apextest.RegisterProjectRuntimeForRequest(machine, index); err != nil {
							t.Fatal(err)
						}
						result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, Machine: machine, PageName: "DescribeOwner"})
						if err != nil || result.Error != nil || !strings.Contains(result.HTML, "owned-describe-ok") {
							t.Fatalf("render: %v / %+v", err, result)
						}
					}
				})
			}
		}
	}
}

func writeDescribeHostFile(t *testing.T, path, source string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
}

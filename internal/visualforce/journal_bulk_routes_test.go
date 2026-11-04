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

func TestVisualforceJournalBulkRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, host := range []string{"59.0", "67.0"} {
			t.Run(fmt.Sprintf("apex=%s/vf=%s", api, host), func(t *testing.T) {
				root := t.TempDir()
				writeDescribeHostFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				classes := filepath.Join(root, "force-app/main/default/classes")
				pages := filepath.Join(root, "force-app/main/default/pages")
				writeDescribeHostFile(t, filepath.Join(classes, "JournalBulkController.cls"), `public class JournalBulkController { public String getResult() { List<Account> rows = new List<Account>(); for (Integer i = 0; i < 40; i++) rows.add(new Account(Name = 'Before')); insert rows; for (Account row : rows) row.Name = 'After'; update rows; return String.valueOf(rows.size()); } }`)
				writeDescribeHostFile(t, filepath.Join(classes, "JournalBulkController.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				writeDescribeHostFile(t, filepath.Join(pages, "Bulk.page"), `<apex:page controller="JournalBulkController">{!result}</apex:page>`)
				writeDescribeHostFile(t, filepath.Join(pages, "Bulk.page-meta.xml"), "<ApexPage><apiVersion>"+host+"</apiVersion></ApexPage>")
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
				machine := vm.New(nil)
				machine.SetOrg(&org)
				machine.SetIsolationJournal(storage.NewIsolationJournal(&org))
				if err := apextest.RegisterProjectRuntimeForRequest(machine, typesys.Build(p, schema)); err != nil {
					t.Fatal(err)
				}
				for i := 1; i <= 2; i++ {
					result, err := visualforce.RenderPage(visualforce.PageRenderRequest{Project: p, VFIndex: vf, Org: &org, Machine: machine, PageName: "Bulk"})
					if err != nil || result.Error != nil || !strings.Contains(result.HTML, "40") {
						t.Fatalf("render %d: %v / %+v", i, err, result)
					}
					records := org.Objects["Account"].Records
					if len(records) != 40*i {
						t.Fatalf("render %d: records=%d", i, len(records))
					}
					for _, row := range records {
						if row.Fields["Name"].String != "After" {
							t.Fatalf("updated name: %#v", row.Fields["Name"])
						}
					}
				}
			})
		}
	}
}

package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func TestOpportunityReportScopeNative(t *testing.T) {
	capture := "../resource/testdata/report_rows_capture/opportunity-scope"
	raw, err := os.ReadFile(filepath.Join(capture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) == 4 {
			rows[parts[0]+"/"+parts[1]+"/"+parts[2]] = parts[3]
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "seealldata"} {
			for _, name := range []string{"Absent", "Organization", "User"} {
				t.Run(api+"/"+route+"/"+name, func(t *testing.T) {
					root := t.TempDir()
					writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
					base := filepath.Join(root, "force-app/main/default/reports")
					writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
					data, e := os.ReadFile(filepath.Join(capture, "reports/B6_Reports/B6_"+name+".report-meta.xml"))
					if e != nil {
						t.Fatal(e)
					}
					writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_"+name+".report-meta.xml"), string(data))
					want := strings.Split(rows[api+"/"+route+"/opportunity-scope-B6_"+name], ",")
					if len(want) != 2 {
						t.Fatal("missing native row")
					}
					p, e := project.Load(root)
					if e != nil {
						t.Fatal(e)
					}
					org := storage.NewOrgState()
					storage.EnsureStandardObject(&org, "User")
					storage.EnsureStandardObject(&org, "Opportunity")
					users := org.Objects["User"]
					users.Records[storage.ID(storage.DefaultUserID)] = storage.Record{ID: storage.ID(storage.DefaultUserID), Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(storage.DefaultUserID)}}
					org.Objects["User"] = users
					opportunities := org.Objects["Opportunity"]
					for i := 0; i < 2; i++ {
						id := storage.ID(fmt.Sprintf("006%012d", i+1))
						owner := storage.ID(storage.DefaultUserID)
						if i == 1 {
							owner = storage.ID("005000000000002")
						}
						opportunities.Records[id] = storage.Record{ID: id, Object: "Opportunity", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue(fmt.Sprintf("B6 %d", i)), "CloseDate": storage.DateValue("2026-11-03"), "OwnerId": storage.IDValue(owner)}, System: storage.SystemFields{OwnerID: owner}}
					}
					org.Objects["Opportunity"] = opportunities
					if e = resource.ApplyProject(&org, p); e != nil {
						t.Fatal(e)
					}
					m := New(nil)
					m.SetOrg(&org)
					m.fakeNow = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
					if route == "seealldata" {
						m.EnableTestContext()
						m.SetTestSeeAllData(true)
					}
					code := fmt.Sprintf(`Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1];System.assertEquals('%s',Reports.ReportManager.describeReport(r.Id).getReportMetadata().getScope());System.assertEquals(%s,Reports.ReportManager.runReport(r.Id,false).getFactMap().get('T!T').getAggregates()[0].getValue());`, name, want[0], want[1])
					program, e := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
					if e != nil {
						t.Fatal(e)
					}
					if _, e = m.Execute(program); e != nil {
						t.Fatal(e)
					}
				})
			}
		}
	}
}

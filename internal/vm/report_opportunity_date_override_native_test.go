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

func TestOpportunityReportDateOverrideNative(t *testing.T) {
	capture := "../resource/testdata/report_rows_capture/opportunity-date-override"
	raw, err := os.ReadFile(filepath.Join(capture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		p := strings.SplitN(line, "\t", 4)
		if len(p) == 4 {
			rows[p[0]+"/"+p[1]+"/"+p[2]] = p[3]
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
					for i := 0; i < 3; i++ {
						id := storage.ID(fmt.Sprintf("006%012d", i+1))
						owner := storage.ID(storage.DefaultUserID)
						if i == 1 {
							owner = storage.ID("005000000000002")
						}
						closeDate := "2026-11-03"
						if i == 2 {
							closeDate = "2027-06-03"
						}
						opportunities.Records[id] = storage.Record{ID: id, Object: "Opportunity", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue(fmt.Sprintf("B6 %d", i)), "CloseDate": storage.DateValue(closeDate), "OwnerId": storage.IDValue(owner)}, System: storage.SystemFields{OwnerID: owner, CreatedDate: "2026-11-03T21:04:22Z"}}
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
					code := fmt.Sprintf(`Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1];`, name)
					for _, tc := range []struct{ name, start, end string }{{"inside", "'2000-01-01'", "'2100-01-01'"}, {"future", "'2099-01-01'", "'2100-01-01'"}, {"open", "null", "null"}, {"invalid", "'not-date'", "'2100-01-01'"}} {
						want := strings.Split(rows[api+"/"+route+"/opportunity-date-B6_"+name+"-"+tc.name], "~")
						setup := fmt.Sprintf(`Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();m.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM',%s,%s));`, tc.start, tc.end)
						if tc.name == "invalid" {
							if len(want) != 2 {
								t.Fatal("missing rejection row")
							}
							code += fmt.Sprintf(`{%s Boolean caught=false;try{Reports.ReportManager.runReport(r.Id,m,false);}catch(reports.InvalidFilterException e){caught=true;System.assertEquals('%s',e.getMessage());}System.assert(caught);}`, setup, want[1])
							continue
						}
						if len(want) != 5 {
							t.Fatal("missing success row")
						}
						literal := func(s string) string {
							if s == "null" {
								return "null"
							}
							return "'" + s + "'"
						}
						code += fmt.Sprintf(`{%s Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,m,false);Reports.StandardDateFilter d=x.getReportMetadata().getStandardDateFilter();System.assertEquals(%s,x.getFactMap().get('T!T').getAggregates()[0].getValue());System.assertEquals('%s',d.getColumn());System.assertEquals('%s',d.getDurationValue());System.assertEquals(%s,d.getStartDate());System.assertEquals(%s,d.getEndDate());}`, setup, want[0], want[1], want[2], literal(want[3]), literal(want[4]))
					}
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

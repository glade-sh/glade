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

func TestOpportunityReportTimeFrameNative(t *testing.T) {
	capture := "../resource/testdata/report_rows_capture/opportunity-timeframe"
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
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
				base := filepath.Join(root, "force-app/main/default/reports")
				writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
				code := ""
				for _, name := range []string{"Inside", "Future", "Open"} {
					data, e := os.ReadFile(filepath.Join(capture, "reports/B6_Reports/B6_"+name+".report-meta.xml"))
					if e != nil {
						t.Fatal(e)
					}
					writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_"+name+".report-meta.xml"), string(data))
					want := strings.Split(rows[api+"/"+route+"/opportunity-timeframe-B6_"+name], "~")
					if len(want) != 5 {
						t.Fatal("missing native row")
					}
					literal := func(s string) string {
						if s == "null" {
							return "null"
						}
						return "'" + s + "'"
					}
					code += fmt.Sprintf(`{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1];Reports.StandardDateFilter d=Reports.ReportManager.describeReport(r.Id).getReportMetadata().getStandardDateFilter();System.assertEquals('%s',d.getColumn());System.assertEquals('%s',d.getDurationValue());System.assertEquals(%s,d.getStartDate());System.assertEquals(%s,d.getEndDate());System.assertEquals(%s,Reports.ReportManager.runReport(r.Id,false).getFactMap().get('T!T').getAggregates()[0].getValue());}`, name, want[0], want[1], literal(want[2]), literal(want[3]), want[4])
				}
				p, e := project.Load(root)
				if e != nil {
					t.Fatal(e)
				}
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Opportunity")
				opportunities := org.Objects["Opportunity"]
				id := storage.ID("006000000000001")
				opportunities.Records[id] = storage.Record{ID: id, Object: "Opportunity", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue("B6 Outside Quarter"), "CloseDate": storage.DateValue("2027-06-03")}, System: storage.SystemFields{CreatedDate: "2026-11-03T21:04:22Z"}}
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

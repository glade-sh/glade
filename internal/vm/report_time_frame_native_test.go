package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func TestReportUnboundedTimeFrameNative(t *testing.T) {
	capture, err := os.ReadFile("../resource/testdata/report_rows_capture/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(capture)), "\n") {
		r := strings.SplitN(line, "\t", 4)
		if len(r) == 4 {
			rows[r[0]+"/"+r[1]+"/"+r[2]] = r[3]
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
				for _, name := range []string{"AccountCreated", "AccountDue", "ContactActivity", "ContactCreated", "Bounded"} {
					f := filepath.Join("../resource/testdata/report_rows_capture/time-frame/reports/B6_Reports", "B6_"+name+".report-meta.xml")
					data, e := os.ReadFile(f)
					if e != nil {
						t.Fatal(e)
					}
					writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_"+name+".report-meta.xml"), string(data))
					native := rows[api+"/"+route+"/time-frame-B6_"+name]
					fields := strings.Split(native, ";")
					if len(fields) != 5 {
						t.Fatalf("invalid row: %q", native)
					}
					col := strings.TrimPrefix(fields[0], "column=")
					duration := strings.TrimPrefix(fields[1], "duration=")
					count := strings.TrimPrefix(fields[4], "count=")
					if name == "Bounded" {
						code += `{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Bounded' LIMIT 1];Reports.StandardDateFilter d=Reports.ReportManager.describeReport(r.Id).getReportMetadata().getStandardDateFilter();System.assertEquals('2099-01-01',d.getStartDate());System.assertEquals('2099-12-31',d.getEndDate());System.assertEquals(0,Reports.ReportManager.runReport(r.Id,false).getFactMap().get('T!T').getAggregates()[0].getValue());}`
						continue
					}
					if fields[2] != "start=null" || fields[3] != "end=null" || (count != "0" && count != "1") {
						t.Fatalf("unexpected row %q", native)
					}
					code += fmt.Sprintf(`{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1];Reports.StandardDateFilter d=Reports.ReportManager.describeReport(r.Id).getReportMetadata().getStandardDateFilter();System.assertEquals('%s',d.getColumn());System.assertEquals('%s',d.getDurationValue());System.assertEquals(null,d.getStartDate());System.assertEquals(null,d.getEndDate());Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,false);System.assertEquals(%s,x.getFactMap().get('T!T').getAggregates()[0].getValue());}`, name, col, duration, count)
				}
				p, e := project.Load(root)
				if e != nil {
					t.Fatal(e)
				}
				org := testDataOrg()
				storage.EnsureStandardObject(&org, "Contact")
				id := storage.ID("001000000000001")
				a := org.Objects["Account"]
				a.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue("Sample Account for Entitlements")}, System: storage.SystemFields{CreatedDate: "2026-11-03T17:53:26Z"}}
				org.Objects["Account"] = a
				if e = resource.ApplyProject(&org, p); e != nil {
					t.Fatal(e)
				}
				m := New(nil)
				m.SetOrg(&org)
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

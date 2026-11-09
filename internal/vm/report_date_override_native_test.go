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

func TestReportDateOverrideNative(t *testing.T) {
	raw, err := os.ReadFile("../resource/testdata/report_rows_capture/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
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
				data, e := os.ReadFile("../resource/testdata/report_rows_capture/date-override/reports/B6_Reports/B6_Accounts.report-meta.xml")
				if e != nil {
					t.Fatal(e)
				}
				writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Accounts.report-meta.xml"), string(data))
				type testCase struct{ name, start, end string }
				cases := []testCase{{"inside", "2000-01-01", "2100-01-01"}, {"future", "2099-01-01", "2100-01-01"}, {"open", "", ""}, {"invalid", "not-date", "2100-01-01"}, {"badstart2", "bad-start", "2100-01-01"}, {"badend", "2000-01-01", "bad-end"}, {"reversed", "2100-01-01", "2000-01-01"}}
				literal := func(s string) string {
					if s == "" || s == "null" {
						return "null"
					}
					return "'" + s + "'"
				}
				code := ""
				for _, tc := range cases {
					native := rows[api+"/"+route+"/date-override-"+tc.name]
					parts := strings.Split(native, ":")
					if len(parts) < 3 {
						t.Fatalf("bad native row %q", native)
					}
					setup := fmt.Sprintf(`Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();m.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM',%s,%s));`, literal(tc.start), literal(tc.end))
					if parts[0] == "ok" {
						if len(parts) != 6 {
							t.Fatalf("bad success row %q", native)
						}
						code += fmt.Sprintf(`{%s Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,m,false);Reports.StandardDateFilter d=x.getReportMetadata().getStandardDateFilter();System.assertEquals('%s',d.getColumn());System.assertEquals('%s',d.getDurationValue());System.assertEquals(%s,d.getStartDate());System.assertEquals(%s,d.getEndDate());System.assertEquals(%s,x.getFactMap().get('T!T').getAggregates()[0].getValue());}`, setup, parts[1], parts[2], literal(parts[3]), literal(parts[4]), parts[5])
					} else {
						if len(parts) != 3 || parts[1] != "reports.InvalidFilterException" {
							t.Fatalf("bad error row %q", native)
						}
						code += fmt.Sprintf(`{%s Boolean caught=false;try{Reports.ReportManager.runReport(r.Id,m,false);}catch(reports.InvalidFilterException e){caught=true;System.assertEquals('%s',e.getMessage());}System.assert(caught);}`, setup, parts[2])
					}
				}
				for _, tc := range []struct{ name, column string }{{"badcolumn", "NOT_A_DATE"}, {"badcolumn2", "OTHER_DATE"}} {
					parts := strings.SplitN(rows[api+"/"+route+"/date-override-"+tc.name], ":", 3)
					if len(parts) != 3 || parts[0] != "error" || parts[1] != "reports.InvalidFilterException" {
						t.Fatalf("bad native row for %s: %v", tc.name, parts)
					}
					code += fmt.Sprintf(`{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();m.setStandardDateFilter(new Reports.StandardDateFilter('%s','CUSTOM','2000-01-01','2100-01-01'));Boolean caught=false;try{Reports.ReportManager.runReport(r.Id,m,false);}catch(reports.InvalidFilterException e){caught=true;System.assertEquals('%s',e.getMessage());}System.assert(caught);}`, tc.column, parts[2])
				}
				// An uncaptured second metadata change must retain the hosted boundary.
				code += `{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();m.setDescription('Changed');m.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM','2000-01-01','2100-01-01'));Boolean rejected=false;try{Reports.ReportManager.runReport(r.Id,m,false);}catch(System.UnsupportedOperationException e){rejected=true;}System.assert(rejected);}`
				p, e := project.Load(root)
				if e != nil {
					t.Fatal(e)
				}
				// The native capture used a scratch org with the standard Account
				// catalog, not testDataOrg's unrelated synthetic custom fields.
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Account")
				id := storage.ID("001000000000001")
				a := org.Objects["Account"]
				a.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue("Sample Account for Entitlements")}, System: storage.SystemFields{CreatedDate: "2026-11-03T17:53:26Z"}}
				org.Objects["Account"] = a
				if e = resource.ApplyProject(&org, p); e != nil {
					t.Fatal(e)
				}
				m := New(nil)
				m.SetOrg(&org)
				m.SetCurrentUser(storage.Record{ID: storage.DefaultUserID, Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(storage.DefaultUserID)}})
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

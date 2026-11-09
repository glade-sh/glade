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

func TestReportContactDateOverrideNative(t *testing.T) {
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
				type testCase struct{ name, start, end string }
				cases := []testCase{{"inside", "2000-01-01", "2100-01-01"}, {"future", "2099-01-01", "2100-01-01"}, {"open", "", ""}, {"invalid", "not-date", "2100-01-01"}, {"badend", "2000-01-01", "bad-end"}, {"reversed", "2100-01-01", "2000-01-01"}}
				literal := func(s string) string {
					if s == "" || s == "null" {
						return "null"
					}
					return "'" + s + "'"
				}
				code := ""
				for _, name := range []string{"Org", "User"} {
					path := filepath.Join("../resource/testdata/report_rows_capture/contact-date-override/reports/B6_Reports", "B6_"+name+".report-meta.xml")
					data, e := os.ReadFile(path)
					if e != nil {
						t.Fatal(e)
					}
					writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_"+name+".report-meta.xml"), string(data))
					for _, tc := range cases {
						native := rows[api+"/"+route+"/contact-override-B6_"+name+"-"+tc.name]
						parts := strings.Split(native, ":")
						if len(parts) < 3 {
							t.Fatalf("bad native row %q", native)
						}
						setup := fmt.Sprintf(`Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();m.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM',%s,%s));`, name, literal(tc.start), literal(tc.end))
						if parts[0] == "ok" {
							if len(parts) != 4 {
								t.Fatalf("bad success row %q", native)
							}
							code += fmt.Sprintf(`{%s Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,m,false);Reports.StandardDateFilter d=x.getReportMetadata().getStandardDateFilter();System.assertEquals(%s,d.getStartDate());System.assertEquals(%s,d.getEndDate());System.assertEquals(%s,x.getFactMap().get('T!T').getAggregates()[0].getValue());}`, setup, literal(parts[1]), literal(parts[2]), parts[3])
						} else {
							if len(parts) != 3 || parts[1] != "reports.InvalidFilterException" {
								t.Fatalf("bad error row %q", native)
							}
							code += fmt.Sprintf(`{%s Boolean caught=false;try{Reports.ReportManager.runReport(r.Id,m,false);}catch(reports.InvalidFilterException e){caught=true;System.assertEquals('%s',e.getMessage());}System.assert(caught);}`, setup, parts[2])
						}
					}
				}
				p, e := project.Load(root)
				if e != nil {
					t.Fatal(e)
				}
				org := testDataOrg()
				storage.EnsureStandardObject(&org, "Contact")
				aid := storage.ID("001000000000001")
				a := org.Objects["Account"]
				a.Records[aid] = storage.Record{ID: aid, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(aid), "Name": storage.StringValue("B6 Parent"), "OwnerId": storage.IDValue(storage.DefaultUserID)}, System: storage.SystemFields{OwnerID: storage.ID(storage.DefaultUserID)}}
				org.Objects["Account"] = a
				cid := storage.ID("003000000000001")
				contacts := org.Objects["Contact"]
				contacts.Records[cid] = storage.Record{ID: cid, Object: "Contact", Fields: map[string]storage.Value{"Id": storage.IDValue(cid), "LastName": storage.StringValue("B6 Contact"), "AccountId": storage.IDValue(aid)}, System: storage.SystemFields{CreatedDate: "2026-11-03T21:04:22Z"}}
				org.Objects["Contact"] = contacts
				if e = resource.ApplyProject(&org, p); e != nil {
					t.Fatal(e)
				}
				m := New(nil)
				m.SetOrg(&org)
				m.SetCurrentUser(storage.Record{ID: storage.ID(storage.DefaultUserID), Object: "User"})
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

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

func TestReportAdjacentNative(t *testing.T) {
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
				code := ""
				for _, name := range []string{"Accounts", "Contacts", "Opportunities"} {
					path := filepath.Join("../resource/testdata/report_rows_capture/adjacent-reports/reports/B6_Reports", "B6_"+name+".report-meta.xml")
					data, e := os.ReadFile(path)
					if e != nil {
						t.Fatal(e)
					}
					writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_"+name+".report-meta.xml"), string(data))
					native := rows[api+"/"+route+"/adjacent-B6_"+name]
					v := strings.Split(native, ":")
					if len(v) != 6 {
						t.Fatalf("bad native row %q", native)
					}
					code += fmt.Sprintf(`{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,false);System.assertEquals('%s',m.getDescription());System.assertEquals(%s,m.getShowGrandTotal());System.assertEquals(%s,m.getShowSubtotals());System.assertEquals('%s',x.getReportMetadata().getDescription());System.assertEquals(%s,x.getReportMetadata().getShowGrandTotal());System.assertEquals(%s,x.getReportMetadata().getShowSubtotals());}`, name, v[0], v[1], v[2], v[3], v[4], v[5])
				}
				for _, q := range []struct{ name, start, end string }{{"inside", "'2000-01-01'", "'2100-01-01'"}, {"future", "'2099-01-01'", "'2100-01-01'"}, {"open", "null", "null"}} {
					want := rows[api+"/"+route+"/adjacent-"+q.name]
					if want != "0" && want != "1" {
						t.Fatalf("bad native count %q", want)
					}
					code += fmt.Sprintf(`{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();m.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM',%s,%s));System.assertEquals(%s,Reports.ReportManager.runReport(r.Id,m,false).getFactMap().get('T!T').getAggregates()[0].getValue());}`, q.start, q.end, want)
				}
				p, e := project.Load(root)
				if e != nil {
					t.Fatal(e)
				}
				org := testDataOrg()
				storage.EnsureStandardObject(&org, "Contact")
				storage.EnsureStandardObject(&org, "Opportunity")
				id := storage.ID("001000000000001")
				a := org.Objects["Account"]
				a.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue("B6 Owner"), "OwnerId": storage.IDValue(storage.DefaultUserID)}, System: storage.SystemFields{CreatedDate: "2026-11-03T17:53:26Z", OwnerID: storage.ID(storage.DefaultUserID)}}
				org.Objects["Account"] = a
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

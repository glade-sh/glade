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

func TestReportNoScopeOverrideNative(t *testing.T) {
	capture := "../resource/testdata/report_rows_capture/no-scope-override"
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
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
				base := filepath.Join(root, "force-app/main/default/reports")
				writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
				code := ""
				for _, name := range []string{"AccountAbsent", "AccountUser", "ContactAbsent", "ContactUser"} {
					data, e := os.ReadFile(filepath.Join(capture, "reports/B6_Reports/B6_"+name+".report-meta.xml"))
					if e != nil {
						t.Fatal(e)
					}
					writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_"+name+".report-meta.xml"), string(data))
					vals := strings.Split(rows[api+"/"+route+"/no-scope-override-B6_"+name], ",")
					if len(vals) != 4 {
						t.Fatalf("missing native row for %s", name)
					}
					code += fmt.Sprintf(`{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1]; Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();System.assertEquals('%s',m.getScope());`, name, vals[0])
					for i, dates := range [][2]string{{"'2000-01-01'", "'2100-01-01'"}, {"'2099-01-01'", "'2100-01-01'"}, {"null", "null"}} {
						code += fmt.Sprintf(`m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();m.setStandardDateFilter(new Reports.StandardDateFilter('CREATED_DATE','CUSTOM',%s,%s));System.assertEquals(%s,Reports.ReportManager.runReport(r.Id,m,false).getFactMap().get('T!T').getAggregates()[0].getValue());`, dates[0], dates[1], vals[i+1])
					}
					code += "}"
				}
				p, e := project.Load(root)
				if e != nil {
					t.Fatal(e)
				}
				org := testDataOrg()
				storage.EnsureStandardObject(&org, "Contact")
				aid := storage.ID("001000000000001")
				a := org.Objects["Account"]
				a.Records[aid] = storage.Record{ID: aid, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(aid), "Name": storage.StringValue("B6 Parent"), "OwnerId": storage.IDValue(storage.DefaultUserID)}, System: storage.SystemFields{OwnerID: storage.ID(storage.DefaultUserID), CreatedDate: "2026-11-03T21:04:22Z"}}
				org.Objects["Account"] = a
				cid := storage.ID("003000000000001")
				c := org.Objects["Contact"]
				c.Records[cid] = storage.Record{ID: cid, Object: "Contact", Fields: map[string]storage.Value{"Id": storage.IDValue(cid), "LastName": storage.StringValue("B6 Contact"), "AccountId": storage.IDValue(aid)}, System: storage.SystemFields{CreatedDate: "2026-11-03T21:04:22Z"}}
				org.Objects["Contact"] = c
				if e = resource.ApplyProject(&org, p); e != nil {
					t.Fatal(e)
				}
				m := New(nil)
				m.SetOrg(&org)
				m.SetCurrentUser(storage.Record{ID: storage.ID(storage.DefaultUserID), Object: "User"})
				if route == "istest" {
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

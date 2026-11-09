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

func TestReportZeroDimensionNative(t *testing.T) {
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
				data, e := os.ReadFile("../resource/testdata/report_rows_capture/dimension-zero/reports/B6_Reports/B6_Zero.report-meta.xml")
				if e != nil {
					t.Fatal(e)
				}
				writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Zero.report-meta.xml"), string(data))
				native := rows[api+"/"+route+"/dimension-zero"]
				parts := strings.Split(native, "|")
				if len(parts) != 6 {
					t.Fatalf("bad native row %q", native)
				}
				code := fmt.Sprintf(`Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Zero' LIMIT 1];Reports.ReportResults a=Reports.ReportManager.runReport(r.Id,false);Reports.ReportResults b=Reports.ReportManager.runReport(r.Id,true);System.assertEquals('%s',JSON.serialize(a.getGroupingsDown()));System.assertEquals(%s,a.getGroupingsDown().getGroupings().size());System.assertEquals('%s',JSON.serialize(a.getGroupingsAcross()));List<String> x=new List<String>(a.getFactMap().keySet());List<String> y=new List<String>(b.getFactMap().keySet());x.sort();y.sort();System.assertEquals('%s',String.join(x,','));System.assertEquals('%s',String.join(y,','));System.assertEquals(%s,a.getFactMap().get('T!T').getAggregates()[0].getValue());`, parts[0], parts[1], parts[2], parts[3], parts[4], parts[5])
				p, e := project.Load(root)
				if e != nil {
					t.Fatal(e)
				}
				org := testDataOrg()
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

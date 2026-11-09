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

func TestReportTotalsNative(t *testing.T) {
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
				for _, name := range []string{"TabAbsent", "TabFalse", "TabTrue", "SumAbsent", "SumFalse", "SumMixed"} {
					path := filepath.Join("../resource/testdata/report_rows_capture/report-totals/reports/B6_Reports", "B6_"+name+".report-meta.xml")
					data, e := os.ReadFile(path)
					if e != nil {
						t.Fatal(e)
					}
					writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_"+name+".report-meta.xml"), string(data))
					native := rows[api+"/"+route+"/totals-B6_"+name]
					v := strings.Split(native, ",")
					if len(v) != 4 {
						t.Fatalf("bad native row %q", native)
					}
					for _, x := range v {
						if x != "true" && x != "false" {
							t.Fatalf("bad boolean %q", x)
						}
					}
					facts := rows[api+"/"+route+"/totals-facts-B6_"+name]
					if facts == "" {
						t.Fatal("missing fact row")
					}
					code += fmt.Sprintf(`{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();Reports.ReportResults a=Reports.ReportManager.runReport(r.Id,false);Reports.ReportResults b=Reports.ReportManager.runReport(r.Id,true);System.assertEquals(%s,m.getShowGrandTotal());System.assertEquals(%s,m.getShowSubtotals());System.assertEquals(%s,a.getReportMetadata().getShowGrandTotal());System.assertEquals(%s,a.getReportMetadata().getShowSubtotals());List<String> x=new List<String>(a.getFactMap().keySet());List<String> y=new List<String>(b.getFactMap().keySet());x.sort();y.sort();String f=String.join(x,',')+','+String.join(y,',')+','+a.getFactMap().get('T!T').getAggregates()[0].getValue()+','+a.getGroupingsDown().getGroupings().size();System.assertEquals('%s',f);}`, name, v[0], v[1], v[2], v[3], facts)
				}
				p, e := project.Load(root)
				if e != nil {
					t.Fatal(e)
				}
				org := testDataOrg()
				id := storage.ID("001000000000001")
				a := org.Objects["Account"]
				a.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue("Sample Account for Entitlements")}}
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

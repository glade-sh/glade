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

func TestReportDimensionsNative(t *testing.T) {
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
				for _, name := range []string{"TabAbsent", "SumAbsent"} {
					path := filepath.Join("../resource/testdata/report_rows_capture/report-totals/reports/B6_Reports", "B6_"+name+".report-meta.xml")
					data, e := os.ReadFile(path)
					if e != nil {
						t.Fatal(e)
					}
					writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_"+name+".report-meta.xml"), string(data))
					native := rows[api+"/"+route+"/totals-dimensions-B6_"+name]
					values := strings.Split(native, ",")
					if len(values) != 4 {
						t.Fatalf("bad native row %q", native)
					}
					for _, v := range values {
						if v != "0" && v != "1" {
							t.Fatalf("bad dimension count %q", v)
						}
					}
					if !strings.Contains(rows[api+"/"+route+"/dimension-json"], `{"groupings":null}`) {
						t.Fatal("missing native empty dimension JSON")
					}
					code += fmt.Sprintf(`{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1];Reports.ReportResults a=Reports.ReportManager.runReport(r.Id,false);Reports.ReportResults b=Reports.ReportManager.runReport(r.Id,true);System.assertEquals(%s,a.getGroupingsDown().getGroupings().size());System.assertEquals(%s,a.getGroupingsAcross().getGroupings().size());System.assertEquals(%s,b.getGroupingsDown().getGroupings().size());System.assertEquals(%s,b.getGroupingsAcross().getGroupings().size());System.assertEquals('{"groupings":null}',JSON.serialize(a.getGroupingsAcross()));System.assertEquals('{"groupings":null}',JSON.serialize(b.getGroupingsAcross()));`, name, values[0], values[1], values[2], values[3])
					if name == "TabAbsent" {
						code += `System.assertEquals('{"groupings":null}',JSON.serialize(a.getGroupingsDown()));System.assertEquals('{"groupings":null}',JSON.serialize(b.getGroupingsDown()));`
					}
					code += "}"
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

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

func TestReportTimeBoundsNative(t *testing.T) {
	data, err := os.ReadFile("../resource/testdata/report_rows_capture/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		r := strings.SplitN(line, "\t", 4)
		if len(r) == 4 {
			rows[r[0]+"/"+r[1]+"/"+r[2]] = r[3]
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "seealldata"} {
			for _, zone := range []string{"default", "pacific-kiritimati"} {
				t.Run(api+"/"+route+"/"+zone, func(t *testing.T) {
					root := t.TempDir()
					writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
					base := filepath.Join(root, "force-app/main/default/reports")
					writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
					code := ""
					stamp := ""
					for _, name := range []string{"After", "AtEnd", "AtStart", "Before", "EndOnly", "Inside", "OnDay", "StartOnly"} {
						fixture := filepath.Join("../resource/testdata/report_rows_capture/time-boundaries/reports/B6_Reports", "B6_"+name+".report-meta.xml")
						raw, e := os.ReadFile(fixture)
						if e != nil {
							t.Fatal(e)
						}
						writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_"+name+".report-meta.xml"), string(raw))
						native := rows[api+"/"+route+"/bound-"+zone+"-B6_"+name]
						parts := strings.Split(native, ";")
						if len(parts) != 4 {
							t.Fatalf("bad native row %q", native)
						}
						start := strings.TrimPrefix(parts[0], "start=")
						end := strings.TrimPrefix(parts[1], "end=")
						stamp = strings.TrimPrefix(parts[2], "recordUtc=")
						count := strings.TrimPrefix(parts[3], "count=")
						val := func(s string) string {
							if s == "null" {
								return "null"
							}
							return "'" + s + "'"
						}
						if count != "0" && count != "1" {
							t.Fatalf("bad native count %q", count)
						}
						code += fmt.Sprintf(`{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1];Reports.StandardDateFilter d=Reports.ReportManager.describeReport(r.Id).getReportMetadata().getStandardDateFilter();System.assertEquals(%s,d.getStartDate());System.assertEquals(%s,d.getEndDate());Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,false);System.assertEquals(%s,x.getFactMap().get('T!T').getAggregates()[0].getValue());}`, name, val(start), val(end), count)
					}
					p, e := project.Load(root)
					if e != nil {
						t.Fatal(e)
					}
					org := testDataOrg()
					id := storage.ID("001000000000001")
					a := org.Objects["Account"]
					a.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue("Sample Account for Entitlements")}, System: storage.SystemFields{CreatedDate: stamp}}
					org.Objects["Account"] = a
					if e = resource.ApplyProject(&org, p); e != nil {
						t.Fatal(e)
					}
					m := New(nil)
					m.SetOrg(&org)
					tz := "America/Los_Angeles"
					if zone == "pacific-kiritimati" {
						tz = "Pacific/Kiritimati"
					}
					m.SetCurrentUser(storage.Record{ID: storage.ID(storage.DefaultUserID), Object: "User", Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue(tz)}})
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
}

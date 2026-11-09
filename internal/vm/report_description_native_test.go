package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
)

func TestReportDescriptionNative(t *testing.T) {
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
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
				base := filepath.Join(root, "force-app/main/default/reports")
				writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
				code := ""
				for _, name := range []string{"Absent", "Alpha", "Beta", "Empty"} {
					path := filepath.Join("../resource/testdata/report_rows_capture/report-description/reports/B6_Reports", "B6_"+name+".report-meta.xml")
					raw, e := os.ReadFile(path)
					if e != nil {
						t.Fatal(e)
					}
					writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_"+name+".report-meta.xml"), string(raw))
					native := rows[api+"/"+route+"/description-B6_"+name]
					parts := strings.Split(native, ";")
					if len(parts) != 2 {
						t.Fatalf("bad native row %q", native)
					}
					value := func(p, pre string) string {
						if !strings.HasPrefix(p, pre) {
							t.Fatalf("bad native field %q", p)
						}
						raw := strings.TrimPrefix(p, pre)
						if raw == "null" {
							return "null"
						}
						var x string
						if e := json.Unmarshal([]byte(raw), &x); e != nil {
							t.Fatal(e)
						}
						return "'" + strings.ReplaceAll(strings.ReplaceAll(x, `\`, `\\`), "'", `\'`) + "'"
					}
					desc, run := value(parts[0], "describe="), value(parts[1], "run=")
					code += fmt.Sprintf(`{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1];System.assertEquals(%s,Reports.ReportManager.describeReport(r.Id).getReportMetadata().getDescription());System.assertEquals(%s,Reports.ReportManager.runReport(r.Id,false).getReportMetadata().getDescription());}`, name, desc, run)
				}
				p, e := project.Load(root)
				if e != nil {
					t.Fatal(e)
				}
				org := testDataOrg()
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

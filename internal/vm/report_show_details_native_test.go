package vm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
)

// TestReportShowDetailsNative consumes the captured describe and explicit-run
// values for omitted, false and true showDetails at both supported versions.
func TestReportShowDetailsNative(t *testing.T) {
	capture, err := os.ReadFile("../resource/testdata/report_rows_capture/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(capture)), "\n") {
		row := strings.SplitN(line, "\t", 4)
		if len(row) == 4 {
			rows[row[0]+"/"+row[1]+"/"+row[2]] = row[3]
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
				for _, item := range []struct{ name, tag, captured string }{{"Absent", "", "absent"}, {"False", "<showDetails>false</showDetails>", "false"}, {"True", "<showDetails>true</showDetails>", "true"}} {
					native := rows[api+"/"+route+"/show-details-"+item.captured]
					var desc, no, yes string
					fields := strings.Split(native, ";")
					if len(fields) != 3 {
						t.Fatalf("invalid native row: %q", native)
					}
					desc = strings.TrimPrefix(fields[0], "describe=")
					no = strings.TrimPrefix(fields[1], "runFalse=")
					yes = strings.TrimPrefix(fields[2], "runTrue=")
					for _, value := range []string{desc, no, yes} {
						if value != "true" && value != "false" {
							t.Fatalf("invalid boolean: %q", value)
						}
					}
					xml := `<Report><name>B6 ` + item.name + `</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns>` + item.tag + `</Report>`
					writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_"+item.name+".report-meta.xml"), xml)
					overloadRow := rows[api+"/"+route+"/show-overloads-B6_"+item.name]
					pieces := strings.Split(overloadRow, ";")
					if len(pieces) != 2 || pieces[0] != "describe="+desc || !strings.HasPrefix(pieces[1], "results=") {
						t.Fatalf("invalid overload row: %q", overloadRow)
					}
					pairs := strings.Split(strings.TrimPrefix(pieces[1], "results="), "|")
					if len(pairs) != 6 {
						t.Fatalf("invalid overload results: %q", overloadRow)
					}
					code += fmt.Sprintf(`{Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_%s' LIMIT 1]; Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();System.assertEquals(%s,m.getHasDetailRows());`, item.name, desc)
					calls := []string{"Reports.ReportManager.runReport(r.Id)", "Reports.ReportManager.runReport(r.Id,false)", "Reports.ReportManager.runReport(r.Id,true)", "Reports.ReportManager.runReport(r.Id,m)", "Reports.ReportManager.runReport(r.Id,m,false)", "Reports.ReportManager.runReport(r.Id,m,true)"}
					for i, pair := range pairs {
						values := strings.Split(pair, ",")
						if len(values) != 2 {
							t.Fatalf("invalid overload pair: %q", pair)
						}
						for _, value := range values {
							if value != "true" && value != "false" {
								t.Fatalf("invalid overload boolean: %q", value)
							}
						}
						code += fmt.Sprintf(`{Reports.ReportResults x=%s;System.assertEquals(%s,x.getHasDetailRows());System.assertEquals(%s,x.getReportMetadata().getHasDetailRows());}`, calls[i], values[0], values[1])
					}
					code += "}"
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				if err = resource.ApplyProject(&org, p); err != nil {
					t.Fatal(err)
				}
				m := New(nil)
				m.SetOrg(&org)
				if route == "seealldata" {
					m.EnableTestContext()
					m.SetTestSeeAllData(true)
				}
				program, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = m.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

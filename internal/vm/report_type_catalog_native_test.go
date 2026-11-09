package vm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func TestProjectReportTypeCatalogNative(t *testing.T) {
	dir := "../resource/testdata/report_rows_capture/catalog-locale-week"
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		p := strings.SplitN(line, "\t", 4)
		if len(p) != 4 {
			t.Fatalf("bad native row %q", line)
		}
		parts := strings.Split(strings.TrimPrefix(p[2], "locale-week-"), "-")
		if len(parts) != 2 {
			t.Fatalf("bad case %s", p[2])
		}
		locale, name := parts[0], parts[1]
		rows++
		t.Run(p[0]+"/"+p[1]+"/"+locale+"/"+name, func(t *testing.T) {
			root := t.TempDir()
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default/reports")
			writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			reportName := strings.ToUpper(name[:1]) + name[1:]
			data, e := os.ReadFile(filepath.Join(dir, "reports/B6_Reports/B6_"+reportName+".report-meta.xml"))
			if e != nil {
				t.Fatal(e)
			}
			writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_"+reportName+".report-meta.xml"), string(data))
			projectDef, e := project.Load(root)
			if e != nil {
				t.Fatal(e)
			}
			org := storage.NewOrgState()
			if e = resource.ApplyProject(&org, projectDef); e != nil {
				t.Fatal(e)
			}
			m := New(nil)
			m.SetOrg(&org)
			m.fakeNow = time.Date(2026, 11, 4, 1, 0, 0, 0, time.UTC)
			m.SetCurrentUser(storage.Record{ID: storage.ID(storage.DefaultUserID), Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(storage.DefaultUserID), "TimeZoneSidKey": storage.StringValue("Pacific/Kiritimati"), "LocaleSidKey": storage.StringValue(locale), "LanguageLocaleKey": storage.StringValue("en_US")}})
			if p[1] == "seealldata" {
				m.EnableTestContext()
				m.SetTestSeeAllData(true)
			}
			code := `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_` + reportName + `' LIMIT 1];Reports.ReportTypeMetadata m=Reports.ReportManager.describeReport(r.Id).getReportTypeMetadata();System.assertNotEquals(null,m);System.debug('B6CAT|'+JSON.serialize(m));`
			if name == "opportunity" {
				code += `Reports.StandardFilterInfo f=m.getStandardFilterInfos().get('open');Object x=f;String s=String.valueOf(x instanceof Reports.StandardFilterInfoPicklist);if(x instanceof Reports.StandardFilterInfoPicklist){Reports.StandardFilterInfoPicklist q=(Reports.StandardFilterInfoPicklist)x;s+='~'+q.getDefaultValue()+'~'+q.getFilterValues().size()+'~'+q.getLabel()+'~'+String.valueOf(q.getType());}System.debug('B6PICK|'+s);`
			}
			program, e := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: p[0]})
			if e != nil {
				t.Fatal(e)
			}
			out, e := m.Execute(program)
			if e != nil {
				t.Fatal(e)
			}
			if len(out.Debug) == 0 || !strings.HasPrefix(out.Debug[0], "B6CAT|") {
				t.Fatal("missing catalog output")
			}
			if name == "opportunity" {
				pick, e := os.ReadFile("../resource/testdata/report_rows_capture/catalog-picklist-type/org.tsv")
				if e != nil {
					t.Fatal(e)
				}
				expected := ""
				for _, line := range strings.Split(strings.TrimSpace(string(pick)), "\n")[1:] {
					row := strings.SplitN(line, "\t", 4)
					if len(row) == 4 && row[0] == p[0] && row[1] == p[1] {
						expected = row[3]
						break
					}
				}
				if expected == "" || len(out.Debug) != 2 || out.Debug[1] != "B6PICK|"+expected {
					t.Fatalf("picklist native mismatch: %v", out.Debug)
				}
			} else if len(out.Debug) != 1 {
				t.Fatalf("unexpected output: %v", out.Debug)
			}
			wantRaw, e := os.ReadFile(filepath.Join(dir, locale+"-"+name+"-"+strings.TrimSuffix(p[0], ".0")+".json"))
			if e != nil {
				t.Fatal(e)
			}
			var got, want any
			if e = json.Unmarshal([]byte(strings.TrimPrefix(out.Debug[0], "B6CAT|")), &got); e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(wantRaw, &want); e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("catalog differs from native %s %s", locale, name)
			}
		})
	}
	if rows != 24 {
		t.Fatalf("got %d native rows", rows)
	}
}

package vm

import (
	"encoding/json"
	"fmt"
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

// The native matrix contains baseline, hidden and granted fields on all three
// report objects; hidden/granted snapshots include their full catalogs.
func TestProjectReportCustomCatalogNative(t *testing.T) {
	dir := "../resource/testdata/report_rows_capture/type-catalog-three"
	rows, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	partial, err := os.ReadFile("../resource/testdata/report_rows_capture/catalog-partial-fields/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := append(strings.Split(strings.TrimSpace(string(rows)), "\n")[1:], strings.Split(strings.TrimSpace(string(partial)), "\n")[1:]...)
	n := 0
	for _, line := range lines {
		p := strings.SplitN(line, "\t", 4)
		if len(p) != 4 {
			t.Fatal("invalid native row")
		}
		parts := strings.Split(strings.TrimPrefix(p[2], "catalog-"), "-")
		if len(parts) != 2 || parts[0] == "baseline" {
			continue
		}
		state, name := parts[0], parts[1]
		n++
		t.Run(p[0]+"/"+p[1]+"/"+state+"/"+name, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			copyFixture := func(from, to string) {
				b, e := os.ReadFile(from)
				if e != nil {
					t.Fatal(e)
				}
				writeVMTestFile(t, to, string(b))
			}
			copyFixture(filepath.Join(dir, "reports/B6_Reports.reportFolder-meta.xml"), filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml"))
			reportName := strings.ToUpper(name[:1]) + name[1:]
			copyFixture(filepath.Join(dir, "reports/B6_Reports/B6_"+reportName+".report-meta.xml"), filepath.Join(base, "reports/B6_Reports/B6_"+reportName+".report-meta.xml"))
			for _, object := range []string{"Account", "Contact", "Opportunity"} {
				for _, field := range []string{"B6_Text__c", "B6_Date__c", "B6_Amount__c"} {
					path := filepath.Join("objects", object, "fields", field+".field-meta.xml")
					copyFixture(filepath.Join(dir, "custom", path), filepath.Join(base, path))
				}
			}
			projectDef, e := project.Load(root)
			if e != nil {
				t.Fatal(e)
			}
			org := storage.NewOrgState()
			if e = resource.ApplyProject(&org, projectDef); e != nil {
				t.Fatal(e)
			}
			// Project schema loading precedes resource application in the runner.
			for _, object := range []string{"Account", "Contact", "Opportunity"} {
				storage.EnsureStandardObject(&org, object)
				o := org.Objects[object]
				o.Definition = o.Definition.Clone()
				o.Definition.Fields["B6_Text__c"] = storage.Field{APIName: "B6_Text__c", Label: "B6 Text", Type: storage.FieldString, Length: 30}
				o.Definition.Fields["B6_Date__c"] = storage.Field{APIName: "B6_Date__c", Label: "B6 Date", Type: storage.FieldDate}
				o.Definition.Fields["B6_Amount__c"] = storage.Field{APIName: "B6_Amount__c", Label: "B6 Amount", Type: storage.FieldDecimal}
				org.Objects[object] = o
			}
			if state == "visible" || state == "partial" {
				for _, obj := range []string{"PermissionSet", "PermissionSetAssignment", "FieldPermissions"} {
					storage.EnsureStandardObject(&org, obj)
				}
				ps := "0PS000000000099"
				put := func(object, id string, fields map[string]storage.Value) {
					o := org.Objects[object]
					o.Records[storage.ID(id)] = storage.Record{ID: storage.ID(id), Object: object, Fields: fields}
					org.Objects[object] = o
				}
				put("PermissionSet", ps, map[string]storage.Value{"Name": storage.StringValue("SyntheticCatalog")})
				put("PermissionSetAssignment", "0Pa000000000099", map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue(storage.ID(ps))})
				i := 0
				for _, object := range []string{"Account", "Contact", "Opportunity"} {
					for _, field := range []string{"B6_Text__c", "B6_Date__c", "B6_Amount__c"} {
						if state == "partial" && !((object == "Account" && field == "B6_Text__c") || (object == "Contact" && field == "B6_Date__c") || (object == "Opportunity" && field == "B6_Amount__c")) {
							continue
						}
						i++
						put("FieldPermissions", fmt.Sprintf("0FP000000000%03d", i), map[string]storage.Value{"ParentId": storage.IDValue(storage.ID(ps)), "SObjectType": storage.StringValue(object), "Field": storage.StringValue(object + "." + field), "PermissionsRead": storage.BooleanValue(true)})
					}
				}
			}
			m := New(nil)
			m.SetOrg(&org)
			m.fakeNow = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
			m.SetCurrentUser(storage.Record{ID: storage.ID(storage.DefaultUserID), Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(storage.DefaultUserID), "TimeZoneSidKey": storage.StringValue("UTC"), "LocaleSidKey": storage.StringValue("en_US"), "LanguageLocaleKey": storage.StringValue("en_US")}})
			if p[1] == "seealldata" {
				m.EnableTestContext()
				m.SetTestSeeAllData(true)
			}
			code := `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_` + reportName + `' LIMIT 1];Reports.ReportTypeMetadata m=Reports.ReportManager.describeReport(r.Id).getReportTypeMetadata();System.assertNotEquals(null,m);System.debug('B6CAT|'+JSON.serialize(m));`
			if state == "partial" {
				code = `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_` + reportName + `' LIMIT 1];Reports.ReportTypeMetadata m=Reports.ReportManager.describeReport(r.Id).getReportTypeMetadata();System.assertNotEquals(null,m);String s='';for(Reports.ReportTypeColumnCategory c:m.getCategories()){if(c.getLabel().endsWith('Custom Info')){List<String> keys=new List<String>(c.getColumns().keySet());keys.sort();s+=c.getLabel()+'~'+String.join(keys,',')+';';}}System.debug('B6PART|'+s);`
			}
			program, e := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: p[0]})
			if e != nil {
				t.Fatal(e)
			}
			out, e := m.Execute(program)
			if e != nil {
				t.Fatal(e)
			}
			if state == "partial" {
				if len(out.Debug) != 1 || out.Debug[0] != "B6PART|"+p[3] {
					t.Fatalf("partial catalog: %v; native %s", out.Debug, p[3])
				}
				return
			}
			if len(out.Debug) != 1 || !strings.HasPrefix(out.Debug[0], "B6CAT|") {
				t.Fatal("missing catalog")
			}
			expected, e := os.ReadFile(filepath.Join(dir, state+"-"+name+"-"+strings.TrimSuffix(p[0], ".0")+".json"))
			if e != nil {
				t.Fatal(e)
			}
			var got, want any
			if e = json.Unmarshal([]byte(strings.TrimPrefix(out.Debug[0], "B6CAT|")), &got); e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(expected, &want); e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("catalog differs from captured %s %s", state, name)
			}
		})
	}
	if n != 36 {
		t.Fatalf("got %d native rows", n)
	}
}

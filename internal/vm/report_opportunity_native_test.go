package vm

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func TestOpportunityReportNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "test"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
				base := filepath.Join(root, "force-app/main/default/reports")
				writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
				writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Opportunities.report-meta.xml"), `<Report><name>B6 Opportunities</name><format>Tabular</format><reportType>Opportunity</reportType><scope>organization</scope><columns><field>OPPORTUNITY_NAME</field></columns><columns><field>AMOUNT</field></columns></Report>`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				storage.EnsureStandardObject(&org, "Opportunity")
				state := org.Objects["Opportunity"]
				state.Records = map[storage.ID]storage.Record{}
				for i, name := range []string{"B6 Opportunity One", "B6 Opportunity Two", "B6 Zero Probability", "B6 Outside Quarter", "B6 Negative", "B6 Zero Amount"} {
					id := storage.ID(fmt.Sprintf("006%012d", i+1))
					date := "2026-11-03"
					if i == 3 {
						date = "2027-05-03"
					}
					fields := map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue(name), "CloseDate": storage.DateValue(date)}
					if i == 0 {
						fields["Amount"] = storage.DecimalValue("1234.5")
					}
					if i == 2 {
						fields["Amount"] = storage.DecimalValue("25")
						fields["Probability"] = storage.DecimalValue("0")
					}
					if i == 4 {
						fields["Amount"] = storage.DecimalValue("-123.45")
					}
					if i == 5 {
						fields["Amount"] = storage.DecimalValue("0")
					}
					state.Records[id] = storage.Record{ID: id, Object: "Opportunity", Fields: fields}
				}
				org.Objects["Opportunity"] = state
				if err = resource.ApplyProject(&org, p); err != nil {
					t.Fatal(err)
				}
				m := New(nil)
				m.SetOrg(&org)
				m.fakeNow = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
				if route == "test" {
					m.EnableTestContext()
					m.SetTestSeeAllData(true)
				}
				code := `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Opportunities' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();System.assertEquals('Opportunity',m.getReportType().getType());System.assertEquals('Opportunities',m.getReportType().getLabel());System.assertEquals('CLOSE_DATE',m.getStandardDateFilter().getColumn());System.assertEquals('THIS_FISCAL_QUARTER',m.getStandardDateFilter().getDurationValue());Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,true);Reports.ReportFactWithDetails f=(Reports.ReportFactWithDetails)x.getFactMap().get('T!T');System.assertEquals(5,f.getAggregates()[0].getValue());Set<String> names=new Set<String>();for(Reports.ReportDetailRow row:f.getRows()){List<Reports.ReportDataCell> c=row.getDataCells();names.add(c[0].getLabel());if(c[0].getLabel()=='B6 Opportunity One'){System.assertEquals('$1,234.50',c[1].getLabel());Reports.ReportCurrency v=(Reports.ReportCurrency)c[1].getValue();System.assertEquals(1234.5,v.getAmount());}if(c[0].getLabel()=='B6 Negative')System.assertEquals('-$123.45',c[1].getLabel());if(c[0].getLabel()=='B6 Zero Amount')System.assertEquals('$0.00',c[1].getLabel());if(c[0].getLabel()=='B6 Opportunity Two'){System.assertEquals(null,c[1].getValue());System.assertEquals('-',c[1].getLabel());}}System.assertEquals(new Set<String>{'B6 Opportunity One','B6 Opportunity Two','B6 Zero Probability','B6 Negative','B6 Zero Amount'},names);`
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

package vm

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
)

func TestProjectReportDescribeNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default/reports")
			writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
			writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Summary.report-meta.xml"), `<Report><name>B6 Summary</name><format>Summary</format><reportType>AccountList</reportType><scope>organization</scope></Report>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			source := `
List<Report> rows = [SELECT Id, DeveloperName FROM Report];
System.assertEquals(2, rows.size());
for (Report r : rows) {
 Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();
 if (r.DeveloperName=='B6_Accounts') {
  System.assertEquals('B6 Accounts',m.getName());
  System.assertEquals(Reports.ReportFormat.TABULAR,m.getReportFormat());
  System.assertEquals('ACCOUNT.NAME',m.getDetailColumns()[0]);
 } else if (r.DeveloperName=='B6_Summary') {
  System.assertEquals('B6 Summary',m.getName());
  System.assertEquals(Reports.ReportFormat.SUMMARY,m.getReportFormat());
  System.assertEquals(0,m.getDetailColumns().size());
 }
}`
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMissingProjectReportNativeRejection(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := testDataOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			code := `
for (Id key : new List<Id>{null,Id.valueOf('00O000000000001'),Id.valueOf('001000000000001')}) {
 Boolean describeRejected=false; Boolean runRejected=false; Boolean metadataRejected=false; Boolean metadataFalseRejected=false;
 try { Reports.ReportManager.describeReport(key); } catch(System.NoDataFoundException e) { describeRejected=true; System.assertEquals('The data you’re trying to access is unavailable.',e.getMessage()); }
 try { Reports.ReportManager.runReport(key,true); } catch(System.NoDataFoundException e) { runRejected=true; System.assertEquals('The data you’re trying to access is unavailable.',e.getMessage()); }
 try { Reports.ReportManager.runReport(key,(Reports.ReportMetadata)null); } catch(System.NoDataFoundException e) { metadataRejected=true; }
 try { Reports.ReportManager.runReport(key,(Reports.ReportMetadata)null,false); } catch(System.NoDataFoundException e) { metadataFalseRejected=true; }
 System.assert(describeRejected); System.assert(runRejected); System.assert(metadataRejected); System.assert(metadataFalseRejected);
}`
			program, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestContactReportDescribeNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default/reports")
			writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Contacts.report-meta.xml"), `<Report><name>B6 Contacts</name><format>Tabular</format><reportType>ContactList</reportType><scope>organization</scope><columns><field>LAST_NAME</field></columns></Report>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			code := `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Contacts' LIMIT 1]; Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata(); System.assertEquals('B6 Contacts',m.getName()); System.assertEquals(Reports.ReportFormat.TABULAR,m.getReportFormat()); System.assertEquals('LAST_NAME',m.getDetailColumns()[0]); System.assertEquals('ContactList',m.getReportType().getType()); System.assertEquals('Contacts & Accounts',m.getReportType().getLabel());`
			program, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReportExtendedColumnsNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default/reports")
			writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
			writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Contacts.report-meta.xml"), `<Report><name>B6 Contacts</name><format>Tabular</format><reportType>ContactList</reportType><scope>organization</scope><columns><field>LAST_NAME</field></columns></Report>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			code := `for(Report r:[SELECT Id,DeveloperName FROM Report]){Reports.ReportExtendedMetadata e=Reports.ReportManager.describeReport(r.Id).getReportExtendedMetadata();String col=r.DeveloperName=='B6_Accounts'?'ACCOUNT.NAME':'LAST_NAME';String label=r.DeveloperName=='B6_Accounts'?'Account Name':'Last Name';System.assertEquals(label,e.getDetailColumnInfo().get(col).getLabel());System.assertEquals(Reports.ColumnDataType.STRING_DATA,e.getDetailColumnInfo().get(col).getDataType());System.assertEquals('Record Count',e.getAggregateColumnInfo().get('RowCount').getLabel());}`
			program, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReportNoColumnsMetadataNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default/reports")
			writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_NoGroup.report-meta.xml"), `<Report><name>B6 No Group</name><format>Summary</format><reportType>AccountList</reportType><scope>organization</scope></Report>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			code := `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_NoGroup' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();Map<String,Object> raw=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(m));System.assertEquals(null,raw.get('detailColumns'));`
			program, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReportCountValueTypeNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "test"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
				base := filepath.Join(root, "force-app/main/default/reports")
				writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
				writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
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
				if route == "test" {
					m.EnableTestContext()
					m.SetTestSeeAllData(true)
				}
				code := `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Object v=Reports.ReportManager.runReport(r.Id,false).getFactMap().get('T!T').getAggregates()[0].getValue();System.assertEquals(true,v instanceof Decimal);System.assertEquals(false,v instanceof Integer);Boolean rejected=false;try{Integer n=(Integer)v;}catch(System.TypeException e){rejected=true;}System.assertEquals(true,rejected);`
				prog, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = m.Execute(prog); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

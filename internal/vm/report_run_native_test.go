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

func TestProjectReportRunNative(t *testing.T) {
	capture, err := os.ReadFile("../resource/testdata/report_rows_capture/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	observed := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(capture)), "\n") {
		row := strings.SplitN(line, "\t", 4)
		if len(row) == 4 {
			observed[row[0]+"/"+row[1]+"/"+row[2]] = row[3]
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "test", "isolated"} {
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
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Account")
				names := []string{"Sample Account for Entitlements", "B6 First", "B6 Second"}
				for i := 0; i < 6; i++ {
					captureRoute, caseID := "anonymous", fmt.Sprintf("overload-%d", i)
					if route == "test" {
						captureRoute, caseID = "seealldata", "overloads"
					}
					if route == "isolated" {
						captureRoute, caseID = "isolated", "report-testsetup-visibility"
					}
					if observed[api+"/"+captureRoute+"/"+caseID] == "" {
						t.Fatalf("missing native row %s/%s/%s", api, captureRoute, caseID)
					}
				}
				accounts := org.Objects["Account"]
				for i, name := range names {
					id := storage.ID(fmt.Sprintf("001%012d", i+1))
					accounts.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue(name)}}
				}
				org.Objects["Account"] = accounts
				if err = resource.ApplyProject(&org, p); err != nil {
					t.Fatal(err)
				}
				var reportID storage.ID
				for id := range org.Objects["Report"].Records {
					reportID = id
				}
				if reportID == "" {
					t.Fatal("missing report")
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if route != "anonymous" {
					machine.EnableTestContext()
					machine.SetTestSeeAllData(route == "test")
				}
				var capturedCount int
				if _, err := fmt.Sscanf(observed[api+"/anonymous/overload-0"], "count=%d", &capturedCount); err != nil {
					t.Fatal(err)
				}
				source := fmt.Sprintf(`
insert new Account(Name='B6 Test Only');
Id reportId=Id.valueOf('%s');
Reports.ReportMetadata m=Reports.ReportManager.describeReport(reportId).getReportMetadata();
List<Reports.ReportResults> results=new List<Reports.ReportResults>{Reports.ReportManager.runReport(reportId),Reports.ReportManager.runReport(reportId,false),Reports.ReportManager.runReport(reportId,true),Reports.ReportManager.runReport(reportId,m),Reports.ReportManager.runReport(reportId,m,false),Reports.ReportManager.runReport(reportId,m,true),Reports.ReportManager.runReport(reportId,(Reports.ReportMetadata)null),Reports.ReportManager.runReport(reportId,(Reports.ReportMetadata)null,true)};
for(Integer i=0;i<results.size();i++) {
 System.assertEquals(%d,results[i].getFactMap().get('T!T').getAggregates()[0].getValue());
 System.assertEquals(i==2 || i==5 || i==7,results[i].getHasDetailRows());
}`, reportID, capturedCount)
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
}

func TestProjectSummaryReportRunNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "test"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
				base := filepath.Join(root, "force-app/main/default/reports")
				writeVMTestFile(t, filepath.Join(base, "B6_Other.reportFolder-meta.xml"), `<ReportFolder><name>Other B6</name></ReportFolder>`)
				writeVMTestFile(t, filepath.Join(base, "B6_Other/B6_Summary.report-meta.xml"), `<Report><name>B6 Summary</name><format>Summary</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT_NUMBER</field></columns><groupingsDown><field>ACCOUNT.NAME</field><sortOrder>Asc</sortOrder></groupingsDown></Report>`)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Account")
				accounts := org.Objects["Account"]
				for i, name := range []string{"B6 First", "B6 Second", "Sample Account for Entitlements"} {
					id := storage.ID(fmt.Sprintf("001%012d", i+1))
					accounts.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue(name)}}
				}
				org.Objects["Account"] = accounts
				if err = resource.ApplyProject(&org, p); err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "test" {
					machine.EnableTestContext()
					machine.SetTestSeeAllData(true)
				}
				code := `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Summary' LIMIT 1]; Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();System.assertEquals(Reports.ReportFormat.SUMMARY,m.getReportFormat());System.assertEquals('ACCOUNT_NUMBER',m.getDetailColumns()[0]); Reports.ReportResults a=Reports.ReportManager.runReport(r.Id,false);Reports.ReportResults b=Reports.ReportManager.runReport(r.Id,true);System.assertEquals(3,a.getFactMap().get('T!T').getAggregates()[0].getValue());System.assertEquals(3,b.getFactMap().get('T!T').getAggregates()[0].getValue());System.assertEquals(4,a.getFactMap().size());System.assertEquals(4,b.getFactMap().size());System.assertEquals(false,a.getHasDetailRows());System.assertEquals(true,b.getHasDetailRows());for(String key:b.getFactMap().keySet()){if(key!='T!T'){Reports.ReportFactWithDetails f=(Reports.ReportFactWithDetails)b.getFactMap().get(key);System.assertEquals(null,f.getRows()[0].getDataCells()[0].getValue());System.assertEquals('-',f.getRows()[0].getDataCells()[0].getLabel());}}`
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
}

func TestContactReportRunNative(t *testing.T) {
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
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			storage.EnsureStandardObject(&org, "Contact")
			accountID := storage.ID("001000000000001")
			accounts := org.Objects["Account"]
			accounts.Records[accountID] = storage.Record{ID: accountID, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(accountID), "Name": storage.StringValue("B6 First")}}
			org.Objects["Account"] = accounts
			contacts := org.Objects["Contact"]
			for i, name := range []string{"B6 Last One", "B6 Last Two", "B6 Linked One", "B6 Linked Two"} {
				id := storage.ID(fmt.Sprintf("003%012d", i+1))
				fields := map[string]storage.Value{"Id": storage.IDValue(id), "LastName": storage.StringValue(name)}
				if i >= 2 {
					fields["AccountId"] = storage.IDValue(accountID)
				}
				contacts.Records[id] = storage.Record{ID: id, Object: "Contact", Fields: fields}
			}
			org.Objects["Contact"] = contacts
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			code := `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Contacts' LIMIT 1];Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,true);Reports.ReportFactWithDetails f=(Reports.ReportFactWithDetails)x.getFactMap().get('T!T');System.assertEquals(2,f.getAggregates()[0].getValue());Set<String> labels=new Set<String>();for(Reports.ReportDetailRow row:f.getRows())labels.add(row.getDataCells()[0].getLabel());System.assertEquals(new Set<String>{'B6 Linked One','B6 Linked Two'},labels);Map<String,Id> ids=new Map<String,Id>();for(Contact c:[SELECT Id,LastName FROM Contact])ids.put(c.LastName,c.Id);for(Reports.ReportDetailRow row:f.getRows())System.assertEquals(ids.get(row.getDataCells()[0].getLabel()),row.getDataCells()[0].getValue());`
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

func TestProjectReportCommittedUpdatesNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default/reports")
			writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			accounts := org.Objects["Account"]
			for i, name := range []string{"B6 First", "B6 Second", "Sample Account for Entitlements"} {
				id := storage.ID(fmt.Sprintf("001%012d", i+1))
				accounts.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue(name)}}
			}
			org.Objects["Account"] = accounts
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			machine.SetTestSeeAllData(true)
			code := `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Account a=[SELECT Id,Name FROM Account WHERE Name='B6 First' LIMIT 1];Account b=[SELECT Id,Name FROM Account WHERE Name='B6 Second' LIMIT 1];a.Name='B6 Modified';update a;delete b;Reports.ReportFactWithDetails f=(Reports.ReportFactWithDetails)Reports.ReportManager.runReport(r.Id,true).getFactMap().get('T!T');System.assertEquals(3,f.getAggregates()[0].getValue());Set<String> labels=new Set<String>();for(Reports.ReportDetailRow row:f.getRows()) labels.add(row.getDataCells()[0].getLabel());System.assertEquals(new Set<String>{'B6 First','B6 Second','Sample Account for Entitlements'},labels);`
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

func TestReportWithoutColumnsNativeRejection(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default/reports")
			writeVMTestFile(t, filepath.Join(base, "B6_Other.reportFolder-meta.xml"), `<ReportFolder><name>Other B6</name></ReportFolder>`)
			writeVMTestFile(t, filepath.Join(base, "B6_Other/B6_Summary.report-meta.xml"), `<Report><name>B6 Summary</name><format>Summary</format><reportType>AccountList</reportType><scope>organization</scope><groupingsDown><field>ACCOUNT.NAME</field><sortOrder>Asc</sortOrder></groupingsDown></Report>`)
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
			code := `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Summary' LIMIT 1];Boolean caught=false;try{Reports.ReportManager.runReport(r.Id,true);}catch(Reports.MetadataException e){caught=true;System.assertEquals('Can\'t run the report because it doesn\'t have any columns selected. Be sure to add fields as columns to the report through the user interface.',e.getMessage());}System.assert(caught);`
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

func TestReportSavepointRoutesNative(t *testing.T) {
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
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Account")
				if err = resource.ApplyProject(&org, p); err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "test" {
					machine.EnableTestContext()
					machine.SetTestSeeAllData(true)
				}
				want := "true"
				if route == "test" {
					want = "false"
				}
				code := `Savepoint s=Database.setSavepoint();Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Reports.ReportManager.runReport(r.Id,false);Boolean caught=false;try{Database.rollback(s);}catch(System.CalloutException e){caught=true;System.assertEquals('Attempted to rollback in another transaction, probably after a callout.',e.getMessage());}System.assertEquals(` + want + `,caught);`
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
}

func TestSummaryPopulatedCellsNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default/reports")
			writeVMTestFile(t, filepath.Join(base, "B6_Other.reportFolder-meta.xml"), `<ReportFolder><name>Other B6</name></ReportFolder>`)
			writeVMTestFile(t, filepath.Join(base, "B6_Other/B6_Summary.report-meta.xml"), `<Report><name>B6 Summary</name><format>Summary</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT_NUMBER</field></columns><groupingsDown><field>ACCOUNT.NAME</field><sortOrder>Asc</sortOrder></groupingsDown></Report>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			accounts := org.Objects["Account"]
			for i, name := range []string{"B6 First", "B6 Second", "B6 Other Owner", "Sample Account for Entitlements"} {
				id := storage.ID(fmt.Sprintf("001%012d", i+1))
				fields := map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue(name)}
				if i < 2 {
					fields["AccountNumber"] = storage.StringValue(fmt.Sprintf("B6-%d00", i+1))
				}
				accounts.Records[id] = storage.Record{ID: id, Object: "Account", Fields: fields}
			}
			org.Objects["Account"] = accounts
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			code := `Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Summary' LIMIT 1];Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,true);Set<String> labels=new Set<String>();Integer nullCount=0;for(String key:x.getFactMap().keySet()){if(key!='T!T'){Reports.ReportFactWithDetails f=(Reports.ReportFactWithDetails)x.getFactMap().get(key);Reports.ReportDataCell c=f.getRows()[0].getDataCells()[0];labels.add(c.getLabel());if(c.getValue()==null)nullCount++;else System.assertEquals(c.getLabel(),c.getValue());}}System.assertEquals(new Set<String>{'B6-100','B6-200','-'},labels);System.assertEquals(2,nullCount);`
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

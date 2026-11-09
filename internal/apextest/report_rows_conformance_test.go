package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// TestReportRowsOrgConformance consumes independent native 62/67 anonymous,
// isolated, and SeeAllData observations. Metadata and classes are synthetic.
func TestReportRowsOrgConformance(t *testing.T) {
	body, err := os.ReadFile("../resource/testdata/report_rows_capture/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		row := strings.SplitN(line, "\t", 4)
		if len(row) != 4 {
			t.Fatalf("invalid capture row: %q", line)
		}
		rows = append(rows, row)
	}
	captured := map[string]string{}
	dashboards := map[string][]map[string]string{}
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[1] + "/" + row[2]
		if row[2] == "B6ROW-Dashboard" {
			fields := map[string]string{}
			for _, part := range strings.Split(row[3], "|") {
				k, v, ok := strings.Cut(part, "=")
				if ok {
					fields[k] = v
				}
			}
			dashboards[row[0]] = append(dashboards[row[0]], fields)
			continue
		}
		captured[key] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			if len(dashboards[api]) != 2 {
				t.Fatalf("dashboard captures for %s: %v", api, dashboards[api])
			}
			for _, route := range []string{"isolated", "seealldata"} {
				for _, id := range []string{"accounts", "dashboard", "folders"} {
					if captured[api+"/"+route+"/"+id] == "" {
						t.Fatalf("missing %s/%s/%s", api, route, id)
					}
				}
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			writeFile(t, filepath.Join(base, "reports/B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format></Report>`)
			for _, d := range dashboards[api] {
				folder := "B6_Dashboards"
				if d["DeveloperName"] == "B6_Other_Dashboard" {
					folder = "B6_Another"
				}
				writeFile(t, filepath.Join(base, "dashboards", folder+".dashboardFolder-meta.xml"), `<DashboardFolder><name>`+d["FolderName"]+`</name></DashboardFolder>`)
				writeFile(t, filepath.Join(base, "dashboards", folder, d["DeveloperName"]+".dashboard-meta.xml"), `<Dashboard><title>`+d["Title"]+`</title><dashboardType>`+d["Type"]+`</dashboardType></Dashboard>`)
			}
			checks := func(route string) string {
				reportWant, dashWant, folderWant := 0, 0, 2
				if route == "anonymous" {
					if _, err := fmt.Sscanf(captured[api+"/anonymous/apex-count"], "%d", &reportWant); err != nil {
						t.Fatal(err)
					}
					dashWant = len(dashboards[api])
				} else {
					for _, pair := range []struct {
						id  string
						dst *int
					}{{"accounts", &reportWant}, {"dashboard", &dashWant}, {"folders", &folderWant}} {
						if _, err := fmt.Sscanf(captured[api+"/"+route+"/"+pair.id], "count=%d", pair.dst); err != nil {
							t.Fatal(err)
						}
					}
				}
				return fmt.Sprintf(`System.assertEquals(%d,[SELECT COUNT() FROM Report WHERE DeveloperName='B6_Accounts']);System.assertEquals(%d,[SELECT COUNT() FROM Dashboard WHERE DeveloperName IN ('B6_Dashboard','B6_Other_Dashboard')]);System.assertEquals(%d,[SELECT COUNT() FROM Folder WHERE DeveloperName IN ('B6_Reports','B6_Dashboards')]);`, reportWant, dashWant, folderWant)
			}
			for _, route := range []string{"isolated", "seealldata"} {
				annotation := "@IsTest"
				if route == "seealldata" {
					annotation = "@IsTest(SeeAllData=true)"
				}
				name := "B6Rows" + route
				source := annotation + " private class " + name + " { @IsTest static void observed() {" + checks(route) + "} }"
				writeFile(t, filepath.Join(base, "classes", name+".cls"), source)
				writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			machine := vm.New(nil)
			machine.SetOrg(&org)
			program, err := vm.CompileAnonymousWithOptions(checks("anonymous"), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatalf("anonymous: %v", err)
			}
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 || summary.Errors != 0 {
				t.Fatalf("named: %+v; %v", summary, firstRunProblem(run))
			}
		})
	}
}

// TestReportSetupSnapshotNative checks the runner's pre-@TestSetup report view.
// The scratch capture is B6Setup in report_rows_capture/org.tsv.
func TestReportSetupSnapshotNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			writeFile(t, filepath.Join(base, "reports/B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			var id string
			for key := range org.Objects["Report"].Records {
				id = string(key)
			}
			if id == "" {
				t.Fatal("report not materialized")
			}
			count := 0
			for _, r := range org.Objects["Account"].Records {
				if !r.System.IsDeleted {
					count++
				}
			}
			for _, route := range []string{"isolated"} {
				annotation := "@IsTest"
				name := "B6Setup" + route
				source := fmt.Sprintf(`%s private class %s { @TestSetup static void setup(){insert new Account(Name='Test setup account');} @IsTest static void observed(){Reports.ReportResults x=Reports.ReportManager.runReport(Id.valueOf('%s'),false);System.assertEquals(%d,x.getFactMap().get('T!T').getAggregates()[0].getValue());}}`, annotation, name, id, count)
				writeFile(t, filepath.Join(base, "classes", name+".cls"), source)
				writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index = loadTestIndex(t, root)
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
				t.Fatalf("setup snapshot: %+v; %v", summary, firstRunProblem(run))
			}
		})
	}
}

// TestReportWarmMetadataNative ensures the same captured report metadata is
// used on a warm project route and refreshed after the metadata file changes.
func TestReportWarmMetadataNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name><folderShares><accessLevel>View</accessLevel><sharedTo>synthetic.report.user@example.invalid</sharedTo><sharedToType>User</sharedToType></folderShares></ReportFolder>`)
			path := filepath.Join(base, "reports/B6_Reports/B6_Accounts.report-meta.xml")
			report := func(name string) string {
				flag, start, description, totals := "false", "2026-11-02", "Alpha notes", "true"
				if name == "After" {
					flag, start, description, totals = "true", "2026-11-03", "Beta &amp; Gamma", "false"
				}
				return `<Report><name>` + name + `</name><description>` + description + `</description><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns><showDetails>` + flag + `</showDetails><showGrandTotal>` + totals + `</showGrandTotal><showSubTotals>` + totals + `</showSubTotals><timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>INTERVAL_CUSTOM</interval><startDate>` + start + `</startDate></timeFrameFilter></Report>`
			}
			writeFile(t, path, report("Before"))
			writeFile(t, filepath.Join(base, "classes/B6Warm.cls"), `@IsTest(SeeAllData=true) private class B6Warm { @IsTest static void observed(){Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();System.assert(false,m.getName()+':'+m.getHasDetailRows()+':'+m.getStandardDateFilter().getStartDate()+':'+m.getDescription()+':'+m.getShowGrandTotal()+':'+m.getShowSubtotals());} }`)
			writeFile(t, filepath.Join(base, "classes/B6Warm.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			catalogFixture := "../resource/testdata/report_rows_capture/type-catalog-three"
			for _, object := range []string{"Account", "Contact", "Opportunity"} {
				for _, field := range []string{"B6_Text__c", "B6_Date__c", "B6_Amount__c"} {
					path := filepath.Join("objects", object, "fields", field+".field-meta.xml")
					data, err := os.ReadFile(filepath.Join(catalogFixture, "custom", path))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(base, path), string(data))
				}
			}
			for _, name := range []string{"Contact", "Opportunity"} {
				data, err := os.ReadFile(filepath.Join(catalogFixture, "reports/B6_Reports/B6_"+name+".report-meta.xml"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(base, "reports/B6_Reports/B6_"+name+".report-meta.xml"), string(data))
			}
			permissions, err := os.ReadFile(filepath.Join(catalogFixture, "permissionsets/B6AllCatalog.permissionset-meta.xml"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(base, "permissionsets/B6CatalogProject.permissionset-meta.xml"), string(permissions))
			writeFile(t, filepath.Join(base, "classes/B6CatalogCustomProject.cls"), `@IsTest(SeeAllData=true) private class B6CatalogCustomProject { @IsTest static void observed(){PermissionSet p=[SELECT Id FROM PermissionSet WHERE Name='B6CatalogProject' LIMIT 1];insert new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(),PermissionSetId=p.Id);Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Reports.ReportTypeMetadata m=Reports.ReportManager.describeReport(r.Id).getReportTypeMetadata();System.assertNotEquals(null,m);System.assertEquals(4,m.getCategories().size());Reports.ReportTypeColumnCategory c=m.getCategories()[3];System.assertEquals('Account: Custom Info',c.getLabel());System.assertEquals(3,c.getColumns().size());System.assertEquals('B6 Text',c.getColumns().get('Account.B6_Text__c').getLabel());System.assertEquals('B6 Date',c.getColumns().get('Account.B6_Date__c').getLabel());System.assertEquals('B6 Amount',c.getColumns().get('Account.B6_Amount__c').getLabel());Integer seen=0;for(Report other:[SELECT Id,DeveloperName FROM Report WHERE DeveloperName IN ('B6_Contact','B6_Opportunity')]){seen++;Reports.ReportTypeMetadata x=Reports.ReportManager.describeReport(other.Id).getReportTypeMetadata();System.assertNotEquals(null,x);Integer n=other.DeveloperName=='B6_Contact'?8:9;String primary=other.DeveloperName=='B6_Contact'?'Contact: Custom Info':'Opportunity: Custom Info';System.assertEquals(n,x.getCategories().size());System.assertEquals(primary,x.getCategories()[n-2].getLabel());System.assertEquals('Account: Custom Info',x.getCategories()[n-1].getLabel());System.assertEquals(3,x.getCategories()[n-2].getColumns().size());System.assertEquals(3,x.getCategories()[n-1].getColumns().size());}System.assertEquals(2,seen);} }`)
			writeFile(t, filepath.Join(base, "classes/B6CatalogCustomProject.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeFile(t, filepath.Join(base, "classes/B6SecurityProject.cls"), `@IsTest(SeeAllData=true) private class B6SecurityProject { @IsTest static void observed(){Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];User u=new User(Alias='b6sec',Email='synthetic.report.user@example.invalid',EmailEncodingKey='UTF-8',LastName='Synthetic',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',Username='synthetic.report.user@example.invalid');insert u;System.runAs(u){Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];System.assertNotEquals(null,Reports.ReportManager.describeReport(r.Id));System.assertNotEquals(null,Reports.ReportManager.runReport(r.Id,false));}} }`)
			writeFile(t, filepath.Join(base, "classes/B6SecurityProject.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			InvalidateRuntimeCaches()
			opts := Options{Parallelism: 1, PerfCounters: true, SelectedClasses: []string{"B6Warm"}}
			verify := func(phase, want string, run testreport.Run) {
				t.Helper()
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("%s missing case: %+v", phase, run.Summary())
				}
				c := run.Suites[0].Cases[0]
				if c.Problem == nil || !strings.Contains(c.Problem.Message, want) {
					t.Fatalf("%s: %+v", phase, c)
				}
			}
			verify("cold", "Before:false:2026-11-02:Alpha notes:true:true", Run(index, opts))
			ResetPerfCounters()
			verify("warm", "Before:false:2026-11-02:Alpha notes:true:true", Run(index, opts))
			if SnapshotPerfCounters().Phases.MemoryCacheHits == 0 {
				t.Fatal("warm run did not hit runtime memory cache")
			}
			writeFile(t, path, report("After"))
			verify("changed", "After:true:2026-11-03:Beta & Gamma:false:false", Run(index, opts))
			security := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"B6SecurityProject"}})
			if summary := security.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
				t.Fatalf("runAs security route: %+v; %v", summary, firstRunProblem(security))
			}
			catalog := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"B6CatalogCustomProject"}})
			if summary := catalog.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
				t.Fatalf("custom catalog project route: %+v; %v", summary, firstRunProblem(catalog))
			}
		})
	}
}

func TestReportQueueableCommittedViewNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			writeFile(t, filepath.Join(base, "reports/B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
			seed := orgFromIndex(loadTestIndex(t, root))
			var reportID string
			for id := range seed.Objects["Report"].Records {
				reportID = string(id)
			}
			if reportID == "" {
				t.Fatal("report not materialized")
			}
			baseline := 0
			for _, record := range seed.Objects["Account"].Records {
				if !record.System.IsDeleted {
					baseline++
				}
			}
			code := `@IsTest(SeeAllData=true) private class B6QueueReport { private class Worker implements Queueable {Id rid;Worker(Id value){rid=value;}public void execute(QueueableContext ctx){Reports.ReportResults r=Reports.ReportManager.runReport(rid,false);Decimal n=(Decimal)r.getFactMap().get('T!T').getAggregates()[0].getValue();insert new Account(Name='B6 Async Count '+n);}} @IsTest static void observed(){Id rid=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1].Id;Integer beforeCount=[SELECT COUNT() FROM Account];insert new Account(Name='B6 Parent Only');Test.startTest();System.enqueueJob(new Worker(rid));Test.stopTest();Account a=[SELECT Name FROM Account WHERE Name LIKE 'B6 Async Count %' LIMIT 1];System.assertEquals('B6 Async Count '+beforeCount,a.Name);} }`
			isolated := strings.Replace(code, "@IsTest(SeeAllData=true) private class B6QueueReport", "@IsTest private class B6QueueIsolated", 1)
			isolated = strings.Replace(isolated, "Id rid=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1].Id;", "Id rid=Id.valueOf('"+reportID+"');", 1)
			isolated = strings.Replace(isolated, "'B6 Async Count '+beforeCount,a.Name", fmt.Sprintf("'B6 Async Count '+%d,a.Name", baseline), 1)
			writeFile(t, filepath.Join(base, "classes/B6QueueIsolated.cls"), isolated)
			writeFile(t, filepath.Join(base, "classes/B6QueueIsolated.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeFile(t, filepath.Join(base, "classes/B6QueueReport.cls"), code)
			writeFile(t, filepath.Join(base, "classes/B6QueueReport.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 || summary.Errors != 0 {
				t.Fatalf("queued report: %+v; %v", summary, firstRunProblem(run))
			}
		})
	}
}

// TestContactReportWarmDateNative checks Contact report date metadata through
// the runner's cold, warm, and refreshed project paths.
func TestContactReportWarmDateNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, reportType := range []string{"ContactList", "Opportunity"} {
			t.Run(api+"/"+reportType, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				base := filepath.Join(root, "force-app/main/default")
				writeFile(t, filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name><folderShares><accessLevel>View</accessLevel><sharedTo>synthetic.report.user@example.invalid</sharedTo><sharedToType>User</sharedToType></folderShares></ReportFolder>`)
				path := filepath.Join(base, "reports/B6_Reports/B6_Contacts.report-meta.xml")
				cold, changed, end := "2026-11-02", "2026-11-03", ""
				if reportType == "Opportunity" {
					cold, changed, end = "2000-01-01", "2099-01-01", "<endDate>2100-01-01</endDate>"
				}
				report := func(start string) string {
					column := "LAST_NAME"
					if reportType == "Opportunity" {
						column = "OPPORTUNITY_NAME"
					}
					return `<Report><name>B6 Contacts</name><format>Tabular</format><reportType>` + reportType + `</reportType><scope>organization</scope><columns><field>` + column + `</field></columns><timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>INTERVAL_CUSTOM</interval><startDate>` + start + `</startDate>` + end + `</timeFrameFilter></Report>`
				}
				writeFile(t, path, report(cold))
				dtoDir := "../resource/testdata/report_rows_capture/type-dto"
				dtoSource, err := os.ReadFile(filepath.Join(dtoDir, "dto.apex"))
				if err != nil {
					t.Fatal(err)
				}
				rows, err := os.ReadFile(filepath.Join(dtoDir, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				want := ""
				for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n") {
					parts := strings.SplitN(line, "\t", 4)
					if len(parts) == 4 && parts[0] == api && parts[1] == "istest" {
						want += strings.TrimPrefix(parts[2], "dto-") + "=" + parts[3] + ";"
					}
				}
				if want == "" {
					t.Fatal("missing native DTO rows")
				}
				want = strings.ReplaceAll(strings.ReplaceAll(want, `\`, `\\`), `'`, `\'`)
				dtoCode := strings.TrimSuffix(string(dtoSource), "System.debug('B6DTO|'+out);")
				if dtoCode == string(dtoSource) {
					t.Fatal("missing captured DTO terminator")
				}
				catalogName := "contact"
				if reportType == "Opportunity" {
					catalogName = "opportunity"
				}
				catalogDir := "../resource/testdata/report_rows_capture/type-catalog-roundtrip"
				version := strings.TrimSuffix(api, ".0")
				before, err := os.ReadFile(filepath.Join(catalogDir, "before-"+catalogName+"-"+version+".json"))
				if err != nil {
					t.Fatal(err)
				}
				after, err := os.ReadFile(filepath.Join(catalogDir, "after-"+catalogName+"-"+version+".json"))
				if err != nil {
					t.Fatal(err)
				}
				apexLiteral := func(raw []byte) string {
					return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(string(raw)), `\`, `\\`), `'`, `\'`)
				}
				roundCode := `Reports.ReportTypeMetadata round=(Reports.ReportTypeMetadata)JSON.deserialize('` + apexLiteral(before) + `',Reports.ReportTypeMetadata.class);for(Reports.ReportTypeColumnCategory c:round.getCategories()){for(Reports.ReportTypeColumn f:c.getColumns().values()){if(f.getFilterValues()!=null && !f.getFilterValues().isEmpty()){} }}System.assertEquals(JSON.deserializeUntyped('` + apexLiteral(after) + `'),JSON.deserializeUntyped(JSON.serialize(round)));`
				emptyDir := "../resource/testdata/report_rows_capture/type-dto-empty"
				emptySource, err := os.ReadFile(filepath.Join(emptyDir, "empty.apex"))
				if err != nil {
					t.Fatal(err)
				}
				emptyRows, err := os.ReadFile(filepath.Join(emptyDir, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				emptyWant := ""
				for _, line := range strings.Split(strings.TrimSpace(string(emptyRows)), "\n") {
					parts := strings.SplitN(line, "\t", 4)
					if len(parts) == 4 && parts[0] == api && parts[1] == "istest" {
						emptyWant += strings.TrimPrefix(parts[2], "dto-empty-") + "=" + parts[3] + ";"
					}
				}
				if emptyWant == "" {
					t.Fatal("missing native empty DTO rows")
				}
				emptyCode := strings.TrimSuffix(string(emptySource), "System.debug('B6EMPTY|'+out);")
				if emptyCode == string(emptySource) {
					t.Fatal("missing captured empty DTO terminator")
				}
				emptyCode += `System.assertEquals('` + apexLiteral([]byte(emptyWant)) + `',out);`
				getterDir := "../resource/testdata/report_rows_capture/type-dto-getters"
				getterSource, err := os.ReadFile(filepath.Join(getterDir, "getters.apex"))
				if err != nil {
					t.Fatal(err)
				}
				getterRows, err := os.ReadFile(filepath.Join(getterDir, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				getterWant := ""
				for _, line := range strings.Split(strings.TrimSpace(string(getterRows)), "\n") {
					parts := strings.SplitN(line, "\t", 4)
					if len(parts) == 4 && parts[0] == api && parts[1] == "istest" {
						getterWant += strings.TrimPrefix(parts[2], "dto-getter-") + "=" + parts[3] + ";"
					}
				}
				if getterWant == "" {
					t.Fatal("missing native DTO getter rows")
				}
				getterCode := strings.TrimSuffix(string(getterSource), "System.debug('B6GETTERS|'+out);")
				if getterCode == string(getterSource) {
					t.Fatal("missing captured getter terminator")
				}
				getterCode += `System.assertEquals('` + apexLiteral([]byte(getterWant)) + `',out);`
				writeFile(t, filepath.Join(base, "classes/B6WarmContact.cls"), `@IsTest(SeeAllData=true) private class B6WarmContact { @IsTest static void observed(){Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Contacts' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();System.assert(false,m.getStandardDateFilter().getStartDate());} }`)
				writeFile(t, filepath.Join(base, "classes/B6WarmContact.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				liveCount, liveLabel, liveScope, liveScopes := "6", "Contact: General", "my", "5"
				if reportType == "Opportunity" {
					liveCount, liveLabel, liveScope, liveScopes = "7", "Opportunity Information", "organization", "4"
				}
				liveCode := `Report live=[SELECT Id FROM Report WHERE DeveloperName='B6_Contacts' LIMIT 1];Reports.ReportTypeMetadata lm=Reports.ReportManager.describeReport(live.Id).getReportTypeMetadata();System.assertNotEquals(null,lm);System.assertEquals(` + liveCount + `,lm.getCategories().size());System.assertEquals('` + liveLabel + `',lm.getCategories()[0].getLabel());System.assertEquals('` + liveScope + `',lm.getScopeInfo().getDefaultValue());System.assertEquals(` + liveScopes + `,lm.getScopeInfo().getValues().size());`
				if reportType == "Opportunity" {
					liveCode += `Object f=lm.getStandardFilterInfos().get('open');System.assertEquals(true,f instanceof Reports.StandardFilterInfoPicklist);Reports.StandardFilterInfoPicklist q=(Reports.StandardFilterInfoPicklist)f;System.assertEquals('all',q.getDefaultValue());System.assertEquals(4,q.getFilterValues().size());`
				}
				standardCode := `Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];User u=new User(Alias='b6std',Email='synthetic.report.user@example.invalid',EmailEncodingKey='UTF-8',LastName='Synthetic',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',Username='synthetic.report.user@example.invalid');insert u;System.runAs(u){Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Contacts' LIMIT 1];Reports.ReportTypeMetadata m=Reports.ReportManager.describeReport(r.Id).getReportTypeMetadata();System.assertNotEquals(null,m);System.assertEquals(` + liveCount + `,m.getCategories().size());System.assertEquals('` + liveLabel + `',m.getCategories()[0].getLabel());System.assertEquals('` + liveScope + `',m.getScopeInfo().getDefaultValue());System.assertEquals(` + liveScopes + `,m.getScopeInfo().getValues().size());}`
				writeFile(t, filepath.Join(base, "classes/B6StandardCatalogProject.cls"), `@IsTest(SeeAllData=true) private class B6StandardCatalogProject { @IsTest static void observed(){`+standardCode+`} }`)
				writeFile(t, filepath.Join(base, "classes/B6StandardCatalogProject.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				writeFile(t, filepath.Join(base, "classes/B6DTOProject.cls"), `@IsTest(SeeAllData=true) private class B6DTOProject { @IsTest static void observed(){{`+dtoCode+`System.assertEquals('`+want+`',out);}{`+emptyCode+`}{`+getterCode+`}`+roundCode+liveCode+`} }`)
				writeFile(t, filepath.Join(base, "classes/B6DTOProject.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				writeFile(t, filepath.Join(base, "classes/B6DTOIsolated.cls"), `@IsTest private class B6DTOIsolated { @IsTest static void observed(){{`+dtoCode+`System.assertEquals('`+want+`',out);}{`+emptyCode+`}{`+getterCode+`}} }`)
				writeFile(t, filepath.Join(base, "classes/B6DTOIsolated.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				index := loadTestIndex(t, root)
				InvalidateRuntimeCaches()
				opts := Options{Parallelism: 1, PerfCounters: true, SelectedClasses: []string{"B6WarmContact"}}
				verify := func(phase, want string, run testreport.Run) {
					t.Helper()
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("%s missing case: %+v", phase, run.Summary())
					}
					c := run.Suites[0].Cases[0]
					if c.Problem == nil || !strings.Contains(c.Problem.Message, want) {
						t.Fatalf("%s: %+v", phase, c)
					}
				}
				verify("cold", cold, Run(index, opts))
				ResetPerfCounters()
				verify("warm", cold, Run(index, opts))
				if SnapshotPerfCounters().Phases.MemoryCacheHits == 0 {
					t.Fatal("warm run did not hit runtime memory cache")
				}
				writeFile(t, path, report(changed))
				verify("changed", changed, Run(index, opts))
				projectRuns := 1
				if reportType == "Opportunity" {
					projectRuns = 2
				}
				for phase := 0; phase < projectRuns; phase++ {
					run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"B6DTOProject"}})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
						t.Fatalf("DTO project phase %d: %+v; %v", phase, summary, firstRunProblem(run))
					}
				}
				standard := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"B6StandardCatalogProject"}})
				if summary := standard.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
					t.Fatalf("standard user catalog route: %+v; %v", summary, firstRunProblem(standard))
				}
				if reportType == "Opportunity" {
					run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"B6DTOIsolated"}})
					if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
						t.Fatalf("isolated DTO route: %+v; %v", summary, firstRunProblem(run))
					}
				}
			})
		}
	}
}

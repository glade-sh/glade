package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

func TestReportDashboardVisibilityNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			writeFile(t, filepath.Join(base, "reports/B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format></Report>`)
			writeFile(t, filepath.Join(base, "dashboards/B6_Dashboards.dashboardFolder-meta.xml"), `<DashboardFolder><name>B6 Dashboards</name></DashboardFolder>`)
			writeFile(t, filepath.Join(base, "dashboards/B6_Dashboards/B6_Dashboard.dashboard-meta.xml"), `<Dashboard><title>B6 Dashboard</title><dashboardType>LoggedInUser</dashboardType></Dashboard>`)
			classes := map[string]string{
				"B6Isolated": `@IsTest private class B6Isolated { @IsTest static void check() { System.assertEquals(0,[SELECT COUNT() FROM Report WHERE DeveloperName='B6_Accounts']); System.assertEquals(0,[SELECT COUNT() FROM Dashboard WHERE DeveloperName='B6_Dashboard']); System.assertEquals(2,[SELECT COUNT() FROM Folder WHERE DeveloperName IN ('B6_Reports','B6_Dashboards')]); } }`,
				"B6SeeAll":   `@IsTest(SeeAllData=true) private class B6SeeAll { @IsTest static void check() { System.assertEquals(1,[SELECT COUNT() FROM Report WHERE DeveloperName='B6_Accounts']); System.assertEquals(1,[SELECT COUNT() FROM Dashboard WHERE DeveloperName='B6_Dashboard']); System.assertEquals(2,[SELECT COUNT() FROM Folder WHERE DeveloperName IN ('B6_Reports','B6_Dashboards')]); } }`,
			}
			for name, source := range classes {
				writeFile(t, filepath.Join(base, "classes", name+".cls"), source)
				writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 || summary.Errors != 0 {
				data, _ := json.Marshal(run)
				t.Fatalf("visibility: %s", data)
			}
		})
	}
}

func TestDashboardIsolationNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "dashboards/B6_Dashboards.dashboardFolder-meta.xml"), `<DashboardFolder><name>B6 Dashboards</name></DashboardFolder>`)
			writeFile(t, filepath.Join(base, "dashboards/B6_Dashboards/B6_Dashboard.dashboard-meta.xml"), `<Dashboard><title>B6 Dashboard</title><dashboardType>LoggedInUser</dashboardType></Dashboard>`)
			writeFile(t, filepath.Join(base, "classes/B6DashOnly.cls"), `@IsTest private class B6DashOnly { @IsTest static void check() { System.assertEquals(0,[SELECT COUNT() FROM Dashboard WHERE DeveloperName='B6_Dashboard']); } }`)
			writeFile(t, filepath.Join(base, "classes/B6DashOnly.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
				data, _ := json.Marshal(run)
				t.Fatalf("dashboard visibility: %s", data)
			}
		})
	}
}

func TestReportDashboardLocatorVisibilityNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, object := range []string{"Report", "Dashboard"} {
			t.Run(api+"/"+object, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				base := filepath.Join(root, "force-app/main/default")
				writeFile(t, filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
				writeFile(t, filepath.Join(base, "reports/B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format></Report>`)
				writeFile(t, filepath.Join(base, "dashboards/B6_Dashboards.dashboardFolder-meta.xml"), `<DashboardFolder><name>B6 Dashboards</name></DashboardFolder>`)
				writeFile(t, filepath.Join(base, "dashboards/B6_Dashboards/B6_Dashboard.dashboard-meta.xml"), `<Dashboard><title>B6 Dashboard</title><dashboardType>LoggedInUser</dashboardType></Dashboard>`)
				name := "B6_Accounts"
				if object == "Dashboard" {
					name = "B6_Dashboard"
				}
				for _, route := range []string{"isolated", "all"} {
					ann := "@IsTest"
					want := 0
					if route == "all" {
						ann = "@IsTest(SeeAllData=true)"
						want = 1
					}
					class := "B6Locator" + route
					code := fmt.Sprintf(`%s private class %s { @IsTest static void check() { String q='SELECT Id FROM %s WHERE DeveloperName=\'%s\'';System.assertEquals(%d,Database.query(q).size());Database.QueryLocatorIterator it=Database.getQueryLocator(q).iterator();Integer n=0;while(it.hasNext()){it.next();n++;}System.assertEquals(%d,n); } }`, ann, class, object, name, want, want)
					writeFile(t, filepath.Join(base, "classes", class+".cls"), code)
					writeFile(t, filepath.Join(base, "classes", class+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				}
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 || summary.Errors != 0 {
					data, _ := json.Marshal(run)
					t.Fatalf("query routes: %s", data)
				}
			})
		}
	}
}

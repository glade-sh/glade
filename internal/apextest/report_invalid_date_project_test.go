package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Supplemental project-runner route for the captured standard Account catalog.
func TestReportInvalidDateColumnProject(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			writeFile(t, filepath.Join(base, "reports/B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
			writeFile(t, filepath.Join(base, "classes/B6InvalidDate.cls"), `@IsTest(SeeAllData=true) private class B6InvalidDate { @IsTest static void observed(){Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();m.setStandardDateFilter(new Reports.StandardDateFilter('NOT_A_DATE','CUSTOM','2000-01-01','2100-01-01'));Boolean caught=false;try{Reports.ReportManager.runReport(r.Id,m,false);}catch(Exception e){caught=true;System.assertEquals('reports.InvalidFilterException',e.getTypeName());System.assertEquals('[The column NOT_A_DATE specified for the standard date filter is invalid.]',e.getMessage());}System.assert(caught);} }`)
			writeFile(t, filepath.Join(base, "classes/B6InvalidDate.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			InvalidateRuntimeCaches()
			for _, phase := range []string{"cold", "warm"} {
				run := Run(index, Options{Parallelism: 1, SelectedClasses: []string{"B6InvalidDate"}})
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || summary.Errors != 0 {
					t.Fatalf("%s: %+v; %v", phase, summary, firstRunProblem(run))
				}
			}
		})
	}
}

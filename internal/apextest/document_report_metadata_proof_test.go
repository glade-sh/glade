package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF211 admits the two API62/project40 Document and Report row queries.
func TestDocumentAndReportMetadataAPI62Project40Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"40.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeMetadataRows62Access.cls"), `@IsTest private class GladeMetadataRows62Access {
 @IsTest(SeeAllData=true) static void deployedDocumentSupportsOriginalDownloadLookup() {
  List<Document> docs=[SELECT Name, Id FROM Document WHERE Name='GladePersonalSite62Access.css' LIMIT 1];
  System.assertEquals(1,docs.size());
  System.assertNotEquals(null,docs[0].Id);
  String imageid=docs[0].Id;
  String url='/servlet/servlet.FileDownload?file='+imageid.substring(0,15);
  System.assertEquals(15,url.substringAfter('file=').length());
  System.assertEquals(0,[SELECT COUNT() FROM Document WHERE Name='GladeMissingPersonalSite62.css']);
 }
 @IsTest(SeeAllData=true) static void deployedReportSupportsOriginalDeveloperNameLookup() {
  List<Report> reports=[SELECT Id FROM Report WHERE DeveloperName='GladeUpcoming62Access'];
  System.assertEquals(1,reports.size());
  System.assertNotEquals(null,reports[0].Id);
  PageReference redirect=new PageReference('/'+reports[0].Id);
  redirect.setRedirect(true);
  System.assertNotEquals(null,redirect);
  System.assertEquals(0,[SELECT COUNT() FROM Report WHERE DeveloperName='GladeMissingUpcoming62']);
 }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeMetadataRows62Access.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/GladeDocs62Access.documentFolder-meta.xml"), `<DocumentFolder xmlns="http://soap.sforce.com/2006/04/metadata"><accessType>Public</accessType><name>Glade Docs 62</name><publicFolderAccess>ReadOnly</publicFolderAccess></DocumentFolder>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/GladeDocs62Access/GladePersonalSite62Access.css"), "body { color: black; }\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/GladeDocs62Access/GladePersonalSite62Access.css-meta.xml"), `<Document xmlns="http://soap.sforce.com/2006/04/metadata"><internalUseOnly>false</internalUseOnly><name>GladePersonalSite62Access.css</name><public>true</public></Document>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/reports/GladeReports62Access.reportFolder-meta.xml"), `<ReportFolder xmlns="http://soap.sforce.com/2006/04/metadata"><accessType>Public</accessType><name>Glade Reports 62</name><publicFolderAccess>ReadWrite</publicFolderAccess></ReportFolder>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/reports/GladeReports62Access/GladeUpcoming62Access.report-meta.xml"), `<Report xmlns="http://soap.sforce.com/2006/04/metadata"><columns><field>LAST_NAME</field></columns><format>Tabular</format><name>Glade Upcoming 62</name><reportType>ContactList</reportType><scope>organization</scope><showDetails>true</showDetails><timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>INTERVAL_CUSTOM</interval></timeFrameFilter></Report>`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("SF211 document/report metadata: %s", data)
	}
}

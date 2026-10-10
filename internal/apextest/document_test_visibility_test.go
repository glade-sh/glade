package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// The conformance matrix supplies native Document visibility expectations.
// These local preservation cases exercise setup-created data and both runner
// isolation paths around the same captured baseline.
func TestDocumentTestVisibilityPreservesSetupAndCloneState(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	classes := map[string]string{
		"DocumentOrdinaryTest": `@IsTest private class DocumentOrdinaryTest {
 static Integer initialCount=[SELECT COUNT() FROM Document WHERE DeveloperName='OwnedSourceDocument'];
 @IsTest static void ordinary() {
  System.assertEquals(0,initialCount);
  System.assertEquals(0,[SELECT COUNT() FROM Document WHERE DeveloperName='OwnedSourceDocument']);
  System.assertEquals(1,[SELECT COUNT() FROM ApexPage WHERE Name='OwnedVisiblePage']);
  Document created=new Document(Name='OwnedMethod.txt',DeveloperName='OwnedMethod',Body=Blob.valueOf('method'),FolderId=UserInfo.getUserId());
  insert created;
  System.assertEquals(1,[SELECT COUNT() FROM Document WHERE Id=:created.Id]);
  created.Name='OwnedUpdated.txt'; update created;
  System.assertEquals('OwnedUpdated.txt',[SELECT Name FROM Document WHERE Id=:created.Id].Name);
  delete created;
  System.assertEquals(0,[SELECT COUNT() FROM Document WHERE Id=:created.Id]);
 }
}`,
		"DocumentClassDataTest": `@IsTest(SeeAllData=true) private class DocumentClassDataTest {
 static Integer initialCount=[SELECT COUNT() FROM Document WHERE DeveloperName='OwnedSourceDocument'];
 @IsTest static void classData() {
  System.assertEquals(1,initialCount);
  System.assertEquals(1,[SELECT COUNT() FROM Document WHERE DeveloperName='OwnedSourceDocument']);
 }
}`,
		"DocumentMethodDataTest": `@IsTest private class DocumentMethodDataTest {
 @IsTest(SeeAllData=true) static void methodData() {
  System.assertEquals(1,[SELECT COUNT() FROM Document WHERE DeveloperName='OwnedSourceDocument']);
 }
}`,
		"DocumentSetupDataTest": `@IsTest private class DocumentSetupDataTest {
 @TestSetup static void seed() {
  System.assertEquals(0,[SELECT COUNT() FROM Document WHERE DeveloperName='OwnedSourceDocument']);
  insert new Document(Name='OwnedSetup.txt',DeveloperName='OwnedSetup',Body=Blob.valueOf('setup'),FolderId=UserInfo.getUserId());
 }
 @IsTest static void readsSetup() {
  System.assertEquals(0,[SELECT COUNT() FROM Document WHERE DeveloperName='OwnedSourceDocument']);
  System.assertEquals('OwnedSetup.txt',[SELECT Name FROM Document WHERE DeveloperName='OwnedSetup'].Name);
 }
 @IsTest static void changesSetup() {
  Document seeded=[SELECT Id,Name FROM Document WHERE DeveloperName='OwnedSetup'];
  seeded.Name='OwnedSetupChanged.txt'; update seeded;
  System.assertEquals('OwnedSetupChanged.txt',[SELECT Name FROM Document WHERE Id=:seeded.Id].Name);
  System.assertEquals(0,[SELECT COUNT() FROM Document WHERE DeveloperName='OwnedSourceDocument']);
 }
}`,
	}
	for name, source := range classes {
		writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), source)
	}
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedFolder.documentFolder-meta.xml"), `<DocumentFolder xmlns="http://soap.sforce.com/2006/04/metadata"><accessType>Public</accessType><name>Owned Folder</name><publicFolderAccess>ReadOnly</publicFolderAccess></DocumentFolder>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedFolder/OwnedSourceDocument.txt"), "owned document body")
	writeFile(t, filepath.Join(root, "force-app/main/default/documents/OwnedFolder/OwnedSourceDocument.txt-meta.xml"), `<Document xmlns="http://soap.sforce.com/2006/04/metadata"><name>OwnedSourceDocument.txt</name><public>true</public></Document>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/OwnedVisiblePage.page"), "<apex:page>Owned</apex:page>")
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/OwnedVisiblePage.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><label>Owned Visible Page</label></ApexPage>`)
	index := loadTestIndex(t, root)
	for _, testCase := range Discover(index, Options{}) {
		want := testCase.ClassName == "DocumentClassDataTest" || testCase.ClassName == "DocumentMethodDataTest"
		if testCase.SeeAllData != want {
			t.Fatalf("%s.%s SeeAllData=%t, want %t", testCase.ClassName, testCase.MethodName, testCase.SeeAllData, want)
		}
	}
	for _, row := range []struct {
		name     string
		selected []string
		total    int
	}{
		{"no setup journal", []string{"DocumentOrdinaryTest", "DocumentClassDataTest", "DocumentMethodDataTest"}, 3},
		{"setup journal and method clones", nil, 5},
	} {
		t.Run(row.name, func(t *testing.T) {
			run := Run(index, Options{SelectedClasses: row.selected, NoDiskCache: true})
			if summary := run.Summary(); summary.Total != row.total || summary.Passed != row.total || summary.Errors != 0 {
				data, _ := json.Marshal(run)
				t.Fatalf("Document test visibility: %s", data)
			}
		})
	}
}

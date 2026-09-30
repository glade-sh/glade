package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact API62/project40 fixture admitted by SF196.
func TestLowercaseCookieAPI62Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "40.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeVolunteersCookie62Proof.cls"), `@IsTest private class GladeVolunteersCookie62Proof {
 @IsTest static void originalLowercaseCookieCachesContactId() {
  Contact owned=new Contact(LastName='Owned cookie'); insert owned;
  Id contactId=owned.Id;
  Test.setCurrentPage(new PageReference('/apex/GladeOwnedCookie'));
  Cookie cId = new cookie('contactIdPersonalSite', contactId, null, -1, false);
  System.assertEquals(String.valueOf(contactId),cId.getValue());
  System.assertEquals(-1,cId.getMaxAge());
  System.assertEquals(false,cId.isSecure());
  ApexPages.currentPage().setCookies(new Cookie[] {cId});
  Cookie cached=ApexPages.currentPage().getCookies().get('contactIdPersonalSite');
  System.assertNotEquals(null,cached);
  System.assertEquals(String.valueOf(contactId),cached.getValue());
 }
 @IsTest static void capitalizedCookieIdControl() {
  Contact owned=new Contact(LastName='Owned control'); insert owned;
  Id contactId=owned.Id;
  Cookie control=new Cookie('contactIdPersonalSite',contactId,null,-1,false);
  System.assertEquals(String.valueOf(contactId),control.getValue());
  System.assertEquals(-1,control.getMaxAge());
  System.assertEquals(false,control.isSecure());
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeVolunteersCookie62Proof.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Passed != 2 || got.Total != 2 {
		data, _ := json.Marshal(run)
		t.Fatalf("cookie proof: %s", data)
	}
}

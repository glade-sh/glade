package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Native rows: scratch org, API 62.0. Deploying retrieved source that declares
// the standard ContactPointAddress.ParentId field as MasterDetail leaves the
// field a nillable Account/Individual reference with no required-field error.
func TestStandardFieldDeclaredMasterDetailStaysOptional(t *testing.T) {
	root := t.TempDir()
	const base = "force-app/main/default/"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, base, "objects/ContactPointAddress/fields/ParentId.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>ParentId</fullName>
    <trackHistory>false</trackHistory>
    <type>MasterDetail</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, base, "classes/CpaProbe.cls"), `@isTest
public class CpaProbe {
  @isTest static void run() {
    String o='';
    Schema.DescribeFieldResult d = ContactPointAddress.ParentId.getDescribe();
    o += '\nDESC|type='+d.getType()+' nillable='+d.isNillable()+' cr='+d.isCreateable()+' up='+d.isUpdateable()+' defaulted='+d.isDefaultedOnCreate()+' refs='+d.getReferenceTo();
    try { ContactPointAddress c = new ContactPointAddress(Name='X'); insert c; o+='\nINS|ok'; } catch(Exception e){ o+='\nINS|'+e.getMessage(); }
    try { ContactPointAddress c = new ContactPointAddress(Name='Y'); Database.SaveResult r = Database.insert(c,false); o+='\nDB|'+r.isSuccess(); } catch(Exception e){ o+='\nDB|'+e.getMessage(); }
    System.assert(false, o);
  }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/CpaProbe.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>\n")
	run := Run(loadTestIndex(t, root), Options{})
	var got string
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				got = strings.TrimPrefix(c.Problem.Message, "Assertion Failed: ")
			}
		}
	}
	const want = "\nDESC|type=REFERENCE nillable=true cr=true up=true defaulted=false refs=(Account, Individual)\nINS|ok\nDB|true"
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("native ContactPointAddress rows differ\ngot:\n%s\nwant:\n%s", got, want)
	}
}

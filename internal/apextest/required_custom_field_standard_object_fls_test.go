package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Native rows: scratch org, API 62.0. A universally required custom field on User stays accessible under a profile that lists another User field; an optional one without a row does not.
func TestRequiredCustomFieldOnStandardObjectStaysAccessible(t *testing.T) {
	root := t.TempDir()
	const base = "force-app/main/default/"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, base, "objects/User/fields/ReqU__c.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>ReqU__c</fullName><defaultValue>&quot;INSS&quot;</defaultValue><label>ReqU</label><type>Picklist</type><required>true</required>
    <valueSet><restricted>true</restricted><valueSetDefinition><sorted>false</sorted><value><fullName>INSS</fullName><default>true</default><label>INSS</label></value><value><fullName>TGSS</fullName><default>false</default><label>TGSS</label></value></valueSetDefinition></valueSet>
</CustomField>
`)
	writeFile(t, filepath.Join(root, base, "objects/User/fields/OptU__c.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>OptU__c</fullName><label>OptU</label><type>Text</type><length>40</length><required>false</required>
</CustomField>
`)
	writeFile(t, filepath.Join(root, base, "objects/User/fields/OptV__c.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>OptV__c</fullName><label>OptV</label><type>Text</type><length>40</length><required>false</required>
</CustomField>
`)
	writeFile(t, filepath.Join(root, base, "profiles/Probe Gene\u0301rico.profile-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<Profile xmlns="http://soap.sforce.com/2006/04/metadata">
    <custom>true</custom>
    <userLicense>Salesforce</userLicense>
    <fieldPermissions><editable>true</editable><field>Probe__c.Other__c</field><readable>true</readable></fieldPermissions>
    <fieldPermissions><editable>true</editable><field>User.OptU__c</field><readable>true</readable></fieldPermissions><objectPermissions><allowCreate>true</allowCreate><allowDelete>true</allowDelete><allowEdit>true</allowEdit><allowRead>true</allowRead><modifyAllRecords>false</modifyAllRecords><object>Probe__c</object><viewAllRecords>false</viewAllRecords></objectPermissions>
    <userPermissions><enabled>true</enabled><name>ApiEnabled</name></userPermissions>
</Profile>
`)
	writeFile(t, filepath.Join(root, base, "classes/UserReqProbe.cls"), `@isTest
public class UserReqProbe {
  static String o='';
  static void probe(String prof) {
    Profile p=[SELECT Id FROM Profile WHERE Name=:prof];
    User u=new User(Alias='ufl',Email='ufl@example.com',EmailEncodingKey='UTF-8',LastName='Ufl',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',Username='ufl'+prof.length()+System.currentTimeMillis()+'@example.com');
    insert u;
    System.runAs(u){
      Map<String,Schema.SObjectField> fm = Schema.SObjectType.User.fields.getMap();
      for (String f : new List<String>{'ReqU__c','OptU__c','OptV__c'}) {
        String d = fm.containsKey(f) ? 'acc='+fm.get(f).getDescribe().isAccessible() : 'absent';
        String q;
        try { Database.query('SELECT Id, '+f+' FROM User WHERE Id = \''+u.Id+'\' WITH SECURITY_ENFORCED'); q='ok'; } catch(Exception e){ q='EXC'; }
        o += '\nUF|'+prof+'|'+f+'|'+d+'|'+q;
      }
    }
  }
  @isTest static void run() {
    probe('Probe Genérico');
    System.assert(false, o);
  }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/UserReqProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{})
	var got string
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				got = strings.TrimPrefix(c.Problem.Message, "Assertion Failed: ")
			}
		}
	}
	want := `UF|Probe Genérico|ReqU__c|acc=true|ok
UF|Probe Genérico|OptU__c|acc=true|ok
UF|Probe Genérico|OptV__c|acc=false|EXC`
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("native rows differ\ngot:\n%s\nwant:\n%s", got, want)
	}
}

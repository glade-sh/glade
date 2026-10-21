package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Native rows: scratch org, API 62.0 class, custom profiles that list another
// field of the object, and an object with a tab. Name (Text), OwnerId and the
// system fields (including LastViewedDate and LastReferencedDate) are not
// permissionable, so the profiles keep them accessible; Name and OwnerId are
// createable and updateable with object CRUD.
func TestProfileFieldPermissionsKeepNonPermissionableFieldsOnCustomObject(t *testing.T) {
	root := t.TempDir()
	const base = "force-app/main/default/"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, base, "objects/Probe__c/Probe__c.object-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <deploymentStatus>Deployed</deploymentStatus>
    <enableActivities>false</enableActivities>
    <enableBulkApi>true</enableBulkApi>
    <enableFeeds>false</enableFeeds>
    <enableHistory>false</enableHistory>
    <enableLicensing>false</enableLicensing>
    <enableReports>true</enableReports>
    <enableSearch>false</enableSearch>
    <enableSharing>true</enableSharing>
    <enableStreamingApi>true</enableStreamingApi>
    <externalSharingModel>Private</externalSharingModel>
    <label>Probe</label>
    <nameField><label>Expediente</label><type>Text</type></nameField>
    <pluralLabel>Probes</pluralLabel>
    <sharingModel>ReadWrite</sharingModel>
    <visibility>Public</visibility>
</CustomObject>
`)
	writeFile(t, filepath.Join(root, base, "objects/Probe__c/fields/Other__c.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Other__c</fullName><label>Other__c</label><length>40</length><required>false</required><type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, base, "objects/Probe__c/fields/Req__c.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Req__c</fullName><label>Req__c</label><length>40</length><required>true</required><type>Text</type>
</CustomField>
`)
	writeFile(t, filepath.Join(root, base, "tabs/Probe__c.tab-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomTab xmlns="http://soap.sforce.com/2006/04/metadata">
    <customObject>true</customObject>
    <motif>Custom20: Cloud</motif>
</CustomTab>
`)
	writeFile(t, filepath.Join(root, base, "profiles/Probe Gene\u0301rico.profile-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<Profile xmlns="http://soap.sforce.com/2006/04/metadata">
    <custom>true</custom>
    <userLicense>Salesforce</userLicense>
    <fieldPermissions><editable>true</editable><field>Probe__c.Other__c</field><readable>true</readable></fieldPermissions>
    <objectPermissions><allowCreate>true</allowCreate><allowDelete>true</allowDelete><allowEdit>true</allowEdit><allowRead>true</allowRead><modifyAllRecords>false</modifyAllRecords><object>Probe__c</object><viewAllRecords>false</viewAllRecords></objectPermissions>
    <userPermissions><enabled>true</enabled><name>ApiEnabled</name></userPermissions>
</Profile>
`)
	writeFile(t, filepath.Join(root, base, "profiles/Probe RO.profile-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<Profile xmlns="http://soap.sforce.com/2006/04/metadata">
    <custom>true</custom>
    <userLicense>Salesforce</userLicense>
    <fieldPermissions><editable>false</editable><field>Probe__c.Other__c</field><readable>true</readable></fieldPermissions>
    <objectPermissions><allowCreate>false</allowCreate><allowDelete>false</allowDelete><allowEdit>false</allowEdit><allowRead>true</allowRead><modifyAllRecords>false</modifyAllRecords><object>Probe__c</object><viewAllRecords>false</viewAllRecords></objectPermissions>
    <userPermissions><enabled>true</enabled><name>ApiEnabled</name></userPermissions>
</Profile>
`)
	writeFile(t, filepath.Join(root, base, "classes/F2.cls"), `@isTest
public class F2 {
  static String o='';
  static String descr(String prof) {
    Profile p=[SELECT Id FROM Profile WHERE Name=:prof];
    User u=new User(Alias='prc',Email='prc@example.com',EmailEncodingKey='UTF-8',LastName='Prc',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',Username='prc'+prof.length()+System.currentTimeMillis()+'@example.com');
    insert u;
    String r='';
    System.runAs(u){
      Map<String,Schema.SObjectField> fm = Schema.SObjectType.Probe__c.fields.getMap();
      for (String f : new List<String>{'Name','Req__c','Other__c','OwnerId','CreatedDate','CreatedById','LastModifiedDate','LastModifiedById','SystemModstamp','IsDeleted','LastViewedDate','LastReferencedDate'}) {
        Schema.DescribeFieldResult d = fm.containsKey(f) ? fm.get(f).getDescribe() : null;
        r += '\nDESC|'+prof+'|'+f+'|'+(d==null?'absent':'acc='+d.isAccessible()+' cr='+d.isCreateable()+' up='+d.isUpdateable());
      }
      for (String q : new String[]{'SELECT Id, Name FROM Probe__c WITH SECURITY_ENFORCED','SELECT Id, OwnerId FROM Probe__c WITH SECURITY_ENFORCED','SELECT Id, LastViewedDate FROM Probe__c WITH SECURITY_ENFORCED','SELECT Id, LastReferencedDate FROM Probe__c WITH SECURITY_ENFORCED','SELECT Id, IsDeleted, SystemModstamp, CreatedById, LastModifiedById, LastModifiedDate FROM Probe__c WITH SECURITY_ENFORCED'}) { try { Database.query(q); r+='\nQ|'+prof+'|'+q+'|ok'; } catch(Exception e){ r+='\nQ|'+prof+'|'+q+'|'+e.getMessage(); } }
      try { insert new Probe__c(Name='ins',Req__c='x'); r+='\nINS|'+prof+'|ok'; } catch(Exception e){ r+='\nINS|'+prof+'|'+e.getMessage(); }
    }
    return r;
  }
  @isTest static void run() {
    insert new Probe__c(Name='n1',Req__c='r',Other__c='o');
    o += descr('Probe RO');
    o += descr('Probe Genérico');
    System.assert(false, o);
  }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/F2.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>\n")
	run := Run(loadTestIndex(t, root), Options{})
	var got string
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			if c.Problem != nil {
				got = strings.TrimPrefix(c.Problem.Message, "Assertion Failed: ")
			}
		}
	}
	want := `
DESC|Probe RO|Name|acc=true cr=false up=false
DESC|Probe RO|Req__c|acc=true cr=false up=false
DESC|Probe RO|Other__c|acc=true cr=false up=false
DESC|Probe RO|OwnerId|acc=true cr=false up=false
DESC|Probe RO|CreatedDate|acc=true cr=false up=false
DESC|Probe RO|CreatedById|acc=true cr=false up=false
DESC|Probe RO|LastModifiedDate|acc=true cr=false up=false
DESC|Probe RO|LastModifiedById|acc=true cr=false up=false
DESC|Probe RO|SystemModstamp|acc=true cr=false up=false
DESC|Probe RO|IsDeleted|acc=true cr=false up=false
DESC|Probe RO|LastViewedDate|acc=true cr=false up=false
DESC|Probe RO|LastReferencedDate|acc=true cr=false up=false
Q|Probe RO|SELECT Id, Name FROM Probe__c WITH SECURITY_ENFORCED|ok
Q|Probe RO|SELECT Id, OwnerId FROM Probe__c WITH SECURITY_ENFORCED|ok
Q|Probe RO|SELECT Id, LastViewedDate FROM Probe__c WITH SECURITY_ENFORCED|ok
Q|Probe RO|SELECT Id, LastReferencedDate FROM Probe__c WITH SECURITY_ENFORCED|ok
Q|Probe RO|SELECT Id, IsDeleted, SystemModstamp, CreatedById, LastModifiedById, LastModifiedDate FROM Probe__c WITH SECURITY_ENFORCED|ok
INS|Probe RO|ok
DESC|Probe Genérico|Name|acc=true cr=true up=true
DESC|Probe Genérico|Req__c|acc=true cr=true up=true
DESC|Probe Genérico|Other__c|acc=true cr=true up=true
DESC|Probe Genérico|OwnerId|acc=true cr=true up=true
DESC|Probe Genérico|CreatedDate|acc=true cr=false up=false
DESC|Probe Genérico|CreatedById|acc=true cr=false up=false
DESC|Probe Genérico|LastModifiedDate|acc=true cr=false up=false
DESC|Probe Genérico|LastModifiedById|acc=true cr=false up=false
DESC|Probe Genérico|SystemModstamp|acc=true cr=false up=false
DESC|Probe Genérico|IsDeleted|acc=true cr=false up=false
DESC|Probe Genérico|LastViewedDate|acc=true cr=false up=false
DESC|Probe Genérico|LastReferencedDate|acc=true cr=false up=false
Q|Probe Genérico|SELECT Id, Name FROM Probe__c WITH SECURITY_ENFORCED|ok
Q|Probe Genérico|SELECT Id, OwnerId FROM Probe__c WITH SECURITY_ENFORCED|ok
Q|Probe Genérico|SELECT Id, LastViewedDate FROM Probe__c WITH SECURITY_ENFORCED|ok
Q|Probe Genérico|SELECT Id, LastReferencedDate FROM Probe__c WITH SECURITY_ENFORCED|ok
Q|Probe Genérico|SELECT Id, IsDeleted, SystemModstamp, CreatedById, LastModifiedById, LastModifiedDate FROM Probe__c WITH SECURITY_ENFORCED|ok
INS|Probe Genérico|ok`
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("native field access rows differ\ngot:\n%s\nwant:\n%s", got, want)
	}
}

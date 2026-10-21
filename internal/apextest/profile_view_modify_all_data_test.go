package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Native rows: scratch org, API 62.0, Probe__c with Private sharing. A profile user permission ViewAllData shows other users' records but denies update and delete; ModifyAllData allows both.
func TestProfileViewAllDataAndModifyAllDataLiftRecordSharing(t *testing.T) {
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
    <sharingModel>Private</sharingModel>
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
	writeFile(t, filepath.Join(root, base, "profiles/Probe RO.profile-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<Profile xmlns="http://soap.sforce.com/2006/04/metadata">
    <custom>true</custom>
    <userLicense>Salesforce</userLicense>
    <fieldPermissions><editable>false</editable><field>Probe__c.Other__c</field><readable>true</readable></fieldPermissions>
    <objectPermissions><allowCreate>false</allowCreate><allowDelete>false</allowDelete><allowEdit>false</allowEdit><allowRead>true</allowRead><modifyAllRecords>false</modifyAllRecords><object>Probe__c</object><viewAllRecords>false</viewAllRecords></objectPermissions>
    <userPermissions><enabled>true</enabled><name>ApiEnabled</name></userPermissions>
</Profile>
`)
	writeFile(t, filepath.Join(root, base, "profiles/Probe VAD.profile-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<Profile xmlns="http://soap.sforce.com/2006/04/metadata">
    <custom>true</custom>
    <userLicense>Salesforce</userLicense>
    <fieldPermissions><editable>true</editable><field>Probe__c.Other__c</field><readable>true</readable></fieldPermissions>
    <objectPermissions><allowCreate>true</allowCreate><allowDelete>true</allowDelete><allowEdit>true</allowEdit><allowRead>true</allowRead><modifyAllRecords>false</modifyAllRecords><object>Probe__c</object><viewAllRecords>false</viewAllRecords></objectPermissions>
    <userPermissions><enabled>true</enabled><name>ApiEnabled</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewDataLeakageEvents</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewEventLogFiles</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewPlatformEvents</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewPublicDashboards</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewPublicReports</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewRoles</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewSetup</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewAllData</name></userPermissions>
</Profile>
`)
	writeFile(t, filepath.Join(root, base, "profiles/Probe MAD.profile-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<Profile xmlns="http://soap.sforce.com/2006/04/metadata">
    <custom>true</custom>
    <userLicense>Salesforce</userLicense>
    <fieldPermissions><editable>true</editable><field>Probe__c.Other__c</field><readable>true</readable></fieldPermissions>
    <objectPermissions><allowCreate>true</allowCreate><allowDelete>true</allowDelete><allowEdit>true</allowEdit><allowRead>true</allowRead><modifyAllRecords>false</modifyAllRecords><object>Probe__c</object><viewAllRecords>false</viewAllRecords></objectPermissions>
    <userPermissions><enabled>true</enabled><name>ApiEnabled</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewDataLeakageEvents</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewEventLogFiles</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewPlatformEvents</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewPublicDashboards</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewPublicReports</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewRoles</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewSetup</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ViewAllData</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>AssignTopics</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ConnectOrgToEnvironmentHub</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ConvertLeads</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>CreateCustomizeDashboards</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>CreateCustomizeFilters</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>CreateCustomizeReports</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>CreateDashboardFolders</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>CreateReportFolders</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>CreateTopics</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>DeleteTopics</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>EditEvent</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>EditMyDashboards</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>EditMyReports</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>EditPublicDocuments</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>EditPublicFilters</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>EditPublicTemplates</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>EditTask</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>EditTopics</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ImportLeads</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ManageCategories</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ManageDashbdsInPubFolders</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ManageNetworks</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ManageReportsInPubFolders</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ModifyMetadata</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>RunReports</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>SolutionImport</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>TransferAnyEntity</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>TransferAnyLead</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>UseTeamReassignWizards</name></userPermissions>
    <userPermissions><enabled>true</enabled><name>ModifyAllData</name></userPermissions>
</Profile>
`)
	writeFile(t, filepath.Join(root, base, "classes/ShareProbe.cls"), `@isTest
public class ShareProbe {
  static String o='';
  static void probe(String prof, Id recId) {
    Profile p=[SELECT Id FROM Profile WHERE Name=:prof];
    User u=new User(Alias='shr',Email='shr@example.com',EmailEncodingKey='UTF-8',LastName='Shr',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',Username='shr'+prof.length()+System.currentTimeMillis()+'@example.com');
    insert u;
    System.runAs(u){
      o += '\nSH|'+prof+'|count='+[SELECT COUNT() FROM Probe__c WHERE Id=:recId];
      List<Probe__c> rows = [SELECT Id, Name FROM Probe__c WHERE Id=:recId];
      o += '|rows='+rows.size();
      try { o += '|enforced='+[SELECT Id, Name FROM Probe__c WHERE Id=:recId WITH SECURITY_ENFORCED].size(); } catch (Exception e) { o += '|enforced EXC'; }
      if (rows.size() > 0) {
        try { update new Probe__c(Id=recId, Other__c='x'); o += '|update=ok'; } catch (DmlException e) { o += '|update='+e.getDmlStatusCode(0); }
        try { delete new Probe__c(Id=recId); o += '|delete=ok'; } catch (DmlException e) { o += '|delete='+e.getDmlStatusCode(0); }
      } else {
        try { update new Probe__c(Id=recId, Other__c='x'); o += '|update=ok'; } catch (DmlException e) { o += '|update='+e.getDmlStatusCode(0); }
      }
    }
  }
  @isTest static void run() {
    for (String prof : new List<String>{'Probe RO','Probe VAD','Probe MAD'}) {
      Probe__c rec = new Probe__c(Name='n'+prof, Req__c='r'); insert rec;
      probe(prof, rec.Id);
    }
    System.assert(false, o);
  }
}
`)
	writeFile(t, filepath.Join(root, base, "classes/ShareProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
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
	want := `SH|Probe RO|count=0|rows=0|enforced=0|update=INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY
SH|Probe VAD|count=1|rows=1|enforced=1|update=INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY|delete=INSUFFICIENT_ACCESS_OR_READONLY
SH|Probe MAD|count=1|rows=1|enforced=1|update=ok|delete=ok`
	if strings.TrimSpace(got) != strings.TrimSpace(want) {
		t.Fatalf("native rows differ\ngot:\n%s\nwant:\n%s", got, want)
	}
}

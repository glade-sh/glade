package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Mirrors the admitted API62 successor: a parent after-undelete trigger sees
// its cascade-restored MasterDetail child, while an independently deleted
// sibling remains in the recycle bin.
func TestRunMasterDetailCascadeUndeleteTimingAPI62Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeMasterDetailUndeleteTiming62.cls"), "public class GladeMasterDetailUndeleteTiming62 {\n public static List<Id> observedChildIds=new List<Id>();\n public static void record(Set<Id> parentIds) {\n  observedChildIds=new List<Id>();\n  for(GladeMasterTimingChild62__c child:[SELECT Id FROM GladeMasterTimingChild62__c WHERE Parent__c IN :parentIds]) observedChildIds.add(child.Id);\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeMasterDetailUndeleteTiming62.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeMasterDetailUndeleteTiming62Test.cls"), "@IsTest private class GladeMasterDetailUndeleteTiming62Test {\n @IsTest static void parentAfterUndeleteSeesOnlyCascadeRestoredChildren() {\n  GladeMasterTimingParent62__c parent=new GladeMasterTimingParent62__c(Name='owned parent');insert parent;\n  GladeMasterTimingChild62__c cascaded=new GladeMasterTimingChild62__c(Name='cascade child',Parent__c=parent.Id);insert cascaded;Id cascadedId=cascaded.Id;\n  GladeMasterTimingChild62__c independentlyDeleted=new GladeMasterTimingChild62__c(Name='independent child',Parent__c=parent.Id);insert independentlyDeleted;Id independentlyDeletedId=independentlyDeleted.Id;\n  delete independentlyDeleted;delete parent;\n  System.assertEquals(0,[SELECT COUNT() FROM GladeMasterTimingChild62__c WHERE Parent__c=:parent.Id]);\n  undelete parent;\n  System.assertEquals(1,GladeMasterDetailUndeleteTiming62.observedChildIds.size());\n  System.assertEquals(cascadedId,GladeMasterDetailUndeleteTiming62.observedChildIds[0]);\n  List<GladeMasterTimingChild62__c> active=[SELECT Id,Parent__c,IsDeleted FROM GladeMasterTimingChild62__c WHERE Parent__c=:parent.Id];\n  System.assertEquals(1,active.size());System.assertEquals(cascadedId,active[0].Id);System.assertEquals(false,active[0].IsDeleted);System.assertEquals(parent.Id,active[0].Parent__c);\n  GladeMasterTimingChild62__c retainedDeleted=[SELECT Id,Parent__c,IsDeleted FROM GladeMasterTimingChild62__c WHERE Id=:independentlyDeletedId ALL ROWS];\n  System.assertEquals(true,retainedDeleted.IsDeleted);System.assertEquals(parent.Id,retainedDeleted.Parent__c);\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeMasterDetailUndeleteTiming62Test.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers/GladeMasterTimingParent62Trigger.trigger"), "trigger GladeMasterTimingParent62Trigger on GladeMasterTimingParent62__c (after undelete) {\n GladeMasterDetailUndeleteTiming62.record(Trigger.newMap.keySet());\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers/GladeMasterTimingParent62Trigger.trigger-meta.xml"), "<ApexTrigger xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexTrigger>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeMasterTimingParent62__c/GladeMasterTimingParent62__c.object-meta.xml"), "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><deploymentStatus>Deployed</deploymentStatus><label>Glade Master Timing Parent 62</label><nameField><label>Glade Master Timing Parent 62 Name</label><type>Text</type></nameField><pluralLabel>Glade Master Timing Parents 62</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeMasterTimingChild62__c/GladeMasterTimingChild62__c.object-meta.xml"), "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><deploymentStatus>Deployed</deploymentStatus><label>Glade Master Timing Child 62</label><nameField><label>Glade Master Timing Child 62 Name</label><type>Text</type></nameField><pluralLabel>Glade Master Timing Children 62</pluralLabel><sharingModel>ControlledByParent</sharingModel></CustomObject>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeMasterTimingChild62__c/fields/Parent__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>Parent__c</fullName><externalId>false</externalId><label>Parent</label><referenceTo>GladeMasterTimingParent62__c</referenceTo><relationshipLabel>Glade Master Timing Children 62</relationshipLabel><relationshipName>GladeMasterTimingChildren62</relationshipName><relationshipOrder>0</relationshipOrder><reparentableMasterDetail>false</reparentableMasterDetail><trackFeedHistory>false</trackFeedHistory><trackTrending>false</trackTrending><type>MasterDetail</type><writeRequiresMasterRead>true</writeRequiresMasterRead></CustomField>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"volunteers-masterdetail-cascade-undelete-trigger-api62-project40-successor\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"40.0\"}")

	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 || got.Failed != 0 {
		body, _ := json.Marshal(run)
		t.Fatalf("admitted master-detail undelete timing contract: %s", body)
	}
}

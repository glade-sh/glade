package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Wave75: the original three API53 source cases, including JSON-roundtripped lookup IDs.
func TestRunChildQueryIDRepresentationContracts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeChildIdentity53.cls"), "@IsTest private class GladeChildIdentity53 {\n private class Envelope { public Id parentId; }\n private class Reader implements Queueable {\n  private Id parentId; private Set<Id> expected;\n  Reader(Id parentId,Set<Id> expected){this.parentId=parentId;this.expected=expected;}\n  public void execute(QueueableContext context){assertView(parentId,expected);}\n }\n private static void assertView(Id parentId,Set<Id> expected){\n  List<Contact> direct=[SELECT Id,AccountId FROM Contact WHERE AccountId=:parentId];\n  System.assertEquals(expected,new Map<Id,Contact>(direct).keySet(),'direct lookup population');\n  Account parent=[SELECT Id,(SELECT Id,AccountId FROM Contacts) FROM Account WHERE Id=:parentId];\n  System.assertEquals(expected,new Map<Id,Contact>(parent.Contacts).keySet(),'inline child population');\n  List<Account> dynamicParents=Database.query('SELECT Id,(SELECT Id,AccountId FROM Contacts) FROM Account WHERE Id=:parentId');\n  System.assertEquals(1,dynamicParents.size());\n  System.assertEquals(expected,new Map<Id,Contact>(dynamicParents[0].Contacts).keySet(),'dynamic child population');\n  for(Contact child:parent.Contacts)System.assertEquals(parent.Id,child.AccountId,'lookup identity');\n }\n private static void exercise(Integer mode){\n  Account parent=new Account(Name='Owned child identity');insert parent;\n  Contact normal=new Contact(LastName='Normal',AccountId=parent.Id);insert normal;\n  Set<Id> expected=new Set<Id>{normal.Id};\n  assertView(parent.Id,expected);\n  Id assigned=parent.Id;\n  if(mode==1){\n   Envelope payload=new Envelope();payload.parentId=parent.Id;\n   String wire=JSON.serialize(payload);\n   Map<String,Object> raw=(Map<String,Object>)JSON.deserializeUntyped(wire);\n   System.assertEquals(18,String.valueOf(raw.get('parentId')).length(),'JSON ID width');\n   Envelope decoded=(Envelope)JSON.deserialize(wire,Envelope.class);\n   assigned=decoded.parentId;\n  }else if(mode==2){\n   String text=String.valueOf(parent.Id);System.assertEquals(18,text.length());\n   assigned=Id.valueOf(text.substring(0,15));\n  }\n  System.assertEquals(parent.Id,assigned,'equivalent parent ID');\n  Test.startTest();\n  Contact added=new Contact(LastName='Added',AccountId=assigned);insert added;expected.add(added.Id);\n  assertView(parent.Id,expected);\n  System.enqueueJob(new Reader(parent.Id,expected));\n  Test.stopTest();\n  assertView(parent.Id,expected);\n }\n @IsTest static void directAssignedChild(){exercise(0);}\n @IsTest static void jsonRoundTripAssignedChild(){exercise(1);}\n @IsTest static void explicitFifteenAssignedChild(){exercise(2);}\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeChildIdentity53.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"53.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 3 || got.Passed != 3 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("child ID representation contracts: %s", data)
	}
}

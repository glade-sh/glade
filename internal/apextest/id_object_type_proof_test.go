package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact API66 synthetic and inserted ID contracts admitted by Salesforce wave163.
func TestIDGetSObjectTypeAPI66Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeIdType66Proof.cls"), "@IsTest private class GladeIdType66Proof {\n @IsTest static void originalContactFakeIdsResolveType() {\n  Contact row=(Contact)JSON.deserialize(JSON.serialize(new Map<String,Object>{'Id'=>'00300000000000000B','MasterRecordId'=>'00300000000000000A'}),Contact.class);\n  System.assertEquals('00300000000000000B',String.valueOf(row.Id));\n  System.assertEquals('00300000000000000A',String.valueOf(row.MasterRecordId));\n  Id masterId=row.MasterRecordId;\n  Id childId=row.Id;\n  System.assertEquals(Contact.SObjectType,masterId.getSObjectType());\n  System.assertEquals(Contact.SObjectType,childId.getSObjectType());\n }\n @IsTest static void insertedContactIdResolvesType() {\n  Contact row=new Contact(LastName='Glade Id Type Owned');\n  insert row;\n  Id validId=row.Id;\n  System.assertNotEquals(null,validId);\n  System.assertEquals(Contact.SObjectType,validId.getSObjectType());\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeIdType66Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>66.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"namespace\": \"\", \"sourceApiVersion\": \"66.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("ID object type proof: %s", data)
	}
}

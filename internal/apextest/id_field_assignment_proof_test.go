package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact original API66 assertions admitted by Salesforce wave160.
func TestIDFieldAssignmentAndJSONAPI66Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeIdError66Proof.cls"), "@IsTest private class GladeIdError66Proof {\n @IsTest static void invalidIdAssignmentIsCatchableAndAtomic() {\n  Account row=new Account(Name='preserved'); Boolean caught=false;\n  try { row.Id='BadRecordID'; System.assert(false,'Expected invalid Id rejection'); }\n  catch(System.StringException e) { caught=true; System.assertEquals('Invalid id: BadRecordID',e.getMessage()); }\n  System.assert(caught); System.assertEquals(null,row.Id); System.assertEquals('preserved',row.Name);\n }\n @IsTest static void originalContactFakeIdsDeserialize() {\n  Contact row=(Contact)JSON.deserialize(JSON.serialize(new Map<String,Object>{'Id'=>'00300000000000000B','MasterRecordId'=>'00300000000000000A'}),Contact.class);\n  System.assertEquals('00300000000000000B',String.valueOf(row.Id));\n  System.assertEquals('00300000000000000A',String.valueOf(row.MasterRecordId));\n  System.assertNotEquals(row.Id,row.MasterRecordId);\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeIdError66Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>66.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"66.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("ID proof: %s", data)
	}
}

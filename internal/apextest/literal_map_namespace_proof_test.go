package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact plain-map literal key contract admitted at API44/project61 by SF179.
func TestLiteralMapNamespaceKeysAPI44Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeLiteralMapKeys44Proof.cls"), "@IsTest private class GladeLiteralMapKeys44Proof {\n @IsTest static void namespaceShapedStringKeysRemainDistinct() {\n  Map<String,Object> fields=new Map<String,Object>{'GladeMemberType44__c'=>'BASE'};\n  System.assertEquals(false,fields.containsKey('PKG__GladeMemberType44__c'));\n  System.assertEquals(null,fields.get('PKG__GladeMemberType44__c'));\n  Integer beforeSize=fields.size();\n  fields.put('PKG__GladeMemberType44__c','MOCK');\n  System.assertEquals(beforeSize+1,fields.size());\n  System.assertEquals(true,fields.containsKey('GladeMemberType44__c'));\n  System.assertEquals(true,fields.containsKey('PKG__GladeMemberType44__c'));\n  System.assertEquals('BASE',fields.get('GladeMemberType44__c'));\n  System.assertEquals('MOCK',fields.get('PKG__GladeMemberType44__c'));\n  Map<String,Object> restored=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(fields));\n  System.assertEquals(2,restored.size());\n  System.assertEquals(true,restored.containsKey('GladeMemberType44__c'));\n  System.assertEquals(true,restored.containsKey('PKG__GladeMemberType44__c'));\n  System.assertEquals('BASE',restored.get('GladeMemberType44__c'));\n  System.assertEquals('MOCK',restored.get('PKG__GladeMemberType44__c'));\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeLiteralMapKeys44Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>44.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"namespace\": \"\", \"sourceApiVersion\": \"61.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	data, _ := json.Marshal(run)
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 || got.Skipped != 0 {
		t.Fatalf("literal map proof: %s", data)
	}
}

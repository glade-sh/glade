package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact original API66 contract admitted by Salesforce wave155.
func TestSOQLDescendingDefaultNullOrderAPI66(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeNullOrder66Proof.cls"), "@IsTest private class GladeNullOrder66Proof {\n @IsTest static void descendingDefaultAndExplicitNullPlacement() {\n  Account parent = new Account(Name='Null ordering proof'); insert parent;\n  insert new List<Contact>{new Contact(LastName='empty',AccountId=parent.Id),new Contact(LastName='high',AccountId=parent.Id,Phone='z'),new Contact(LastName='low',AccountId=parent.Id,Phone='a')};\n  List<Contact> defaults = [SELECT LastName FROM Contact WHERE AccountId=:parent.Id ORDER BY Phone DESC];\n  List<Contact> firsts = [SELECT LastName FROM Contact WHERE AccountId=:parent.Id ORDER BY Phone DESC NULLS FIRST];\n  List<Contact> lasts = [SELECT LastName FROM Contact WHERE AccountId=:parent.Id ORDER BY Phone DESC NULLS LAST];\n  System.assertEquals('empty', defaults[0].LastName);\n  System.assertEquals('high', defaults[1].LastName);\n  System.assertEquals('low', defaults[2].LastName);\n  System.assertEquals('empty', firsts[0].LastName);\n  System.assertEquals('high', lasts[0].LastName);\n  System.assertEquals('empty', lasts[2].LastName);\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeNullOrder66Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>66.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"66.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("null order proof: %s", data)
	}
}

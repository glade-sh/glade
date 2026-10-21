package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Profile names come from file names. A checkout can hold decomposed Unicode
// names (NFD) while Apex source and Salesforce use composed names (NFC).
func TestProfileNameFromDecomposedFileNameMatchesComposedQuery(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/profiles/Genérico.profile-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Profile xmlns=\"http://soap.sforce.com/2006/04/metadata\"><custom>true</custom></Profile>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProfileNfcProbe.cls"), "@IsTest private class ProfileNfcProbe {\n @IsTest static void composedNameFindsProfile(){\n  Profile p=[SELECT Id FROM Profile WHERE Name='Genérico' LIMIT 1];\n  System.assertNotEquals(null,p.Id);\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProfileNfcProbe.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>\n")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("profile NFC lookup failed: %s", data)
	}
}

// A getRecordTypeInfosByDeveloperName reference carries no label. It must not
// replace the label declared in the RecordType metadata.
func TestDeveloperNameReferenceKeepsDeclaredRecordTypeLabel(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/RtLabel__c/RtLabel__c.object-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><deploymentStatus>Deployed</deploymentStatus><label>Rt Label</label><nameField><label>N</label><type>Text</type></nameField><pluralLabel>Rt Labels</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/RtLabel__c/recordTypes/Tipo_de_via.recordType-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<RecordType xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>Tipo_de_via</fullName><active>true</active><label>Tipo de vía</label></RecordType>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/RtLabelProbe.cls"), "@IsTest private class RtLabelProbe {\n @IsTest static void labelSurvivesDeveloperNameLookup(){\n  Id viaId=RtLabel__c.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Tipo_de_via').getRecordTypeId();\n  Schema.RecordTypeInfo info=RtLabel__c.SObjectType.getDescribe().getRecordTypeInfosByName().get('Tipo de vía');\n  System.assertNotEquals(null,info,'label key');\n  System.assertEquals(viaId,info.getRecordTypeId());\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/RtLabelProbe.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>\n")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("record type label lost: %s", data)
	}
}

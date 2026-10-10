package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact four named assertions and component APIs admitted by Salesforce SF147.
func TestCalculatedDateDefaultRoundTripAPI31Caller29(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeDateFactory31.cls"), "@IsTest public class GladeDateFactory31 {\n    public static GladeDateDefault__c defaults() {\n        return (GladeDateDefault__c)GladeDateDefault__c.SObjectType.newSObject(null, true);\n    }\n    public static String serialized(SObject row) { return JSON.serialize(row); }\n    public static GladeDateDefault__c roundTrip(SObject row) {\n        Map<String, Object> values = (Map<String, Object>)JSON.deserializeUntyped(JSON.serialize(row));\n        return (GladeDateDefault__c)JSON.deserialize(JSON.serialize(values), SObject.class);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeDateDefault29Proof.cls"), "@IsTest private class GladeDateDefault29Proof {\n    private static void assertBlank(GladeDateDefault__c row) {\n        System.assertEquals(null, row.InvoiceDueDate__c);\n        Map<String, Object> values = (Map<String, Object>)JSON.deserializeUntyped(GladeDateFactory31.serialized(row));\n        System.assertEquals(null, values.get('InvoiceDueDate__c'));\n        GladeDateDefault__c copy = GladeDateFactory31.roundTrip(row);\n        System.assertNotEquals(null, copy);\n        System.assertEquals(null, copy.InvoiceDueDate__c);\n    }\n    @IsTest static void blankDefaultsRoundTrip() {\n        assertBlank(GladeDateFactory31.defaults());\n    }\n    @IsTest static void ordinaryNewRoundTrip() {\n        assertBlank(new GladeDateDefault__c());\n    }\n    @IsTest static void populatedDateAndTermRoundTrip() {\n        GladeDateDefault__c row = new GladeDateDefault__c(Name='populated', InvoiceDate__c=Date.newInstance(2026, 1, 1), InvoiceTerm__c=30);\n        insert row;\n        row = [SELECT InvoiceDate__c, InvoiceTerm__c, InvoiceDueDate__c FROM GladeDateDefault__c WHERE Id=:row.Id];\n        System.assertEquals(Date.newInstance(2026, 1, 31), row.InvoiceDueDate__c);\n        GladeDateDefault__c copy = GladeDateFactory31.roundTrip(row);\n        System.assertEquals(Date.newInstance(2026, 1, 31), copy.InvoiceDueDate__c);\n        System.assertEquals(Date.newInstance(2026, 1, 1), copy.InvoiceDate__c);\n        System.assertEquals(30, copy.InvoiceTerm__c);\n    }\n    @IsTest static void blankDatePopulatedTermRoundTrip() {\n        GladeDateDefault__c row = new GladeDateDefault__c(Name='blank date', InvoiceTerm__c=5);\n        insert row;\n        row = [SELECT InvoiceDate__c, InvoiceTerm__c, InvoiceDueDate__c FROM GladeDateDefault__c WHERE Id=:row.Id];\n        System.assertEquals(5, row.InvoiceTerm__c);\n        assertBlank(row);\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeDateFactory31.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>31.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeDateDefault29Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>29.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDateDefault__c/GladeDateDefault__c.object-meta.xml"), "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><deploymentStatus>Deployed</deploymentStatus><label>Date Default</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Date Defaults</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDateDefault__c/fields/InvoiceDate__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>InvoiceDate__c</fullName><label>Invoice Date</label><required>false</required><type>Date</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDateDefault__c/fields/InvoiceTerm__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>InvoiceTerm__c</fullName><label>Invoice Term</label><precision>3</precision><required>false</required><scale>0</scale><type>Number</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDateDefault__c/fields/InvoiceDueDate__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>InvoiceDueDate__c</fullName><formula>InvoiceDate__c +  InvoiceTerm__c</formula><formulaTreatBlanksAs>BlankAsZero</formulaTreatBlanksAs><label>Invoice Due Date</label><required>false</required><type>Date</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"61.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("report: %s", data)
	if got := run.Summary(); got.Total != 4 || got.Passed != 4 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		t.Fatalf("date default proof: %s", data)
	}
}

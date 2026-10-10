package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact source and metadata from the accepted API31/36/39 formula-default packet.
func TestNewSObjectFormulaDefaultsVisibilityAPI31To39(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDefaultFormula__c/GladeDefaultFormula__c.object-meta.xml"), "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><deploymentStatus>Deployed</deploymentStatus><label>Default Formula</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Default Formulas</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDefaultFormula__c/fields/SubTotal__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>SubTotal__c</fullName><label>SubTotal__c</label><precision>18</precision><scale>2</scale><type>Currency</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDefaultFormula__c/fields/Discounts__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>Discounts__c</fullName><label>Discounts__c</label><precision>18</precision><scale>2</scale><type>Currency</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDefaultFormula__c/fields/TotalShipping__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>TotalShipping__c</fullName><label>TotalShipping__c</label><precision>18</precision><scale>2</scale><type>Currency</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDefaultFormula__c/fields/TotalTax__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>TotalTax__c</fullName><label>TotalTax__c</label><precision>18</precision><scale>2</scale><type>Currency</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDefaultFormula__c/fields/GrandTotal__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>GrandTotal__c</fullName><label>GrandTotal__c</label><precision>18</precision><scale>2</scale><type>Currency</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDefaultFormula__c/fields/TotalPayment__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>TotalPayment__c</fullName><label>TotalPayment__c</label><precision>18</precision><scale>2</scale><type>Currency</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDefaultFormula__c/fields/Total__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>Total__c</fullName><formula>SubTotal__c + Discounts__c + TotalShipping__c + TotalTax__c</formula><formulaTreatBlanksAs>BlankAsZero</formulaTreatBlanksAs><label>Total__c</label><precision>18</precision><scale>2</scale><type>Currency</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDefaultFormula__c/fields/Balance__c.field-meta.xml"), "<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>Balance__c</fullName><formula>GrandTotal__c - TotalPayment__c</formula><formulaTreatBlanksAs>BlankAsZero</formulaTreatBlanksAs><label>Balance__c</label><precision>18</precision><scale>2</scale><type>Currency</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeFormulaFactory31.cls"), "public class GladeFormulaFactory31 {public static GladeDefaultFormula__c build(){return (GladeDefaultFormula__c)GladeDefaultFormula__c.SObjectType.newSObject(null,true);}}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeFormulaFactory31.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>31.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeFormulaVisibility36.cls"), "@IsTest private class GladeFormulaVisibility36 {@IsTest static void plainFresh(){GladeDefaultFormula__c row=new GladeDefaultFormula__c();System.assertEquals(null,row.Total__c);}@IsTest static void defaultsFresh(){GladeDefaultFormula__c row=GladeFormulaFactory31.build();System.assertEquals(0,row.Total__c);}@IsTest static void defaultsAfterInsert(){GladeDefaultFormula__c row=GladeFormulaFactory31.build();row.Name='before';insert row;System.assertEquals(0,row.Total__c);System.assertEquals(0,[SELECT Total__c FROM GladeDefaultFormula__c WHERE Id=:row.Id].Total__c);}@IsTest static void queriedCurrentValues(){GladeDefaultFormula__c row=GladeFormulaFactory31.build();row.Name='values';row.SubTotal__c=12;row.Discounts__c=-2;row.TotalTax__c=1;insert row;System.assertEquals(11,[SELECT Total__c FROM GladeDefaultFormula__c WHERE Id=:row.Id].Total__c);}}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeFormulaVisibility36.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>36.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeFormulaVisibility39.cls"), "@IsTest private class GladeFormulaVisibility39 {@IsTest static void defaultBalance(){GladeDefaultFormula__c row=GladeFormulaFactory31.build();System.assertEquals(0,row.Balance__c);}@IsTest static void plainBalance(){GladeDefaultFormula__c row=new GladeDefaultFormula__c();System.assertEquals(null,row.Balance__c);}}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeFormulaVisibility39.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>39.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"61.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 6 || got.Passed != 6 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("formula default visibility: %s", data)
	}
}

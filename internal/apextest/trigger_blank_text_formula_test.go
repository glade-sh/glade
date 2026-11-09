package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// A text formula that evaluates to an empty string is blank and reads as null.
// A before update trigger that compares the formula on Trigger.new and
// Trigger.oldMap must see no change when an unrelated field is updated; the
// trigger here clears a stored field only when the formula differs.
func TestBeforeUpdateTriggerBlankTextFormulaComparesEqual(t *testing.T) {
	root := t.TempDir()
	meta := func(path, body string) {
		writeFile(t, filepath.Join(root, "force-app/main/default", path), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+body+"\n")
	}
	meta("objects/Item__c/Item__c.object-meta.xml", `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Item</label><pluralLabel>Items</pluralLabel><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel><nameField><label>Name</label><type>Text</type></nameField></CustomObject>`)
	field := func(name, body string) {
		meta("objects/Item__c/fields/"+name+".field-meta.xml", `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>`+name+`</fullName>`+body+`</CustomField>`)
	}
	field("Tax__c", `<defaultValue>0</defaultValue><label>Tax</label><precision>11</precision><scale>2</scale><required>false</required><type>Currency</type>`)
	field("Note__c", `<label>Note</label><length>50</length><required>false</required><type>Text</type><unique>false</unique>`)
	field("Street__c", `<label>Street</label><length>50</length><required>false</required><type>Text</type><unique>false</unique>`)
	field("Postal__c", `<label>Postal</label><length>50</length><required>false</required><type>Text</type><unique>false</unique>`)
	field("Country__c", `<label>Country</label><length>50</length><required>false</required><type>Text</type><unique>false</unique>`)
	field("City__c", `<label>City</label><length>50</length><required>false</required><type>Text</type><unique>false</unique>`)
	field("Label__c", `<formula>If(IsBlank(Street__c), '', Street__c &amp; BR()) &amp; If(IsBlank(City__c), '', City__c &amp; ', ') &amp; Postal__c &amp; ' ' &amp; Country__c</formula><label>Label</label><required>false</required><type>Text</type><unique>false</unique>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers/ItemTrigger.trigger"), "trigger ItemTrigger on Item__c (before update) {\n for (Item__c row : Trigger.new) {\n  Item__c previous = Trigger.oldMap.get(row.Id);\n  if (row.Label__c != previous.Label__c) { row.Tax__c = null; }\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers/ItemTrigger.trigger-meta.xml"), "<ApexTrigger xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>29.0</apiVersion><status>Active</status></ApexTrigger>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/BlankFormulaTest.cls"), "@IsTest private class BlankFormulaTest {\n @IsTest static void unrelatedUpdateKeepsTax() {\n  Item__c item = new Item__c(Name = 'a', Tax__c = 5.00);\n  insert item;\n  item.Note__c = 'changed';\n  update item;\n  System.assertEquals(5.00, [SELECT Tax__c FROM Item__c WHERE Id = :item.Id].Tax__c, 'tax kept');\n }\n @IsTest static void changedFormulaInputClearsTax() {\n  Item__c item = new Item__c(Name = 'a', Tax__c = 5.00);\n  insert item;\n  item.Street__c = '1 Main';\n  update item;\n  System.assertEquals(null, [SELECT Tax__c FROM Item__c WHERE Id = :item.Id].Tax__c, 'tax cleared');\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/BlankFormulaTest.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>59.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"59.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("blank text formula comparison: %s", data)
	}
}

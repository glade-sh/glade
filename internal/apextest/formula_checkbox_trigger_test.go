package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// A formula checkbox over a parent lookup with BlankAsZero is false, never
// null, when the lookup is empty, including in Trigger.new of an after-update
// trigger, where Apex code commonly uses it directly in a condition.
func TestFormulaCheckboxBlankParentIsFalseInAfterUpdateTrigger(t *testing.T) {
	api := "62.0"
	files := map[string]string{
		"force-app/main/default/objects/Holder__c/Holder__c.object-meta.xml":     `<CustomObject><label>Holder</label><pluralLabel>Holders</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`,
		"force-app/main/default/objects/Holder__c/fields/Flag__c.field-meta.xml": `<CustomField><fullName>Flag__c</fullName><label>Flag</label><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`,
		"force-app/main/default/objects/Item__c/Item__c.object-meta.xml":         `<CustomObject><label>Item</label><pluralLabel>Items</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`,
		"force-app/main/default/objects/Item__c/fields/Holder__c.field-meta.xml": `<CustomField><fullName>Holder__c</fullName><label>Holder</label><type>Lookup</type><referenceTo>Holder__c</referenceTo><relationshipName>Holder</relationshipName><deleteConstraint>SetNull</deleteConstraint></CustomField>`,
		"force-app/main/default/objects/Item__c/fields/IsHeld__c.field-meta.xml": `<CustomField><fullName>IsHeld__c</fullName><label>Is Held</label><type>Checkbox</type><formula>Holder__r.Flag__c</formula><formulaTreatBlanksAs>BlankAsZero</formulaTreatBlanksAs></CustomField>`,
		"force-app/main/default/objects/Item__c/fields/Note__c.field-meta.xml":   `<CustomField><fullName>Note__c</fullName><label>Note</label><type>Text</type><length>80</length></CustomField>`,
		"force-app/main/default/triggers/ItemAfter.trigger": `trigger ItemAfter on Item__c (after insert, after update) {
    for (Item__c item : Trigger.new) {
        if (item.IsHeld__c && item.Note__c == 'never') { item.addError('unexpected'); }
        System.assertEquals(false, item.IsHeld__c);
    }
}`,
		"force-app/main/default/triggers/ItemAfter.trigger-meta.xml": `<ApexTrigger><apiVersion>62.0</apiVersion><status>Active</status></ApexTrigger>`,
	}
	method := `Item__c item = new Item__c(Name = 'Loose'); insert item; item.Note__c = 'changed'; update item;
System.assertEquals(false, [SELECT IsHeld__c FROM Item__c WHERE Id = :item.Id].IsHeld__c);`
	index := flowCoreIndex(t, api, map[string]string{"FormulaCheckboxProbe": "@IsTest public class FormulaCheckboxProbe { @IsTest static void blankParent() {" + method + "} }"}, files)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	run := Run(index, Options{SelectedClasses: []string{"FormulaCheckboxProbe"}, NoDiskCache: true, Parallelism: 1})
	if run.Summary().Total != 1 || len(run.Suites) != 1 {
		t.Fatalf("route selection: %+v", run.Summary())
	}
	if result := run.Suites[0].Cases[0]; result.Status != "pass" {
		t.Fatalf("%s: %+v", result.Status, result.Problem)
	}
}

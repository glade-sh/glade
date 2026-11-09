package apextest

import (
	"path/filepath"
	"testing"
)

// Native flow_refs record rows at API 67.0 (testdata/conformance/flow_refs/
// record.json the captured row; flow_capture_permissions/refs.json grants the formula
// field read-only) update a record queried with its formula field under default
// user-mode DML, and the update succeeds. DML never writes a formula field, so
// its read-only field permission does not deny the statement. The same holds
// for an auto-number field (field_key_upsert the captured row at API 67.0).
func TestUserModeDMLIgnoresQueriedFormulaField(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	objectDir := filepath.Join(root, "force-app/main/default/objects/FormulaNote__c")
	writeFile(t, filepath.Join(objectDir, "FormulaNote__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Formula Note</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Formula Notes</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(objectDir, "fields/Text__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Text__c</fullName><label>Text</label><length>40</length><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(objectDir, "fields/Echo__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Echo__c</fullName><label>Echo</label><formula>Text__c</formula><type>Text</type></CustomField>`)
	writeFile(t, filepath.Join(objectDir, "fields/Seq__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Seq__c</fullName><label>Seq</label><displayFormat>S-{000}</displayFormat><startingNumber>1</startingNumber><type>AutoNumber</type></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/permissionsets/FormulaNoteAccess.permissionset-meta.xml"), `<PermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><fieldPermissions><editable>true</editable><field>FormulaNote__c.Text__c</field><readable>true</readable></fieldPermissions><fieldPermissions><editable>false</editable><field>FormulaNote__c.Echo__c</field><readable>true</readable></fieldPermissions><fieldPermissions><editable>false</editable><field>FormulaNote__c.Seq__c</field><readable>true</readable></fieldPermissions><label>FormulaNoteAccess</label><objectPermissions><allowCreate>true</allowCreate><allowDelete>true</allowDelete><allowEdit>true</allowEdit><allowRead>true</allowRead><modifyAllRecords>false</modifyAllRecords><object>FormulaNote__c</object><viewAllRecords>false</viewAllRecords></objectPermissions></PermissionSet>`)
	writeCapturedAssignments(t, root, "FormulaNoteAccess")
	path := filepath.Join(root, "force-app/main/default/classes/FormulaNoteTest.cls")
	writeFile(t, path, `@IsTest private class FormulaNoteTest {
    @IsTest static void updatesQueriedFormulaRecord() {
        FormulaNote__c note = new FormulaNote__c(Name = 'n', Text__c = 'a');
        insert note;
        note = [SELECT Text__c, Echo__c FROM FormulaNote__c WHERE Id = :note.Id];
        System.assertEquals('a', note.Echo__c);
        note.Text__c = 'b';
        update note;
        note = [SELECT Text__c, Echo__c, Seq__c FROM FormulaNote__c WHERE Id = :note.Id];
        Database.update(note, AccessLevel.USER_MODE);
        System.assertEquals('b', [SELECT Echo__c FROM FormulaNote__c WHERE Id = :note.Id].Echo__c);
    }
}`)
	writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"FormulaNoteTest"}, NoDiskCache: true, Parallelism: 1})
	if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("%+v %s", summary, firstRunProblem(run))
	}
}

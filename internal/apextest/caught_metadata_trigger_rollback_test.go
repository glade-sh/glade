package apextest

import (
	"path/filepath"
	"testing"
)

// Compose the null-negation observation in expressions_control.json R092 with
// the after-update exception observation in triggers.json R257. The source
// metadata lookup is deliberately absent in one case and present in its control.
func TestCaughtMetadataTriggerRollback(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(base, "objects/SwitchSetting__mdt/SwitchSetting__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Switch Setting</label><pluralLabel>Switch Settings</pluralLabel><visibility>Public</visibility></CustomObject>`)
			writeFile(t, filepath.Join(base, "objects/SwitchSetting__mdt/fields/Enabled__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Enabled__c</fullName><label>Enabled</label><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)
			writeFile(t, filepath.Join(base, "customMetadata/SwitchSetting.Seed.md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>Seed</label><protected>false</protected><values><field>Enabled__c</field><value xsi:type="xsd:boolean">false</value></values></CustomMetadata>`)
			for name, source := range map[string]string{
				"NullableSwitch": `public class NullableSwitch {
 public static String selected = 'Missing';
 public static Boolean bypass = false;
 public static Boolean enabled() {
  Boolean value;
  List<SwitchSetting__mdt> rows = [SELECT Enabled__c FROM SwitchSetting__mdt WHERE DeveloperName = :selected];
  if (!rows.isEmpty()) value = rows[0].Enabled__c;
  return value;
 }
}`,
				"TwoStageSave": `public class TwoStageSave {
 public static String faultType;
 public static String faultMessage;
 public static String stage;
 public static Integer stagedCount;
 public static Boolean save() {
  Savepoint checkpoint = Database.setSavepoint();
  try {
   stage = 'insert';
   insert new Account(Name = 'Created in first stage');
   Contact child = new Contact(LastName = 'Created in first stage');
   insert child;
   stagedCount = [SELECT COUNT() FROM Account];
   stage = 'update';
   child.Description = 'Second stage';
   update child;
   stage = 'complete';
   return true;
  } catch (Exception error) {
   faultType = error.getTypeName();
   faultMessage = error.getMessage();
   Database.rollback(checkpoint);
   return false;
  }
 }
}`,
				"CaughtTriggerRollbackTest": `@IsTest private class CaughtTriggerRollbackTest {
 @IsTest static void missingMetadataRollsBackTheFirstStage() {
  insert new Account(Name = 'Original');
  System.assertEquals(null, NullableSwitch.enabled());
  System.assertEquals(false, TwoStageSave.save());
  System.assertEquals('update', TwoStageSave.stage);
  System.assertEquals(2, TwoStageSave.stagedCount);
  System.assertEquals(true, TwoStageSave.faultType.endsWith('DmlException'));
  System.assertEquals(true, TwoStageSave.faultMessage.contains('CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY'));
  System.assertEquals(true, TwoStageSave.faultMessage.contains('Attempt to de-reference a null object'));
  System.assertEquals(1, [SELECT COUNT() FROM Account]);
  System.assertEquals('Original', [SELECT Name FROM Account LIMIT 1].Name);
  System.assertEquals(0, [SELECT COUNT() FROM Contact]);
 }
 @IsTest static void presentMetadataCompletesBothStages() {
  insert new Account(Name = 'Original');
  NullableSwitch.selected = 'Seed';
  System.assertEquals(false, NullableSwitch.enabled());
  System.assertEquals(true, TwoStageSave.save());
  System.assertEquals('complete', TwoStageSave.stage);
  System.assertEquals(null, TwoStageSave.faultType);
  System.assertEquals(2, [SELECT COUNT() FROM Account]);
  System.assertEquals(1, [SELECT COUNT() FROM Contact]);
 }
}`,
			} {
				writeFile(t, filepath.Join(base, "classes", name+".cls"), source)
				writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			writeFile(t, filepath.Join(base, "triggers/ContactSwitch.trigger"), `trigger ContactSwitch on Contact (after update) {
 if (NullableSwitch.bypass || !NullableSwitch.enabled()) return;
}`)
			writeFile(t, filepath.Join(base, "triggers/ContactSwitch.trigger-meta.xml"), `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexTrigger>`)
			run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
				t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
			}
		})
	}
}

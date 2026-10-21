package apextest

import (
	"path/filepath"
	"testing"
)

func TestControlledParentQueryPreservesRequiredPermission(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	base := filepath.Join(root, "force-app/main/default")
	writeFile(t, filepath.Join(base, "customPermissions/MayReview.customPermission-meta.xml"), `<CustomPermission xmlns="http://soap.sforce.com/2006/04/metadata"><isLicensed>false</isLicensed><label>May review</label></CustomPermission>`)
	writeFile(t, filepath.Join(base, "objects/GateItem__c/GateItem__c.object-meta.xml"), `<CustomObject><label>Gate item</label><pluralLabel>Gate items</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(base, "objects/GateMessage__c/GateMessage__c.object-meta.xml"), `<CustomObject><label>Gate message</label><pluralLabel>Gate messages</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><sharingModel>ControlledByParent</sharingModel></CustomObject>`)
	writeFile(t, filepath.Join(base, "objects/GateMessage__c/fields/Parent__c.field-meta.xml"), `<CustomField><fullName>Parent__c</fullName><label>Parent</label><type>MasterDetail</type><referenceTo>GateItem__c</referenceTo><relationshipLabel>Messages</relationshipLabel><relationshipName>Messages</relationshipName></CustomField>`)
	writeFile(t, filepath.Join(base, "objects/GateMessage__c/fields/Status__c.field-meta.xml"), `<CustomField><fullName>Status__c</fullName><label>Status</label><type>Text</type><length>40</length></CustomField>`)
	writeFile(t, filepath.Join(base, "objects/GateMessage__c/fields/Body__c.field-meta.xml"), `<CustomField><fullName>Body__c</fullName><label>Body</label><type>LongTextArea</type><length>32768</length><visibleLines>3</visibleLines></CustomField>`)
	writeFile(t, filepath.Join(base, "classes/GatePayload.cls"), `public class GatePayload {
  /** A plain value or a {"reference":...} marker is accepted. */
  public static String resolve(String raw) { return raw; }
}`)
	writeFile(t, filepath.Join(base, "classes/PermissionGate.cls"), `public inherited sharing class PermissionGate {
  public static String required(Id itemId, String key) {
    for (GateMessage__c row : [SELECT Body__c FROM GateMessage__c
      WHERE Parent__c=:itemId AND Status__c='Pending'
      WITH SYSTEM_MODE ORDER BY CreatedDate DESC, Id DESC LIMIT 50]) {
      Map<String,Object> data = (Map<String,Object>)JSON.deserializeUntyped(GatePayload.resolve(row.Body__c));
      if (data.get('waiting') == true && key.equalsIgnoreCase((String)data.get('key'))) {
        return (String)data.get('permission');
      }
    }
    return null;
  }
  public static void enforce(Id itemId, String key) {
    String permission = required(itemId, key);
    Boolean bypass = ![SELECT Id FROM PermissionSetAssignment
      WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSet.PermissionsModifyAllData=TRUE
      WITH SYSTEM_MODE LIMIT 1].isEmpty();
    if (String.isNotBlank(permission) && !FeatureManagement.checkPermission(permission) && !bypass) {
      throw new AuraHandledException('blocked');
    }
  }
}`)
	writeFile(t, filepath.Join(base, "classes/PermissionGateTest.cls"), `
@IsTest private class PermissionGateTest {
  @IsTest static void limitsAdminBypassToAssignedPermissionSet() {
    Profile profile = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
    User person = new User(ProfileId=profile.Id, Alias='review', LastName='Reviewer',
      Email='reviewer@example.invalid', Username='reviewer.gate@example.invalid',
      EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US', LocaleSidKey='en_US',
      TimeZoneSidKey='America/Los_Angeles');
    System.runAs(new User(Id=UserInfo.getUserId())) {
      insert person;
      PermissionSet grant = new PermissionSet(Name='ReviewGrant', Label='Review grant');
      insert grant;
      Id permission = [SELECT Id FROM CustomPermission WHERE DeveloperName='MayReview' LIMIT 1].Id;
      insert new SetupEntityAccess(ParentId=grant.Id, SetupEntityId=permission);
      insert new PermissionSetAssignment(AssigneeId=person.Id, PermissionSetId=grant.Id);
    }
    GateItem__c item = new GateItem__c(Name='Item');
    String payload = JSON.serialize(new Map<String,Object>{'waiting'=>true, 'key'=>'review', 'permission'=>'AbsentRequirement'});
    System.runAs(new User(Id=UserInfo.getUserId())) {
      insert item;
      insert new GateMessage__c(Name='Message', Parent__c=item.Id, Status__c='Pending', Body__c=payload);
    }
    System.assert([SELECT COUNT() FROM PermissionSetAssignment
      WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSet.PermissionsModifyAllData=TRUE
      WITH SYSTEM_MODE] > 0, 'administrator positive control');
    System.runAs(person) {
      System.assertEquals(true, FeatureManagement.checkPermission('MayReview'));
      System.assertEquals(false, FeatureManagement.checkPermission('AbsentRequirement'));
      Id current = UserInfo.getUserId();
      System.assertEquals(0, [SELECT COUNT() FROM PermissionSetAssignment
        WHERE AssigneeId=:current AND PermissionSet.PermissionsModifyAllData=TRUE
        WITH SYSTEM_MODE], 'local bind must exclude administrator assignment');
      System.assertEquals(0, [SELECT COUNT() FROM PermissionSetAssignment
        WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSet.PermissionsModifyAllData=TRUE
        WITH SYSTEM_MODE], 'method bind must exclude administrator assignment');
      List<GateMessage__c> messages = [SELECT Body__c FROM GateMessage__c
        WHERE Parent__c=:item.Id AND Status__c='Pending'
        WITH SYSTEM_MODE ORDER BY CreatedDate DESC, Id DESC LIMIT 50];
      System.assertEquals(1, messages.size(), 'pending child query');
      Boolean omitted = false;
      try { Id unselected = messages[0].Parent__c; }
      catch (SObjectException expected) { omitted = true; }
      System.assertEquals(true, omitted, 'sharing must not load an unselected master field');
      System.assertEquals(payload, GatePayload.resolve(messages[0].Body__c));
      Map<String,Object> state = (Map<String,Object>)JSON.deserializeUntyped(messages[0].Body__c);
      System.assertEquals(true, state.get('waiting') == true, 'boxed Boolean guard');
      System.assertEquals(true, 'review'.equalsIgnoreCase((String)state.get('key')), 'key guard');
      System.assertEquals('AbsentRequirement', PermissionGate.required(item.Id, 'review'), 'required permission');
      Boolean blocked = false;
      try { PermissionGate.enforce(item.Id, 'review'); }
      catch (AuraHandledException expected) { blocked = true; }
      System.assertEquals(true, blocked, 'missing requirement blocks');
    }
  }
}
`)
	run := Run(loadTestIndex(t, root), Options{Parallelism: 1})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		for _, suite := range run.Suites {
			for _, result := range suite.Cases {
				if result.Problem != nil {
					t.Logf("%s", result.Problem.Message)
				}
			}
		}
		t.Fatalf("summary = %#v", got)
	}
}

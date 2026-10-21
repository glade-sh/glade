@IsTest public class PrivateSetupVisibilityTest {
    @TestSetup static void seed() { insert new PrivateProbe__c(Name='Seed'); }
    @IsTest static void visibility() {
        Profile profile=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
        User actor=new User(Username='private.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='priv',Email='private@example.invalid',LastName='SyntheticPrivateReader',ProfileId=profile.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
        insert actor;
        PermissionSet grant=[SELECT Id FROM PermissionSet WHERE Name='SyntheticPrivateRead' LIMIT 1];
        insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=grant.Id);
        Map<String,Object> row=new Map<String,Object>{'outside'=>[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Seed']};
        System.runAs(actor) {
            row.put('inside',[SELECT COUNT() FROM PrivateProbe__c WHERE Name='Seed']);
            row.put('withoutSharing',PrivateSetupReader.countRows());
            try { PrivateProbe__c record=[SELECT Id FROM PrivateProbe__c WHERE Name='Seed' LIMIT 1];row.put('singleton',true); }
            catch (Exception error) { row.put('exceptionType',error.getTypeName());row.put('message',error.getMessage()); }
        }
        System.assert(false,'OBSERVED|visibility|'+JSON.serialize(row));
    }
}

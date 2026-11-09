@isTest
private class SynthFlsCapture67 {
    private static User syntheticUser(Id profileId,String suffix,String uniqueKey) {
        return new User(ProfileId=profileId,Alias='sfls'+suffix,Email='synthetic-'+suffix+'@example.invalid',Username='synthetic-'+uniqueKey+'-'+suffix+'@example.invalid',LastName='Synthetic '+suffix,TimeZoneSidKey='GMT',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',SynthUserField__c='synthetic-'+suffix);
    }
    @isTest static void capture_matrix() {
        SynthFlsProbe67.beginCapture('test');
        // Existing native profiles are attested before execution; no profile is cloned or expanded.
        Profile minimal=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' AND UserLicense.Name='Salesforce' WITH SYSTEM_MODE];
        Profile standard=[SELECT Id FROM Profile WHERE Name='Standard User' AND UserLicense.Name='Salesforce' WITH SYSTEM_MODE];
        PermissionSet grants=[SELECT Id FROM PermissionSet WHERE Name='SynthCaptureRestricted' AND IsOwnedByProfile=false WITH SYSTEM_MODE];
        String uniqueKey=EncodingUtil.convertToHex(Crypto.generateAesKey(128));
        User restricted=syntheticUser(minimal.Id,'min',uniqueKey);User control=syntheticUser(standard.Id,'std',uniqueKey);
        User administrator=[SELECT Id FROM User WHERE Id=:UserInfo.getUserId() WITH SYSTEM_MODE];
        System.runAs(administrator) {
        insert as system new List<User>{restricted,control};
        insert as system new List<PermissionSetAssignment>{new PermissionSetAssignment(AssigneeId=restricted.Id,PermissionSetId=grants.Id),new PermissionSetAssignment(AssigneeId=control.Id,PermissionSetId=grants.Id)};
        }
        SynthFlsProbe67.auditActor('minimal',restricted.Id,minimal.Id);
        SynthFlsProbe67.auditActor('standard',control.Id,standard.Id);
        System.runAs(restricted) {SynthFlsProbe67.captureActor('minimal',control.Id,true);}
        System.runAs(control) {SynthFlsProbe67.captureActor('standard',restricted.Id,false);}
        SynthFlsProbe67.adminRecordAccess();
        String carrier=SynthFlsProbe67.finishCapture();
        // Intentional rollback + evidence carrier. Native Outcome MUST remain Fail.
        System.assert(false,carrier);
    }
}

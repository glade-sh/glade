@IsTest public class RecordOperationProbeTest {
    @TestSetup static void seed() {
        insert new PermissionSet(Name='SyntheticExportAccess',Label='Synthetic export access');
        Profile profile = [SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
        insert new User(Username='synthetic.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='synth',Email='synthetic@example.invalid',LastName='SyntheticReader',ProfileId=profile.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
    }
    @IsTest static void identifierAccount() { emit('identifierAccount',RecordOperationProbe.observe('identifierAccount')); }
    @IsTest static void identifierCustom() { emit('identifierCustom',RecordOperationProbe.observe('identifierCustom')); }
    @IsTest static void mergeAll() { emit('mergeAll',RecordOperationProbe.observe('mergeAll')); }
    @IsTest static void mergePartial() { emit('mergePartial',RecordOperationProbe.observe('mergePartial')); }
    @IsTest static void mergeEmpty() { emit('mergeEmpty',RecordOperationProbe.observe('mergeEmpty')); }
    @IsTest static void setupVisibility() {
        Map<String,Object> row = new Map<String,Object>{'outside'=>[SELECT COUNT() FROM PermissionSet WHERE Name='SyntheticExportAccess']};
        User actor=[SELECT Id FROM User WHERE LastName='SyntheticReader' LIMIT 1];
        System.runAs(actor) {
            row.put('inside',[SELECT COUNT() FROM PermissionSet WHERE Name='SyntheticExportAccess']);
            try { row.put('singleton',lookup().Name); }
            catch (Exception error) { row.put('exceptionType',error.getTypeName()); }
        }
        emit('setupVisibility',row);
    }
    private static PermissionSet lookup() { return [SELECT Id,Name FROM PermissionSet WHERE Name='SyntheticExportAccess' LIMIT 1]; }
    private static void emit(String method,Map<String,Object> row) { System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(row)); }
}

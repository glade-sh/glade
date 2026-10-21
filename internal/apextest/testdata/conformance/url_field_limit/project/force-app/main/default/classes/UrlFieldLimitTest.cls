@IsTest
private class UrlFieldLimitTest {
    @IsTest static void hierarchy255() { observe('hierarchy255', true, 255); }
    @IsTest static void hierarchy256() { observe('hierarchy256', true, 256); }
    @IsTest static void record255() { observe('record255', false, 255); }
    @IsTest static void record256() { observe('record256', false, 256); }
    private static void observe(String label, Boolean hierarchy, Integer size) {
        String prefix = 'https://example.invalid/';
        String value = prefix + 'x'.repeat(size - prefix.length());
        Map<String, Object> result = new Map<String, Object>{'inputLength' => value.length()};
        try {
            if (hierarchy) {
                result.put('describeLength', LinkSettings__c.Endpoint__c.getDescribe().getLength());
                LinkSettings__c setting = new LinkSettings__c(SetupOwnerId = UserInfo.getOrganizationId(), Endpoint__c = value);
                Database.upsert(setting, AccessLevel.SYSTEM_MODE);
                LinkSettings__c saved = [SELECT Endpoint__c FROM LinkSettings__c WHERE Id = :setting.Id];
                result.put('storedLength', saved.Endpoint__c.length());
            } else {
                result.put('describeLength', LinkRecord__c.Endpoint__c.getDescribe().getLength());
                LinkRecord__c record = new LinkRecord__c(Name = 'Example', Endpoint__c = value);
                Database.upsert(record, AccessLevel.SYSTEM_MODE);
                LinkRecord__c saved = [SELECT Endpoint__c FROM LinkRecord__c WHERE Id = :record.Id];
                result.put('storedLength', saved.Endpoint__c.length());
            }
            result.put('status', 'success');
        } catch (DmlException e) {
            result.put('status', String.valueOf(e.getDmlStatusCode(0)));
        }
        System.assert(false, 'OBSERVED|' + label + '|' + JSON.serialize(result));
    }
}

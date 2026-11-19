@IsTest
private class PrefixCapture67 {
    @IsTest
    static void organizationSingleRow() {
        List<Organization> rows = [SELECT Id FROM Organization];
        System.assertEquals(1, rows.size());
        System.assertEquals(UserInfo.getOrganizationId(), rows[0].Id);
    }

    @IsTest
    static void customPrefixMatchesDescribe() {
        PrefixProbe__c probe = new PrefixProbe__c(Name = 'probe');
        insert probe;
        Schema.SObjectType probeType = PrefixProbe__c.SObjectType;
        System.assertEquals(probeType.getDescribe().getKeyPrefix(), String.valueOf(probe.Id).left(3));
        System.assertEquals(probeType, probe.Id.getSObjectType());
        System.assertEquals(probeType, Id.valueOf(String.valueOf(probe.Id)).getSObjectType());
    }

    @IsTest
    static void standardPrefixMatchesDescribe() {
        Account acct = new Account(Name = 'probe');
        insert acct;
        System.assertEquals('001', String.valueOf(acct.Id).left(3));
        System.assertEquals(Account.SObjectType.getDescribe().getKeyPrefix(), String.valueOf(acct.Id).left(3));
        System.assertEquals(Account.SObjectType, acct.Id.getSObjectType());
    }
}

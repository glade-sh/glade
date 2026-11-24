@IsTest
private class ShareEventProbeTest {
    private static User mk(String profileName, String alias) {
        Profile p = [SELECT Id FROM Profile WHERE Name = :profileName];
        User u = new User(Username = alias + Math.random() + '@example.test.org', Alias = alias, Email = 'a@example.test',
            EmailEncodingKey = 'UTF-8', LastName = 'T', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US',
            ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles');
        insert u;
        return u;
    }

    private static String run(Boolean shareToOwner) { return run(shareToOwner, false); }

    private static String run(Boolean shareToOwner, Boolean accessLevel) {
        User u = mk('System Administrator', 'adm');
        User other = mk('System Administrator', 'oth');
        String out;
        System.runAs(u) {
            Doc__c d = new Doc__c(Name = 'd');
            insert d;
            Id target = shareToOwner ? UserInfo.getUserId() : other.Id;
            insert new Doc__Share(ParentId = d.Id, UserOrGroupId = target, AccessLevel = 'Read', RowCause = Schema.Doc__Share.RowCause.Rsn__c);
            Integer direct = [SELECT COUNT() FROM Doc__Share WHERE RowCause IN ('Rsn__c') AND ParentId IN (:d.Id) WITH USER_MODE];
            Test.startTest();
            if (accessLevel) EventBus.publishWithAccessLevel(new Evt__e(DocId__c = String.valueOf(d.Id)), AccessLevel.SYSTEM_MODE);
            else EventBus.publish(new Evt__e(DocId__c = String.valueOf(d.Id)));
            Test.stopTest();
            List<Res__c> res = [SELECT Name FROM Res__c];
            out = 'direct' + direct + ';trig=' + (res.isEmpty() ? 'none' : res[0].Name);
        }
        return out;
    }

    @IsTest static void accessOwner() { System.assert(false, 'OUT:' + run(true, true)); }
    @IsTest static void accessOther() { System.assert(false, 'OUT:' + run(false, true)); }
    @IsTest static void ownerShare() { System.assert(false, 'OUT:' + run(true)); }
    @IsTest static void otherShare() { System.assert(false, 'OUT:' + run(false)); }
}

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

    private static String run(Boolean shareToOwner, Boolean accessLevel) { return run(shareToOwner, accessLevel, false); }

    private static String run(Boolean shareToOwner, Boolean accessLevel, Boolean extra) {
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
            else if (extra) EventBus.publish(new Evt__e(DocId__c = String.valueOf(d.Id), Extra__c = 'x'));
            else EventBus.publish(new Evt__e(DocId__c = String.valueOf(d.Id)));
            Test.stopTest();
            List<Res__c> res = [SELECT Name FROM Res__c];
            out = 'direct' + direct + ';trig=' + (res.isEmpty() ? 'none' : res[0].Name);
        }
        return out;
    }

    @IsTest static void extraOwner() { System.assert(false, 'OUT:' + run(true, false, true)); }
    @IsTest static void extraOther() { System.assert(false, 'OUT:' + run(false, false, true)); }
    @IsTest static void accessOwner() { System.assert(false, 'OUT:' + run(true, true)); }
    @IsTest static void accessOther() { System.assert(false, 'OUT:' + run(false, true)); }
    @IsTest static void ownerShare() { System.assert(false, 'OUT:' + run(true)); }
    @IsTest static void otherShare() { System.assert(false, 'OUT:' + run(false)); }
    private static User mk(String alias) {
        return mk('System Administrator', alias);
    }

    private static String run(String route, Boolean two) {
        User u = mk('adm');
        User other = mk('oth');
        String out;
        System.runAs(u) {
            Doc__c d = new Doc__c(Name = 'd');
            insert d;
            if (two) {
                Doc__c d2 = new Doc__c(Name = 'd2');
                insert d2;
                insert new Doc__Share(ParentId = d2.Id, UserOrGroupId = other.Id, AccessLevel = 'Read', RowCause = Schema.Doc__Share.RowCause.Rsn__c);
            }
            insert new Doc__Share(ParentId = d.Id, UserOrGroupId = other.Id, AccessLevel = 'Read', RowCause = Schema.Doc__Share.RowCause.Rsn__c);
            Test.startTest();
            Evt__e e = new Evt__e(DocId__c = String.valueOf(d.Id));
            if (route == 'many') { e.Extra__c = 'x'; e.ExtraTwo__c = 'y'; }
            if (route == 'flag') e.Flag__c = true;
            if (route == 'extra') e.Extra__c = 'x';
            if (route == 'user') EventBus.publishWithAccessLevel(e, AccessLevel.USER_MODE);
            else if (route == 'nullsys') EventBus.publishWithAccessLevel(e, null, AccessLevel.SYSTEM_MODE);
            else if (route == 'nulluser') EventBus.publishWithAccessLevel(e, null, AccessLevel.USER_MODE);
            else if (route == 'system') EventBus.publishWithAccessLevel(e, AccessLevel.SYSTEM_MODE);
            else EventBus.publish(e);
            Test.stopTest();
            List<Res__c> res = [SELECT Name FROM Res__c];
            out = res.isEmpty() ? 'none' : res[0].Name;
        }
        return out;
    }

    @IsTest static void userMode() { System.assert(false, 'OUT:' + run('user', false)); }
    @IsTest static void nullSystem() { System.assert(false, 'OUT:' + run('nullsys', false)); }
    @IsTest static void nullUser() { System.assert(false, 'OUT:' + run('nulluser', false)); }
    @IsTest static void manyFields() { System.assert(false, 'OUT:' + run('many', false)); }
    @IsTest static void otherType() { System.assert(false, 'OUT:' + run('flag', false)); }
    @IsTest static void twoExtra() { System.assert(false, 'OUT:' + run('extra', true)); }
    @IsTest static void twoSystem() { System.assert(false, 'OUT:' + run('system', true)); }
}

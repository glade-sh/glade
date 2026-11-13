@isTest private class PclGuestTest {
    static String cnt(String q) {
        try { return String.valueOf(Database.countQuery(q)); } catch (Exception e) { return 'E'; }
    }
    @isTest static void run() {
        UserRole r = new UserRole(Name = 'Pcl Owner Role'); insert r;
        User owner = new User(Alias = 'pclown', Email = 'o@example.com', EmailEncodingKey = 'UTF-8', LastName = 'pclown', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', TimeZoneSidKey = 'America/Los_Angeles',
            ProfileId = [SELECT Id FROM Profile WHERE Name = 'System Administrator' LIMIT 1].Id, UserRoleId = r.Id, UserName = 'pclown' + Math.abs(Crypto.getRandomInteger()) + '@example.com');
        insert owner;
        Case c; Lead l;
        System.runAs(owner) { c = new Case(Subject = 'PclOther'); insert c; l = new Lead(LastName = 'PclLead', Company = 'PclCo'); insert l; }
        List<Profile> ps = [SELECT Id, Name FROM Profile WHERE UserType = 'Guest' ORDER BY Name];
        System.assert(!ps.isEmpty(), 'no guest profile');
        User g = new User(Alias = 'pclgst', Email = 'g@example.com', EmailEncodingKey = 'UTF-8', LastName = 'pclgst', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', TimeZoneSidKey = 'America/Los_Angeles',
            ProfileId = ps[0].Id, UserName = 'pclgst' + Math.abs(Crypto.getRandomInteger()) + '@example.com');
        insert g;
        String m = 'guest|r=';
        for (Id i : new List<Id>{c.Id, l.Id}) {
            List<UserRecordAccess> a = [SELECT RecordId, HasReadAccess, HasEditAccess FROM UserRecordAccess WHERE UserId = :g.Id AND RecordId = :i];
            m += a.isEmpty() ? 'N' : (a[0].HasReadAccess ? '1' : '0');
        }
        m += '|e=';
        for (Id i : new List<Id>{c.Id, l.Id}) {
            List<UserRecordAccess> a = [SELECT RecordId, HasReadAccess, HasEditAccess FROM UserRecordAccess WHERE UserId = :g.Id AND RecordId = :i];
            m += a.isEmpty() ? 'N' : (a[0].HasEditAccess ? '1' : '0');
        }
        System.runAs(g) {
            m += '|q=' + cnt('SELECT count() FROM Case WHERE Id = \'' + c.Id + '\'') + ',' + cnt('SELECT count() FROM Lead WHERE Id = \'' + l.Id + '\'');
            m += '|a=' + (Schema.SObjectType.Case.isAccessible() ? '1' : '0') + (Schema.SObjectType.Lead.isAccessible() ? '1' : '0');
            m += '|c=' + (Schema.SObjectType.Case.isCreateable() ? '1' : '0') + (Schema.SObjectType.Lead.isCreateable() ? '1' : '0');
        }
        System.assert(false, m);
    }
}

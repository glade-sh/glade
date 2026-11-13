public class PclMsgSupport {
    public static String q(String soql) {
        try { Database.countQuery(soql); return 'ok'; } catch (Exception e) { return e.getTypeName() + ':' + e.getMessage(); }
    }
    static User mk(String alias, Id pid, Id rid, Id cid) {
        return new User(Alias = alias, Email = alias + '@example.com', EmailEncodingKey = 'UTF-8', LastName = alias, LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', TimeZoneSidKey = 'America/Los_Angeles',
            ProfileId = pid, UserRoleId = rid, ContactId = cid, UserName = alias + Math.abs(Crypto.getRandomInteger()) + '@example.com');
    }
    public static User guest() {
        User g = mk('pclgst', [SELECT Id FROM Profile WHERE UserType = 'Guest' ORDER BY Name LIMIT 1].Id, null, null);
        insert g; return g;
    }
    public static User community() {
        UserRole r = new UserRole(Name = 'Pcl Owner Role'); insert r;
        User owner = mk('pclown', [SELECT Id FROM Profile WHERE Name = 'System Administrator' LIMIT 1].Id, r.Id, null);
        insert owner;
        Contact con;
        System.runAs(owner) { Account a = new Account(Name = 'PclAcct'); insert a; con = new Contact(LastName = 'PclCon', AccountId = a.Id); insert con; }
        User cc = mk('pclprt', [SELECT Id FROM Profile WHERE Name = 'Customer Community User' LIMIT 1].Id, null, con.Id);
        insert cc; return cc;
    }
}

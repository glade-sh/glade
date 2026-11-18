public class PclSupport {
    public static User owner; static Account acct; static Contact con; static Case caseOther; static Case caseOwn; static Lead leadOther;
    static Integer seq = 0;
    public static Id profileId(String n) { return [SELECT Id FROM Profile WHERE Name = :n LIMIT 1].Id; }
    public static User newUser(String alias, Id pid, Id rid, Id cid) {
        seq++;
        return new User(Alias = alias, Email = alias + '@example.com', EmailEncodingKey = 'UTF-8', LastName = alias,
            LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US', TimeZoneSidKey = 'America/Los_Angeles',
            ProfileId = pid, UserRoleId = rid, ContactId = cid, UserName = alias + seq + '.' + Math.abs(Crypto.getRandomInteger()) + '@example.com');
    }
    public static void setup() {
        UserRole r = new UserRole(Name = 'Pcl Owner Role');
        insert r;
        owner = newUser('pclown', profileId('System Administrator'), r.Id, null);
        insert owner;
        System.runAs(owner) {
            acct = new Account(Name = 'PclAcct'); insert acct;
            con = new Contact(LastName = 'PclCon', AccountId = acct.Id); insert con;
            caseOther = new Case(Subject = 'PclOther'); insert caseOther;
            caseOwn = new Case(Subject = 'PclOwn', ContactId = con.Id, AccountId = acct.Id); insert caseOwn;
            leadOther = new Lead(LastName = 'PclLead', Company = 'PclCo'); insert leadOther;
        }
    }
    static String bits(User u, Boolean edit) {
        Map<Id, UserRecordAccess> m = new Map<Id, UserRecordAccess>();
        String s = '';
        for (Id i : new List<Id>{caseOther.Id, caseOwn.Id, leadOther.Id}) {
            UserRecordAccess a = [SELECT RecordId, HasReadAccess, HasEditAccess FROM UserRecordAccess WHERE UserId = :u.Id AND RecordId = :i];
            s += (edit ? a.HasEditAccess : a.HasReadAccess) ? '1' : '0';
        }
        return s;
    }
    static String cnt(String q) {
        try { return String.valueOf(Database.countQuery(q)); } catch (Exception e) { return 'E'; }
    }
    public static void observe(String tag, User u) {
        String m = tag + '|r=' + bits(u, false) + '|e=' + bits(u, true);
        System.runAs(u) {
            m += '|q=' + cnt('SELECT count() FROM Case WHERE Id = \'' + caseOther.Id + '\'')
                + ',' + cnt('SELECT count() FROM Case WHERE Id = \'' + caseOwn.Id + '\'')
                + ',' + cnt('SELECT count() FROM Lead WHERE Id = \'' + leadOther.Id + '\'');
            m += '|a=' + (Schema.SObjectType.Case.isAccessible() ? '1' : '0') + (Schema.SObjectType.Lead.isAccessible() ? '1' : '0');
        }
        System.assert(false, m);
    }
    public static User portal(String profileName) {
        User u = newUser('pclprt', profileId(profileName), null, con.Id);
        insert u;
        return u;
    }
}

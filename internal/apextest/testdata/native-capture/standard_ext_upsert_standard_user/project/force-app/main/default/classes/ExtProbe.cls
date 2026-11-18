public class ExtProbe {
    static Integer n = 0;
    static String salt = String.valueOf(Math.abs(Crypto.getRandomInteger()));
    static String ex(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    static String errs(List<Database.Error> es) { return es[0].getStatusCode() + '|' + es[0].getMessage() + '|' + es[0].getFields(); }
    static String idk(Id i, Id baseId) { return i == baseId ? 'base' : (i == null ? 'null' : 'new'); }
    static String state(String ext) {
        List<Account> l = [SELECT Id, Name, Text__c FROM Account WHERE Ext__c = :ext WITH SYSTEM_MODE];
        return 'n=' + l.size() + (l.isEmpty() ? '' : ' name=' + l[0].Name + ' text=' + l[0].Text__c);
    }
    static Account mk(String kind, String ext, Id baseId) {
        if (kind == 'upd') { return new Account(Name = 'changed', Ext__c = ext, Text__c = 'y'); }
        if (kind == 'ins') { return new Account(Name = 'fresh', Ext__c = ext + 'n', Text__c = 'y'); }
        if (kind == 'ctl') { return new Account(Name = 'changed', Ext__c = ext); }
        return new Account(Id = baseId, Name = 'changed', Ext__c = ext, Text__c = 'y');
    }
    public static List<String> run() {
        List<String> out = new List<String>();
        for (String kind : new List<String>{'upd', 'ins', 'ctl'}) {
            for (String op : new List<String>{'stmt', 'dbDefault', 'dbFalse', 'dbUser', 'dbFalseUser', 'stmtUser'}) {
                n++;
                String ext = 'e' + salt + '_' + n;
                Account base = new Account(Name = 'base', Ext__c = ext);
                Database.insert(base, AccessLevel.SYSTEM_MODE);
                String key = op + '_' + kind;
                try {
                    Account r = mk(kind, ext, base.Id);
                    if (op == 'stmt') { upsert r Account.Ext__c; out.add(key + '=ok id=' + idk(r.Id, base.Id)); }
                    else if (op == 'stmtUser') { upsert as user r Account.Ext__c; out.add(key + '=ok id=' + idk(r.Id, base.Id)); }
                    else {
                        Database.UpsertResult[] u;
                        if (op == 'dbDefault') { u = Database.upsert(new List<Account>{r}, Account.Ext__c); }
                        else if (op == 'dbFalse') { u = Database.upsert(new List<Account>{r}, Account.Ext__c, false); }
                        else if (op == 'dbUser') { u = Database.upsert(new List<Account>{r}, Account.Ext__c, true, AccessLevel.USER_MODE); }
                        else { u = Database.upsert(new List<Account>{r}, Account.Ext__c, false, AccessLevel.USER_MODE); }
                        out.add(key + '=' + (u[0].isSuccess() ? 'ok created=' + u[0].isCreated() + ' id=' + idk(u[0].getId(), base.Id) : 'rowErr ' + errs(u[0].getErrors())));
                    }
                } catch (Exception e) { out.add(key + '=EX ' + ex(e)); }
                out.add(key + '.state=' + state(ext) + ' | newExt ' + state(ext + 'n'));
            }
        }
        return out;
    }
}

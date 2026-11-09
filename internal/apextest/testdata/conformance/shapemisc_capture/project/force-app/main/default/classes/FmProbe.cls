public class FmProbe {
    static Id baseId;
    static String ex(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    static String res(Database.SaveResult[] r) { return r[0].isSuccess() ? 'ok' : 'rowErr ' + r[0].getErrors()[0].getStatusCode() + '|' + r[0].getErrors()[0].getMessage(); }
    static String ures(Database.UpsertResult[] r) { return r[0].isSuccess() ? 'ok' : 'rowErr ' + r[0].getErrors()[0].getStatusCode() + '|' + r[0].getErrors()[0].getMessage(); }
    static Probe_Item__c mk(String kind) {
        if (kind == 'q') { return [SELECT Id, Name, Form__c, Auto__c FROM Probe_Item__c WHERE Id = :baseId WITH SYSTEM_MODE]; }
        if (kind == 'qc') { return [SELECT Id, Name, Form__c, Auto__c FROM Probe_Item__c WHERE Id = :baseId WITH SYSTEM_MODE][0].clone(false, true, false, false); }
        if (kind == 'j') { return (Probe_Item__c) JSON.deserialize('{"Name":"j","Form__c":"v","Auto__c":"S-9"}', Probe_Item__c.class); }
        if (kind == 'jf') { return (Probe_Item__c) JSON.deserialize('{"Name":"j","Form__c":"v"}', Probe_Item__c.class); }
        if (kind == 'ja') { return (Probe_Item__c) JSON.deserialize('{"Name":"j","Auto__c":"S-9"}', Probe_Item__c.class); }
        if (kind == 'jid') { return (Probe_Item__c) JSON.deserialize('{"Id":"' + baseId + '","Name":"j","Form__c":"v"}', Probe_Item__c.class); }
        if (kind == 'text') { return new Probe_Item__c(Name = 'x', Text__c = 'y'); }
        return new Probe_Item__c(Name = 'plain');
    }
    public static List<String> run() {
        List<String> out = new List<String>();
        Probe_Item__c base = new Probe_Item__c(Name = 'base');
        Database.insert(base, AccessLevel.SYSTEM_MODE);
        baseId = base.Id;
        for (String kind : new List<String>{'plain', 'text', 'qc', 'j', 'jf', 'ja'}) {
            try { insert mk(kind); out.add('insert_' + kind + '=ok'); } catch (Exception e) { out.add('insert_' + kind + '=EX ' + ex(e)); }
            try { out.add('dbInsertDefault_' + kind + '=' + res(Database.insert(new List<Probe_Item__c>{mk(kind)}))); } catch (Exception e) { out.add('dbInsertDefault_' + kind + '=EX ' + ex(e)); }
            try { out.add('dbInsertFalse_' + kind + '=' + res(Database.insert(new List<Probe_Item__c>{mk(kind)}, false))); } catch (Exception e) { out.add('dbInsertFalse_' + kind + '=EX ' + ex(e)); }
            try { out.add('dbInsertUser_' + kind + '=' + res(Database.insert(new List<Probe_Item__c>{mk(kind)}, AccessLevel.USER_MODE))); } catch (Exception e) { out.add('dbInsertUser_' + kind + '=EX ' + ex(e)); }
            try { out.add('dbInsertFalseUser_' + kind + '=' + res(Database.insert(new List<Probe_Item__c>{mk(kind)}, false, AccessLevel.USER_MODE))); } catch (Exception e) { out.add('dbInsertFalseUser_' + kind + '=EX ' + ex(e)); }
            try { upsert mk(kind); out.add('upsertInsertPath_' + kind + '=ok'); } catch (Exception e) { out.add('upsertInsertPath_' + kind + '=EX ' + ex(e)); }
            try { out.add('dbUpsertUser_' + kind + '=' + ures(Database.upsert(new List<Probe_Item__c>{mk(kind)}, AccessLevel.USER_MODE))); } catch (Exception e) { out.add('dbUpsertUser_' + kind + '=EX ' + ex(e)); }
            try { out.add('dbUpsertFalseUser_' + kind + '=' + ures(Database.upsert(new List<Probe_Item__c>{mk(kind)}, false, AccessLevel.USER_MODE))); } catch (Exception e) { out.add('dbUpsertFalseUser_' + kind + '=EX ' + ex(e)); }
        }
        for (String kind : new List<String>{'q', 'jid'}) {
            try { update mk(kind); out.add('update_' + kind + '=ok'); } catch (Exception e) { out.add('update_' + kind + '=EX ' + ex(e)); }
            try { upsert mk(kind); out.add('upsert_' + kind + '=ok'); } catch (Exception e) { out.add('upsert_' + kind + '=EX ' + ex(e)); }
            try { out.add('dbUpdateDefault_' + kind + '=' + res(Database.update(new List<Probe_Item__c>{mk(kind)}))); } catch (Exception e) { out.add('dbUpdateDefault_' + kind + '=EX ' + ex(e)); }
            try { out.add('dbUpdateUser_' + kind + '=' + res(Database.update(new List<Probe_Item__c>{mk(kind)}, AccessLevel.USER_MODE))); } catch (Exception e) { out.add('dbUpdateUser_' + kind + '=EX ' + ex(e)); }
            try { out.add('dbUpdateFalseUser_' + kind + '=' + res(Database.update(new List<Probe_Item__c>{mk(kind)}, false, AccessLevel.USER_MODE))); } catch (Exception e) { out.add('dbUpdateFalseUser_' + kind + '=EX ' + ex(e)); }
            try { out.add('dbUpsertUser_' + kind + '=' + ures(Database.upsert(new List<Probe_Item__c>{mk(kind)}, AccessLevel.USER_MODE))); } catch (Exception e) { out.add('dbUpsertUser_' + kind + '=EX ' + ex(e)); }
            try { out.add('dbUpsertFalseUser_' + kind + '=' + ures(Database.upsert(new List<Probe_Item__c>{mk(kind)}, false, AccessLevel.USER_MODE))); } catch (Exception e) { out.add('dbUpsertFalseUser_' + kind + '=EX ' + ex(e)); }
        }
        return out;
    }
}
